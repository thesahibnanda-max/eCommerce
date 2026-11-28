package snapshotService

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	spreadsheetRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/spreadsheet"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

type Service interface {
	Run(ctx context.Context) (Result, error)
	Status() Status
}

type SpreadsheetsProvider interface {
	Spreadsheets(ctx context.Context) (spreadsheetRepository.Repository, error)
}

type Params struct {
	Spreadsheets SpreadsheetsProvider
	Sources      []Source
	Logger       *slog.Logger
}

type service struct {
	spreadsheets SpreadsheetsProvider
	sources      []Source
	logger       *slog.Logger
	now          func() (time.Time, error)

	run     sync.Mutex
	mu      sync.RWMutex
	running bool
	last    *Result
}

func NewService(p Params) (Service, error) {
	if p.Spreadsheets == nil || len(p.Sources) == 0 {
		return nil, ErrParamsInvalid
	}
	seen := make(map[string]struct{}, len(p.Sources))
	sources := make([]Source, 0, len(p.Sources))
	for _, source := range p.Sources {
		name := strings.TrimSpace(source.Database)
		if _, dup := seen[name]; name == "" || dup || source.Dump == nil {
			return nil, ErrParamsInvalid
		}
		seen[name] = struct{}{}
		sources = append(sources, Source{Database: name, Dump: source.Dump})
	}
	logger := p.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &service{spreadsheets: p.Spreadsheets, sources: sources, logger: logger, now: utils.TimeNow}, nil
}

func (s *service) Run(ctx context.Context) (result Result, err error) {
	defer recoverAsError(&err)
	if err := utils.CheckContext(ctx); err != nil {
		return Result{}, err
	}
	if !s.run.TryLock() {
		return Result{}, ErrAlreadyRunning
	}
	defer s.run.Unlock()
	sheets, err := s.spreadsheets.Spreadsheets(ctx)
	if err != nil {
		return Result{}, err
	}
	s.setRunning(true)
	defer s.setRunning(false)

	startedAt, err := s.now()
	if err != nil {
		return Result{}, err
	}
	result = Result{StartedAt: startedAt, Databases: make([]DatabaseResult, 0, len(s.sources))}
	var failures []error
	for _, source := range s.sources {
		outcome, err := s.snapshot(ctx, sheets, source, startedAt)
		if err != nil {
			outcome.Error = err.Error()
			result.Failed++
			failures = append(failures, fmt.Errorf("%s: %w", source.Database, err))
			s.logger.Error("snapshot failed", "database", source.Database, "error", err.Error())
		}
		result.Databases = append(result.Databases, outcome)
	}
	if result.FinishedAt, err = s.now(); err != nil {
		result.FinishedAt = startedAt
	}
	s.mu.Lock()
	last := result
	s.last = &last
	s.mu.Unlock()
	if len(failures) > 0 {
		return result, fmt.Errorf("%w: %w", ErrDatabasesFailed, errors.Join(failures...))
	}
	return result, nil
}

func (s *service) snapshot(ctx context.Context, spreadsheets spreadsheetRepository.Repository, source Source, takenAt time.Time) (DatabaseResult, error) {
	outcome := DatabaseResult{Database: source.Database}
	tables, err := source.Dump(ctx)
	if err != nil {
		return outcome, err
	}
	sheets := make([]spreadsheetRepository.Sheet, 0, len(tables))
	for _, table := range tables {
		rows := make([]spreadsheetRepository.Row, 0, len(table.Rows))
		for _, record := range table.Rows {
			rows = append(rows, spreadsheetRepository.Row{Key: record.Key, JSON: record.JSON})
		}
		sheets = append(sheets, spreadsheetRepository.Sheet{Name: table.Name, Rows: rows})
	}
	if len(sheets) == 0 {
		sheets = append(sheets, spreadsheetRepository.Sheet{Name: emptySheetName})
	}
	written, err := spreadsheets.ReplaceDatabase(ctx, spreadsheetRepository.Snapshot{Database: source.Database, TakenAt: takenAt, Sheets: sheets})
	if err != nil {
		return outcome, err
	}
	outcome.URL, outcome.Sheets, outcome.Rows = written.URL, written.Sheets, written.Rows
	return outcome, nil
}

func (s *service) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	status := Status{Running: s.running}
	if s.last != nil {
		last := *s.last
		last.Databases = append([]DatabaseResult(nil), s.last.Databases...)
		status.Last = &last
	}
	return status
}

func (s *service) setRunning(running bool) {
	s.mu.Lock()
	s.running = running
	s.mu.Unlock()
}
