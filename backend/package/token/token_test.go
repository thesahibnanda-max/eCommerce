package token

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func mustNow(t *testing.T) time.Time {
	t.Helper()
	now, err := utils.TimeNow()
	if err != nil {
		t.Fatalf("TimeNow: %v", err)
	}
	return now
}

func newTestTokenizer(t *testing.T, now time.Time) *tokenizer {
	t.Helper()
	tk, err := NewTokenizer(testSecret)
	if err != nil {
		t.Fatalf("NewTokenizer: %v", err)
	}
	impl := tk.(*tokenizer)
	impl.now = func() (time.Time, error) { return now, nil }
	return impl
}

func decode(t *testing.T, token string) claims {
	t.Helper()
	parts := strings.Split(token, delimiter)
	if len(parts) != 2 {
		t.Fatalf("token %q has %d parts, want 2", token, len(parts))
	}

	mac := hmac.New(sha256.New, []byte(testSecret))
	mac.Write([]byte(parts[0]))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(parts[1]), []byte(want)) {
		t.Fatalf("signature does not verify with HMAC-SHA256")
	}

	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("payload is not raw URL base64: %v", err)
	}
	c, err := utils.FromStringToObjectJSON[claims](string(raw))
	if err != nil {
		t.Fatalf("payload is not JSON claims: %v", err)
	}
	return c
}

func TestNewTokenizer(t *testing.T) {
	if _, err := NewTokenizer("short"); !errors.Is(err, ErrSecretTooShort) {
		t.Errorf("short secret err = %v, want ErrSecretTooShort", err)
	}
	if _, err := NewTokenizer(""); !errors.Is(err, ErrSecretTooShort) {
		t.Errorf("empty secret err = %v, want ErrSecretTooShort", err)
	}
	if _, err := NewTokenizer(testSecret[:minSecretLength-1]); !errors.Is(err, ErrSecretTooShort) {
		t.Errorf("31-byte secret err = %v, want ErrSecretTooShort", err)
	}

	tk, err := NewTokenizer(testSecret)
	if err != nil {
		t.Fatalf("NewTokenizer: %v", err)
	}
	impl := tk.(*tokenizer)
	if impl.usedRefreshTokens == nil {
		t.Error("usedRefreshTokens map not initialized")
	}
	if string(impl.secret) != testSecret {
		t.Errorf("internal secret = %q, want the bytes of the given string", impl.secret)
	}
}

func TestGenerateNewTokens_ClaimsAndExpiry(t *testing.T) {
	now := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	tk := newTestTokenizer(t, now)

	access, refresh, err := tk.GenerateNewTokens(map[string]any{"user_id": "u1", "role": "admin"})
	if err != nil {
		t.Fatalf("GenerateNewTokens: %v", err)
	}

	a := decode(t, access)
	r := decode(t, refresh)

	if a.Type != tokenTypeAccess || r.Type != tokenTypeRefresh {
		t.Errorf("types = %q / %q, want access / refresh", a.Type, r.Type)
	}
	if a.IssuedAt != now.Unix() || r.IssuedAt != now.Unix() {
		t.Errorf("iat = %d / %d, want %d", a.IssuedAt, r.IssuedAt, now.Unix())
	}
	if got := time.Unix(a.ExpiresAt, 0).Sub(now); got != 15*time.Minute {
		t.Errorf("access lifetime = %v, want 15m", got)
	}
	if got := time.Unix(r.ExpiresAt, 0).Sub(now); got != 24*time.Hour {
		t.Errorf("refresh lifetime = %v, want 24h", got)
	}
	if !utils.ValidateULID(a.ID) || !utils.ValidateULID(r.ID) || a.ID == r.ID {
		t.Errorf("token IDs = %q / %q, want two different ULIDs", a.ID, r.ID)
	}

	for name, c := range map[string]claims{"access": a, "refresh": r} {
		if p := c.Payload; p["user_id"] != "u1" || p["role"] != "admin" {
			t.Errorf("%s payload = %#v, want user_id=u1 role=admin", name, c.Payload)
		}
	}
}

func TestGenerateNewTokens_UniqueEveryCall(t *testing.T) {
	tk := newTestTokenizer(t, mustNow(t))
	payload := map[string]any{"user_id": "u1"}

	seen := make(map[string]struct{})
	for i := 0; i < 100; i++ {
		access, refresh, err := tk.GenerateNewTokens(payload)
		if err != nil {
			t.Fatal(err)
		}
		if access == refresh {
			t.Fatal("access and refresh tokens are identical")
		}
		for _, tok := range []string{access, refresh} {
			if _, dup := seen[tok]; dup {
				t.Fatalf("token repeated for the same payload: %s", tok)
			}
			seen[tok] = struct{}{}
		}
	}
}

func TestGenerateNewTokens_URLSafe(t *testing.T) {
	tk := newTestTokenizer(t, mustNow(t))
	access, refresh, err := tk.GenerateNewTokens(map[string]any{"q": "??>>~~ünïcödé//++=="})
	if err != nil {
		t.Fatal(err)
	}
	for _, tok := range []string{access, refresh} {
		if strings.ContainsAny(tok, "+/= ") {
			t.Errorf("token contains characters that are not URL safe: %s", tok)
		}
		if strings.Count(tok, delimiter) != 1 {
			t.Errorf("token has %d delimiters, want 1", strings.Count(tok, delimiter))
		}
	}
}

func TestGenerateNewTokens_TamperingBreaksSignature(t *testing.T) {
	tk := newTestTokenizer(t, mustNow(t))
	access, _, err := tk.GenerateNewTokens(map[string]any{"user_id": "u1", "role": "user"})
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(access, delimiter)
	forged := strings.Replace(decodeRaw(t, parts[0]), `"role":"user"`, `"role":"admin"`, 1)
	forgedEncoded := base64.RawURLEncoding.EncodeToString([]byte(forged))

	mac := hmac.New(sha256.New, []byte(testSecret))
	mac.Write([]byte(forgedEncoded))
	if hmac.Equal([]byte(parts[1]), []byte(base64.RawURLEncoding.EncodeToString(mac.Sum(nil)))) {
		t.Error("original signature still valid after changing the payload")
	}

	other := newTestTokenizer(t, mustNow(t))
	other.secret = []byte("ffffffffffffffffffffffffffffffff")
	otherAccess, _, _ := other.GenerateNewTokens(map[string]any{"user_id": "u1", "role": "user"})
	if strings.Split(otherAccess, delimiter)[1] == parts[1] {
		t.Error("different secrets produced the same signature")
	}
}

func decodeRaw(t *testing.T, s string) string {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestGenerateNewTokens_Errors(t *testing.T) {
	tk := newTestTokenizer(t, mustNow(t))
	var nilMap map[string]any

	tests := []struct {
		name    string
		payload map[string]any
		wantErr error
	}{
		{"nil", nil, ErrPayloadInvalid},
		{"nil map variable", nilMap, ErrPayloadInvalid},
		{"value cannot encode chan", map[string]any{"c": make(chan int)}, ErrPayloadEncode},
		{"value cannot encode func", map[string]any{"f": func() {}}, ErrPayloadEncode},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			access, refresh, err := tk.GenerateNewTokens(tt.payload)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if access != "" || refresh != "" {
				t.Error("tokens returned alongside an error")
			}
		})
	}
}

func TestGenerateNewTokens_ValidPayloadKinds(t *testing.T) {
	tk := newTestTokenizer(t, mustNow(t))
	for name, payload := range map[string]map[string]any{
		"empty":     {},
		"scalars":   {"user_id": "u1", "age": 30, "admin": true, "score": 9.5},
		"nested":    {"user": map[string]any{"id": "u1", "roles": []string{"admin", "user"}}},
		"nil value": {"deleted_at": nil},
	} {
		if _, _, err := tk.GenerateNewTokens(payload); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestGenerateNewTokens_Concurrent(t *testing.T) {
	tk := newTestTokenizer(t, mustNow(t))
	var wg sync.WaitGroup
	var mu sync.Mutex
	seen := make(map[string]struct{})
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				access, refresh, err := tk.GenerateNewTokens(map[string]any{"user_id": "u1"})
				if err != nil {
					t.Error(err)
					return
				}
				mu.Lock()
				seen[access] = struct{}{}
				seen[refresh] = struct{}{}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if len(seen) != 8*50*2 {
		t.Errorf("unique tokens = %d, want %d", len(seen), 8*50*2)
	}
}

func signRaw(tk *tokenizer, rawJSON string) string {
	encoded := base64.RawURLEncoding.EncodeToString([]byte(rawJSON))
	return encoded + delimiter + base64.RawURLEncoding.EncodeToString(tk.mac(encoded))
}

func TestVerifyAccessToken_RoundTrip(t *testing.T) {
	now := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	tk := newTestTokenizer(t, now)
	payload := map[string]any{
		"user_id": "u1",
		"admin":   true,
		"age":     30,
		"roles":   []string{"admin", "user"},
		"profile": map[string]any{"city": "Delhi"},
	}

	access, _, err := tk.GenerateNewTokens(payload)
	if err != nil {
		t.Fatal(err)
	}
	got, err := tk.VerifyAccessToken(access)
	if err != nil {
		t.Fatalf("VerifyAccessToken: %v", err)
	}
	if got["user_id"] != "u1" || got["admin"] != true || got["age"] != float64(30) {
		t.Errorf("payload = %#v", got)
	}
	if roles, ok := got["roles"].([]any); !ok || len(roles) != 2 || roles[0] != "admin" {
		t.Errorf("roles = %#v", got["roles"])
	}
	if profile, ok := got["profile"].(map[string]any); !ok || profile["city"] != "Delhi" {
		t.Errorf("profile = %#v", got["profile"])
	}

	empty, _, _ := tk.GenerateNewTokens(map[string]any{})
	if got, err := tk.VerifyAccessToken(empty); err != nil || got == nil || len(got) != 0 {
		t.Errorf("empty payload = %#v, %v; want empty non-nil map", got, err)
	}
}

func TestVerifyAccessToken_Expiry(t *testing.T) {
	issued := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	tk := newTestTokenizer(t, issued)
	access, _, err := tk.GenerateNewTokens(map[string]any{"user_id": "u1"})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		at      time.Time
		wantErr error
	}{
		{"right after issue", issued, nil},
		{"one second before expiry", issued.Add(15*time.Minute - time.Second), nil},
		{"exactly at expiry", issued.Add(15 * time.Minute), ErrTokenExpired},
		{"long after expiry", issued.Add(24 * time.Hour), ErrTokenExpired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tk.now = func() (time.Time, error) { return tt.at, nil }
			_, err := tk.VerifyAccessToken(access)
			if !errors.Is(err, tt.wantErr) && !(tt.wantErr == nil && err == nil) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestVerifyAccessToken_RejectsRefreshToken(t *testing.T) {
	tk := newTestTokenizer(t, mustNow(t))
	_, refresh, err := tk.GenerateNewTokens(map[string]any{"user_id": "u1"})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := tk.VerifyAccessToken(refresh); !errors.Is(err, ErrTokenWrongType) || got != nil {
		t.Errorf("refresh token as access = %v, %v; want ErrTokenWrongType", got, err)
	}
}

func TestVerifyAccessToken_RejectsForgedAndMalformed(t *testing.T) {
	now := mustNow(t)
	tk := newTestTokenizer(t, now)
	access, _, err := tk.GenerateNewTokens(map[string]any{"user_id": "u1", "role": "user"})
	if err != nil {
		t.Fatal(err)
	}
	encoded, signature, _ := strings.Cut(access, delimiter)

	forgedPayload := base64.RawURLEncoding.EncodeToString([]byte(strings.Replace(decodeRaw(t, encoded), `"role":"user"`, `"role":"admin"`, 1)))

	other := newTestTokenizer(t, now)
	other.secret = []byte("ffffffffffffffffffffffffffffffff")
	otherAccess, _, _ := other.GenerateNewTokens(map[string]any{"user_id": "u1"})

	flipped := []byte(signature)
	if flipped[0] == 'A' {
		flipped[0] = 'B'
	} else {
		flipped[0] = 'A'
	}

	exp := now.Add(time.Hour).Unix()
	id := utils.NewULID()

	tests := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"no delimiter", encoded + signature},
		{"empty payload part", delimiter + signature},
		{"empty signature part", encoded + delimiter},
		{"extra delimiter", access + delimiter + "x"},
		{"payload changed", forgedPayload + delimiter + signature},
		{"signature changed", encoded + delimiter + string(flipped)},
		{"signature truncated", encoded + delimiter + signature[:len(signature)-4]},
		{"signature not base64", encoded + delimiter + "!!!!"},
		{"signed with another secret", otherAccess},
		{"std base64 padding", encoded + "==" + delimiter + signature},
		{"signed but not json", signRaw(tk, "not json")},
		{"signed but missing id", signRaw(tk, `{"typ":"access","exp":`+itoa(exp)+`,"payload":{}}`)},
		{"signed but bad id", signRaw(tk, `{"jti":"x","typ":"access","exp":`+itoa(exp)+`,"payload":{}}`)},
		{"signed but missing exp", signRaw(tk, `{"jti":"`+id+`","typ":"access","payload":{}}`)},
		{"signed but missing payload", signRaw(tk, `{"jti":"`+id+`","typ":"access","exp":`+itoa(exp)+`}`)},
		{"signed but payload not object", signRaw(tk, `{"jti":"`+id+`","typ":"access","exp":`+itoa(exp)+`,"payload":"u1"}`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tk.VerifyAccessToken(tt.token)
			if !errors.Is(err, ErrTokenInvalid) {
				t.Errorf("err = %v, want ErrTokenInvalid", err)
			}
			if got != nil {
				t.Errorf("payload %v returned for invalid token", got)
			}
		})
	}

	if _, err := tk.VerifyAccessToken(signRaw(tk, `{"jti":"`+id+`","typ":"other","exp":`+itoa(exp)+`,"payload":{}}`)); !errors.Is(err, ErrTokenWrongType) {
		t.Errorf("unknown type err = %v, want ErrTokenWrongType", err)
	}
}

func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}

func TestRefreshAccessToken_RotatesTokens(t *testing.T) {
	now := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	tk := newTestTokenizer(t, now)
	_, refresh, err := tk.GenerateNewTokens(map[string]any{"user_id": "u1", "roles": []string{"admin"}})
	if err != nil {
		t.Fatal(err)
	}

	later := now.Add(20 * time.Hour)
	tk.now = func() (time.Time, error) { return later, nil }

	access, newRefresh, err := tk.RefreshAccessToken(refresh)
	if err != nil {
		t.Fatalf("RefreshAccessToken: %v", err)
	}
	if newRefresh == refresh {
		t.Error("refresh token not rotated")
	}

	payload, err := tk.VerifyAccessToken(access)
	if err != nil {
		t.Fatalf("new access token invalid: %v", err)
	}
	if roles, ok := payload["roles"].([]any); payload["user_id"] != "u1" || !ok || len(roles) != 1 || roles[0] != "admin" {
		t.Errorf("payload not carried over: %#v", payload)
	}

	a, r := decode(t, access), decode(t, newRefresh)
	if a.IssuedAt != later.Unix() || r.IssuedAt != later.Unix() {
		t.Errorf("new tokens issued at %d/%d, want %d", a.IssuedAt, r.IssuedAt, later.Unix())
	}
	if got := time.Unix(r.ExpiresAt, 0).Sub(later); got != 24*time.Hour {
		t.Errorf("new refresh lifetime = %v, want a fresh 24h", got)
	}

	if _, _, err := tk.RefreshAccessToken(newRefresh); err != nil {
		t.Errorf("rotated refresh token rejected: %v", err)
	}
}

func TestRefreshAccessToken_RejectsReuse(t *testing.T) {
	tk := newTestTokenizer(t, mustNow(t))
	_, refresh, err := tk.GenerateNewTokens(map[string]any{"user_id": "u1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := tk.RefreshAccessToken(refresh); err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	access, newRefresh, err := tk.RefreshAccessToken(refresh)
	if !errors.Is(err, ErrTokenReused) {
		t.Fatalf("second use err = %v, want ErrTokenReused", err)
	}
	if access != "" || newRefresh != "" {
		t.Error("tokens returned for a reused refresh token")
	}
}

func TestRefreshAccessToken_Errors(t *testing.T) {
	issued := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	tk := newTestTokenizer(t, issued)
	access, refresh, err := tk.GenerateNewTokens(map[string]any{"user_id": "u1"})
	if err != nil {
		t.Fatal(err)
	}
	other := newTestTokenizer(t, issued)
	other.secret = []byte("ffffffffffffffffffffffffffffffff")
	_, foreign, _ := other.GenerateNewTokens(map[string]any{"user_id": "u1"})

	tests := []struct {
		name    string
		token   string
		at      time.Time
		wantErr error
	}{
		{"access token", access, issued, ErrTokenWrongType},
		{"garbage", "nope", issued, ErrTokenInvalid},
		{"empty", "", issued, ErrTokenInvalid},
		{"other secret", foreign, issued, ErrTokenInvalid},
		{"expired refresh", refresh, issued.Add(24 * time.Hour), ErrTokenExpired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tk.now = func() (time.Time, error) { return tt.at, nil }
			a, r, err := tk.RefreshAccessToken(tt.token)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if a != "" || r != "" {
				t.Error("tokens returned alongside an error")
			}
		})
	}

	tk.now = func() (time.Time, error) { return issued.Add(time.Hour), nil }
	if _, _, err := tk.RefreshAccessToken(refresh); err != nil {
		t.Errorf("failed attempts must not burn a valid refresh token: %v", err)
	}
}

func TestRefreshAccessToken_ConcurrentReuseOnlyOneWins(t *testing.T) {
	tk := newTestTokenizer(t, mustNow(t))
	_, refresh, err := tk.GenerateNewTokens(map[string]any{"user_id": "u1"})
	if err != nil {
		t.Fatal(err)
	}

	const workers = 32
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins, reused := 0, 0
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := tk.RefreshAccessToken(refresh)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				wins++
			case errors.Is(err, ErrTokenReused):
				reused++
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()
	if wins != 1 || reused != workers-1 {
		t.Errorf("wins=%d reused=%d, want exactly 1 win", wins, reused)
	}
}

func TestRefreshAccessToken_CacheHoldsLast100UsedTokens(t *testing.T) {
	tk := newTestTokenizer(t, mustNow(t))
	refreshes := make([]string, usedTokenCache+1)
	for i := range refreshes {
		_, r, err := tk.GenerateNewTokens(map[string]any{"user_id": "u1"})
		if err != nil {
			t.Fatal(err)
		}
		refreshes[i] = r
	}
	for i := 0; i < usedTokenCache; i++ {
		if _, _, err := tk.RefreshAccessToken(refreshes[i]); err != nil {
			t.Fatalf("refresh %d: %v", i, err)
		}
	}
	if _, _, err := tk.RefreshAccessToken(refreshes[0]); !errors.Is(err, ErrTokenReused) {
		t.Fatalf("token within the last %d used must be rejected, err = %v", usedTokenCache, err)
	}

	if _, _, err := tk.RefreshAccessToken(refreshes[usedTokenCache]); err != nil {
		t.Fatal(err)
	}
	if _, _, err := tk.RefreshAccessToken(refreshes[1]); !errors.Is(err, ErrTokenReused) {
		t.Errorf("refreshes[1] is still in the cache and must be rejected, err = %v", err)
	}
	if _, _, err := tk.RefreshAccessToken(refreshes[0]); err != nil {
		t.Errorf("oldest used token should have been evicted and accepted again, err = %v", err)
	}
}
