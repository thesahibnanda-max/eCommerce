package dependencyInjection

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/thesahibnanda-max/eCommerce/backend/package/smtp"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

var testEnv = map[string]string{
	"TOKEN_SECRET":            "test-token-secret-0123456789abcdef0123",
	"ADMIN_KEY":               "test-admin-key",
	"SMTP_SENDER_1_EMAIL":     "sender1@example.com",
	"SMTP_SENDER_1_USERNAME":  "sender1@example.com",
	"SMTP_SENDER_1_PASSWORD":  "test-password-1",
	"SMTP_SENDER_2_EMAIL":     "sender2@example.com",
	"SMTP_SENDER_2_USERNAME":  "sender2@example.com",
	"SMTP_SENDER_2_PASSWORD":  "test-password-2",
	"INSTAMOJO_CLIENT_ID":     "",
	"INSTAMOJO_CLIENT_SECRET": "",
	"INSTAMOJO_PRIVATE_SALT":  "",
}

func TestMain(m *testing.M) {
	for key, value := range testEnv {
		if err := os.Setenv(key, value); err != nil {
			panic(err)
		}
	}
	os.Exit(m.Run())
}

func testConfig(t *testing.T) configStruct {
	t.Helper()
	config, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	return config
}

func Test_ConfigUsesEnvSecrets(t *testing.T) {
	config := testConfig(t)
	if config.Token.Secret != testEnv["TOKEN_SECRET"] || config.Admin.Key != testEnv["ADMIN_KEY"] {
		t.Fatalf("token/admin secrets not read from env")
	}
	if len(config.SMTP.Senders) != 2 || config.SMTP.Senders[0].Password != testEnv["SMTP_SENDER_1_PASSWORD"] || config.SMTP.Senders[1].Password != testEnv["SMTP_SENDER_2_PASSWORD"] {
		t.Fatalf("smtp passwords not read from env")
	}
}

func Test_NewApp(t *testing.T) {
	config := testConfig(t)
	t.Chdir(t.TempDir())
	a, err := NewApp(context.Background())
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	if a.Router() == nil {
		t.Fatalf("Router() = nil")
	}
	if a.Port() != config.Server.Port {
		t.Fatalf("Port() = %d, want %d", a.Port(), config.Server.Port)
	}
	for _, file := range []string{config.Storage.UsersFile, config.Storage.ProductsFile, config.Storage.ImagesFile, config.Storage.CartsFile, config.Storage.OrdersFile, config.Storage.AddressesFile, config.Payments.PaymentsFile} {
		if _, err := os.Stat(filepath.Join(config.Storage.Directory, file)); err != nil {
			t.Fatalf("storage file %q missing: %v", file, err)
		}
	}
	if err := a.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := a.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	again, err := NewApp(context.Background())
	if err != nil {
		t.Fatalf("NewApp() after Close error = %v", err)
	}
	if err := again.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func Test_NewApp_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewApp(ctx); !errors.Is(err, utils.ErrContextInvalid) {
		t.Fatalf("NewApp() error = %v, want %v", err, utils.ErrContextInvalid)
	}
}

func Test_ValidatePort(t *testing.T) {
	for _, port := range []uint{0, maxPort + 1} {
		if err := validatePort(port); !errors.Is(err, ErrServerPortInvalid) {
			t.Fatalf("validatePort(%d) error = %v", port, err)
		}
	}
	if err := validatePort(3000); err != nil {
		t.Fatalf("validatePort(3000) error = %v", err)
	}
}

func Test_PrepareStorage(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "data")
	got, mode, err := prepareStorage(" "+dir+" ", "0600")
	if err != nil || got != dir || mode != 0o600 {
		t.Fatalf("prepareStorage() = %q, %o, %v", got, mode, err)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("directory not created: %v", err)
	}
	if _, _, err := prepareStorage("  ", "0600"); !errors.Is(err, ErrStorageDirectoryRequired) {
		t.Fatalf("empty directory error = %v", err)
	}
	for _, perm := range []string{"", "abc", "0", "0999", "17777"} {
		if _, _, err := prepareStorage(dir, perm); !errors.Is(err, ErrStoragePermissionInvalid) {
			t.Fatalf("prepareStorage(%q) error = %v", perm, err)
		}
	}
}

func Test_SMTPSecurity(t *testing.T) {
	tests := map[string]smtp.Security{"starttls": smtp.SecurityStartTLS, " SSL ": smtp.SecuritySSL, "none": smtp.SecurityNone}
	for value, want := range tests {
		got, err := smtpSecurity(value)
		if err != nil || got != want {
			t.Fatalf("smtpSecurity(%q) = %v, %v", value, got, err)
		}
	}
	if _, err := smtpSecurity("tls"); !errors.Is(err, ErrSMTPSecurityInvalid) {
		t.Fatalf("smtpSecurity(tls) error = %v", err)
	}
	if _, err := smtpSenders([]senderConfig{{Security: "bad"}}); !errors.Is(err, ErrSMTPSecurityInvalid) {
		t.Fatalf("smtpSenders() error = %v", err)
	}
}

func Test_JoinClose(t *testing.T) {
	var order []int
	boom := errors.New("boom")
	closeFn := joinClose([]func() error{
		func() error { order = append(order, 1); return nil },
		func() error { order = append(order, 2); return boom },
	})
	err := closeFn()
	if !errors.Is(err, ErrCloseFailed) || !errors.Is(err, boom) {
		t.Fatalf("joinClose() error = %v", err)
	}
	if len(order) != 2 || order[0] != 2 || order[1] != 1 {
		t.Fatalf("close order = %v, want [2 1]", order)
	}
}

func Test_LimiterConfig(t *testing.T) {
	config := testConfig(t)
	cfg := limiterConfig(config.RateLimit)
	for _, group := range []string{"auth", "otp", "catalog", "images", "cart", "admin", "info", "orders", "payments"} {
		if _, ok := cfg.Rules[group]; !ok {
			t.Fatalf("config.yaml missing rate limit group %q", group)
		}
	}
	if config.CORS.FrontendPort != 4073 {
		t.Fatalf("cors frontend port = %d, want 4073", config.CORS.FrontendPort)
	}
}
