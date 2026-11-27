package spreadsheetRepository

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/sheets/v4"
)

const forbiddenSheetChars = "[]:*?/\\"

func normalizeSnapshot(snapshot Snapshot) (Snapshot, int, error) {
	database := strings.TrimSpace(snapshot.Database)
	if database == "" || utf8.RuneCountInString(database) > maxDatabaseRunes {
		return Snapshot{}, 0, ErrDatabaseInvalid
	}
	if snapshot.TakenAt.IsZero() {
		return Snapshot{}, 0, ErrTakenAtRequired
	}
	if len(snapshot.Sheets) == 0 {
		return Snapshot{}, 0, ErrSheetsRequired
	}
	out := Snapshot{Database: database, TakenAt: snapshot.TakenAt, Sheets: make([]Sheet, 0, len(snapshot.Sheets))}
	seen := make(map[string]struct{}, len(snapshot.Sheets))
	total := 0
	for _, sheet := range snapshot.Sheets {
		name := strings.TrimSpace(sheet.Name)
		if !validSheetName(name) {
			return Snapshot{}, 0, fmt.Errorf("%w: %q", ErrSheetNameInvalid, sheet.Name)
		}
		folded := strings.ToLower(name)
		if _, ok := seen[folded]; ok {
			return Snapshot{}, 0, fmt.Errorf("%w: %q", ErrSheetNameDuplicate, name)
		}
		seen[folded] = struct{}{}
		rows := make([]Row, 0, len(sheet.Rows))
		for _, row := range sheet.Rows {
			key := strings.TrimSpace(row.Key)
			raw := strings.TrimSpace(row.JSON)
			if key == "" && raw == "" {
				continue
			}
			if err := validateRow(name, key, raw); err != nil {
				return Snapshot{}, 0, err
			}
			rows = append(rows, Row{Key: key, JSON: raw})
		}
		total += len(rows)
		out.Sheets = append(out.Sheets, Sheet{Name: name, Rows: rows})
	}
	return out, total, nil
}

func validSheetName(name string) bool {
	if name == "" || utf8.RuneCountInString(name) > maxSheetNameRunes {
		return false
	}
	return !strings.ContainsAny(name, forbiddenSheetChars)
}

func validateRow(sheet, key, raw string) error {
	if key == "" || raw == "" {
		return fmt.Errorf("%w: sheet %q key %q", ErrRowInvalid, sheet, key)
	}
	if utf8.RuneCountInString(key) > maxCellRunes || utf8.RuneCountInString(raw) > maxCellRunes {
		return fmt.Errorf("%w: sheet %q key %q", ErrCellTooLarge, sheet, truncateKey(key))
	}
	if _, err := utils.FromStringToObjectJSON[any](raw); err != nil {
		return fmt.Errorf("%w: sheet %q key %q: %w", ErrRowInvalid, sheet, key, err)
	}
	return nil
}

func truncateKey(key string) string {
	if utf8.RuneCountInString(key) <= 60 {
		return key
	}
	return string([]rune(key)[:60]) + "…"
}

func (r *repository) title(database string) string {
	return r.titlePrefix + " - " + database
}

func (r *repository) spreadsheetID(ctx context.Context, database string) (string, error) {
	if id, ok := r.ids[database]; ok {
		return id, nil
	}
	title := r.title(database)
	query := fmt.Sprintf("name = '%s' and mimeType = '%s' and trashed = false", escapeQuery(title), spreadsheetMIME)
	if r.folderID != "" {
		query += fmt.Sprintf(" and '%s' in parents", escapeQuery(r.folderID))
	}
	var list *drive.FileList
	if err := r.retry(ctx, func() (err error) {
		list, err = r.drive.Files.List().Q(query).OrderBy("createdTime").PageSize(1).Fields("files(id)").Context(ctx).Do()
		return err
	}); err != nil {
		return "", err
	}
	if len(list.Files) > 0 && list.Files[0].Id != "" {
		r.ids[database] = list.Files[0].Id
		return list.Files[0].Id, nil
	}
	file := &drive.File{Name: title, MimeType: spreadsheetMIME}
	if r.folderID != "" {
		file.Parents = []string{r.folderID}
	}
	var created *drive.File
	if err := r.retry(ctx, func() (err error) {
		created, err = r.drive.Files.Create(file).Fields("id").Context(ctx).Do()
		return err
	}); err != nil {
		return "", err
	}
	if created.Id == "" {
		return "", fmt.Errorf("%w: drive returned no file id", ErrRequestFailed)
	}
	r.ids[database] = created.Id
	return created.Id, nil
}

func (r *repository) syncTabs(ctx context.Context, id string, wanted []Sheet) error {
	var spreadsheet *sheets.Spreadsheet
	if err := r.retry(ctx, func() (err error) {
		spreadsheet, err = r.sheets.Spreadsheets.Get(id).Fields("sheets(properties(sheetId,title))").Context(ctx).Do()
		return err
	}); err != nil {
		return err
	}
	existing := make(map[string]*sheets.SheetProperties, len(spreadsheet.Sheets))
	for _, sheet := range spreadsheet.Sheets {
		if sheet.Properties != nil {
			existing[strings.ToLower(sheet.Properties.Title)] = sheet.Properties
		}
	}
	requests := make([]*sheets.Request, 0, len(wanted)+len(existing))
	kept := make(map[int64]struct{}, len(wanted))
	for _, sheet := range wanted {
		grid := gridFor(len(sheet.Rows))
		if current, ok := existing[strings.ToLower(sheet.Name)]; ok {
			kept[current.SheetId] = struct{}{}
			requests = append(requests, &sheets.Request{UpdateSheetProperties: &sheets.UpdateSheetPropertiesRequest{
				Properties: &sheets.SheetProperties{SheetId: current.SheetId, Title: sheet.Name, GridProperties: grid, ForceSendFields: []string{"SheetId"}},
				Fields:     "title,gridProperties(rowCount,columnCount,frozenRowCount)",
			}})
			continue
		}
		requests = append(requests, &sheets.Request{AddSheet: &sheets.AddSheetRequest{
			Properties: &sheets.SheetProperties{Title: sheet.Name, GridProperties: grid},
		}})
	}
	for _, current := range existing {
		if _, ok := kept[current.SheetId]; ok {
			continue
		}
		requests = append(requests, &sheets.Request{DeleteSheet: &sheets.DeleteSheetRequest{SheetId: current.SheetId, ForceSendFields: []string{"SheetId"}}})
	}
	return r.retry(ctx, func() error {
		_, err := r.sheets.Spreadsheets.BatchUpdate(id, &sheets.BatchUpdateSpreadsheetRequest{Requests: requests}).Context(ctx).Do()
		return err
	})
}

func gridFor(rows int) *sheets.GridProperties {
	return &sheets.GridProperties{
		RowCount:       int64(max(rows+1, 2)),
		ColumnCount:    columnCount,
		FrozenRowCount: 1,
	}
}

func (r *repository) writeValues(ctx context.Context, id string, snapshot Snapshot) error {
	ranges := make([]string, 0, len(snapshot.Sheets))
	for _, sheet := range snapshot.Sheets {
		ranges = append(ranges, quoteSheet(sheet.Name))
	}
	if err := r.retry(ctx, func() error {
		_, err := r.sheets.Spreadsheets.Values.BatchClear(id, &sheets.BatchClearValuesRequest{Ranges: ranges}).Context(ctx).Do()
		return err
	}); err != nil {
		return err
	}
	takenAt := snapshot.TakenAt.Format(time.RFC3339)
	var (
		pending []*sheets.ValueRange
		size    int
	)
	flush := func() error {
		if len(pending) == 0 {
			return nil
		}
		batch := pending
		pending, size = nil, 0
		return r.retry(ctx, func() error {
			_, err := r.sheets.Spreadsheets.Values.BatchUpdate(id, &sheets.BatchUpdateValuesRequest{ValueInputOption: valueInputRaw, Data: batch}).Context(ctx).Do()
			return err
		})
	}
	for _, sheet := range snapshot.Sheets {
		block := &sheets.ValueRange{Range: cellRange(sheet.Name, 1), Values: [][]any{{headerKey, headerTakenAt, headerJSON}}}
		for i, row := range sheet.Rows {
			cost := len(row.Key) + len(row.JSON) + len(takenAt) + 16
			if size > 0 && size+cost > r.chunkBytes {
				if len(block.Values) > 0 {
					pending = append(pending, block)
				}
				if err := flush(); err != nil {
					return err
				}
				block = &sheets.ValueRange{Range: cellRange(sheet.Name, i+2)}
			}
			block.Values = append(block.Values, []any{row.Key, takenAt, row.JSON})
			size += cost
		}
		pending = append(pending, block)
	}
	return flush()
}

func quoteSheet(name string) string {
	return "'" + strings.ReplaceAll(name, "'", "''") + "'"
}

func cellRange(name string, row int) string {
	return quoteSheet(name) + "!A" + strconv.Itoa(row)
}

func escapeQuery(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, `\`, `\\`), `'`, `\'`)
}

func (r *repository) retry(ctx context.Context, call func() error) error {
	wait := r.backoff
	var last error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		last = call()
		if last == nil {
			return nil
		}
		if !retryable(last) || attempt == maxAttempts {
			break
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("%w: %w", ErrCtxInvalid, ctx.Err())
		case <-timer.C:
		}
		wait *= 2
	}
	return fmt.Errorf("%w: %w", ErrRequestFailed, last)
}

func retryable(err error) bool {
	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) {
		return apiErr.Code == http.StatusTooManyRequests || apiErr.Code >= http.StatusInternalServerError
	}
	return false
}

func recoverAsError(err *error) {
	if recovered := recover(); recovered != nil {
		*err = fmt.Errorf("%w: %v", ErrPanicked, recovered)
	}
}
