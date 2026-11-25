package usersService_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/thesahibnanda-max/eCommerce/backend/package/otp"
	usersRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/users"
	usersService "github.com/thesahibnanda-max/eCommerce/backend/package/services/users"
	"github.com/thesahibnanda-max/eCommerce/backend/package/smtp"
	"github.com/thesahibnanda-max/eCommerce/backend/package/token"
)

const (
	testSecret   = "0123456789abcdef0123456789abcdef"
	testPassword = "Secr3t!pass"
)

var bg = context.Background()

type fakeMailer struct {
	mu   sync.Mutex
	sent []smtp.SendMailRequest
	err  error
}

func (m *fakeMailer) SendMail(_ context.Context, req smtp.SendMailRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.sent = append(m.sent, req)
	return nil
}

func (m *fakeMailer) lastCode(t *testing.T) string {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.sent) == 0 {
		t.Fatal("no otp mail sent")
	}
	code, _, _ := strings.Cut(strings.TrimPrefix(m.sent[len(m.sent)-1].Body, "Your OTP is: "), "\n")
	return code
}

type fixture struct {
	svc       usersService.Service
	repo      usersRepository.Repository
	tokenizer token.Tokenizer
	mailer    *fakeMailer
}

func newFixture(t *testing.T, oneValidationEnough bool) fixture {
	t.Helper()
	return newFixtureWithOTP(t, oneValidationEnough, otp.Config{})
}

func newFixtureWithOTP(t *testing.T, oneValidationEnough bool, otpCfg otp.Config) fixture {
	t.Helper()
	repo, err := usersRepository.NewRepository([]string{t.TempDir(), "users.db"}, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	tk, err := token.NewTokenizer(testSecret)
	if err != nil {
		t.Fatal(err)
	}
	mailer := &fakeMailer{}
	emailOTP, err := otp.NewMailOTP(mailer, otpCfg)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := usersService.NewService(tk, repo, emailOTP, oneValidationEnough)
	if err != nil {
		t.Fatal(err)
	}
	return fixture{svc: svc, repo: repo, tokenizer: tk, mailer: mailer}
}

var phoneSeq atomic.Uint32

func nextPhone() string {
	return fmt.Sprintf("+9198765%05d", phoneSeq.Add(1))
}

func (f fixture) create(t *testing.T, email, phone string) usersService.CreateUserResponse {
	t.Helper()
	resp, err := f.svc.CreateUser(bg, usersService.CreateUserRequest{Name: "A", Email: email, PhoneNumber: phone, Password: testPassword})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return resp
}

func TestNewService(t *testing.T) {
	f := newFixture(t, false)
	var nilRepo usersRepository.Repository
	var nilTokenizer token.Tokenizer

	emailOTP, _ := otp.NewMailOTP(&fakeMailer{}, otp.Config{})
	var nilOTP otp.OTP
	if _, err := usersService.NewService(f.tokenizer, f.repo, nilOTP, false); !errors.Is(err, usersService.ErrEmailOTPRequired) {
		t.Errorf("nil otp err = %v", err)
	}
	if _, err := usersService.NewService(nilTokenizer, f.repo, emailOTP, false); !errors.Is(err, usersService.ErrTokenizerRequired) {
		t.Errorf("nil tokenizer err = %v", err)
	}
	if _, err := usersService.NewService(f.tokenizer, nilRepo, emailOTP, false); !errors.Is(err, usersService.ErrUsersRepoRequired) {
		t.Errorf("nil repo err = %v", err)
	}
}

func TestCreateUser(t *testing.T) {
	f := newFixture(t, false)
	existingPhone := nextPhone()
	resp := f.create(t, "A@X.com", existingPhone)

	if resp.ID == "" || resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Fatalf("incomplete response: %+v", resp)
	}
	payload, err := f.tokenizer.VerifyAccessToken(resp.AccessToken)
	if err != nil {
		t.Fatalf("access token invalid: %v", err)
	}
	if len(payload) != 1 || payload["id"] != resp.ID {
		t.Errorf("token payload = %v, want only {id: %s}", payload, resp.ID)
	}
	if _, _, err := f.tokenizer.RefreshAccessToken(resp.RefreshToken); err != nil {
		t.Errorf("refresh token invalid: %v", err)
	}

	if _, err := f.svc.CreateUser(bg, usersService.CreateUserRequest{Name: "B", Email: "b@x.com", PhoneNumber: nextPhone(), Password: "weak"}); !errors.Is(err, usersRepository.ErrPasswordInvalid) {
		t.Errorf("weak password err = %v", err)
	}

	contactCases := map[string]struct {
		email   string
		phone   string
		wantErr error
	}{
		"missing email":   {"", "+919812345678", usersService.ErrEmailRequired},
		"blank email":     {"   ", "+919812345678", usersService.ErrEmailRequired},
		"missing phone":   {"c@x.com", "", usersService.ErrPhoneNumberRequired},
		"blank phone":     {"c@x.com", " \t", usersService.ErrPhoneNumberRequired},
		"missing both":    {"", "", usersService.ErrEmailRequired},
		"invalid phone":   {"c@x.com", "12345", usersService.ErrPhoneNumberInvalid},
		"invalid email":   {"nope", "+919812345678", usersService.ErrEmailInvalid},
		"duplicate phone": {"d@x.com", "", usersService.ErrPhoneAlreadyExists},
		"duplicate email": {"a@x.com", "+919812340000", usersService.ErrEmailAlreadyExists},
	}
	for name, tc := range contactCases {
		t.Run(name, func(t *testing.T) {
			phone := tc.phone
			if name == "duplicate phone" {
				phone = existingPhone
			}
			_, err := f.svc.CreateUser(bg, usersService.CreateUserRequest{Name: "C", Email: tc.email, PhoneNumber: phone, Password: testPassword})
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
	if _, err := f.svc.CreateUser(nil, usersService.CreateUserRequest{}); !errors.Is(err, usersService.ErrInvalidContext) {
		t.Errorf("nil ctx err = %v", err)
	}
}

func TestGetUserAndVerifyPassword_Success(t *testing.T) {
	f := newFixture(t, false)
	created := f.create(t, "a@x.com", "+919876543210")

	for name, req := range map[string]usersService.GetUserAndVerifyPasswordRequest{
		"by email":                   {Email: " A@X.COM ", Password: testPassword},
		"by phone":                   {PhoneNumber: "98765 43210", Password: testPassword},
		"email wins when both given": {Email: "a@x.com", PhoneNumber: "garbage", Password: testPassword},
	} {
		t.Run(name, func(t *testing.T) {
			resp, err := f.svc.GetUserAndVerifyPassword(bg, req)
			if err != nil {
				t.Fatalf("GetUserAndVerifyPassword: %v", err)
			}
			if resp.ID != created.ID {
				t.Errorf("ID = %q, want %q", resp.ID, created.ID)
			}
			payload, err := f.tokenizer.VerifyAccessToken(resp.AccessToken)
			if err != nil || len(payload) != 1 || payload["id"] != created.ID {
				t.Errorf("access token payload = %v, %v", payload, err)
			}
			if _, _, err := f.tokenizer.RefreshAccessToken(resp.RefreshToken); err != nil {
				t.Errorf("refresh token invalid: %v", err)
			}
		})
	}
}

func TestGetUserAndVerifyPassword_Errors(t *testing.T) {
	f := newFixture(t, false)
	f.create(t, "a@x.com", nextPhone())

	tests := []struct {
		name    string
		ctx     context.Context
		req     usersService.GetUserAndVerifyPasswordRequest
		wantErr error
	}{
		{"nil ctx", nil, usersService.GetUserAndVerifyPasswordRequest{Email: "a@x.com", Password: testPassword}, usersService.ErrInvalidContext},
		{"no contact", bg, usersService.GetUserAndVerifyPasswordRequest{Password: testPassword}, usersService.ErrContactRequired},
		{"blank contact", bg, usersService.GetUserAndVerifyPasswordRequest{Email: "  ", PhoneNumber: " ", Password: testPassword}, usersService.ErrContactRequired},
		{"wrong password", bg, usersService.GetUserAndVerifyPasswordRequest{Email: "a@x.com", Password: "Wrong!pass1"}, usersService.ErrInvalidCredentials},
		{"unknown email", bg, usersService.GetUserAndVerifyPasswordRequest{Email: "b@x.com", Password: testPassword}, usersService.ErrInvalidCredentials},
		{"unknown phone", bg, usersService.GetUserAndVerifyPasswordRequest{PhoneNumber: "+919876543210", Password: testPassword}, usersService.ErrInvalidCredentials},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := f.svc.GetUserAndVerifyPassword(tt.ctx, tt.req)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if resp != (usersService.GetUserAndVerifyPasswordResponse{}) {
				t.Errorf("response returned alongside error: %+v", resp)
			}
		})
	}
}

func TestGetUserAndVerifyPassword_DoesNotRequireValidation(t *testing.T) {
	for _, oneEnough := range []bool{false, true} {
		f := newFixture(t, oneEnough)
		created := f.create(t, "a@x.com", "+919876543210")
		resp, err := f.svc.GetUserAndVerifyPassword(bg, usersService.GetUserAndVerifyPasswordRequest{Email: "a@x.com", Password: testPassword})
		if err != nil || resp.ID != created.ID {
			t.Errorf("oneEnough=%v: unvalidated user login = %q, %v; want allowed", oneEnough, resp.ID, err)
		}
	}
}

func TestVerifyAccessToken_ReturnsCurrentUser(t *testing.T) {
	f := newFixture(t, false)
	created, err := f.svc.CreateUser(bg, usersService.CreateUserRequest{Name: " Sahib ", Email: "A@X.com", PhoneNumber: "98765 43210", Password: testPassword})
	if err != nil {
		t.Fatal(err)
	}

	got, err := f.svc.VerifyAccessToken(bg, usersService.VerifyAccessTokenRequest{AccessToken: "  " + created.AccessToken + "\n"})
	if err != nil {
		t.Fatalf("VerifyAccessToken: %v", err)
	}
	want := usersService.VerifyAccessTokenResponse{ID: created.ID, Name: "Sahib", Email: "a@x.com", PhoneNumber: "+919876543210", IsValidated: false}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}

	if _, err := f.repo.ValidateEmail(bg, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.ValidatePhoneNumber(bg, created.ID); err != nil {
		t.Fatal(err)
	}
	got, err = f.svc.VerifyAccessToken(bg, usersService.VerifyAccessTokenRequest{AccessToken: created.AccessToken})
	if err != nil || !got.IsValidated {
		t.Errorf("token issued before validation must reflect the current state: %+v, %v", got, err)
	}
}

func TestVerifyAccessToken_ValidationFlag(t *testing.T) {
	type validate struct{ email, phone bool }
	tests := []struct {
		name         string
		oneEnough    bool
		email, phone string
		validate     validate
		want         bool
	}{
		{"all required: nothing validated", false, "a@x.com", "+919876543210", validate{}, false},
		{"all required: only email validated", false, "a@x.com", "+919876543210", validate{email: true}, false},
		{"all required: only phone validated", false, "a@x.com", "+919876543210", validate{phone: true}, false},
		{"all required: both validated", false, "a@x.com", "+919876543210", validate{true, true}, true},
		{"one enough: nothing validated", true, "a@x.com", "+919876543210", validate{}, false},
		{"one enough: only email validated", true, "a@x.com", "+919876543210", validate{email: true}, true},
		{"one enough: only phone validated", true, "a@x.com", "+919876543210", validate{phone: true}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, tt.oneEnough)
			created := f.create(t, tt.email, tt.phone)
			if tt.validate.email {
				if _, err := f.repo.ValidateEmail(bg, created.ID); err != nil {
					t.Fatal(err)
				}
			}
			if tt.validate.phone {
				if _, err := f.repo.ValidatePhoneNumber(bg, created.ID); err != nil {
					t.Fatal(err)
				}
			}
			got, err := f.svc.VerifyAccessToken(bg, usersService.VerifyAccessTokenRequest{AccessToken: created.AccessToken})
			if err != nil {
				t.Fatalf("unvalidated users must not error, got %v", err)
			}
			if got.IsValidated != tt.want {
				t.Errorf("IsValidated = %v, want %v", got.IsValidated, tt.want)
			}
		})
	}
}

func TestVerifyAccessToken_Errors(t *testing.T) {
	f := newFixture(t, false)
	created := f.create(t, "a@x.com", nextPhone())

	other, err := token.NewTokenizer("ffffffffffffffffffffffffffffffff")
	if err != nil {
		t.Fatal(err)
	}
	foreign, _, _ := other.GenerateNewTokens(map[string]any{"id": created.ID})
	noID, _, _ := f.tokenizer.GenerateNewTokens(map[string]any{"name": "A"})
	numericID, _, _ := f.tokenizer.GenerateNewTokens(map[string]any{"id": 42})
	badID, _, _ := f.tokenizer.GenerateNewTokens(map[string]any{"id": "not-a-ulid"})
	unknownUser, _, _ := f.tokenizer.GenerateNewTokens(map[string]any{"id": "01JCZZZZZZZZZZZZZZZZZZZZZZ"})

	tests := []struct {
		name    string
		ctx     context.Context
		token   string
		wantErr error
	}{
		{"nil ctx", nil, created.AccessToken, usersService.ErrInvalidContext},
		{"empty", bg, "", usersService.ErrTokenInvalid},
		{"garbage", bg, "not-a-token", usersService.ErrTokenInvalid},
		{"refresh token", bg, created.RefreshToken, usersService.ErrTokenWrongType},
		{"signed with another secret", bg, foreign, usersService.ErrTokenInvalid},
		{"payload without id", bg, noID, usersService.ErrTokenInvalid},
		{"payload id not a string", bg, numericID, usersService.ErrTokenInvalid},
		{"payload id not a ulid", bg, badID, usersService.ErrTokenInvalid},
		{"user does not exist", bg, unknownUser, usersService.ErrUserNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := f.svc.VerifyAccessToken(tt.ctx, usersService.VerifyAccessTokenRequest{AccessToken: tt.token})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if got != (usersService.VerifyAccessTokenResponse{}) {
				t.Errorf("response returned alongside error: %+v", got)
			}
		})
	}
}

func TestRefreshAccessToken_RotatesTokens(t *testing.T) {
	f := newFixture(t, false)
	created := f.create(t, "a@x.com", nextPhone())

	resp, err := f.svc.RefreshAccessToken(bg, usersService.RefreshAccessTokenRequest{RefreshToken: " " + created.RefreshToken + "\n"})
	if err != nil {
		t.Fatalf("RefreshAccessToken: %v", err)
	}
	if resp.ID != created.ID {
		t.Errorf("ID = %q, want %q", resp.ID, created.ID)
	}
	if resp.RefreshToken == created.RefreshToken || resp.AccessToken == created.AccessToken {
		t.Error("tokens were not rotated")
	}

	verified, err := f.svc.VerifyAccessToken(bg, usersService.VerifyAccessTokenRequest{AccessToken: resp.AccessToken})
	if err != nil || verified.ID != created.ID {
		t.Errorf("new access token = %+v, %v", verified, err)
	}

	next, err := f.svc.RefreshAccessToken(bg, usersService.RefreshAccessTokenRequest{RefreshToken: resp.RefreshToken})
	if err != nil || next.ID != created.ID {
		t.Errorf("rotated refresh token rejected: %+v, %v", next, err)
	}
}

func TestRefreshAccessToken_RejectsReuse(t *testing.T) {
	f := newFixture(t, false)
	created := f.create(t, "a@x.com", nextPhone())
	req := usersService.RefreshAccessTokenRequest{RefreshToken: created.RefreshToken}

	if _, err := f.svc.RefreshAccessToken(bg, req); err != nil {
		t.Fatal(err)
	}
	resp, err := f.svc.RefreshAccessToken(bg, req)
	if !errors.Is(err, usersService.ErrTokenReused) {
		t.Fatalf("reuse err = %v, want ErrTokenReused", err)
	}
	if resp != (usersService.RefreshAccessTokenResponse{}) {
		t.Errorf("response returned for reused token: %+v", resp)
	}
}

func TestRefreshAccessToken_Errors(t *testing.T) {
	f := newFixture(t, false)
	created := f.create(t, "a@x.com", nextPhone())

	other, err := token.NewTokenizer("ffffffffffffffffffffffffffffffff")
	if err != nil {
		t.Fatal(err)
	}
	_, foreign, _ := other.GenerateNewTokens(map[string]any{"id": created.ID})
	_, noID, _ := f.tokenizer.GenerateNewTokens(map[string]any{"name": "A"})
	_, badID, _ := f.tokenizer.GenerateNewTokens(map[string]any{"id": "not-a-ulid"})
	_, unknownUser, _ := f.tokenizer.GenerateNewTokens(map[string]any{"id": "01JCZZZZZZZZZZZZZZZZZZZZZZ"})

	tests := []struct {
		name    string
		ctx     context.Context
		token   string
		wantErr error
	}{
		{"nil ctx", nil, created.RefreshToken, usersService.ErrInvalidContext},
		{"empty", bg, "", usersService.ErrTokenInvalid},
		{"garbage", bg, "not-a-token", usersService.ErrTokenInvalid},
		{"access token", bg, created.AccessToken, usersService.ErrTokenWrongType},
		{"signed with another secret", bg, foreign, usersService.ErrTokenInvalid},
		{"payload without id", bg, noID, usersService.ErrTokenInvalid},
		{"payload id not a ulid", bg, badID, usersService.ErrTokenInvalid},
		{"user does not exist", bg, unknownUser, usersService.ErrUserNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := f.svc.RefreshAccessToken(tt.ctx, usersService.RefreshAccessTokenRequest{RefreshToken: tt.token})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if resp != (usersService.RefreshAccessTokenResponse{}) {
				t.Errorf("response returned alongside error: %+v", resp)
			}
		})
	}

	if _, err := f.svc.RefreshAccessToken(bg, usersService.RefreshAccessTokenRequest{RefreshToken: created.RefreshToken}); err != nil {
		t.Errorf("failed attempts must not burn the user's valid refresh token: %v", err)
	}
}

func TestStartEmailValidation(t *testing.T) {
	f := newFixture(t, false)
	created := f.create(t, "Buyer@X.com", nextPhone())

	resp, err := f.svc.StartEmailValidation(bg, usersService.StartEmailValidationRequest{AccessToken: " " + created.AccessToken + " "})
	if err != nil {
		t.Fatalf("StartEmailValidation: %v", err)
	}
	if resp.ID != created.ID || resp.Email != "buyer@x.com" {
		t.Errorf("response = %+v", resp)
	}
	if len(f.mailer.sent) != 1 || f.mailer.sent[0].To != "buyer@x.com" {
		t.Fatalf("otp mails = %+v", f.mailer.sent)
	}
	if code := f.mailer.lastCode(t); len(code) != 6 {
		t.Errorf("otp %q is not 6 digits", code)
	}

	if _, err := f.svc.StartEmailValidation(bg, usersService.StartEmailValidationRequest{AccessToken: created.AccessToken}); !errors.Is(err, usersService.ErrOTPResendTooSoon) {
		t.Errorf("immediate resend err = %v, want ErrOTPResendTooSoon", err)
	}
}

func TestStartEmailValidation_Errors(t *testing.T) {
	f := newFixture(t, false)
	validated := f.create(t, "v@x.com", nextPhone())
	if _, err := f.repo.ValidateEmail(bg, validated.ID); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		ctx     context.Context
		token   string
		wantErr error
	}{
		{"nil ctx", nil, validated.AccessToken, usersService.ErrInvalidContext},
		{"garbage token", bg, "nope", usersService.ErrTokenInvalid},
		{"refresh token", bg, validated.RefreshToken, usersService.ErrTokenWrongType},
		{"already validated", bg, validated.AccessToken, usersService.ErrEmailAlreadyValidated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := f.svc.StartEmailValidation(tt.ctx, usersService.StartEmailValidationRequest{AccessToken: tt.token})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if resp != (usersService.StartEmailValidationResponse{}) {
				t.Errorf("response returned alongside error: %+v", resp)
			}
		})
	}
	if n := len(f.mailer.sent); n != 0 {
		t.Errorf("sent %d otp mails for rejected requests, want 0", n)
	}

	f.mailer.err = errors.New("smtp down")
	unvalidated := f.create(t, "u@x.com", nextPhone())
	if _, err := f.svc.StartEmailValidation(bg, usersService.StartEmailValidationRequest{AccessToken: unvalidated.AccessToken}); !errors.Is(err, otp.ErrSendFailed) {
		t.Errorf("smtp failure err = %v, want otp.ErrSendFailed", err)
	}
}

func TestValidateEmail_Success(t *testing.T) {
	f := newFixture(t, true)
	created := f.create(t, "a@x.com", nextPhone())
	if _, err := f.svc.StartEmailValidation(bg, usersService.StartEmailValidationRequest{AccessToken: created.AccessToken}); err != nil {
		t.Fatal(err)
	}

	resp, err := f.svc.ValidateEmail(bg, usersService.ValidateEmailRequest{AccessToken: created.AccessToken, OTP: " " + f.mailer.lastCode(t) + " "})
	if err != nil {
		t.Fatalf("ValidateEmail: %v", err)
	}
	if resp != (usersService.ValidateEmailResponse{ID: created.ID, IsValidated: true}) {
		t.Errorf("response = %+v", resp)
	}

	user, _ := f.repo.GetUserByID(bg, created.ID)
	if !user.IsEmailValidated {
		t.Error("email not marked validated in the repository")
	}
	verified, err := f.svc.VerifyAccessToken(bg, usersService.VerifyAccessTokenRequest{AccessToken: created.AccessToken})
	if err != nil || !verified.IsValidated {
		t.Errorf("VerifyAccessToken after validation = %+v, %v", verified, err)
	}

	again, err := f.svc.ValidateEmail(bg, usersService.ValidateEmailRequest{AccessToken: created.AccessToken, OTP: "anything"})
	if err != nil || !again.IsValidated {
		t.Errorf("already validated user = %+v, %v; want IsValidated true", again, err)
	}
}

func TestValidateEmail_RejectedOTPReturnsFalse(t *testing.T) {
	f := newFixtureWithOTP(t, false, otp.Config{MaxAttempts: 2})
	created := f.create(t, "a@x.com", nextPhone())
	start := func() {
		t.Helper()
		if _, err := f.svc.StartEmailValidation(bg, usersService.StartEmailValidationRequest{AccessToken: created.AccessToken}); err != nil {
			t.Fatal(err)
		}
	}
	validate := func(code string) (usersService.ValidateEmailResponse, error) {
		return f.svc.ValidateEmail(bg, usersService.ValidateEmailRequest{AccessToken: created.AccessToken, OTP: code})
	}
	wantFalse := func(name string, resp usersService.ValidateEmailResponse, err, wantErr error) {
		t.Helper()
		if !errors.Is(err, wantErr) {
			t.Errorf("%s: err = %v, want %v", name, err, wantErr)
		}
		if resp != (usersService.ValidateEmailResponse{ID: created.ID, IsValidated: false}) {
			t.Errorf("%s: response = %+v, want ID with IsValidated false", name, resp)
		}
	}

	resp, err := validate("123456")
	wantFalse("no otp requested", resp, err, usersService.ErrOTPNotFound)

	start()
	code := f.mailer.lastCode(t)
	wrong := "000000"
	if wrong == code {
		wrong = "111111"
	}
	resp, err = validate(wrong)
	wantFalse("wrong otp", resp, err, usersService.ErrOTPInvalid)
	resp, err = validate(wrong)
	wantFalse("too many attempts", resp, err, usersService.ErrOTPTooManyAttempts)
	resp, err = validate(code)
	wantFalse("correct code after lockout", resp, err, usersService.ErrOTPNotFound)

	user, _ := f.repo.GetUserByID(bg, created.ID)
	if user.IsEmailValidated {
		t.Error("email validated despite rejected otps")
	}
}

func TestValidateEmail_ExpiredOTPReturnsFalse(t *testing.T) {
	f := newFixtureWithOTP(t, false, otp.Config{TTL: 50 * time.Millisecond, ResendCooldown: time.Millisecond})
	created := f.create(t, "a@x.com", nextPhone())
	if _, err := f.svc.StartEmailValidation(bg, usersService.StartEmailValidationRequest{AccessToken: created.AccessToken}); err != nil {
		t.Fatal(err)
	}
	code := f.mailer.lastCode(t)
	time.Sleep(80 * time.Millisecond)

	resp, err := f.svc.ValidateEmail(bg, usersService.ValidateEmailRequest{AccessToken: created.AccessToken, OTP: code})
	if !errors.Is(err, usersService.ErrOTPExpired) {
		t.Errorf("err = %v, want ErrOTPExpired", err)
	}
	if resp != (usersService.ValidateEmailResponse{ID: created.ID, IsValidated: false}) {
		t.Errorf("response = %+v", resp)
	}
}

func TestValidateEmail_Errors(t *testing.T) {
	f := newFixture(t, false)
	created := f.create(t, "a@x.com", nextPhone())

	tests := []struct {
		name    string
		ctx     context.Context
		token   string
		wantErr error
	}{
		{"nil ctx", nil, created.AccessToken, usersService.ErrInvalidContext},
		{"garbage token", bg, "nope", usersService.ErrTokenInvalid},
		{"refresh token", bg, created.RefreshToken, usersService.ErrTokenWrongType},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := f.svc.ValidateEmail(tt.ctx, usersService.ValidateEmailRequest{AccessToken: tt.token, OTP: "123456"})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if resp != (usersService.ValidateEmailResponse{}) {
				t.Errorf("response returned alongside error: %+v", resp)
			}
		})
	}
}

func TestTokensCarryOnlyUserID(t *testing.T) {
	f := newFixture(t, false)
	created := f.create(t, "a@x.com", "+919876543210")

	login, err := f.svc.GetUserAndVerifyPassword(bg, usersService.GetUserAndVerifyPasswordRequest{Email: "a@x.com", Password: testPassword})
	if err != nil {
		t.Fatal(err)
	}
	refreshed, err := f.svc.RefreshAccessToken(bg, usersService.RefreshAccessTokenRequest{RefreshToken: login.RefreshToken})
	if err != nil {
		t.Fatal(err)
	}

	for name, access := range map[string]string{"create": created.AccessToken, "login": login.AccessToken, "refresh": refreshed.AccessToken} {
		payload, err := f.tokenizer.VerifyAccessToken(access)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(payload) != 1 || payload["id"] != created.ID {
			t.Errorf("%s access token payload = %v, want only {id: %s}", name, payload, created.ID)
		}
	}
}
