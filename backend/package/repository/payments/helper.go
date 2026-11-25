package paymentsRepository

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	paymentsQueries "github.com/thesahibnanda-max/eCommerce/backend/package/repository/payments/queries"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
	stateMachineUtils "github.com/thesahibnanda-max/eCommerce/backend/package/utils/state_machine"
)

const (
	maxIDRunes         = 128
	maxPurposeRunes    = 100
	maxReasonRunes     = 500
	maxReferenceRunes  = 128
	maxURLBytes        = 2048
	maxDedupeRunes     = 256
	maxMetadataBytes   = 8192
	maxEventDataBytes  = 16384
	maxProofImageBytes = 5 << 20
	busyTimeoutMillis  = 5000
	readerConnections  = 8
	maxBatchIDs        = 200
	defaultListLimit   = 20
	maxListLimit       = 100
	eventKindCreated   = "created"
	eventKindProof     = "proof_submitted"
)

var (
	eventKindPattern     = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)
	transactionIDPattern = regexp.MustCompile(`^[A-Za-z0-9]{6,35}$`)
	allowedProofMIME     = map[string]struct{}{"image/jpeg": {}, "image/png": {}, "image/webp": {}}
)

func dsn(path string, writer bool) string {
	params := url.Values{}
	params.Add("_pragma", "foreign_keys(1)")
	params.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", busyTimeoutMillis))
	params.Add("_pragma", "journal_mode(WAL)")
	params.Add("_pragma", "synchronous(NORMAL)")
	params.Add("_pragma", "cache_size(-8000)")
	params.Add("_pragma", "temp_store(MEMORY)")
	if writer {
		params.Add("_txlock", "immediate")
	} else {
		params.Add("_pragma", "query_only(1)")
	}
	return "file:" + path + "?" + params.Encode()
}

func cleanText(value string, maxRunes int) (string, bool) {
	value = strings.TrimSpace(value)
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > maxRunes || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return "", false
	}
	return value, true
}

func validatePaymentID(id string) error {
	if !utils.ValidateULID(id) {
		return ErrPaymentIDInvalid
	}
	return nil
}

func normalizeCreate(input CreateInput) (CreateInput, string, error) {
	orderID, ok := cleanText(input.OrderID, maxIDRunes)
	if !ok || orderID == "" {
		return CreateInput{}, "", ErrOrderIDInvalid
	}
	userID, ok := cleanText(input.UserID, maxIDRunes)
	if !ok || userID == "" {
		return CreateInput{}, "", ErrUserIDInvalid
	}
	if !input.Provider.IsValid() {
		return CreateInput{}, "", ErrProviderInvalid
	}
	if input.Amount < 1 {
		return CreateInput{}, "", ErrAmountInvalid
	}
	purpose, ok := cleanText(input.Purpose, maxPurposeRunes)
	if !ok {
		return CreateInput{}, "", ErrPurposeInvalid
	}
	metadata, err := encodeJSON(nonNilMap(input.Metadata), maxMetadataBytes, ErrMetadataTooLarge)
	if err != nil {
		return CreateInput{}, "", err
	}
	return CreateInput{OrderID: orderID, UserID: userID, Provider: input.Provider, Amount: input.Amount, Purpose: purpose}, metadata, nil
}

func normalizeEvent(event EventInput) (EventInput, string, error) {
	if !event.Source.IsValid() {
		return EventInput{}, "", ErrEventSourceInvalid
	}
	if !eventKindPattern.MatchString(event.Kind) {
		return EventInput{}, "", ErrEventKindInvalid
	}
	dedupe, ok := cleanText(event.DedupeKey, maxDedupeRunes)
	if !ok {
		return EventInput{}, "", ErrEventDedupeKeyInvalid
	}
	data, err := encodeJSON(nonNilMap(event.Data), maxEventDataBytes, ErrEventDataTooLarge)
	if err != nil {
		return EventInput{}, "", err
	}
	return EventInput{Source: event.Source, Kind: event.Kind, DedupeKey: dedupe}, data, nil
}

func (p Patch) isEmpty() bool {
	return p.GatewayReference == nil && p.GatewayPaymentID == nil && p.CheckoutURL == nil && p.FailureReason == nil && len(p.Metadata) == 0
}

func applyPatch(row paymentsQueries.GetPaymentRow, patch Patch) (paymentsQueries.UpdatePaymentFieldsParams, error) {
	params := paymentsQueries.UpdatePaymentFieldsParams{
		GatewayReference: row.GatewayReference,
		GatewayPaymentID: row.GatewayPaymentID,
		CheckoutUrl:      row.CheckoutUrl,
		FailureReason:    row.FailureReason,
		Metadata:         row.Metadata,
		ID:               row.ID,
	}
	if patch.GatewayReference != nil {
		value, ok := cleanText(*patch.GatewayReference, maxReferenceRunes)
		if !ok || value == "" {
			return params, ErrReferenceInvalid
		}
		params.GatewayReference = nullString(value)
	}
	if patch.GatewayPaymentID != nil {
		value, ok := cleanText(*patch.GatewayPaymentID, maxReferenceRunes)
		if !ok || value == "" {
			return params, ErrReferenceInvalid
		}
		params.GatewayPaymentID = nullString(value)
	}
	if patch.CheckoutURL != nil {
		value := strings.TrimSpace(*patch.CheckoutURL)
		if !isWebURL(value) {
			return params, ErrReferenceInvalid
		}
		params.CheckoutUrl = nullString(value)
	}
	if patch.FailureReason != nil {
		value, ok := cleanText(*patch.FailureReason, maxReasonRunes)
		if !ok {
			return params, ErrFailureReasonTooLong
		}
		params.FailureReason = value
	}
	if len(patch.Metadata) > 0 {
		current, err := decodeMap(row.Metadata)
		if err != nil {
			return params, err
		}
		for key, value := range patch.Metadata {
			if value == nil {
				delete(current, key)
				continue
			}
			current[key] = value
		}
		encoded, err := encodeJSON(current, maxMetadataBytes, ErrMetadataTooLarge)
		if err != nil {
			return params, err
		}
		params.Metadata = encoded
	}
	return params, nil
}

func isWebURL(raw string) bool {
	if raw == "" || len(raw) > maxURLBytes {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil {
		return false
	}
	return parsed.Scheme == "https" || parsed.Scheme == "http"
}

func normalizeProof(proof ProofInput) (string, string, error) {
	transactionID := strings.TrimSpace(proof.TransactionID)
	if !transactionIDPattern.MatchString(transactionID) {
		return "", "", ErrProofTransactionID
	}
	if len(proof.Image) == 0 {
		return "", "", ErrProofImageInvalid
	}
	if len(proof.Image) > maxProofImageBytes {
		return "", "", ErrProofImageTooLarge
	}
	mime := http.DetectContentType(proof.Image)
	if _, ok := allowedProofMIME[mime]; !ok {
		return "", "", ErrProofImageInvalid
	}
	return transactionID, mime, nil
}

func encodeJSON[T any](value T, limit int, tooLarge error) (string, error) {
	raw, err := utils.FromObjectToStringJSON(value)
	if err != nil {
		return "", err
	}
	if len(raw) > limit {
		return "", tooLarge
	}
	return raw, nil
}

func decodeMap(raw string) (map[string]any, error) {
	value, err := utils.FromStringToObjectJSON[map[string]any](raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrStoredDataCorrupted, err)
	}
	return nonNilMap(value), nil
}

func nonNilMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}

func (r *repository) toTime(micros int64) time.Time {
	return time.UnixMicro(micros).In(r.location)
}

func (r *repository) toModel(row paymentsQueries.GetPaymentRow, events []paymentsQueries.PaymentEvent) (Model, error) {
	provider := Provider(row.Provider)
	state := State(row.State)
	currency := Currency(row.Currency)
	if !provider.IsValid() || !state.IsValid() || !currency.IsValid() {
		return Model{}, ErrStoredDataCorrupted
	}
	metadata, err := decodeMap(row.Metadata)
	if err != nil {
		return Model{}, err
	}
	model := Model{
		ID:               row.ID,
		OrderID:          row.OrderID,
		UserID:           row.UserID,
		Provider:         provider,
		State:            state,
		Amount:           row.Amount,
		Currency:         currency,
		Purpose:          row.Purpose,
		GatewayReference: row.GatewayReference.String,
		GatewayPaymentID: row.GatewayPaymentID.String,
		CheckoutURL:      row.CheckoutUrl.String,
		FailureReason:    row.FailureReason,
		Metadata:         metadata,
		Events:           make([]Event, 0, len(events)),
		CreatedAt:        r.toTime(row.CreatedAt),
		UpdatedAt:        r.toTime(row.UpdatedAt),
	}
	if row.ProofTransactionID.Valid {
		model.Proof = &Proof{
			TransactionID: row.ProofTransactionID.String,
			ImageMIMEType: row.ProofImageMime.String,
			SubmittedAt:   r.toTime(row.ProofSubmittedAt.Int64),
		}
	}
	for _, event := range events {
		data, err := decodeMap(event.Data)
		if err != nil {
			return Model{}, err
		}
		model.Events = append(model.Events, Event{
			Source:    EventSource(event.Source),
			Kind:      event.Kind,
			FromState: State(event.FromState.String),
			ToState:   State(event.ToState.String),
			Data:      data,
			CreatedAt: r.toTime(event.CreatedAt),
		})
	}
	return model, nil
}

func (r *repository) loadRow(ctx context.Context, q *paymentsQueries.Queries, id string) (paymentsQueries.GetPaymentRow, error) {
	row, err := q.GetPayment(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return row, ErrPaymentNotFound
	}
	return row, err
}

func (r *repository) loadModel(ctx context.Context, q *paymentsQueries.Queries, id string) (Model, error) {
	row, err := r.loadRow(ctx, q, id)
	if err != nil {
		return Model{}, err
	}
	events, err := q.ListEvents(ctx, id)
	if err != nil {
		return Model{}, err
	}
	return r.toModel(row, events)
}

func (r *repository) insertEvent(ctx context.Context, q *paymentsQueries.Queries, paymentID string, event EventInput, data string, from, to State, stamp int64) error {
	return q.InsertEvent(ctx, paymentsQueries.InsertEventParams{
		PaymentID: paymentID,
		Source:    event.Source.String(),
		Kind:      event.Kind,
		FromState: nullString(from.String()),
		ToState:   nullString(to.String()),
		Data:      data,
		DedupeKey: nullString(event.DedupeKey),
		CreatedAt: stamp,
	})
}

func ensureNotDuplicate(ctx context.Context, q *paymentsQueries.Queries, paymentID, dedupeKey string) error {
	if dedupeKey == "" {
		return nil
	}
	count, err := q.EventExists(ctx, paymentsQueries.EventExistsParams{PaymentID: paymentID, DedupeKey: nullString(dedupeKey)})
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrEventDuplicate
	}
	return nil
}

func ensureReferenceFree(ctx context.Context, q *paymentsQueries.Queries, row paymentsQueries.GetPaymentRow, params paymentsQueries.UpdatePaymentFieldsParams) error {
	if !params.GatewayReference.Valid || params.GatewayReference == row.GatewayReference {
		return nil
	}
	other, err := q.GetPaymentByReference(ctx, paymentsQueries.GetPaymentByReferenceParams{Provider: row.Provider, GatewayReference: params.GatewayReference})
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if other.ID != row.ID {
		return ErrReferenceTaken
	}
	return nil
}

func canTransition(from, to State) (bool, error) {
	machine, err := NewStateMachine(from)
	if err != nil {
		return false, err
	}
	return machine.CanTransitionTo(stateMachineUtils.NewState(to)), nil
}

func withTx(ctx context.Context, db *sql.DB, opts *sql.TxOptions, fn func(q *paymentsQueries.Queries) error) (err error) {
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return err
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			_ = tx.Rollback()
			err = fmt.Errorf("%w: %v", ErrTransactionPanicked, recovered)
		}
	}()
	if err = fn(paymentsQueries.New(tx)); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func nullString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func normalizeListQuery(query ListQuery) (ListQuery, string, int64, string, error) {
	if query.Limit == 0 {
		query.Limit = defaultListLimit
	}
	if query.Limit < 1 || query.Limit > maxListLimit {
		return ListQuery{}, "", 0, "", ErrLimitInvalid
	}
	if query.Provider != "" && !query.Provider.IsValid() {
		return ListQuery{}, "", 0, "", ErrProviderInvalid
	}
	states := make([]string, 0, len(query.States))
	for _, state := range query.States {
		if !state.IsValid() {
			return ListQuery{}, "", 0, "", ErrStateInvalid
		}
		states = append(states, state.String())
	}
	statesJSON, err := utils.FromObjectToStringJSON(states)
	if err != nil {
		return ListQuery{}, "", 0, "", err
	}
	cursorAt, cursorID, err := decodeCursor(query.Cursor)
	if err != nil {
		return ListQuery{}, "", 0, "", err
	}
	return query, statesJSON, cursorAt, cursorID, nil
}

func encodeCursor(createdAt int64, id string) (string, error) {
	raw, err := utils.FromObjectToStringJSON(cursor{CreatedAt: createdAt, ID: id})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString([]byte(raw)), nil
}

func decodeCursor(value string) (int64, string, error) {
	if value == "" {
		return math.MaxInt64, "", nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return 0, "", ErrCursorInvalid
	}
	decoded, err := utils.FromStringToObjectJSON[cursor](string(raw))
	if err != nil || !utils.ValidateULID(decoded.ID) {
		return 0, "", ErrCursorInvalid
	}
	return decoded.CreatedAt, decoded.ID, nil
}
