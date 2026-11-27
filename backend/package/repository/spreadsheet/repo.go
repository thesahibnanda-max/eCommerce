package spreadsheetRepository

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

const (
	defaultCompanyName = "eCommerce"
	titleSuffix        = " snapshot"
	spreadsheetMIME    = "application/vnd.google-apps.spreadsheet"
	spreadsheetURL     = "https://docs.google.com/spreadsheets/d/"
	valueInputRaw      = "RAW"
	headerKey          = "Key"
	headerTakenAt      = "Snapshot time"
	headerJSON         = "JSON"
	columnCount        = 3
	maxCellRunes       = 50000
	maxSheetNameRunes  = 100
	maxDatabaseRunes   = 80
	defaultChunkBytes  = 4 << 20
	maxAttempts        = 5
	firstBackoff       = time.Second
)

type Repository interface {
	ReplaceDatabase(ctx context.Context, snapshot Snapshot) (Result, error)
	Close() error
}

type repository struct {
	sheets      *sheets.Service
	drive       *drive.Service
	folderID    string
	titlePrefix string
	chunkBytes  int
	backoff     time.Duration

	mu     sync.Mutex
	ids    map[string]string
	closed bool
}

func NewRepository(ctx context.Context, cfg Config) (Repository, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.ClientID) == "" || strings.TrimSpace(cfg.ClientSecret) == "" || strings.TrimSpace(cfg.RefreshToken) == "" || cfg.Timeout <= 0 {
		return nil, ErrConfigInvalid
	}
	oauth := &oauth2.Config{
		ClientID:     strings.TrimSpace(cfg.ClientID),
		ClientSecret: strings.TrimSpace(cfg.ClientSecret),
		Endpoint:     google.Endpoint,
		Scopes:       []string{drive.DriveFileScope},
	}
	base := &http.Client{Timeout: cfg.Timeout}
	tokenCtx := context.WithValue(context.WithoutCancel(ctx), oauth2.HTTPClient, base)
	client := oauth2.NewClient(tokenCtx, oauth.TokenSource(tokenCtx, &oauth2.Token{RefreshToken: strings.TrimSpace(cfg.RefreshToken)}))
	client.Timeout = cfg.Timeout
	opts := []option.ClientOption{option.WithHTTPClient(client)}
	return newRepositoryWith(ctx, cfg, opts, opts)
}

func newRepositoryWith(ctx context.Context, cfg Config, sheetsOpts, driveOpts []option.ClientOption) (*repository, error) {
	sheetsService, err := sheets.NewService(ctx, sheetsOpts...)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRequestFailed, err)
	}
	driveService, err := drive.NewService(ctx, driveOpts...)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRequestFailed, err)
	}
	company := strings.TrimSpace(cfg.CompanyName)
	if company == "" {
		company = defaultCompanyName
	}
	return &repository{
		sheets:      sheetsService,
		drive:       driveService,
		folderID:    strings.TrimSpace(cfg.FolderID),
		titlePrefix: company + titleSuffix,
		chunkBytes:  defaultChunkBytes,
		backoff:     firstBackoff,
		ids:         make(map[string]string),
	}, nil
}

func (r *repository) ReplaceDatabase(ctx context.Context, snapshot Snapshot) (result Result, err error) {
	defer recoverAsError(&err)
	if err := utils.CheckContext(ctx); err != nil {
		return Result{}, err
	}
	snapshot, rows, err := normalizeSnapshot(snapshot)
	if err != nil {
		return Result{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return Result{}, ErrClosed
	}
	id, err := r.spreadsheetID(ctx, snapshot.Database)
	if err != nil {
		return Result{}, err
	}
	if err := r.syncTabs(ctx, id, snapshot.Sheets); err != nil {
		return Result{}, err
	}
	if err := r.writeValues(ctx, id, snapshot); err != nil {
		return Result{}, err
	}
	return Result{SpreadsheetID: id, URL: spreadsheetURL + id, Sheets: len(snapshot.Sheets), Rows: rows}, nil
}

func (r *repository) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	return nil
}

func NewRow(key string, value any) (Row, error) {
	if strings.TrimSpace(key) == "" {
		return Row{}, ErrRowInvalid
	}
	raw, err := utils.FromObjectToStringJSON(value)
	if err != nil {
		return Row{}, fmt.Errorf("%w: %w", ErrRowInvalid, err)
	}
	return Row{Key: key, JSON: raw}, nil
}
