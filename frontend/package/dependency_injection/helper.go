package frontendDependencyInjection

import (
	rateLimiter "github.com/thesahibnanda-max/eCommerce/backend/package/rate_limiter"
)

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
