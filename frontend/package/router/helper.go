package frontendRouter

import (
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v3"
)

const (
	contentTypeText = "text/plain; charset=utf-8"
)

func (r *routerImpl) registerRoutes() {
	r.app.Get("/", r.handler.ServeStatic)
	r.app.Get("/*", r.handler.ServeStatic)
}

func panicHandler(c fiber.Ctx, _ any) error {
	return respondText(c, http.StatusInternalServerError)
}

func errorHandler(c fiber.Ctx, err error) error {
	if fiberErr, ok := errors.AsType[*fiber.Error](err); ok {
		return respondText(c, fiberErr.Code)
	}
	return respondText(c, http.StatusInternalServerError)
}

func respondText(c fiber.Ctx, status int) error {
	c.Set(fiber.HeaderContentType, contentTypeText)
	return c.Status(status).SendString(http.StatusText(status))
}
