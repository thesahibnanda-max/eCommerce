package googleAuthService

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	credentialsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/credentials"
	spreadsheetRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/spreadsheet"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	credentialName = "google"
	revokeURL      = "https://oauth2.googleapis.com/revoke"
	stateTTL       = 10 * time.Minute
	maxPending     = 16
)

type Service interface {
	Status() Status
	AuthURL(ctx context.Context, request AuthRequest) (string, error)
	Callback(ctx context.Context, request CallbackRequest) (string, error)
	Disconnect(ctx context.Context) error
	Spreadsheets(ctx context.Context) (spreadsheetRepository.Repository, error)
	Close() error
}

type Params struct {
	ClientID     string
	ClientSecret string
	FolderID     string
	CompanyName  string
	Timeout      time.Duration
	Credentials  credentialsRepository.Repository
	Logger       *slog.Logger
}

type connection struct {
	sheets      spreadsheetRepository.Repository
	token       string
	account     string
	connectedAt time.Time
}

type pending struct {
	verifier    string
	callbackURL string
	returnURL   string
	expires     time.Time
}

type service struct {
	clientID        string
	clientSecret    string
	folderID        string
	companyName     string
	timeout         time.Duration
	credentials     credentialsRepository.Repository
	logger          *slog.Logger
	endpoint        oauth2.Endpoint
	revokeURL       string
	httpClient      *http.Client
	now             func() (time.Time, error)
	newSpreadsheets func(ctx context.Context, cfg spreadsheetRepository.Config) (spreadsheetRepository.Repository, error)

	mu      sync.RWMutex
	current *connection
	pending map[string]pending
	closed  bool
}

func NewService(ctx context.Context, p Params) (Service, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return nil, err
	}
	s, err := newService(p)
	if err != nil {
		return nil, err
	}
	if err := s.restore(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func newService(p Params) (*service, error) {
	if strings.TrimSpace(p.ClientID) == "" || strings.TrimSpace(p.ClientSecret) == "" || p.Timeout <= 0 || p.Credentials == nil {
		return nil, ErrParamsInvalid
	}
	logger := p.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &service{
		clientID:        strings.TrimSpace(p.ClientID),
		clientSecret:    strings.TrimSpace(p.ClientSecret),
		folderID:        strings.TrimSpace(p.FolderID),
		companyName:     p.CompanyName,
		timeout:         p.Timeout,
		credentials:     p.Credentials,
		logger:          logger,
		endpoint:        google.Endpoint,
		revokeURL:       revokeURL,
		httpClient:      &http.Client{Timeout: p.Timeout},
		now:             utils.TimeNow,
		newSpreadsheets: spreadsheetRepository.NewRepository,
		pending:         make(map[string]pending),
	}, nil
}

func (s *service) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.current == nil {
		return Status{}
	}
	at := s.current.connectedAt
	return Status{Connected: true, Account: s.current.account, ConnectedAt: &at}
}

func (s *service) AuthURL(ctx context.Context, request AuthRequest) (string, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return "", err
	}
	if !absoluteHTTPURL(request.CallbackURL) || !absoluteHTTPURL(request.ReturnURL) {
		return "", ErrURLInvalid
	}
	now, err := s.now()
	if err != nil {
		return "", err
	}
	state, err := randomState()
	if err != nil {
		return "", err
	}
	verifier := oauth2.GenerateVerifier()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return "", ErrClosed
	}
	s.prunePending(now)
	s.pending[state] = pending{verifier: verifier, callbackURL: request.CallbackURL, returnURL: request.ReturnURL, expires: now.Add(stateTTL)}
	return s.oauthConfig(request.CallbackURL).AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce, oauth2.S256ChallengeOption(verifier)), nil
}

func (s *service) Callback(ctx context.Context, request CallbackRequest) (returnURL string, err error) {
	defer recoverAsError(&err)
	if err := utils.CheckContext(ctx); err != nil {
		return "", err
	}
	now, err := s.now()
	if err != nil {
		return "", err
	}
	flow, ok := s.takePending(strings.TrimSpace(request.State), now)
	if !ok {
		return "", ErrStateInvalid
	}
	if strings.TrimSpace(request.Error) != "" {
		return flow.returnURL, ErrConsentDenied
	}
	if strings.TrimSpace(request.Code) == "" {
		return flow.returnURL, ErrExchangeFailed
	}
	exchangeCtx, cancel := context.WithTimeout(context.WithValue(context.WithoutCancel(ctx), oauth2.HTTPClient, s.httpClient), s.timeout)
	defer cancel()
	token, err := s.oauthConfig(flow.callbackURL).Exchange(exchangeCtx, strings.TrimSpace(request.Code), oauth2.VerifierOption(flow.verifier))
	if err != nil {
		return flow.returnURL, fmt.Errorf("%w: %w", ErrExchangeFailed, err)
	}
	if token.RefreshToken == "" {
		return flow.returnURL, ErrNoRefreshToken
	}
	conn, err := s.connect(ctx, token.RefreshToken, accountFromIDToken(token.Extra("id_token")), now)
	if err != nil {
		return flow.returnURL, err
	}
	if err := s.credentials.Put(ctx, credentialsRepository.Model{Name: credentialName, Secret: conn.token, Account: conn.account, CreatedAt: conn.connectedAt}); err != nil {
		_ = conn.sheets.Close()
		return flow.returnURL, err
	}
	if err := s.swap(conn); err != nil {
		return flow.returnURL, err
	}
	s.logger.Info("google drive connected", "account", conn.account)
	return flow.returnURL, nil
}

func (s *service) Disconnect(ctx context.Context) error {
	if err := utils.CheckContext(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return ErrClosed
	}
	old := s.current
	if err := s.credentials.Delete(ctx, credentialName); err != nil {
		s.mu.Unlock()
		return err
	}
	s.current = nil
	s.mu.Unlock()
	if old == nil {
		return nil
	}
	_ = old.sheets.Close()
	if err := s.revoke(ctx, old.token); err != nil {
		s.logger.Warn("google token revoke failed, it was deleted locally", "error", err.Error())
	}
	s.logger.Info("google drive disconnected", "account", old.account)
	return nil
}

func (s *service) Spreadsheets(ctx context.Context) (spreadsheetRepository.Repository, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, ErrClosed
	}
	if s.current == nil {
		return nil, ErrNotConnected
	}
	return s.current.sheets, nil
}

func (s *service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	if s.current == nil {
		return nil
	}
	err := s.current.sheets.Close()
	s.current = nil
	return err
}

func (s *service) restore(ctx context.Context) error {
	stored, err := s.credentials.Get(ctx, credentialName)
	if errors.Is(err, credentialsRepository.ErrNotFound) {
		return nil
	}
	if errors.Is(err, credentialsRepository.ErrDecryptFailed) || errors.Is(err, credentialsRepository.ErrRecordCorrupted) {
		s.logger.Warn("stored google credential is unreadable, connect google drive again from admin", "error", err.Error())
		return nil
	}
	if err != nil {
		return err
	}
	conn, err := s.connect(ctx, stored.Secret, stored.Account, stored.CreatedAt)
	if err != nil {
		return err
	}
	s.current = conn
	return nil
}
