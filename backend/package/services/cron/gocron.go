package cronService

import (
	"errors"
	"fmt"

	goCron "github.com/go-co-op/gocron/v2"
)

type gocronService struct {
	runner    *runner
	scheduler goCron.Scheduler
}

func newGocronService(r *runner, jobs []Job, _ GocronParams) (*gocronService, error) {
	scheduler, err := goCron.NewScheduler(goCron.WithStopTimeout(stopTimeout))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrParamsInvalid, err)
	}
	for _, job := range jobs {
		if _, err := scheduler.NewJob(
			goCron.DurationJob(job.Interval),
			goCron.NewTask(r.task(job)),
			goCron.WithName(job.Name),
			goCron.WithSingletonMode(goCron.LimitModeReschedule),
		); err != nil {
			return nil, errors.Join(fmt.Errorf("%w: %w", ErrParamsInvalid, err), scheduler.Shutdown())
		}
	}
	return &gocronService{runner: r, scheduler: scheduler}, nil
}

func (s *gocronService) Start() error {
	if err := s.runner.markStarted(); err != nil {
		return err
	}
	s.scheduler.Start()
	return nil
}

func (s *gocronService) Stop() error {
	if !s.runner.markStopped() {
		return nil
	}
	return errors.Join(s.scheduler.Shutdown(), s.runner.wait(stopTimeout))
}
