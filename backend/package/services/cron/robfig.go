package cronService

import (
	"context"
	"errors"
	"log/slog"
	"time"

	robfigCron "github.com/robfig/cron/v3"
)

type robfigService struct {
	runner *runner
	cron   *robfigCron.Cron
}

func newRobfigService(r *runner, jobs []Job, _ RobfigParams) (*robfigService, error) {
	logger := cronLogger{logger: r.logger}
	c := robfigCron.New(robfigCron.WithLogger(logger), robfigCron.WithChain(robfigCron.Recover(logger), robfigCron.SkipIfStillRunning(logger)))
	for _, job := range jobs {
		c.Schedule(robfigCron.Every(job.Interval), robfigCron.FuncJob(r.task(job)))
	}
	return &robfigService{runner: r, cron: c}, nil
}

func (s *robfigService) Start() error {
	if err := s.runner.markStarted(); err != nil {
		return err
	}
	s.cron.Start()
	return nil
}

func (s *robfigService) Stop() error {
	if !s.runner.markStopped() {
		return nil
	}
	stopped := s.cron.Stop()
	select {
	case <-stopped.Done():
	case <-time.After(stopTimeout):
		return ErrStopTimeout
	}
	return errors.Join(s.runner.wait(stopTimeout), contextError(stopped))
}

func contextError(ctx context.Context) error {
	if errors.Is(ctx.Err(), context.Canceled) {
		return nil
	}
	return ctx.Err()
}

type cronLogger struct {
	logger *slog.Logger
}

func (l cronLogger) Info(msg string, keysAndValues ...any) {
	l.logger.Debug("robfig cron: "+msg, keysAndValues...)
}

func (l cronLogger) Error(err error, msg string, keysAndValues ...any) {
	l.logger.Error("robfig cron: "+msg, append([]any{"error", err}, keysAndValues...)...)
}
