package frontendDependencyInjection

import (
	"context"
	"errors"
	"testing"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

func Test_NewApp_SameOrigin(t *testing.T) {
	a, err := NewApp(context.Background(), true)
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	if a.Router().Handler() == nil {
		t.Fatal("Handler() = nil")
	}
}

func Test_NewApp(t *testing.T) {
	a, err := NewApp(context.Background(), false)
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	if a.Router() == nil || a.Port() != 4073 {
		t.Fatalf("app router %v port %d", a.Router(), a.Port())
	}
	if err := a.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func Test_NewApp_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewApp(ctx, false); !errors.Is(err, utils.ErrContextInvalid) {
		t.Fatalf("err = %v", err)
	}
}

func Test_Config(t *testing.T) {
	config, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.Backend.APIPort != 3000 {
		t.Fatalf("api port = %d", config.Backend.APIPort)
	}
	if _, ok := config.RateLimit.Rules["static"]; !ok {
		t.Fatal("static rate limit rule missing")
	}
	for _, port := range []uint{0, 70000} {
		if err := validatePort(port); !errors.Is(err, ErrServerPortInvalid) {
			t.Fatalf("validatePort(%d) = %v", port, err)
		}
	}
}
