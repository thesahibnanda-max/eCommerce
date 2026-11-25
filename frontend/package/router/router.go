package frontendRouter

import (
	"context"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
	frontendHandler "github.com/thesahibnanda-max/eCommerce/frontend/package/handler"
	"github.com/valyala/fasthttp"
)

const (
	bodyLimit = 4 << 10
)

type Router interface {
	ListenAndServeOnPort(ctx context.Context, port uint) chan error
	Handler() fasthttp.RequestHandler
}

type routerImpl struct {
	handler frontendHandler.Handler
	app     *fiber.App
}

func NewRouter(h frontendHandler.Handler) (Router, error) {
	if h == nil {
		return nil, ErrHandlerRequired
	}
	r := &routerImpl{
		handler: h,
		app: fiber.New(fiber.Config{
			BodyLimit:    bodyLimit,
			ErrorHandler: errorHandler,
		}),
	}
	r.app.Use(recover.New(recover.Config{PanicHandler: panicHandler}))
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

func (r *routerImpl) Handler() fasthttp.RequestHandler {
	return r.app.Handler()
}
