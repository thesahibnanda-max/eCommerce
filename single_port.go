package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
)

const (
	envPort             = "PORT"
	singlePortBodyLimit = 64 << 20
	shutdownTimeout     = 10 * time.Second
)

var (
	apiPrefix       = []byte("/v1")
	clientIPHeaders = []string{"CF-Connecting-IP", "True-Client-IP", fasthttp.HeaderXForwardedFor}
)

var errPortInvalid = errors.New("PORT must be a number between 1 and 65535")

func singlePortFromEnv() (uint, bool, error) {
	raw := strings.TrimSpace(os.Getenv(envPort))
	if raw == "" {
		return 0, false, nil
	}
	port, err := strconv.ParseUint(raw, 10, 16)
	if err != nil || port == 0 {
		return 0, false, fmt.Errorf("%w: %q", errPortInvalid, raw)
	}
	return uint(port), true, nil
}

func newSinglePortHandler(api, store fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		if ip := clientIP(&ctx.Request.Header); ip != nil {
			ctx.SetRemoteAddr(&net.TCPAddr{IP: ip})
		}
		if isAPIPath(ctx.Path()) {
			api(ctx)
			return
		}
		store(ctx)
	}
}

func isAPIPath(path []byte) bool {
	if !bytes.HasPrefix(path, apiPrefix) {
		return false
	}
	return len(path) == len(apiPrefix) || path[len(apiPrefix)] == '/'
}

func clientIP(header *fasthttp.RequestHeader) net.IP {
	for _, name := range clientIPHeaders {
		first, _, _ := strings.Cut(string(header.Peek(name)), ",")
		if ip := net.ParseIP(strings.TrimSpace(first)); ip != nil {
			return ip
		}
	}
	return nil
}

func serveSinglePort(ctx context.Context, port uint, handler fasthttp.RequestHandler) error {
	server := &fasthttp.Server{
		Handler:               handler,
		MaxRequestBodySize:    singlePortBodyLimit,
		NoDefaultServerHeader: true,
	}
	errs := make(chan error, 1)
	go func() {
		errs <- server.ListenAndServe(":" + strconv.FormatUint(uint64(port), 10))
	}()
	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return server.ShutdownWithContext(shutdownCtx)
	}
}
