package cronService

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const stopTimeout = 30 * time.Second

var minInterval = time.Minute

func validateJobs(jobs []Job) ([]Job, error) {
	if len(jobs) == 0 {
		return nil, ErrJobsRequired
	}
	seen := make(map[string]struct{}, len(jobs))
	out := make([]Job, 0, len(jobs))
	for _, job := range jobs {
		name := strings.TrimSpace(job.Name)
		if _, dup := seen[name]; name == "" || dup || job.Run == nil || job.Interval < minInterval || job.Timeout <= 0 {
			return nil, fmt.Errorf("%w: %q", ErrJobInvalid, job.Name)
		}
		seen[name] = struct{}{}
		job.Name = name
		out = append(out, job)
	}
	return out, nil
}

type runner struct {
	logger *slog.Logger
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu      sync.Mutex
	started bool
	stopped bool
}

func newRunner(logger *slog.Logger) *runner {
	if logger == nil {
		logger = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &runner{logger: logger, ctx: ctx, cancel: cancel}
}

func (r *runner) markStarted() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped {
		return ErrStopped
	}
	if r.started {
		return ErrAlreadyStarted
	}
	r.started = true
	return nil
}

func (r *runner) markStopped() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped {
		return false
	}
	r.stopped = true
	r.cancel()
	return true
}

func (r *runner) task(job Job) func() {
	var busy atomic.Bool
	return func() {
		if r.ctx.Err() != nil || !busy.CompareAndSwap(false, true) {
			return
		}
		r.wg.Add(1)
		defer r.wg.Done()
		defer busy.Store(false)
		r.execute(job)
	}
}

func (r *runner) execute(job Job) {
	ctx, cancel := context.WithTimeout(r.ctx, job.Timeout)
	defer cancel()
	started := time.Now()
	err := safeRun(ctx, job)
	if err != nil {
		r.logger.Error("cron job failed", "job", job.Name, "duration", time.Since(started).String(), "error", err.Error())
		return
	}
	r.logger.Info("cron job finished", "job", job.Name, "duration", time.Since(started).String())
}

func safeRun(ctx context.Context, job Job) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("%w: %v", ErrPanicked, recovered)
		}
	}()
	return job.Run(ctx)
}

func (r *runner) wait(timeout time.Duration) error {
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return ErrStopTimeout
	}
}
