package spreadsheetRepository

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
	"google.golang.org/api/option"
)

type fakeTab struct {
	id     int64
	title  string
	rows   int64
	cols   int64
	frozen int64
	cells  map[int][]string
}

type fakeGoogle struct {
	mu          sync.Mutex
	names       map[string]string
	parents     map[string]string
	books       map[string][]*fakeTab
	nextFile    int
	nextTab     int64
	creates     int
	lists       int
	valueWrites int
	failNext    int
	inputs      []string
}

func newFakeGoogle() *fakeGoogle {
	return &fakeGoogle{names: map[string]string{}, parents: map[string]string{}, books: map[string][]*fakeTab{}, nextTab: 100}
}

func (f *fakeGoogle) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /drive/files", f.listFiles)
	mux.HandleFunc("POST /drive/files", f.createFile)
	mux.HandleFunc("GET /sheets/v4/spreadsheets/{id}", f.getSpreadsheet)
	mux.HandleFunc("POST /sheets/v4/spreadsheets/{idop}", f.batchUpdate)
	mux.HandleFunc("POST /sheets/v4/spreadsheets/{id}/{op}", f.values)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		if f.failNext > 0 {
			f.failNext--
			f.mu.Unlock()
			writeError(w, http.StatusTooManyRequests, "rate limited")
			return
		}
		f.mu.Unlock()
		mux.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	body, _ := utils.FromObjectToStringJSON(v)
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, body)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	body, _ := utils.FromObjectToStringJSON(map[string]any{"error": map[string]any{"code": code, "message": msg}})
	_, _ = io.WriteString(w, body)
}

func readBody[T any](r *http.Request) (T, error) {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		var zero T
		return zero, err
	}
	return utils.FromStringToObjectJSON[T](string(raw))
}

func (f *fakeGoogle) listFiles(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lists++
	q := r.URL.Query().Get("q")
	start := strings.Index(q, "name = '") + len("name = '")
	end := strings.Index(q, "' and mimeType")
	name := strings.ReplaceAll(q[start:end], `\'`, `'`)
	folder := ""
	if i := strings.Index(q, "and '"); i >= 0 && strings.HasSuffix(q, "' in parents") {
		folder = strings.TrimSuffix(q[i+len("and '"):], "' in parents")
	}
	files := []map[string]string{}
	for id, n := range f.names {
		if n == name && (folder == "" || f.parents[id] == folder) {
			files = append(files, map[string]string{"id": id})
		}
	}
	writeJSON(w, map[string]any{"files": files})
}

func (f *fakeGoogle) createFile(w http.ResponseWriter, r *http.Request) {
	body, err := readBody[struct {
		Name     string   `json:"name"`
		MimeType string   `json:"mimeType"`
		Parents  []string `json:"parents"`
	}](r)
	if err != nil || body.MimeType != spreadsheetMIME {
		writeError(w, http.StatusBadRequest, "bad create")
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.creates++
	f.nextFile++
	id := "file-" + strconv.Itoa(f.nextFile)
	f.names[id] = body.Name
	if len(body.Parents) > 0 {
		f.parents[id] = body.Parents[0]
	}
	f.books[id] = []*fakeTab{{id: 0, title: "Sheet1", rows: 1000, cols: 26, cells: map[int][]string{}}}
	writeJSON(w, map[string]string{"id": id})
}

func (f *fakeGoogle) getSpreadsheet(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	tabs, ok := f.books[r.PathValue("id")]
	if !ok {
		writeError(w, http.StatusNotFound, "no spreadsheet")
		return
	}
	sheets := make([]map[string]any, 0, len(tabs))
	for _, tab := range tabs {
		sheets = append(sheets, map[string]any{"properties": map[string]any{"sheetId": tab.id, "title": tab.title}})
	}
	writeJSON(w, map[string]any{"sheets": sheets})
}

func gridOf(props map[string]any) (int64, int64, int64) {
	grid, _ := props["gridProperties"].(map[string]any)
	num := func(key string) int64 {
		v, _ := grid[key].(float64)
		return int64(v)
	}
	return num("rowCount"), num("columnCount"), num("frozenRowCount")
}

func (f *fakeGoogle) batchUpdate(w http.ResponseWriter, r *http.Request) {
	id, op, _ := strings.Cut(r.PathValue("idop"), ":")
	if op != "batchUpdate" {
		writeError(w, http.StatusNotFound, "unknown op")
		return
	}
	body, err := readBody[struct {
		Requests []map[string]map[string]any `json:"requests"`
	}](r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	tabs, ok := f.books[id]
	if !ok {
		writeError(w, http.StatusNotFound, "no spreadsheet")
		return
	}
	find := func(sheetID int64) *fakeTab {
		for _, tab := range tabs {
			if tab.id == sheetID {
				return tab
			}
		}
		return nil
	}
	for _, req := range body.Requests {
		switch {
		case req["addSheet"] != nil:
			props, _ := req["addSheet"]["properties"].(map[string]any)
			rows, cols, frozen := gridOf(props)
			title, _ := props["title"].(string)
			tabs = append(tabs, &fakeTab{id: f.nextTab, title: title, rows: rows, cols: cols, frozen: frozen, cells: map[int][]string{}})
			f.nextTab++
		case req["updateSheetProperties"] != nil:
			props, _ := req["updateSheetProperties"]["properties"].(map[string]any)
			raw, has := props["sheetId"]
			if !has {
				writeError(w, http.StatusBadRequest, "updateSheetProperties without sheetId")
				return
			}
			tab := find(int64(raw.(float64)))
			if tab == nil {
				writeError(w, http.StatusBadRequest, "unknown sheet")
				return
			}
			tab.title, _ = props["title"].(string)
			tab.rows, tab.cols, tab.frozen = gridOf(props)
			for row := range tab.cells {
				if int64(row) > tab.rows {
					delete(tab.cells, row)
				}
			}
		case req["deleteSheet"] != nil:
			raw, has := req["deleteSheet"]["sheetId"]
			if !has {
				writeError(w, http.StatusBadRequest, "deleteSheet without sheetId")
				return
			}
			target := int64(raw.(float64))
			next := tabs[:0:0]
			for _, tab := range tabs {
				if tab.id != target {
					next = append(next, tab)
				}
			}
			if len(next) == 0 {
				writeError(w, http.StatusBadRequest, "cannot delete the last sheet")
				return
			}
			tabs = next
		}
	}
	f.books[id] = tabs
	writeJSON(w, map[string]any{"spreadsheetId": id})
}

func parseRange(value string) (string, int) {
	title, rest := value, ""
	if strings.HasPrefix(value, "'") {
		i := 1
		for i < len(value) {
			if value[i] == '\'' {
				if i+1 < len(value) && value[i+1] == '\'' {
					i += 2
					continue
				}
				break
			}
			i++
		}
		title = strings.ReplaceAll(value[1:i], "''", "'")
		rest = value[i+1:]
	}
	row := 1
	if strings.HasPrefix(rest, "!A") {
		row, _ = strconv.Atoi(rest[2:])
	}
	return title, row
}

func (f *fakeGoogle) values(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	f.mu.Lock()
	defer f.mu.Unlock()
	tabs, ok := f.books[id]
	if !ok {
		writeError(w, http.StatusNotFound, "no spreadsheet")
		return
	}
	byTitle := func(title string) *fakeTab {
		for _, tab := range tabs {
			if tab.title == title {
				return tab
			}
		}
		return nil
	}
	switch r.PathValue("op") {
	case "values:batchClear":
		body, err := readBody[struct {
			Ranges []string `json:"ranges"`
		}](r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		for _, rng := range body.Ranges {
			title, _ := parseRange(rng)
			if tab := byTitle(title); tab != nil {
				tab.cells = map[int][]string{}
			}
		}
		writeJSON(w, map[string]any{})
	case "values:batchUpdate":
		body, err := readBody[struct {
			ValueInputOption string `json:"valueInputOption"`
			Data             []struct {
				Range  string     `json:"range"`
				Values [][]string `json:"values"`
			} `json:"data"`
		}](r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		f.valueWrites++
		f.inputs = append(f.inputs, body.ValueInputOption)
		for _, block := range body.Data {
			title, start := parseRange(block.Range)
			tab := byTitle(title)
			if tab == nil {
				writeError(w, http.StatusBadRequest, "unknown sheet "+title)
				return
			}
			for i, values := range block.Values {
				row := start + i
				if int64(row) > tab.rows || int64(len(values)) > tab.cols {
					writeError(w, http.StatusBadRequest, fmt.Sprintf("write outside grid %s row %d", title, row))
					return
				}
				tab.cells[row] = values
			}
		}
		writeJSON(w, map[string]any{})
	default:
		writeError(w, http.StatusNotFound, "unknown op")
	}
}

func (f *fakeGoogle) tabs(id string) []*fakeTab {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.books[id]
}

func newTestRepo(t *testing.T, fake *fakeGoogle, cfg Config) *repository {
	t.Helper()
	server := httptest.NewServer(fake.handler())
	t.Cleanup(server.Close)
	client := option.WithHTTPClient(server.Client())
	r, err := newRepositoryWith(context.Background(), cfg,
		[]option.ClientOption{client, option.WithEndpoint(server.URL + "/sheets/")},
		[]option.ClientOption{client, option.WithEndpoint(server.URL + "/drive/")},
	)
	if err != nil {
		t.Fatal(err)
	}
	r.backoff = time.Millisecond
	return r
}

var takenAt = time.Date(2026, 11, 27, 18, 30, 0, 0, time.FixedZone("IST", 5*3600+1800))

func rowsOf(n int, prefix string) []Row {
	rows := make([]Row, n)
	for i := range rows {
		rows[i] = Row{Key: fmt.Sprintf("%s-%03d", prefix, i), JSON: fmt.Sprintf(`{"n":%d}`, i)}
	}
	return rows
}

func TestReplaceDatabase_CreatesFileAndWritesRows(t *testing.T) {
	fake := newFakeGoogle()
	r := newTestRepo(t, fake, Config{})
	res, err := r.ReplaceDatabase(context.Background(), Snapshot{
		Database: "users",
		TakenAt:  takenAt,
		Sheets: []Sheet{
			{Name: "users", Rows: []Row{{Key: "u1", JSON: `{"name":"Asha"}`}, {Key: "u2", JSON: `{"formula":"=1+1"}`}, {}}},
			{Name: "email_index"},
		},
	})
	if err != nil {
		t.Fatalf("ReplaceDatabase: %v", err)
	}
	if res.SpreadsheetID == "" || res.URL != spreadsheetURL+res.SpreadsheetID || res.Sheets != 2 || res.Rows != 2 {
		t.Fatalf("result = %+v", res)
	}
	if fake.creates != 1 || fake.names[res.SpreadsheetID] != "eCommerce snapshot - users" {
		t.Fatalf("creates=%d names=%v", fake.creates, fake.names)
	}
	tabs := fake.tabs(res.SpreadsheetID)
	if len(tabs) != 2 || tabs[0].title != "users" || tabs[1].title != "email_index" {
		t.Fatalf("tabs = %+v", tabs)
	}
	users := tabs[0]
	if users.rows != 3 || users.cols != 3 || users.frozen != 1 {
		t.Fatalf("users grid = %d x %d frozen %d", users.rows, users.cols, users.frozen)
	}
	want := map[int][]string{
		1: {headerKey, headerTakenAt, headerJSON},
		2: {"u1", "2026-11-27T18:30:00+05:30", `{"name":"Asha"}`},
		3: {"u2", "2026-11-27T18:30:00+05:30", `{"formula":"=1+1"}`},
	}
	for row, values := range want {
		if strings.Join(users.cells[row], "|") != strings.Join(values, "|") {
			t.Fatalf("row %d = %v, want %v", row, users.cells[row], values)
		}
	}
	empty := tabs[1]
	if empty.rows != 2 || len(empty.cells) != 1 || empty.cells[1][0] != headerKey {
		t.Fatalf("empty tab = rows %d cells %v", empty.rows, empty.cells)
	}
	for _, input := range fake.inputs {
		if input != valueInputRaw {
			t.Fatalf("value input option = %q, want RAW", input)
		}
	}
}

func TestReplaceDatabase_SecondSnapshotOverwritesSameFile(t *testing.T) {
	fake := newFakeGoogle()
	r := newTestRepo(t, fake, Config{CompanyName: "Aesthetic Indians"})
	first, err := r.ReplaceDatabase(context.Background(), Snapshot{Database: "orders", TakenAt: takenAt, Sheets: []Sheet{
		{Name: "orders", Rows: rowsOf(5, "o")},
		{Name: "old_tab", Rows: rowsOf(1, "x")},
	}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := r.ReplaceDatabase(context.Background(), Snapshot{Database: "orders", TakenAt: takenAt.Add(time.Hour), Sheets: []Sheet{
		{Name: "Orders", Rows: rowsOf(2, "o")},
		{Name: "payments", Rows: rowsOf(1, "p")},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if fake.names[first.SpreadsheetID] != "Aesthetic Indians snapshot - orders" {
		t.Fatalf("title = %q", fake.names[first.SpreadsheetID])
	}
	if first.SpreadsheetID != second.SpreadsheetID || fake.creates != 1 || fake.lists != 1 {
		t.Fatalf("ids %s/%s creates=%d lists=%d", first.SpreadsheetID, second.SpreadsheetID, fake.creates, fake.lists)
	}
	tabs := fake.tabs(second.SpreadsheetID)
	titles := []string{}
	for _, tab := range tabs {
		titles = append(titles, tab.title)
	}
	if strings.Join(titles, ",") != "Orders,payments" {
		t.Fatalf("tabs = %v", titles)
	}
	orders := tabs[0]
	if orders.rows != 3 || len(orders.cells) != 3 || orders.cells[2][1] != "2026-11-27T19:30:00+05:30" {
		t.Fatalf("orders tab rows=%d cells=%v", orders.rows, orders.cells)
	}
}

func TestReplaceDatabase_FindsExistingFileByName(t *testing.T) {
	fake := newFakeGoogle()
	first := newTestRepo(t, fake, Config{FolderID: "folder-1"})
	res, err := first.ReplaceDatabase(context.Background(), Snapshot{Database: "carts", TakenAt: takenAt, Sheets: []Sheet{{Name: "carts", Rows: rowsOf(1, "c")}}})
	if err != nil {
		t.Fatal(err)
	}
	if fake.parents[res.SpreadsheetID] != "folder-1" {
		t.Fatalf("file not created in folder: %v", fake.parents)
	}
	second := newTestRepo(t, fake, Config{FolderID: "folder-1"})
	again, err := second.ReplaceDatabase(context.Background(), Snapshot{Database: "carts", TakenAt: takenAt, Sheets: []Sheet{{Name: "carts"}}})
	if err != nil {
		t.Fatal(err)
	}
	if again.SpreadsheetID != res.SpreadsheetID || fake.creates != 1 {
		t.Fatalf("expected reuse of %s, got %s with %d creates", res.SpreadsheetID, again.SpreadsheetID, fake.creates)
	}
}

func TestReplaceDatabase_ChunksLargeSheets(t *testing.T) {
	fake := newFakeGoogle()
	r := newTestRepo(t, fake, Config{})
	r.chunkBytes = 200
	res, err := r.ReplaceDatabase(context.Background(), Snapshot{Database: "products", TakenAt: takenAt, Sheets: []Sheet{
		{Name: "products", Rows: rowsOf(40, "p")},
		{Name: "variants", Rows: rowsOf(15, "v")},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if fake.valueWrites < 3 {
		t.Fatalf("expected several chunked writes, got %d", fake.valueWrites)
	}
	tabs := fake.tabs(res.SpreadsheetID)
	if len(tabs[0].cells) != 41 || len(tabs[1].cells) != 16 {
		t.Fatalf("cells products=%d variants=%d", len(tabs[0].cells), len(tabs[1].cells))
	}
	for i := 0; i < 40; i++ {
		if got := tabs[0].cells[i+2][0]; got != fmt.Sprintf("p-%03d", i) {
			t.Fatalf("row %d key = %q", i+2, got)
		}
	}
}

func TestReplaceDatabase_RetriesRateLimits(t *testing.T) {
	fake := newFakeGoogle()
	fake.failNext = 2
	r := newTestRepo(t, fake, Config{})
	if _, err := r.ReplaceDatabase(context.Background(), Snapshot{Database: "images", TakenAt: takenAt, Sheets: []Sheet{{Name: "meta", Rows: rowsOf(2, "i")}}}); err != nil {
		t.Fatalf("expected retry to succeed: %v", err)
	}
	fake.failNext = maxAttempts
	r2 := newTestRepo(t, fake, Config{})
	_, err := r2.ReplaceDatabase(context.Background(), Snapshot{Database: "other", TakenAt: takenAt, Sheets: []Sheet{{Name: "meta"}}})
	if !errors.Is(err, ErrRequestFailed) {
		t.Fatalf("err = %v, want ErrRequestFailed", err)
	}
}

func TestReplaceDatabase_Validation(t *testing.T) {
	fake := newFakeGoogle()
	r := newTestRepo(t, fake, Config{})
	big := `{"s":"` + strings.Repeat("a", maxCellRunes) + `"}`
	tests := []struct {
		name     string
		snapshot Snapshot
		want     error
	}{
		{"no database", Snapshot{TakenAt: takenAt, Sheets: []Sheet{{Name: "a"}}}, ErrDatabaseInvalid},
		{"long database", Snapshot{Database: strings.Repeat("d", 81), TakenAt: takenAt, Sheets: []Sheet{{Name: "a"}}}, ErrDatabaseInvalid},
		{"no time", Snapshot{Database: "users", Sheets: []Sheet{{Name: "a"}}}, ErrTakenAtRequired},
		{"no sheets", Snapshot{Database: "users", TakenAt: takenAt}, ErrSheetsRequired},
		{"bad sheet name", Snapshot{Database: "users", TakenAt: takenAt, Sheets: []Sheet{{Name: "a/b"}}}, ErrSheetNameInvalid},
		{"duplicate sheet", Snapshot{Database: "users", TakenAt: takenAt, Sheets: []Sheet{{Name: "Users"}, {Name: "users"}}}, ErrSheetNameDuplicate},
		{"missing key", Snapshot{Database: "users", TakenAt: takenAt, Sheets: []Sheet{{Name: "a", Rows: []Row{{JSON: `{}`}}}}}, ErrRowInvalid},
		{"bad json", Snapshot{Database: "users", TakenAt: takenAt, Sheets: []Sheet{{Name: "a", Rows: []Row{{Key: "k", JSON: `{nope`}}}}}, ErrRowInvalid},
		{"cell too large", Snapshot{Database: "users", TakenAt: takenAt, Sheets: []Sheet{{Name: "a", Rows: []Row{{Key: "k", JSON: big}}}}}, ErrCellTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := r.ReplaceDatabase(context.Background(), tt.snapshot); !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
	if fake.creates != 0 || fake.lists != 0 {
		t.Fatalf("invalid snapshots must not call google: creates=%d lists=%d", fake.creates, fake.lists)
	}
}

func TestReplaceDatabase_ContextAndClose(t *testing.T) {
	fake := newFakeGoogle()
	r := newTestRepo(t, fake, Config{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	snapshot := Snapshot{Database: "users", TakenAt: takenAt, Sheets: []Sheet{{Name: "users"}}}
	if _, err := r.ReplaceDatabase(ctx, snapshot); !errors.Is(err, ErrCtxInvalid) {
		t.Fatalf("cancelled ctx err = %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReplaceDatabase(context.Background(), snapshot); !errors.Is(err, ErrClosed) {
		t.Fatalf("closed err = %v", err)
	}
}

func TestNewRepository_Config(t *testing.T) {
	for _, cfg := range []Config{
		{ClientSecret: "s", RefreshToken: "r", Timeout: time.Second},
		{ClientID: "c", RefreshToken: "r", Timeout: time.Second},
		{ClientID: "c", ClientSecret: "s", Timeout: time.Second},
		{ClientID: "c", ClientSecret: "s", RefreshToken: "r"},
	} {
		if _, err := NewRepository(context.Background(), cfg); !errors.Is(err, ErrConfigInvalid) {
			t.Fatalf("cfg %+v err = %v", cfg, err)
		}
	}
	r, err := NewRepository(context.Background(), Config{ClientID: "c", ClientSecret: "s", RefreshToken: "r", Timeout: time.Second})
	if err != nil || r == nil {
		t.Fatalf("valid config: %v", err)
	}
}

func TestNewRow(t *testing.T) {
	row, err := NewRow("u1", map[string]any{"name": "Asha", "age": 30})
	if err != nil || row.Key != "u1" || !strings.Contains(row.JSON, `"name":"Asha"`) {
		t.Fatalf("row = %+v err = %v", row, err)
	}
	if _, err := NewRow(" ", map[string]any{}); !errors.Is(err, ErrRowInvalid) {
		t.Fatalf("blank key err = %v", err)
	}
}

func TestHelpers(t *testing.T) {
	if got := quoteSheet("it's"); got != "'it''s'" {
		t.Fatalf("quoteSheet = %q", got)
	}
	if got := cellRange("a b", 7); got != "'a b'!A7" {
		t.Fatalf("cellRange = %q", got)
	}
	if got := escapeQuery(`a'b\c`); got != `a\'b\\c` {
		t.Fatalf("escapeQuery = %q", got)
	}
	if title, row := parseRange("'it''s'!A12"); title != "it's" || row != 12 {
		t.Fatalf("parseRange = %q %d", title, row)
	}
}
