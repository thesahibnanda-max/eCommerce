package cronService

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func init() {
	minInterval = 10 * time.Millisecond
}

type engineCase struct {
	name     string
	interval time.Duration
	params   func(jobs []Job) ConstructorParams
}

var engines = []engineCase{
	{"gocron", 50 * time.Millisecond, func(jobs []Job) ConstructorParams { return GocronParams{Jobs: jobs} }},
	{"robfig", time.Second, func(jobs []Job) ConstructorParams { return RobfigParams{Jobs: jobs} }},
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met in time")
}

func TestEngines_RunJobsRepeatedly(t *testing.T) {
	for _, engine := range engines {
		t.Run(engine.name, func(t *testing.T) {
			var runs atomic.Int32
			svc, err := NewService(engine.params([]Job{{Name: "count", Interval: engine.interval, Timeout: time.Second, Run: func(context.Context) error {
				runs.Add(1)
				return nil
			}}}))
			if err != nil {
				t.Fatal(err)
			}
			if err := svc.Start(); err != nil {
				t.Fatal(err)
			}
			if err := svc.Start(); !errors.Is(err, ErrAlreadyStarted) {
				t.Fatalf("second start = %v", err)
			}
			waitFor(t, 5*engine.interval+2*time.Second, func() bool { return runs.Load() >= 2 })
			if err := svc.Stop(); err != nil {
				t.Fatal(err)
			}
			after := runs.Load()
			time.Sleep(2 * engine.interval)
			if runs.Load() != after {
				t.Fatal("job ran after Stop")
			}
			if err := svc.Stop(); err != nil {
				t.Fatalf("second stop = %v", err)
			}
			if err := svc.Start(); !errors.Is(err, ErrStopped) {
				t.Fatalf("start after stop = %v", err)
			}
		})
	}
}

func TestEngines_NeverOverlapAndStopCancelsRunningJob(t *testing.T) {
	for _, engine := range engines {
		t.Run(engine.name, func(t *testing.T) {
			var active, maxActive, runs atomic.Int32
			var cancelled atomic.Bool
			started := make(chan struct{}, 1)
			svc, err := NewService(engine.params([]Job{{Name: "slow", Interval: engine.interval, Timeout: time.Minute, Run: func(ctx context.Context) error {
				now := active.Add(1)
				defer active.Add(-1)
				for {
					peak := maxActive.Load()
					if now <= peak || maxActive.CompareAndSwap(peak, now) {
						break
					}
				}
				runs.Add(1)
				select {
				case started <- struct{}{}:
				default:
				}
				<-ctx.Done()
				cancelled.Store(true)
				return ctx.Err()
			}}}))
			if err != nil {
				t.Fatal(err)
			}
			if err := svc.Start(); err != nil {
				t.Fatal(err)
			}
			select {
			case <-started:
			case <-time.After(5*engine.interval + 2*time.Second):
				t.Fatal("job never started")
			}
			time.Sleep(3 * engine.interval)
			if err := svc.Stop(); err != nil {
				t.Fatal(err)
			}
			if !cancelled.Load() {
				t.Fatal("Stop did not cancel the running job")
			}
			if maxActive.Load() != 1 || runs.Load() != 1 {
				t.Fatalf("overlapping runs: max active %d, runs %d", maxActive.Load(), runs.Load())
			}
		})
	}
}

func TestEngines_RecoverPanicsAndKeepRunning(t *testing.T) {
	for _, engine := range engines {
		t.Run(engine.name, func(t *testing.T) {
			var runs atomic.Int32
			svc, err := NewService(engine.params([]Job{{Name: "panics", Interval: engine.interval, Timeout: time.Second, Run: func(context.Context) error {
				runs.Add(1)
				panic("boom")
			}}}))
			if err != nil {
				t.Fatal(err)
			}
			if err := svc.Start(); err != nil {
				t.Fatal(err)
			}
			waitFor(t, 5*engine.interval+2*time.Second, func() bool { return runs.Load() >= 2 })
			if err := svc.Stop(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestNewService_Validation(t *testing.T) {
	ok := Job{Name: "a", Interval: time.Second, Timeout: time.Second, Run: func(context.Context) error { return nil }}
	cases := map[string][]Job{
		"no jobs":       nil,
		"blank name":    {{Name: " ", Interval: time.Second, Timeout: time.Second, Run: ok.Run}},
		"duplicate":     {ok, ok},
		"no run":        {{Name: "x", Interval: time.Second, Timeout: time.Second}},
		"tiny interval": {{Name: "x", Interval: time.Millisecond, Timeout: time.Second, Run: ok.Run}},
		"no timeout":    {{Name: "x", Interval: time.Second, Run: ok.Run}},
	}
	for name, jobs := range cases {
		for _, engine := range engines {
			if _, err := NewService(engine.params(jobs)); !errors.Is(err, ErrJobsRequired) && !errors.Is(err, ErrJobInvalid) {
				t.Fatalf("%s/%s: err = %v", engine.name, name, err)
			}
		}
	}
	if _, err := NewService(nil); !errors.Is(err, ErrParamsInvalid) {
		t.Fatalf("nil params = %v", err)
	}
}

func TestParseEngineAndParams(t *testing.T) {
	for in, want := range map[string]Engine{"gocron": EngineGocron, " ROBFIG ": EngineRobfig} {
		got, err := ParseEngine(in)
		if err != nil || got != want {
			t.Fatalf("ParseEngine(%q) = %q, %v", in, got, err)
		}
	}
	if _, err := ParseEngine("quartz"); !errors.Is(err, ErrEngineInvalid) {
		t.Fatalf("bad engine = %v", err)
	}
	p, err := Params(EngineRobfig, nil, nil)
	if _, isRobfig := p.(RobfigParams); err != nil || !isRobfig {
		t.Fatalf("Params robfig = %T %v", p, err)
	}
	p, err = Params(EngineGocron, nil, nil)
	if _, isGocron := p.(GocronParams); err != nil || !isGocron {
		t.Fatalf("Params gocron = %T %v", p, err)
	}
	if _, err := Params("x", nil, nil); !errors.Is(err, ErrEngineInvalid) {
		t.Fatalf("Params bad = %v", err)
	}
}
