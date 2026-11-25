package frontendHandler

import (
	"io/fs"
	"net/http"
	"path"
	"strconv"

	"github.com/gofiber/fiber/v3"
	rateLimiter "github.com/thesahibnanda-max/eCommerce/backend/package/rate_limiter"
)

const (
	groupStatic       = "static"
	indexFile         = "index.html"
	maxPort           = 65535
	minCompressBytes  = 512
	metaAPIPort       = "ai-api-port"
	sameOriginAPI     = "same-origin"
	cacheImmutable    = "public, max-age=31536000, immutable"
	cacheRevalidate   = "no-cache"
	immutablePrefix   = "assets/"
	encodingGzip      = "gzip"
	textBadRequest    = "Bad Request"
	textNotFound      = "Not Found"
	textTooMany       = "Too Many Requests"
	contentTypeText   = "text/plain; charset=utf-8"
	contentTypeBinary = "application/octet-stream"
)

type Handler interface {
	ServeStatic(c fiber.Ctx) error
}

type handler struct {
	assets  map[string]*asset
	index   *asset
	csp     string
	limiter rateLimiter.Limiter
}

func NewHandler(files fs.FS, apiPort uint, limiter rateLimiter.Limiter) (Handler, error) {
	if files == nil {
		return nil, ErrFilesRequired
	}
	if apiPort == 0 || apiPort > maxPort {
		return nil, ErrAPIPortInvalid
	}
	return newHandler(files, strconv.FormatUint(uint64(apiPort), 10), limiter)
}

func NewSameOriginHandler(files fs.FS, limiter rateLimiter.Limiter) (Handler, error) {
	if files == nil {
		return nil, ErrFilesRequired
	}
	return newHandler(files, sameOriginAPI, limiter)
}

func newHandler(files fs.FS, api string, limiter rateLimiter.Limiter) (Handler, error) {
	if limiter == nil {
		return nil, ErrRateLimiterRequired
	}
	if !limiter.HasGroup(groupStatic) {
		return nil, ErrRateLimitGroupMissing
	}
	assets, err := loadAssets(files)
	if err != nil {
		return nil, err
	}
	index, err := buildIndex(assets, api)
	if err != nil {
		return nil, err
	}
	return &handler{assets: assets, index: index, csp: securityPolicy(api), limiter: limiter}, nil
}

func (h *handler) ServeStatic(c fiber.Ctx) error {
	h.applySecurityHeaders(c)
	if allowed, err := h.limit(c); err != nil || !allowed {
		return err
	}
	requested, ok := cleanRequestPath(string(c.Request().URI().Path()))
	if !ok {
		return plain(c, http.StatusBadRequest, textBadRequest)
	}
	if requested != "" && requested != indexFile {
		if item, found := h.assets[requested]; found {
			return serve(c, item)
		}
		if path.Ext(requested) != "" {
			return plain(c, http.StatusNotFound, textNotFound)
		}
	}
	return serve(c, h.index)
}
