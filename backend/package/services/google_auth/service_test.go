package googleAuthService

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	credentialsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/credentials"
	spreadsheetRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/spreadsheet"
	"golang.org/x/oauth2"
)

var bg = context.Background()

type fakeSheets struct {
	token  string
	mu     sync.Mutex
	closed bool
}

func (f *fakeSheets) ReplaceDatabase(context.Context, spreadsheetRepository.Snapshot) (spreadsheetRepository.Result, error) {
	return spreadsheetRepository.Result{}, nil
}

func (f *fakeSheets) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func (f *fakeSheets) isClosed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closed
}

type fakeGoogle struct {
	server        *httptest.Server
	mu            sync.Mutex
	verifier      string
	redirect      string
	revoked       []string
	noRefresh     bool
	failExchange  bool
	tokenRequests int
}

func newGoogle(t *testing.T) *fakeGoogle {
	g := &fakeGoogle{}
	g.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		g.mu.Lock()
		defer g.mu.Unlock()
		switch r.URL.Path {
		case "/token":
			g.tokenRequests++
			g.verifier, g.redirect = r.Form.Get("code_verifier"), r.Form.Get("redirect_uri")
			if g.failExchange || r.Form.Get("code") != "good-code" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
				return
			}
			claims := base64.RawURLEncoding.EncodeToString([]byte(`{"email":"owner@example.com"}`))
			refresh := `"refresh_token":"refresh-1",`
			if g.noRefresh {
				refresh = ""
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"access_token":"access-1",%s"token_type":"Bearer","expires_in":3600,"id_token":"h.%s.s"}`, refresh, claims)
		case "/revoke":
			g.revoked = append(g.revoked, r.Form.Get("token"))
		}
	}))
	t.Cleanup(g.server.Close)
	return g
}

func newTestService(t *testing.T, g *fakeGoogle, creds credentialsRepository.Repository) (*service, *[]*fakeSheets) {
	t.Helper()
	s, err := newService(Params{ClientID: "id", ClientSecret: "secret", Timeout: 5 * time.Second, Credentials: creds})
	if err != nil {
		t.Fatal(err)
	}
	built := &[]*fakeSheets{}
	s.endpoint = oauth2.Endpoint{AuthURL: g.server.URL + "/auth", TokenURL: g.server.URL + "/token", AuthStyle: oauth2.AuthStyleInParams}
	s.revokeURL = g.server.URL + "/revoke"
	s.newSpreadsheets = func(_ context.Context, cfg spreadsheetRepository.Config) (spreadsheetRepository.Repository, error) {
		f := &fakeSheets{token: cfg.RefreshToken}
		*built = append(*built, f)
		return f, nil
	}
	if err := s.restore(bg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, built
}

func openCreds(t *testing.T, dir string) credentialsRepository.Repository {
	t.Helper()
	r, err := credentialsRepository.NewRepository([]string{dir, "credentials.db"}, 0o600, "admin-key")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}

func startFlow(t *testing.T, s *service) url.Values {
	t.Helper()
	raw, err := s.AuthURL(bg, AuthRequest{CallbackURL: "https://shop.example/v1/admin/google/callback", ReturnURL: "https://shop.example/admin?tab=backups"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return parsed.Query()
}

func TestConnectFlow(t *testing.T) {
	g := newGoogle(t)
	creds := openCreds(t, t.TempDir())
	s, built := newTestService(t, g, creds)
	if _, err := s.Spreadsheets(bg); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("before connect err = %v", err)
	}
	q := startFlow(t, s)
	if q.Get("access_type") != "offline" || q.Get("prompt") != "consent" || q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" {
		t.Fatalf("auth url params = %v", q)
	}
	if !strings.Contains(q.Get("scope"), "drive.file") || q.Get("redirect_uri") != "https://shop.example/v1/admin/google/callback" {
		t.Fatalf("scope/redirect = %v", q)
	}
	returnURL, err := s.Callback(bg, CallbackRequest{State: q.Get("state"), Code: "good-code"})
	if err != nil {
		t.Fatal(err)
	}
	if returnURL != "https://shop.example/admin?tab=backups" {
		t.Fatalf("return = %q", returnURL)
	}
	if g.verifier == "" || g.redirect != "https://shop.example/v1/admin/google/callback" {
		t.Fatalf("exchange verifier=%q redirect=%q", g.verifier, g.redirect)
	}
	status := s.Status()
	if !status.Connected || status.Account != "owner@example.com" || status.ConnectedAt == nil {
		t.Fatalf("status = %+v", status)
	}
	sheets, err := s.Spreadsheets(bg)
	if err != nil || sheets.(*fakeSheets).token != "refresh-1" {
		t.Fatalf("sheets = %v %v", sheets, err)
	}
	stored, err := creds.Get(bg, credentialName)
	if err != nil || stored.Secret != "refresh-1" || stored.Account != "owner@example.com" {
		t.Fatalf("stored = %+v %v", stored, err)
	}
	if _, err := s.Callback(bg, CallbackRequest{State: q.Get("state"), Code: "good-code"}); !errors.Is(err, ErrStateInvalid) {
		t.Fatalf("reused state err = %v", err)
	}
	if len(*built) != 1 {
		t.Fatalf("built = %d", len(*built))
	}
}

func TestRestoreAfterRestartAndDisconnect(t *testing.T) {
	g := newGoogle(t)
	dir := t.TempDir()
	creds := openCreds(t, dir)
	s, _ := newTestService(t, g, creds)
	q := startFlow(t, s)
	if _, err := s.Callback(bg, CallbackRequest{State: q.Get("state"), Code: "good-code"}); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()

	restarted, built := newTestService(t, g, creds)
	if !restarted.Status().Connected || len(*built) != 1 || (*built)[0].token != "refresh-1" {
		t.Fatalf("restore status=%+v built=%d", restarted.Status(), len(*built))
	}
	if err := restarted.Disconnect(bg); err != nil {
		t.Fatal(err)
	}
	if restarted.Status().Connected || !(*built)[0].isClosed() {
		t.Fatal("disconnect should drop and close the connection")
	}
	if len(g.revoked) != 1 || g.revoked[0] != "refresh-1" {
		t.Fatalf("revoked = %v", g.revoked)
	}
	if _, err := creds.Get(bg, credentialName); !errors.Is(err, credentialsRepository.ErrNotFound) {
		t.Fatalf("credential should be deleted: %v", err)
	}
	if err := restarted.Disconnect(bg); err != nil {
		t.Fatalf("second disconnect err = %v", err)
	}
}

func TestReconnectReplacesOldConnection(t *testing.T) {
	g := newGoogle(t)
	s, built := newTestService(t, g, openCreds(t, t.TempDir()))
	for range 2 {
		q := startFlow(t, s)
		if _, err := s.Callback(bg, CallbackRequest{State: q.Get("state"), Code: "good-code"}); err != nil {
			t.Fatal(err)
		}
	}
	if len(*built) != 2 || !(*built)[0].isClosed() || (*built)[1].isClosed() {
		t.Fatal("old connection should be closed, new one kept")
	}
}

func TestCallbackFailures(t *testing.T) {
	g := newGoogle(t)
	s, _ := newTestService(t, g, openCreds(t, t.TempDir()))

	if _, err := s.Callback(bg, CallbackRequest{State: "unknown", Code: "good-code"}); !errors.Is(err, ErrStateInvalid) {
		t.Fatalf("unknown state err = %v", err)
	}
	q := startFlow(t, s)
	if back, err := s.Callback(bg, CallbackRequest{State: q.Get("state"), Error: "access_denied"}); !errors.Is(err, ErrConsentDenied) || back == "" {
		t.Fatalf("denied = %q %v", back, err)
	}
	q = startFlow(t, s)
	if _, err := s.Callback(bg, CallbackRequest{State: q.Get("state"), Code: "bad-code"}); !errors.Is(err, ErrExchangeFailed) {
		t.Fatalf("bad code err = %v", err)
	}
	g.noRefresh = true
	q = startFlow(t, s)
	if _, err := s.Callback(bg, CallbackRequest{State: q.Get("state"), Code: "good-code"}); !errors.Is(err, ErrNoRefreshToken) {
		t.Fatalf("no refresh err = %v", err)
	}
	if s.Status().Connected {
		t.Fatal("failed callbacks must not connect")
	}

	q = startFlow(t, s)
	s.now = func() (time.Time, error) { return time.Now().Add(stateTTL + time.Minute), nil }
	if _, err := s.Callback(bg, CallbackRequest{State: q.Get("state"), Code: "good-code"}); !errors.Is(err, ErrStateInvalid) {
		t.Fatalf("expired state err = %v", err)
	}
}

func TestAuthURLValidation(t *testing.T) {
	g := newGoogle(t)
	s, _ := newTestService(t, g, openCreds(t, t.TempDir()))
	for _, req := range []AuthRequest{
		{CallbackURL: "/relative", ReturnURL: "https://a.example/admin"},
		{CallbackURL: "https://a.example/cb", ReturnURL: "javascript:alert(1)"},
		{CallbackURL: "ftp://a.example/cb", ReturnURL: "https://a.example/admin"},
	} {
		if _, err := s.AuthURL(bg, req); !errors.Is(err, ErrURLInvalid) {
			t.Fatalf("%+v err = %v", req, err)
		}
	}
	for range maxPending + 5 {
		startFlow(t, s)
	}
	if len(s.pending) > maxPending {
		t.Fatalf("pending = %d", len(s.pending))
	}
	if _, err := newService(Params{ClientID: "id", Timeout: time.Second}); !errors.Is(err, ErrParamsInvalid) {
		t.Fatalf("params err = %v", err)
	}
}

func TestUnreadableCredentialStartsDisconnected(t *testing.T) {
	g := newGoogle(t)
	dir := t.TempDir()
	old, err := credentialsRepository.NewRepository([]string{dir, "credentials.db"}, 0o600, "old-key")
	if err != nil {
		t.Fatal(err)
	}
	if err := old.Put(bg, credentialsRepository.Model{Name: credentialName, Secret: "x"}); err != nil {
		t.Fatal(err)
	}
	_ = old.Close()
	s, _ := newTestService(t, g, openCreds(t, dir))
	if s.Status().Connected {
		t.Fatal("unreadable credential should start disconnected")
	}
}
