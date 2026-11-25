package frontendHandler

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html"
	"io/fs"
	"math"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
)

var contentTypes = map[string]string{
	".js":          "text/javascript; charset=utf-8",
	".mjs":         "text/javascript; charset=utf-8",
	".css":         "text/css; charset=utf-8",
	".html":        "text/html; charset=utf-8",
	".json":        "application/json; charset=utf-8",
	".webmanifest": "application/manifest+json",
	".svg":         "image/svg+xml",
	".woff2":       "font/woff2",
	".woff":        "font/woff",
	".ico":         "image/x-icon",
	".png":         "image/png",
	".webp":        "image/webp",
	".avif":        "image/avif",
	".txt":         "text/plain; charset=utf-8",
}

var compressible = map[string]bool{
	".js": true, ".mjs": true, ".css": true, ".html": true, ".json": true, ".webmanifest": true, ".svg": true, ".txt": true,
}

func loadAssets(root fs.FS) (map[string]*asset, error) {
	assets := make(map[string]*asset)
	err := fs.WalkDir(root, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || strings.HasPrefix(path.Base(name), ".") {
			return nil
		}
		body, err := fs.ReadFile(root, name)
		if err != nil {
			return err
		}
		cache := cacheRevalidate
		if strings.HasPrefix(name, immutablePrefix) {
			cache = cacheImmutable
		}
		assets[name] = newAsset(strings.ToLower(path.Ext(name)), body, cache)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrAssetsLoadFailed, err)
	}
	return assets, nil
}

func newAsset(extension string, body []byte, cache string) *asset {
	sum := sha256.Sum256(body)
	item := &asset{
		contentType: contentTypeFor(extension),
		body:        body,
		etag:        `"` + hex.EncodeToString(sum[:8]) + `"`,
		cache:       cache,
	}
	if compressible[extension] && len(body) >= minCompressBytes {
		item.gzipped = gzipBytes(body)
	}
	return item
}

func buildIndex(assets map[string]*asset, apiPort string) (*asset, error) {
	index, found := assets[indexFile]
	if !found || len(bytes.TrimSpace(index.body)) == 0 {
		return nil, ErrIndexMissing
	}
	return newAsset(".html", injectMeta(index.body, metaAPIPort, apiPort), cacheRevalidate), nil
}

func contentTypeFor(extension string) string {
	if known, found := contentTypes[extension]; found {
		return known
	}
	if detected := mime.TypeByExtension(extension); detected != "" {
		return detected
	}
	return contentTypeBinary
}

func gzipBytes(body []byte) []byte {
	var buffer bytes.Buffer
	writer, err := gzip.NewWriterLevel(&buffer, gzip.BestCompression)
	if err != nil {
		return nil
	}
	if _, err := writer.Write(body); err != nil {
		return nil
	}
	if err := writer.Close(); err != nil {
		return nil
	}
	if buffer.Len() >= len(body) {
		return nil
	}
	return buffer.Bytes()
}

func cleanRequestPath(raw string) (string, bool) {
	if strings.ContainsRune(raw, 0) || strings.Contains(raw, "\\") {
		return "", false
	}
	for segment := range strings.SplitSeq(raw, "/") {
		if segment == ".." {
			return "", false
		}
	}
	return strings.TrimPrefix(path.Clean("/"+raw), "/"), true
}

func injectMeta(index []byte, name string, value string) []byte {
	meta := []byte(`<meta name="` + name + `" content="` + html.EscapeString(value) + `">`)
	position := bytes.Index(bytes.ToLower(index), []byte("</head>"))
	if position < 0 {
		return append(append([]byte{}, meta...), index...)
	}
	out := make([]byte, 0, len(index)+len(meta))
	out = append(out, index[:position]...)
	out = append(out, meta...)
	return append(out, index[position:]...)
}

func securityPolicy(apiPort string) string {
	api := "http://*:" + apiPort + " https://*:" + apiPort
	return "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: " + api +
		"; font-src 'self'; connect-src 'self' " + api +
		"; manifest-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'"
}

func (h *handler) applySecurityHeaders(c fiber.Ctx) {
	c.Set(fiber.HeaderContentSecurityPolicy, h.csp)
	c.Set(fiber.HeaderXContentTypeOptions, "nosniff")
	c.Set(fiber.HeaderXFrameOptions, "DENY")
	c.Set(fiber.HeaderReferrerPolicy, "no-referrer")
	c.Set("Cross-Origin-Opener-Policy", "same-origin")
	c.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
}

func (h *handler) limit(c fiber.Ctx) (bool, error) {
	ok, retryAfter, err := h.limiter.Allow(groupStatic, strings.Clone(c.IP()))
	if err != nil || ok {
		return ok, err
	}
	seconds := max(1, int(math.Ceil(retryAfter.Seconds())))
	c.Set(fiber.HeaderRetryAfter, strconv.Itoa(seconds))
	return false, plain(c, http.StatusTooManyRequests, textTooMany)
}

func serve(c fiber.Ctx, item *asset) error {
	c.Set(fiber.HeaderETag, item.etag)
	c.Set(fiber.HeaderCacheControl, item.cache)
	if match := c.Get(fiber.HeaderIfNoneMatch); match != "" && strings.Contains(match, item.etag) {
		return c.SendStatus(http.StatusNotModified)
	}
	c.Set(fiber.HeaderContentType, item.contentType)
	body := item.body
	if item.gzipped != nil {
		c.Set(fiber.HeaderVary, fiber.HeaderAcceptEncoding)
		if acceptsGzip(c) {
			c.Set(fiber.HeaderContentEncoding, encodingGzip)
			body = item.gzipped
		}
	}
	return c.Status(http.StatusOK).Send(body)
}

func acceptsGzip(c fiber.Ctx) bool {
	return strings.Contains(strings.ToLower(c.Get(fiber.HeaderAcceptEncoding)), encodingGzip)
}

func plain(c fiber.Ctx, status int, message string) error {
	c.Set(fiber.HeaderContentType, contentTypeText)
	return c.Status(status).SendString(message)
}
