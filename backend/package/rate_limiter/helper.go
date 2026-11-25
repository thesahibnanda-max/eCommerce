package rateLimiter

import (
	"fmt"
	"math"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"golang.org/x/time/rate"
)

const (
	retryCeiling = time.Minute
)

func newLimiter(cfg Config, now func() (time.Time, error)) (*limiter, error) {
	if err := validate(cfg); err != nil {
		return nil, err
	}
	groups := make(map[string]*bucketGroup, len(cfg.Rules))
	for name, rule := range cfg.Rules {
		groups[name] = &bucketGroup{
			limit:   rate.Limit(rule.RequestsPerMinute / 60),
			burst:   rule.Burst,
			buckets: expirable.NewLRU[string, *rate.Limiter](cfg.CacheSize, nil, cfg.TTL),
		}
	}
	return &limiter{groups: groups, now: now}, nil
}

func validate(cfg Config) error {
	if cfg.CacheSize <= 0 {
		return fmt.Errorf("%w: cache size must be positive", ErrConfigInvalid)
	}
	if cfg.TTL <= 0 {
		return fmt.Errorf("%w: ttl must be positive", ErrConfigInvalid)
	}
	if len(cfg.Rules) == 0 {
		return fmt.Errorf("%w: at least one rule is required", ErrConfigInvalid)
	}
	for name, rule := range cfg.Rules {
		if name == "" {
			return fmt.Errorf("%w: rule name is required", ErrConfigInvalid)
		}
		if rule.RequestsPerMinute <= 0 || math.IsInf(rule.RequestsPerMinute, 0) || math.IsNaN(rule.RequestsPerMinute) {
			return fmt.Errorf("%w: rule %q requests per minute must be positive", ErrConfigInvalid, name)
		}
		if rule.Burst < 1 {
			return fmt.Errorf("%w: rule %q burst must be at least 1", ErrConfigInvalid, name)
		}
		refill := time.Duration(float64(rule.Burst) / rule.RequestsPerMinute * float64(time.Minute))
		if cfg.TTL < refill {
			return fmt.Errorf("%w: ttl must be at least %s to refill rule %q", ErrConfigInvalid, refill, name)
		}
	}
	return nil
}

func (g *bucketGroup) bucket(key string) *rate.Limiter {
	g.mu.Lock()
	defer g.mu.Unlock()
	if b, ok := g.buckets.Get(key); ok {
		return b
	}
	b := rate.NewLimiter(g.limit, g.burst)
	g.buckets.Add(key, b)
	return b
}
