package paymentsRepository

//go:generate go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"os"
	"path/filepath"
	"time"

	paymentsQueries "github.com/thesahibnanda-max/eCommerce/backend/package/repository/payments/queries"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

type Repository interface {
	CreatePayment(ctx context.Context, input CreateInput) (Model, bool, error)
	GetPayment(ctx context.Context, paymentID string) (Model, error)
	GetPaymentByReference(ctx context.Context, provider Provider, reference string) (Model, error)
	ListOrderPayments(ctx context.Context, orderID string) ([]Model, error)
	ListPayments(ctx context.Context, query ListQuery) (Page, error)
	ListPaymentsForOrders(ctx context.Context, orderIDs []string, withEvents bool) (map[string][]Model, error)
	CountPaymentsByState(ctx context.Context) (map[State]int64, error)
	Transition(ctx context.Context, paymentID string, to State, patch Patch, event EventInput) (Model, bool, error)
	Update(ctx context.Context, paymentID string, patch Patch, event EventInput) (Model, error)
	SubmitProof(ctx context.Context, paymentID string, proof ProofInput) (Model, error)
	GetProofImage(ctx context.Context, paymentID string) ([]byte, string, error)
	Close() error
}

type repository struct {
	writer   *sql.DB
	reader   *sql.DB
	now      func() (time.Time, error)
	location *time.Location
}

func NewRepository(path []string, permission os.FileMode) (Repository, error) {
	file := filepath.Join(path...)
	now, err := utils.TimeNow()
	if err != nil {
		return nil, err
	}
	handle, err := os.OpenFile(file, os.O_CREATE|os.O_RDWR, permission)
	if err != nil {
		return nil, err
	}
	if err := handle.Close(); err != nil {
		return nil, err
	}
	writer, err := sql.Open("sqlite", dsn(file, true))
	if err != nil {
		return nil, err
	}
	writer.SetMaxOpenConns(1)
	if _, err := writer.Exec(schemaSQL); err != nil {
		_ = writer.Close()
		return nil, err
	}
	reader, err := sql.Open("sqlite", dsn(file, false))
	if err != nil {
		_ = writer.Close()
		return nil, err
	}
	reader.SetMaxOpenConns(readerConnections)
	if err := reader.Ping(); err != nil {
		_ = writer.Close()
		_ = reader.Close()
		return nil, err
	}
	return &repository{writer: writer, reader: reader, now: utils.TimeNow, location: now.Location()}, nil
}

func (r *repository) read(ctx context.Context, fn func(q *paymentsQueries.Queries) error) error {
	return withTx(ctx, r.reader, &sql.TxOptions{ReadOnly: true}, fn)
}

func (r *repository) write(ctx context.Context, fn func(q *paymentsQueries.Queries) error) error {
	return withTx(ctx, r.writer, nil, fn)
}

func (r *repository) stamp() (int64, error) {
	now, err := r.now()
	if err != nil {
		return 0, err
	}
	return now.UnixMicro(), nil
}

func (r *repository) CreatePayment(ctx context.Context, input CreateInput) (Model, bool, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Model{}, false, err
	}
	input, metadata, err := normalizeCreate(input)
	if err != nil {
		return Model{}, false, err
	}
	stamp, err := r.stamp()
	if err != nil {
		return Model{}, false, err
	}
	var (
		model   Model
		created bool
	)
	err = r.write(ctx, func(q *paymentsQueries.Queries) error {
		open, err := q.GetOpenPaymentForOrder(ctx, input.OrderID)
		switch {
		case err == nil:
			if Provider(open.Provider) != input.Provider || open.Amount != input.Amount || open.UserID != input.UserID {
				return ErrOpenPaymentConflict
			}
			model, err = r.loadModel(ctx, q, open.ID)
			return err
		case !errors.Is(err, sql.ErrNoRows):
			return err
		}
		id := utils.NewULID()
		if err := q.InsertPayment(ctx, paymentsQueries.InsertPaymentParams{
			ID:        id,
			OrderID:   input.OrderID,
			UserID:    input.UserID,
			Provider:  input.Provider.String(),
			State:     StateCreated.String(),
			Amount:    input.Amount,
			Currency:  CurrencyINR.String(),
			Purpose:   input.Purpose,
			Metadata:  metadata,
			CreatedAt: stamp,
			UpdatedAt: stamp,
		}); err != nil {
			return err
		}
		event := EventInput{Source: EventSourceSystem, Kind: eventKindCreated}
		if err := r.insertEvent(ctx, q, id, event, "{}", "", StateCreated, stamp); err != nil {
			return err
		}
		created = true
		model, err = r.loadModel(ctx, q, id)
		return err
	})
	if err != nil {
		return Model{}, false, err
	}
	return model, created, nil
}

func (r *repository) GetPayment(ctx context.Context, paymentID string) (Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Model{}, err
	}
	if err := validatePaymentID(paymentID); err != nil {
		return Model{}, err
	}
	var model Model
	err := r.read(ctx, func(q *paymentsQueries.Queries) error {
		var err error
		model, err = r.loadModel(ctx, q, paymentID)
		return err
	})
	return model, err
}

func (r *repository) GetPaymentByReference(ctx context.Context, provider Provider, reference string) (Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Model{}, err
	}
	if !provider.IsValid() {
		return Model{}, ErrProviderInvalid
	}
	reference, ok := cleanText(reference, maxReferenceRunes)
	if !ok || reference == "" {
		return Model{}, ErrReferenceInvalid
	}
	var model Model
	err := r.read(ctx, func(q *paymentsQueries.Queries) error {
		row, err := q.GetPaymentByReference(ctx, paymentsQueries.GetPaymentByReferenceParams{Provider: provider.String(), GatewayReference: nullString(reference)})
		if errors.Is(err, sql.ErrNoRows) {
			return ErrPaymentNotFound
		}
		if err != nil {
			return err
		}
		model, err = r.loadModel(ctx, q, row.ID)
		return err
	})
	return model, err
}

func (r *repository) ListOrderPayments(ctx context.Context, orderID string) ([]Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return nil, err
	}
	orderID, ok := cleanText(orderID, maxIDRunes)
	if !ok || orderID == "" {
		return nil, ErrOrderIDInvalid
	}
	models := []Model{}
	err := r.read(ctx, func(q *paymentsQueries.Queries) error {
		rows, err := q.ListOrderPayments(ctx, orderID)
		if err != nil {
			return err
		}
		for _, row := range rows {
			model, err := r.toModel(paymentsQueries.GetPaymentRow(row), nil)
			if err != nil {
				return err
			}
			models = append(models, model)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return models, nil
}

func (r *repository) ListPayments(ctx context.Context, query ListQuery) (Page, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Page{}, err
	}
	query, states, cursorAt, cursorID, err := normalizeListQuery(query)
	if err != nil {
		return Page{}, err
	}
	page := Page{Payments: []Model{}}
	err = r.read(ctx, func(q *paymentsQueries.Queries) error {
		rows, err := q.ListPayments(ctx, paymentsQueries.ListPaymentsParams{
			CursorAt: cursorAt,
			CursorID: cursorID,
			States:   states,
			Provider: query.Provider.String(),
			Limit:    int64(query.Limit + 1),
		})
		if err != nil {
			return err
		}
		for i, row := range rows {
			if i == query.Limit {
				last := rows[i-1]
				page.NextCursor, err = encodeCursor(last.CreatedAt, last.ID)
				return err
			}
			model, err := r.toModel(paymentsQueries.GetPaymentRow(row), nil)
			if err != nil {
				return err
			}
			page.Payments = append(page.Payments, model)
		}
		return nil
	})
	if err != nil {
		return Page{}, err
	}
	return page, nil
}

func (r *repository) ListPaymentsForOrders(ctx context.Context, orderIDs []string, withEvents bool) (map[string][]Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return nil, err
	}
	if len(orderIDs) > maxBatchIDs {
		return nil, ErrBatchTooLarge
	}
	ids := make([]string, 0, len(orderIDs))
	seen := make(map[string]struct{}, len(orderIDs))
	for _, id := range orderIDs {
		id, ok := cleanText(id, maxIDRunes)
		if !ok || id == "" {
			return nil, ErrOrderIDInvalid
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	byOrder := make(map[string][]Model, len(ids))
	if len(ids) == 0 {
		return byOrder, nil
	}
	err := r.read(ctx, func(q *paymentsQueries.Queries) error {
		rows, err := q.ListPaymentsByOrderIDs(ctx, ids)
		if err != nil {
			return err
		}
		events := map[string][]paymentsQueries.PaymentEvent{}
		if withEvents && len(rows) > 0 {
			paymentIDs := make([]string, 0, len(rows))
			for _, row := range rows {
				paymentIDs = append(paymentIDs, row.ID)
			}
			list, err := q.ListEventsByPaymentIDs(ctx, paymentIDs)
			if err != nil {
				return err
			}
			for _, event := range list {
				events[event.PaymentID] = append(events[event.PaymentID], event)
			}
		}
		for _, row := range rows {
			model, err := r.toModel(paymentsQueries.GetPaymentRow(row), events[row.ID])
			if err != nil {
				return err
			}
			byOrder[row.OrderID] = append(byOrder[row.OrderID], model)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return byOrder, nil
}

func (r *repository) CountPaymentsByState(ctx context.Context) (map[State]int64, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return nil, err
	}
	counts := map[State]int64{}
	for _, state := range []State{StateCreated, StateAwaitingPayment, StateVerifying, StateSucceeded, StateFailed, StateExpired} {
		counts[state] = 0
	}
	err := r.read(ctx, func(q *paymentsQueries.Queries) error {
		rows, err := q.CountPaymentsByState(ctx)
		if err != nil {
			return err
		}
		for _, row := range rows {
			counts[State(row.State)] = row.Count
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return counts, nil
}

func (r *repository) Transition(ctx context.Context, paymentID string, to State, patch Patch, event EventInput) (Model, bool, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Model{}, false, err
	}
	if err := validatePaymentID(paymentID); err != nil {
		return Model{}, false, err
	}
	if !to.IsValid() {
		return Model{}, false, ErrStateInvalid
	}
	event, data, err := normalizeEvent(event)
	if err != nil {
		return Model{}, false, err
	}
	stamp, err := r.stamp()
	if err != nil {
		return Model{}, false, err
	}
	var (
		model        Model
		transitioned bool
	)
	err = r.write(ctx, func(q *paymentsQueries.Queries) error {
		row, err := r.loadRow(ctx, q, paymentID)
		if err != nil {
			return err
		}
		from := State(row.State)
		if from == to {
			model, err = r.loadModel(ctx, q, paymentID)
			return err
		}
		allowed, err := canTransition(from, to)
		if err != nil {
			return err
		}
		if !allowed {
			return ErrTransitionNotAllowed
		}
		if err := ensureNotDuplicate(ctx, q, paymentID, event.DedupeKey); err != nil {
			return err
		}
		affected, err := q.UpdatePaymentState(ctx, paymentsQueries.UpdatePaymentStateParams{ToState: to.String(), UpdatedAt: stamp, ID: paymentID, FromState: from.String()})
		if err != nil {
			return err
		}
		if affected != 1 {
			return ErrConcurrentUpdate
		}
		if err := r.applyPatch(ctx, q, row, patch, stamp); err != nil {
			return err
		}
		if err := r.insertEvent(ctx, q, paymentID, event, data, from, to, stamp); err != nil {
			return err
		}
		transitioned = true
		model, err = r.loadModel(ctx, q, paymentID)
		return err
	})
	if err != nil {
		return Model{}, false, err
	}
	return model, transitioned, nil
}

func (r *repository) Update(ctx context.Context, paymentID string, patch Patch, event EventInput) (Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Model{}, err
	}
	if err := validatePaymentID(paymentID); err != nil {
		return Model{}, err
	}
	event, data, err := normalizeEvent(event)
	if err != nil {
		return Model{}, err
	}
	stamp, err := r.stamp()
	if err != nil {
		return Model{}, err
	}
	var model Model
	err = r.write(ctx, func(q *paymentsQueries.Queries) error {
		row, err := r.loadRow(ctx, q, paymentID)
		if err != nil {
			return err
		}
		if err := ensureNotDuplicate(ctx, q, paymentID, event.DedupeKey); err != nil {
			return err
		}
		if err := r.applyPatch(ctx, q, row, patch, stamp); err != nil {
			return err
		}
		if err := r.insertEvent(ctx, q, paymentID, event, data, "", "", stamp); err != nil {
			return err
		}
		model, err = r.loadModel(ctx, q, paymentID)
		return err
	})
	return model, err
}

func (r *repository) SubmitProof(ctx context.Context, paymentID string, proof ProofInput) (Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Model{}, err
	}
	if err := validatePaymentID(paymentID); err != nil {
		return Model{}, err
	}
	transactionID, mime, err := normalizeProof(proof)
	if err != nil {
		return Model{}, err
	}
	stamp, err := r.stamp()
	if err != nil {
		return Model{}, err
	}
	data, err := encodeJSON(map[string]any{"transaction_id": transactionID, "image_mime_type": mime}, maxEventDataBytes, ErrEventDataTooLarge)
	if err != nil {
		return Model{}, err
	}
	var model Model
	err = r.write(ctx, func(q *paymentsQueries.Queries) error {
		row, err := r.loadRow(ctx, q, paymentID)
		if err != nil {
			return err
		}
		from := State(row.State)
		if from != StateAwaitingPayment && from != StateVerifying {
			return ErrProofNotAllowed
		}
		used, err := q.ProofTransactionInUse(ctx, paymentsQueries.ProofTransactionInUseParams{ProofTransactionID: nullString(transactionID), ID: paymentID})
		if err != nil {
			return err
		}
		if used > 0 {
			return ErrProofTransactionReused
		}
		if err := q.UpdatePaymentProof(ctx, paymentsQueries.UpdatePaymentProofParams{
			ProofTransactionID: nullString(transactionID),
			ProofImage:         proof.Image,
			ProofImageMime:     nullString(mime),
			ProofSubmittedAt:   sql.NullInt64{Int64: stamp, Valid: true},
			UpdatedAt:          stamp,
			ID:                 paymentID,
		}); err != nil {
			return err
		}
		if from == StateAwaitingPayment {
			affected, err := q.UpdatePaymentState(ctx, paymentsQueries.UpdatePaymentStateParams{ToState: StateVerifying.String(), UpdatedAt: stamp, ID: paymentID, FromState: from.String()})
			if err != nil {
				return err
			}
			if affected != 1 {
				return ErrConcurrentUpdate
			}
		}
		event := EventInput{Source: EventSourceUser, Kind: eventKindProof}
		if err := r.insertEvent(ctx, q, paymentID, event, data, from, StateVerifying, stamp); err != nil {
			return err
		}
		model, err = r.loadModel(ctx, q, paymentID)
		return err
	})
	return model, err
}

func (r *repository) GetProofImage(ctx context.Context, paymentID string) ([]byte, string, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return nil, "", err
	}
	if err := validatePaymentID(paymentID); err != nil {
		return nil, "", err
	}
	var (
		image []byte
		mime  string
	)
	err := r.read(ctx, func(q *paymentsQueries.Queries) error {
		row, err := q.GetPaymentProofImage(ctx, paymentID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrPaymentNotFound
		}
		if err != nil {
			return err
		}
		if len(row.ProofImage) == 0 {
			return ErrProofNotFound
		}
		image, mime = row.ProofImage, row.ProofImageMime.String
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	return image, mime, nil
}

func (r *repository) Close() error {
	return errors.Join(r.writer.Close(), r.reader.Close())
}

func (r *repository) applyPatch(ctx context.Context, q *paymentsQueries.Queries, row paymentsQueries.GetPaymentRow, patch Patch, stamp int64) error {
	if patch.isEmpty() {
		return nil
	}
	params, err := applyPatch(row, patch)
	if err != nil {
		return err
	}
	if err := ensureReferenceFree(ctx, q, row, params); err != nil {
		return err
	}
	params.UpdatedAt = stamp
	return q.UpdatePaymentFields(ctx, params)
}
