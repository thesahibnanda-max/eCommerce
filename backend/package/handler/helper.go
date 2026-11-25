package handler

import (
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	httpErrorHandler "github.com/thesahibnanda-max/eCommerce/backend/package/handler/internal/error_handler"
	rateLimiter "github.com/thesahibnanda-max/eCommerce/backend/package/rate_limiter"
	adminService "github.com/thesahibnanda-max/eCommerce/backend/package/services/admin"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

func (h *handler) fail(c fiber.Ctx, err error) error {
	if status, resp, ok := h.errors.HandleError(err); ok {
		return respond(c, status, resp)
	}
	return respond(c, http.StatusInternalServerError, httpErrorHandler.Response{Code: codeInternalError, Message: messageInternalError})
}

func (h *handler) limit(c fiber.Ctx, group string) error {
	ok, retryAfter, err := h.limiter.Allow(group, strings.Clone(c.IP()))
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	seconds := retryAfterSeconds(retryAfter)
	c.Set(fiber.HeaderRetryAfter, strconv.Itoa(seconds))
	return httpErrorHandler.ErrRateLimited(seconds)
}

func retryAfterSeconds(d time.Duration) int {
	return max(1, int(math.Ceil(d.Seconds())))
}

func checkRateLimitGroups(limiter rateLimiter.Limiter) error {
	for _, group := range rateLimitGroups {
		if !limiter.HasGroup(group) {
			return fmt.Errorf("%w: %s", ErrRateLimitGroupMissing, group)
		}
	}
	return nil
}

func param(c fiber.Ctx, name string) string {
	return strings.Clone(c.Params(name))
}

func respond(c fiber.Ctx, status int, v any) error {
	body, err := utils.FromObjectToStringJSON(v)
	if err != nil {
		return err
	}
	return c.Status(status).Type("json").SendString(body)
}

func respondCached(c fiber.Ctx, v any) error {
	c.Set(fiber.HeaderCacheControl, catalogCacheControl)
	return respond(c, http.StatusOK, v)
}

func decodeBody[T any](c fiber.Ctx) (T, error) {
	v, err := utils.FromStringToObjectJSON[T](string(c.Body()))
	if err != nil {
		var zero T
		return zero, httpErrorHandler.ErrBodyInvalid(err)
	}
	return v, nil
}

func bearerToken(c fiber.Ctx) (string, error) {
	header := strings.TrimSpace(c.Get(fiber.HeaderAuthorization))
	scheme, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, bearerScheme) || strings.TrimSpace(token) == "" {
		return "", httpErrorHandler.ErrUnauthorizedHeader(fiber.HeaderAuthorization)
	}
	return strings.TrimSpace(token), nil
}

func adminKey(c fiber.Ctx) (string, error) {
	key := strings.TrimSpace(c.Get(headerAdminKey))
	if key == "" {
		return "", httpErrorHandler.ErrUnauthorizedHeader(headerAdminKey)
	}
	return key, nil
}

func queryInt(c fiber.Ctx, name string, fallback int) (int, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, httpErrorHandler.ErrQueryInvalid(name, err)
	}
	return v, nil
}

func queryInt64Ptr(c fiber.Ctx, name string) (*int64, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return nil, nil
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, httpErrorHandler.ErrQueryInvalid(name, err)
	}
	return &v, nil
}

func queryBool(c fiber.Ctx, name string) (bool, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return false, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false, httpErrorHandler.ErrQueryInvalid(name, err)
	}
	return v, nil
}

func queryList(c fiber.Ctx, name string) []string {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return nil
	}
	var out []string
	for part := range strings.SplitSeq(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func page(c fiber.Ctx) (int, int, error) {
	offset, err := queryInt(c, "offset", 0)
	if err != nil {
		return 0, 0, err
	}
	limit, err := queryInt(c, "limit", defaultLimit)
	if err != nil {
		return 0, 0, err
	}
	return offset, limit, nil
}

func readImage(fh *multipart.FileHeader, metadata map[string]any) (adminService.ImageInput, error) {
	f, err := fh.Open()
	if err != nil {
		return adminService.ImageInput{}, httpErrorHandler.ErrFileInvalid(err)
	}
	defer f.Close()
	content, err := io.ReadAll(f)
	if err != nil {
		return adminService.ImageInput{}, httpErrorHandler.ErrFileInvalid(err)
	}
	return adminService.ImageInput{FileName: strings.Clone(fh.Filename), Content: content, Metadata: metadata}, nil
}

func formMetadata(c fiber.Ctx) (map[string]any, error) {
	raw := strings.Clone(strings.TrimSpace(c.FormValue(fieldMetadata)))
	if raw == "" {
		return nil, nil
	}
	m, err := utils.FromStringToObjectJSON[map[string]any](raw)
	if err != nil {
		return nil, httpErrorHandler.BadRequest(codeMetadataInvalid, "metadata must be a json object", err)
	}
	return m, nil
}
