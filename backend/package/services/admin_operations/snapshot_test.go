package adminOperationsService

import (
	"context"
	"errors"
	"testing"
	"time"

	snapshotService "github.com/thesahibnanda-max/eCommerce/backend/package/services/snapshot"
)

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
	return &service{adminKey: "key", snapshots: snapshots, snapshotInterval: 6 * time.Hour, snapshotTimeout: time.Minute}
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
	if err != nil || !resp.Configured || !resp.Running || resp.Interval != "6h" {
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

func TestHumanDuration(t *testing.T) {
	for d, want := range map[time.Duration]string{6 * time.Hour: "6h", 90 * time.Minute: "1h30m", 45 * time.Second: "45s", 30 * time.Minute: "30m"} {
		if got := humanDuration(d); got != want {
			t.Fatalf("humanDuration(%v) = %q, want %q", d, got, want)
		}
	}
}
