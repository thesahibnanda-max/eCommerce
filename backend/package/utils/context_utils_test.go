package utils_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

func TestCheckContext(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	now, err := utils.TimeNow()
	if err != nil {
		t.Fatalf("TimeNow: %v", err)
	}
	expired, cancelExpired := context.WithDeadline(context.Background(), now.Add(-time.Second))
	defer cancelExpired()

	live, cancelLive := context.WithTimeout(context.Background(), time.Hour)
	defer cancelLive()

	tests := []struct {
		name      string
		ctx       context.Context
		wantErr   bool
		wantCause error
	}{
		{name: "background", ctx: context.Background()},
		{name: "live with deadline", ctx: live},
		{name: "nil", ctx: nil, wantErr: true},
		{name: "canceled", ctx: canceled, wantErr: true, wantCause: context.Canceled},
		{name: "deadline exceeded", ctx: expired, wantErr: true, wantCause: context.DeadlineExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := utils.CheckContext(tt.ctx)
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("err = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, utils.ErrContextInvalid) {
				t.Errorf("err = %v, want ErrContextInvalid", err)
			}
			if tt.wantCause != nil && !errors.Is(err, tt.wantCause) {
				t.Errorf("err = %v, want it to wrap %v", err, tt.wantCause)
			}
		})
	}
}
