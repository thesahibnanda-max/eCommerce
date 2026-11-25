package frontendDependencyInjection

import (
	"io/fs"

	rateLimiter "github.com/thesahibnanda-max/eCommerce/backend/package/rate_limiter"
	frontendHandler "github.com/thesahibnanda-max/eCommerce/frontend/package/handler"
)

func newHandler(files fs.FS, apiPort uint, sameOriginAPI bool, limiter rateLimiter.Limiter) (frontendHandler.Handler, error) {
	if sameOriginAPI {
		return frontendHandler.NewSameOriginHandler(files, limiter)
	}
	return frontendHandler.NewHandler(files, apiPort, limiter)
}

func validatePort(port uint) error {
	if port == 0 || port > maxPort {
		return ErrServerPortInvalid
	}
	return nil
}

func limiterConfig(cfg rateLimitConfig) rateLimiter.Config {
	rules := make(map[string]rateLimiter.Rule, len(cfg.Rules))
	for name, rule := range cfg.Rules {
		rules[name] = rateLimiter.Rule{RequestsPerMinute: rule.RequestsPerMinute, Burst: rule.Burst}
	}
	return rateLimiter.Config{CacheSize: cfg.CacheSize, TTL: cfg.TTL, Rules: rules}
}
