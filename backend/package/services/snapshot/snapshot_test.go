package snapshotService

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	dumpRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/dump"
	spreadsheetRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/spreadsheet"
)

type fakeSheets struct {
	mu      sync.Mutex
	written []spreadsheetRepository.Snapshot
	fail    map[string]error
	block   chan struct{}
	entered chan struct{}

	disconnected bool
}

func (f *fakeSheets) ReplaceDatabase(_ context.Context, snapshot spreadsheetRepository.Snapshot) (spreadsheetRepository.Result, error) {
	if f.entered != nil {
		f.entered <- struct{}{}
	}
	if f.block != nil {
		<-f.block
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail[snapshot.Database]; err != nil {
		return spreadsheetRepository.Result{}, err
	}
	f.written = append(f.written, snapshot)
	rows := 0
	for _, sheet := range snapshot.Sheets {
		rows += len(sheet.Rows)
	}
	return spreadsheetRepository.Result{SpreadsheetID: "id-" + snapshot.Database, URL: "https://sheet/" + snapshot.Database, Sheets: len(snapshot.Sheets), Rows: rows}, nil
}

func (f *fakeSheets) Close() error { return nil }

func (f *fakeSheets) Spreadsheets(context.Context) (spreadsheetRepository.Repository, error) {
	if f.disconnected {
		return nil, errNotConnected
	}
	return f, nil
}

var errNotConnected = errors.New("not connected")

func source(name string, tables []dumpRepository.Table, err error) Source {
	return Source{Database: name, Dump: func(context.Context) ([]dumpRepository.Table, error) { return tables, err }}
}

var fixed = time.Date(2026, 11, 28, 10, 0, 0, 0, time.UTC)

func newTestService(t *testing.T, sheets *fakeSheets, sources ...Source) *service {
	t.Helper()
	svc, err := NewService(Params{Spreadsheets: sheets, Sources: sources})
	if err != nil {
		t.Fatal(err)
	}
	s := svc.(*service)
	s.now = func() (time.Time, error) { return fixed, nil }
	return s
}

func TestRun_WritesEveryDatabase(t *testing.T) {
	sheets := &fakeSheets{}
	s := newTestService(t, sheets,
		source("users", []dumpRepository.Table{{Name: "users", Rows: []dumpRepository.Record{{Key: "u1", JSON: `{}`}, {Key: "u2", JSON: `{}`}}}, {Name: "users_by_email"}}, nil),
		source("carts", nil, nil),
	)
	result, err := s.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Databases) != 2 || result.Failed != 0 || !result.StartedAt.Equal(fixed) {
		t.Fatalf("result = %+v", result)
	}
	users := result.Databases[0]
	if users.Database != "users" || users.URL != "https://sheet/users" || users.Sheets != 2 || users.Rows != 2 {
		t.Fatalf("users = %+v", users)
	}
	if len(sheets.written) != 2 || !sheets.written[0].TakenAt.Equal(fixed) {
		t.Fatalf("written = %+v", sheets.written)
	}
	if carts := sheets.written[1]; len(carts.Sheets) != 1 || carts.Sheets[0].Name != emptySheetName {
		t.Fatalf("empty database should get one placeholder tab: %+v", carts)
	}
	status := s.Status()
	if status.Running || status.Last == nil || len(status.Last.Databases) != 2 {
		t.Fatalf("status = %+v", status)
	}
}

func TestRun_OneFailureDoesNotStopTheRest(t *testing.T) {
	boom := errors.New("google said no")
	sheets := &fakeSheets{fail: map[string]error{"products": boom}}
	s := newTestService(t, sheets,
		source("users", []dumpRepository.Table{{Name: "users"}}, nil),
		source("images", nil, errors.New("disk read failed")),
		source("products", []dumpRepository.Table{{Name: "products"}}, nil),
		source("orders", []dumpRepository.Table{{Name: "orders"}}, nil),
	)
	result, err := s.Run(context.Background())
	if !errors.Is(err, ErrDatabasesFailed) || !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if result.Failed != 2 || len(result.Databases) != 4 {
		t.Fatalf("result = %+v", result)
	}
	if result.Databases[1].Error != "disk read failed" || result.Databases[2].Error != "google said no" || result.Databases[3].URL == "" {
		t.Fatalf("databases = %+v", result.Databases)
	}
	if len(sheets.written) != 2 {
		t.Fatalf("healthy databases written = %d", len(sheets.written))
	}
}

func TestRun_RejectsConcurrentRuns(t *testing.T) {
	sheets := &fakeSheets{block: make(chan struct{}), entered: make(chan struct{}, 1)}
	s := newTestService(t, sheets, source("users", []dumpRepository.Table{{Name: "users"}}, nil))
	done := make(chan error, 1)
	go func() {
		_, err := s.Run(context.Background())
		done <- err
	}()
	<-sheets.entered
	if !s.Status().Running {
		t.Fatal("status should report running")
	}
	if _, err := s.Run(context.Background()); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second run err = %v", err)
	}
	close(sheets.block)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if s.Status().Running {
		t.Fatal("status should be idle after run")
	}
}

func TestRun_NotConnected(t *testing.T) {
	sheets := &fakeSheets{disconnected: true}
	s := newTestService(t, sheets, source("users", []dumpRepository.Table{{Name: "users"}}, nil))
	if _, err := s.Run(context.Background()); !errors.Is(err, errNotConnected) {
		t.Fatalf("err = %v", err)
	}
	if status := s.Status(); status.Running || status.Last != nil {
		t.Fatalf("status = %+v", status)
	}
}

func TestNewService_Params(t *testing.T) {
	ok := source("users", nil, nil)
	for name, p := range map[string]Params{
		"no repo":       {Sources: []Source{ok}},
		"no sources":    {Spreadsheets: &fakeSheets{}},
		"blank name":    {Spreadsheets: &fakeSheets{}, Sources: []Source{source(" ", nil, nil)}},
		"duplicate":     {Spreadsheets: &fakeSheets{}, Sources: []Source{ok, ok}},
		"nil dump func": {Spreadsheets: &fakeSheets{}, Sources: []Source{{Database: "x"}}},
	} {
		if _, err := NewService(p); !errors.Is(err, ErrParamsInvalid) {
			t.Fatalf("%s: err = %v", name, err)
		}
	}
	s := newTestService(t, &fakeSheets{}, ok)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Run(ctx); !errors.Is(err, ErrCtxInvalid) {
		t.Fatalf("cancelled ctx err = %v", err)
	}
}
