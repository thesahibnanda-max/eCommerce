package handler_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	credentialsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/credentials"
	usersRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/users"
	adminOperationsService "github.com/thesahibnanda-max/eCommerce/backend/package/services/admin_operations"
	googleAuthService "github.com/thesahibnanda-max/eCommerce/backend/package/services/google_auth"
	snapshotService "github.com/thesahibnanda-max/eCommerce/backend/package/services/snapshot"
)

func newGoogleServices(t *testing.T, dir string, users usersRepository.Repository) (googleAuthService.Service, snapshotService.Service) {
	t.Helper()
	creds, err := credentialsRepository.NewRepository([]string{dir, "credentials.db"}, 0o600, testAdminKey)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = creds.Close() })
	google, err := googleAuthService.NewService(context.Background(), googleAuthService.Params{ClientID: "client-id", ClientSecret: "client-secret", Timeout: time.Second, Credentials: creds})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = google.Close() })
	snapshots, err := snapshotService.NewService(snapshotService.Params{Spreadsheets: google, Sources: []snapshotService.Source{{Database: "users", Dump: users.Dump}}})
	if err != nil {
		t.Fatal(err)
	}
	return google, snapshots
}

func connectGoogle(t *testing.T, f fixture) url.Values {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "http://shop.example/v1/admin/google/connect", strings.NewReader(`{"return_url":"https://shop.example/admin?tab=backups"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Key", testAdminKey)
	req.Header.Set("X-Forwarded-Proto", "https")
	resp, err := f.app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("connect status = %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	body := decode[adminOperationsService.ConnectGoogleResponse](t, response{body: raw})
	parsed, err := url.Parse(body.AuthURL)
	if err != nil || parsed.Host != "accounts.google.com" {
		t.Fatalf("auth url = %q", body.AuthURL)
	}
	return parsed.Query()
}

func callback(t *testing.T, f fixture, query string) *http.Response {
	t.Helper()
	resp, err := f.app.Test(httptest.NewRequest(http.MethodGet, "http://shop.example/v1/admin/google/callback?"+query, nil), fiber.TestConfig{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func TestGoogleConnect_BuildsCallbackFromRequestOrigin(t *testing.T) {
	f := newGoogleFixture(t)
	q := connectGoogle(t, f)
	if q.Get("redirect_uri") != "https://shop.example/v1/admin/google/callback" || q.Get("state") == "" || q.Get("code_challenge") == "" {
		t.Fatalf("auth query = %v", q)
	}
	status := decode[adminOperationsService.SnapshotResponse](t, f.do(t, request{method: http.MethodGet, path: "/v1/admin/snapshot", adminKey: testAdminKey}))
	if !status.Configured || status.Google == nil || status.Google.Connected {
		t.Fatalf("status = %+v", status)
	}
	expectError(t, f.do(t, request{method: http.MethodPost, path: "/v1/admin/snapshot", adminKey: testAdminKey}), http.StatusConflict, "google_not_connected")
	expectError(t, f.do(t, request{method: http.MethodPost, path: "/v1/admin/google/connect", adminKey: "wrong", body: `{"return_url":"https://shop.example/admin"}`}), http.StatusUnauthorized, "admin_unauthorized")
	expectError(t, f.do(t, request{method: http.MethodPost, path: "/v1/admin/google/connect", adminKey: testAdminKey, body: `{"return_url":"javascript:alert(1)"}`}), http.StatusBadRequest, "google_url_invalid")
}

func TestGoogleCallback_RedirectsBackWithOutcome(t *testing.T) {
	f := newGoogleFixture(t)
	q := connectGoogle(t, f)
	resp := callback(t, f, url.Values{"state": {q.Get("state")}, "error": {"access_denied"}}.Encode())
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	location, err := url.Parse(resp.Header.Get("Location"))
	if err != nil || location.Host != "shop.example" || location.Path != "/admin" {
		t.Fatalf("location = %q", resp.Header.Get("Location"))
	}
	back := location.Query()
	if back.Get("tab") != "backups" || back.Get("google") != "error" || back.Get("google_reason") != "google_consent_denied" {
		t.Fatalf("redirect query = %v", back)
	}
	if resp.Header.Get("Cache-Control") != "no-store" || resp.Header.Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("headers = %v", resp.Header)
	}
	if again := callback(t, f, url.Values{"state": {q.Get("state")}, "code": {"x"}}.Encode()); again.StatusCode != http.StatusBadRequest {
		t.Fatalf("reused state status = %d", again.StatusCode)
	}
}

func TestGoogle_NotConfigured(t *testing.T) {
	f := newFixture(t)
	expectError(t, f.do(t, request{method: http.MethodPost, path: "/v1/admin/google/connect", adminKey: testAdminKey, body: `{"return_url":"https://shop.example/admin"}`}), http.StatusServiceUnavailable, "snapshot_not_configured")
	expectError(t, f.do(t, request{method: http.MethodDelete, path: "/v1/admin/google", adminKey: testAdminKey}), http.StatusServiceUnavailable, "snapshot_not_configured")
	if resp := callback(t, f, "state=x&code=y"); resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("callback status = %d", resp.StatusCode)
	}
}
