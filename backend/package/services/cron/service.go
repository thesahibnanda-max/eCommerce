package cronService

import (
	"context"
	"log/slog"
	"time"
)

type Service interface {
	Start() error
	Stop() error
}

type Job struct {
	Name     string
	Interval time.Duration
	Timeout  time.Duration
	Run      func(ctx context.Context) error
}

func NewService(params ConstructorParams) (Service, error) {
	if params == nil {
		return nil, ErrParamsInvalid
	}
	jobs, err := validateJobs(params.jobs())
	if err != nil {
		return nil, err
	}
	r := newRunner(params.logger())
	switch p := params.(type) {
	case GocronParams:
		return newGocronService(r, jobs, p)
	case RobfigParams:
		return newRobfigService(r, jobs, p)
	default:
		return nil, ErrParamsInvalid
	}
}

func Params(engine Engine, jobs []Job, logger *slog.Logger) (ConstructorParams, error) {
	switch engine {
	case EngineGocron:
		return GocronParams{Jobs: jobs, Logger: logger}, nil
	case EngineRobfig:
		return RobfigParams{Jobs: jobs, Logger: logger}, nil
	default:
		return nil, ErrEngineInvalid
	}
}
