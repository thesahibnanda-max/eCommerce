package adminOperationsService

import (
	"context"
	"errors"
	"testing"
	"time"

	spreadsheetRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/spreadsheet"
	googleAuthService "github.com/thesahibnanda-max/eCommerce/backend/package/services/google_auth"
	snapshotService "github.com/thesahibnanda-max/eCommerce/backend/package/services/snapshot"
)

type fakeGoogle struct {
	status       googleAuthService.Status
	authRequest  googleAuthService.AuthRequest
	callback     googleAuthService.CallbackRequest
	disconnected bool
}

func (f *fakeGoogle) Status() googleAuthService.Status { return f.status }

func (f *fakeGoogle) AuthURL(_ context.Context, request googleAuthService.AuthRequest) (string, error) {
	f.authRequest = request
	return "https://accounts.example/auth?state=s", nil
}

func (f *fakeGoogle) Callback(_ context.Context, request googleAuthService.CallbackRequest) (string, error) {
	f.callback = request
	f.status.Connected = true
	return "https://shop.example/admin", nil
}

func (f *fakeGoogle) Disconnect(context.Context) error {
	f.disconnected = true
	f.status = googleAuthService.Status{}
	return nil
}

func (f *fakeGoogle) Spreadsheets(context.Context) (spreadsheetRepository.Repository, error) {
	return nil, googleAuthService.ErrNotConnected
}

func (f *fakeGoogle) Close() error { return nil }

type fakeSnapshots struct {
	result   snapshotService.Result
	err      error
	status   snapshotService.Status
	deadline time.Time
	ctxErr   error
}

func (f *fakeSnapshots) Run(ctx context.Context) (snapshotService.Result, error) {
	f.deadline, _ = ctx.Deadline()
	f.ctxErr = ctx.Err()
	return f.result, f.err
}

func (f *fakeSnapshots) Status() snapshotService.Status { return f.status }

func snapshotTestService(snapshots snapshotService.Service) *service {
	s := &service{adminKey: "key", snapshots: snapshots, snapshotInterval: 6 * time.Hour, snapshotTimeout: time.Minute}
	if snapshots != nil {
		s.google = &fakeGoogle{status: googleAuthService.Status{Connected: true, Account: "owner@example.com"}}
	}
	return s
}

func TestRunSnapshot(t *testing.T) {
	last := &snapshotService.Result{Failed: 1}
	fake := &fakeSnapshots{
		result: snapshotService.Result{Failed: 1, Databases: []snapshotService.DatabaseResult{{Database: "users", URL: "u"}, {Database: "images", Error: "boom"}}},
		err:    snapshotService.ErrDatabasesFailed,
		status: snapshotService.Status{Last: last},
	}
	s := snapshotTestService(fake)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resp, err := s.RunSnapshot(ctx, KeyRequest{AdminKey: "key"})
	if err != nil {
		t.Fatalf("partial failure should still answer: %v", err)
	}
	if !resp.Configured || resp.Interval != "6h" || resp.Result == nil || resp.Result.Failed != 1 || resp.Last != last {
		t.Fatalf("resp = %+v", resp)
	}
	if fake.deadline.IsZero() || time.Until(fake.deadline) > time.Minute {
		t.Fatalf("run should use the snapshot timeout, deadline %v", fake.deadline)
	}

	fake.err = snapshotService.ErrAlreadyRunning
	if _, err := s.RunSnapshot(context.Background(), KeyRequest{AdminKey: "key"}); !errors.Is(err, snapshotService.ErrAlreadyRunning) {
		t.Fatalf("already running err = %v", err)
	}
	if _, err := s.RunSnapshot(context.Background(), KeyRequest{AdminKey: "nope"}); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("bad key err = %v", err)
	}
}

func TestRunSnapshot_SurvivesClientDisconnect(t *testing.T) {
	fake := &fakeSnapshots{}
	s := snapshotTestService(fake)
	ctx, cancel := context.WithCancel(context.Background())
	resp, err := s.RunSnapshot(ctx, KeyRequest{AdminKey: "key"})
	cancel()
	if err != nil || fake.ctxErr != nil || !resp.Configured {
		t.Fatalf("resp %+v err %v ctxErr %v", resp, err, fake.ctxErr)
	}
}

func TestSnapshotStatus(t *testing.T) {
	s := snapshotTestService(&fakeSnapshots{status: snapshotService.Status{Running: true}})
	resp, err := s.SnapshotStatus(context.Background(), KeyRequest{AdminKey: "key"})
	if err != nil || !resp.Configured || !resp.Running || resp.Interval != "6h" || resp.Google == nil || resp.Google.Account != "owner@example.com" {
		t.Fatalf("resp = %+v err = %v", resp, err)
	}
	off := snapshotTestService(nil)
	resp, err = off.SnapshotStatus(context.Background(), KeyRequest{AdminKey: "key"})
	if err != nil || resp.Configured {
		t.Fatalf("off resp = %+v err = %v", resp, err)
	}
	if _, err := off.RunSnapshot(context.Background(), KeyRequest{AdminKey: "key"}); !errors.Is(err, ErrSnapshotsDisabled) {
		t.Fatalf("off run err = %v", err)
	}
}

func TestGoogleConnection(t *testing.T) {
	s := snapshotTestService(&fakeSnapshots{})
	google := s.google.(*fakeGoogle)
	google.status = googleAuthService.Status{}
	req := ConnectGoogleRequest{AdminKey: "key", CallbackURL: "https://shop.example/v1/admin/google/callback", ReturnURL: "https://shop.example/admin"}
	resp, err := s.ConnectGoogle(context.Background(), req)
	if err != nil || resp.AuthURL == "" || google.authRequest.CallbackURL != req.CallbackURL || google.authRequest.ReturnURL != req.ReturnURL {
		t.Fatalf("connect resp = %+v err = %v req = %+v", resp, err, google.authRequest)
	}
	if _, err := s.ConnectGoogle(context.Background(), ConnectGoogleRequest{AdminKey: "nope"}); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("bad key err = %v", err)
	}
	back, err := s.GoogleCallback(context.Background(), GoogleCallbackRequest{State: "s", Code: "c"})
	if err != nil || back != "https://shop.example/admin" || google.callback.State != "s" || google.callback.Code != "c" {
		t.Fatalf("callback = %q %v %+v", back, err, google.callback)
	}
	status, err := s.DisconnectGoogle(context.Background(), KeyRequest{AdminKey: "key"})
	if err != nil || !google.disconnected || status.Google == nil || status.Google.Connected {
		t.Fatalf("disconnect = %+v %v", status, err)
	}
	off := snapshotTestService(nil)
	if _, err := off.ConnectGoogle(context.Background(), req); !errors.Is(err, ErrSnapshotsDisabled) {
		t.Fatalf("off connect err = %v", err)
	}
	if _, err := off.GoogleCallback(context.Background(), GoogleCallbackRequest{}); !errors.Is(err, ErrSnapshotsDisabled) {
		t.Fatalf("off callback err = %v", err)
	}
}

func TestHumanDuration(t *testing.T) {
	for d, want := range map[time.Duration]string{6 * time.Hour: "6h", 90 * time.Minute: "1h30m", 45 * time.Second: "45s", 30 * time.Minute: "30m"} {
		if got := humanDuration(d); got != want {
			t.Fatalf("humanDuration(%v) = %q, want %q", d, got, want)
		}
	}
}
