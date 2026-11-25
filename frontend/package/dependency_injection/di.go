package frontendDependencyInjection

import (
	"context"
	_ "embed"
	"fmt"
	"sync"

	rateLimiter "github.com/thesahibnanda-max/eCommerce/backend/package/rate_limiter"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
	"github.com/thesahibnanda-max/eCommerce/frontend"
	frontendHandler "github.com/thesahibnanda-max/eCommerce/frontend/package/handler"
	frontendRouter "github.com/thesahibnanda-max/eCommerce/frontend/package/router"
)

const (
	maxPort = 65535
)

//go:embed config.yaml
var configString string

var loadConfig = sync.OnceValues(func() (configStruct, error) {
	return utils.ReadParametrisedYAML[configStruct](configString)
})

type App interface {
	Close() error
	Router() frontendRouter.Router
	Port() uint
}

type app struct {
	router frontendRouter.Router
	port   uint
}

func NewApp(ctx context.Context) (App, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return nil, err
	}
	config, err := loadConfig()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConfigInvalid, err)
	}
	if err := validatePort(config.Server.Port); err != nil {
		return nil, err
	}
	files, err := frontend.StaticFiles()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrStaticFilesFailed, err)
	}
	limiter, err := rateLimiter.NewLimiter(limiterConfig(config.RateLimit))
	if err != nil {
		return nil, err
	}
	h, err := frontendHandler.NewHandler(files, config.Backend.APIPort, limiter)
	if err != nil {
		return nil, err
	}
	r, err := frontendRouter.NewRouter(h)
	if err != nil {
		return nil, err
	}
	return &app{router: r, port: config.Server.Port}, nil
}

func (a *app) Router() frontendRouter.Router {
	return a.router
}

func (a *app) Port() uint {
	return a.port
}

func (a *app) Close() error {
	return nil
}
