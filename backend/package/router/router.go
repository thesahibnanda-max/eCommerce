package router

import (
	"context"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/recover"

	"github.com/thesahibnanda-max/eCommerce/backend/package/handler"
)

const (
	bodyLimit            = 64 << 20
	codeInternalError    = "internal_error"
	messageInternalError = "internal server error"
	maxPort              = 65535
	corsMaxAge           = 3600
	headerAdminKey       = "X-Admin-Key"
)

type Router interface {
	ListenAndServeOnPort(ctx context.Context, port uint) chan error
}

type routerImpl struct {
	handler     handler.Handler
	app         *fiber.App
	fiberConfig *fiber.Config
}

func NewRouter(h handler.Handler, frontendPort uint) (Router, error) {
	if h == nil {
		return nil, ErrHandlerRequired
	}
	if frontendPort == 0 || frontendPort > maxPort {
		return nil, ErrFrontendPortInvalid
	}
	r := &routerImpl{
		handler: h,
		fiberConfig: &fiber.Config{
			BodyLimit:    bodyLimit,
			ErrorHandler: errorHandler,
		},
	}
	r.app = fiber.New(*r.fiberConfig)
	r.app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
		PanicHandler:     panicHandler,
	}))
	r.app.Use(cors.New(cors.Config{
		AllowOriginsFunc: allowOriginOnPort(frontendPort),
		AllowMethods:     []string{fiber.MethodGet, fiber.MethodPost, fiber.MethodPut, fiber.MethodDelete, fiber.MethodOptions},
		AllowHeaders:     []string{fiber.HeaderAuthorization, fiber.HeaderContentType, headerAdminKey},
		ExposeHeaders:    []string{fiber.HeaderRetryAfter},
		MaxAge:           corsMaxAge,
	}))
	r.app.Use(compress.New(compress.Config{Level: compress.LevelBestSpeed}))
	r.registerRoutes()
	return r, nil
}

func (r *routerImpl) ListenAndServeOnPort(ctx context.Context, port uint) chan error {
	errs := make(chan error, 1)
	go func() {
		defer close(errs)
		if err := r.app.Listen(":"+strconv.FormatUint(uint64(port), 10), fiber.ListenConfig{
			GracefulContext:       ctx,
			DisableStartupMessage: true,
		}); err != nil {
			errs <- err
		}
	}()
	return errs
}
