package frontendHandler_test

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/gofiber/fiber/v3"
	rateLimiter "github.com/thesahibnanda-max/eCommerce/backend/package/rate_limiter"
	frontendHandler "github.com/thesahibnanda-max/eCommerce/frontend/package/handler"
)

var bigJS = strings.Repeat("console.log('aesthetic');", 100)

func testFiles() fstest.MapFS {
	return fstest.MapFS{
		"index.html":           {Data: []byte("<!doctype html><html><head><title>x</title></head><body><div id=root></div></body></html>")},
		"assets/app-abc123.js": {Data: []byte(bigJS)},
		"assets/tiny.css":      {Data: []byte("a{}")},
		"manifest.webmanifest": {Data: []byte(`{"name":"x"}`)},
		"icon-192.png":         {Data: []byte{0x89, 'P', 'N', 'G'}},
		".hidden":              {Data: []byte("secret")},
	}
}

func newLimiter(t *testing.T, rule rateLimiter.Rule) rateLimiter.Limiter {
	t.Helper()
	l, err := rateLimiter.NewLimiter(rateLimiter.Config{CacheSize: 10, TTL: time.Hour, Rules: map[string]rateLimiter.Rule{"static": rule}})
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func newApp(t *testing.T, rule rateLimiter.Rule) *fiber.App {
	t.Helper()
	h, err := frontendHandler.NewHandler(testFiles(), 3000, newLimiter(t, rule))
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	app := fiber.New()
	app.Get("/", h.ServeStatic)
	app.Get("/*", h.ServeStatic)
	return app
}

func generous() rateLimiter.Rule {
	return rateLimiter.Rule{RequestsPerMinute: 6000, Burst: 1000}
}

func get(t *testing.T, app *fiber.App, method, path string, headers map[string]string) (*http.Response, []byte) {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, body
}

func Test_NewHandler_Errors(t *testing.T) {
	limiter := newLimiter(t, generous())
	other, err := rateLimiter.NewLimiter(rateLimiter.Config{CacheSize: 10, TTL: time.Hour, Rules: map[string]rateLimiter.Rule{"x": generous()}})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		files   fstest.MapFS
		port    uint
		limiter rateLimiter.Limiter
		wantErr error
	}{
		{"nil files", nil, 3000, limiter, frontendHandler.ErrFilesRequired},
		{"zero port", testFiles(), 0, limiter, frontendHandler.ErrAPIPortInvalid},
		{"big port", testFiles(), 70000, limiter, frontendHandler.ErrAPIPortInvalid},
		{"nil limiter", testFiles(), 3000, nil, frontendHandler.ErrRateLimiterRequired},
		{"missing group", testFiles(), 3000, other, frontendHandler.ErrRateLimitGroupMissing},
		{"no index", fstest.MapFS{"a.js": {Data: []byte("x")}}, 3000, limiter, frontendHandler.ErrIndexMissing},
		{"empty index", fstest.MapFS{"index.html": {Data: []byte("  ")}}, 3000, limiter, frontendHandler.ErrIndexMissing},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.files == nil {
				_, err = frontendHandler.NewHandler(nil, tt.port, tt.limiter)
			} else {
				_, err = frontendHandler.NewHandler(tt.files, tt.port, tt.limiter)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func Test_IndexAndSPAFallback(t *testing.T) {
	app := newApp(t, generous())
	for _, path := range []string{"/", "/index.html", "/p/01ABC", "/cart", "/admin"} {
		resp, body := get(t, app, "GET", path, nil)
		if resp.StatusCode != http.StatusOK || !strings.Contains(resp.Header.Get("Content-Type"), "text/html") {
			t.Fatalf("%s = %d %s", path, resp.StatusCode, resp.Header.Get("Content-Type"))
		}
		if !bytes.Contains(body, []byte(`<meta name="ai-api-port" content="3000"></head>`)) {
			t.Fatalf("%s body missing api port meta: %s", path, body)
		}
		if resp.Header.Get("Cache-Control") != "no-cache" {
			t.Fatalf("%s cache = %q", path, resp.Header.Get("Cache-Control"))
		}
	}
}

func Test_Assets(t *testing.T) {
	app := newApp(t, generous())
	resp, body := get(t, app, "GET", "/assets/app-abc123.js", nil)
	if resp.StatusCode != 200 || string(body) != bigJS || resp.Header.Get("Cache-Control") != "public, max-age=31536000, immutable" || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/javascript") {
		t.Fatalf("asset = %d %v", resp.StatusCode, resp.Header)
	}
	if resp.Header.Get("Vary") != "Accept-Encoding" || resp.Header.Get("Content-Encoding") != "" {
		t.Fatalf("plain asset headers = %v", resp.Header)
	}
	resp, body = get(t, app, "GET", "/assets/app-abc123.js", map[string]string{"Accept-Encoding": "br, gzip"})
	if resp.Header.Get("Content-Encoding") != "gzip" {
		t.Fatalf("gzip not used: %v", resp.Header)
	}
	reader, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	unzipped, _ := io.ReadAll(reader)
	if string(unzipped) != bigJS {
		t.Fatalf("gzip body mismatch")
	}
	resp, _ = get(t, app, "GET", "/assets/tiny.css", map[string]string{"Accept-Encoding": "gzip"})
	if resp.Header.Get("Content-Encoding") != "" {
		t.Fatalf("tiny file should not be gzipped")
	}
	resp, _ = get(t, app, "GET", "/manifest.webmanifest", nil)
	if resp.Header.Get("Content-Type") != "application/manifest+json" || resp.Header.Get("Cache-Control") != "no-cache" {
		t.Fatalf("manifest headers = %v", resp.Header)
	}
}

func Test_ETag(t *testing.T) {
	app := newApp(t, generous())
	resp, _ := get(t, app, "GET", "/assets/app-abc123.js", nil)
	etag := resp.Header.Get("ETag")
	if etag == "" {
		t.Fatal("missing etag")
	}
	resp, body := get(t, app, "GET", "/assets/app-abc123.js", map[string]string{"If-None-Match": etag})
	if resp.StatusCode != http.StatusNotModified || len(body) != 0 {
		t.Fatalf("revalidate = %d len %d", resp.StatusCode, len(body))
	}
	resp, _ = get(t, app, "GET", "/assets/app-abc123.js", map[string]string{"If-None-Match": `"other"`})
	if resp.StatusCode != 200 {
		t.Fatalf("mismatched etag = %d", resp.StatusCode)
	}
}

func Test_NotFoundAndBadPaths(t *testing.T) {
	app := newApp(t, generous())
	for _, path := range []string{"/missing.js", "/.hidden", "/assets/nope.css"} {
		if resp, _ := get(t, app, "GET", path, nil); resp.StatusCode != http.StatusNotFound {
			t.Fatalf("%s = %d, want 404", path, resp.StatusCode)
		}
	}
	if resp, _ := get(t, app, "GET", "/a\\b", nil); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("backslash = %d, want 400", resp.StatusCode)
	}
}

func Test_HEAD(t *testing.T) {
	app := newApp(t, generous())
	resp, body := get(t, app, "HEAD", "/assets/app-abc123.js", nil)
	if resp.StatusCode != 200 || len(body) != 0 || resp.Header.Get("ETag") == "" {
		t.Fatalf("HEAD = %d len %d", resp.StatusCode, len(body))
	}
}

func Test_SecurityHeaders(t *testing.T) {
	app := newApp(t, generous())
	resp, _ := get(t, app, "GET", "/", nil)
	csp := resp.Header.Get("Content-Security-Policy")
	for _, want := range []string{"script-src 'self'", "connect-src 'self' http://*:3000 https://*:3000", "img-src 'self' data: blob: http://*:3000", "frame-ancestors 'none'"} {
		if !strings.Contains(csp, want) {
			t.Fatalf("csp %q missing %q", csp, want)
		}
	}
	if resp.Header.Get("X-Content-Type-Options") != "nosniff" || resp.Header.Get("X-Frame-Options") != "DENY" || resp.Header.Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("security headers = %v", resp.Header)
	}
}

func Test_RateLimit(t *testing.T) {
	app := newApp(t, rateLimiter.Rule{RequestsPerMinute: 60, Burst: 2})
	get(t, app, "GET", "/", nil)
	get(t, app, "GET", "/", nil)
	resp, body := get(t, app, "GET", "/", nil)
	if resp.StatusCode != http.StatusTooManyRequests || resp.Header.Get("Retry-After") != "1" || string(body) != "Too Many Requests" {
		t.Fatalf("limited = %d %q %s", resp.StatusCode, resp.Header.Get("Retry-After"), body)
	}
}
