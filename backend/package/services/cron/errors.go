package cronService

const (
	ErrParamsInvalid  = Error("cronService: constructor params are invalid")
	ErrJobsRequired   = Error("cronService: at least one job is required")
	ErrJobInvalid     = Error("cronService: job needs a unique name, a run function, an interval of at least one minute and a positive timeout")
	ErrEngineInvalid  = Error("cronService: engine must be GOCRON or ROBFIG")
	ErrAlreadyStarted = Error("cronService: already started")
	ErrStopped        = Error("cronService: stopped services cannot be restarted")
	ErrStopTimeout    = Error("cronService: running jobs did not finish in time")
	ErrPanicked       = Error("cronService: job panicked")
)

type Error string

func (e Error) Error() string {
	return string(e)
}
