package cronService

import (
	"log/slog"
	"strings"
)

type Engine string

const (
	EngineGocron Engine = "GOCRON"
	EngineRobfig Engine = "ROBFIG"
)

func ParseEngine(value string) (Engine, error) {
	engine := Engine(strings.ToUpper(strings.TrimSpace(value)))
	if engine != EngineGocron && engine != EngineRobfig {
		return "", ErrEngineInvalid
	}
	return engine, nil
}

type ConstructorParams interface {
	engine() Engine
	jobs() []Job
	logger() *slog.Logger
}

type GocronParams struct {
	Jobs   []Job
	Logger *slog.Logger
}

func (p GocronParams) engine() Engine       { return EngineGocron }
func (p GocronParams) jobs() []Job          { return p.Jobs }
func (p GocronParams) logger() *slog.Logger { return p.Logger }

type RobfigParams struct {
	Jobs   []Job
	Logger *slog.Logger
}

func (p RobfigParams) engine() Engine       { return EngineRobfig }
func (p RobfigParams) jobs() []Job          { return p.Jobs }
func (p RobfigParams) logger() *slog.Logger { return p.Logger }
