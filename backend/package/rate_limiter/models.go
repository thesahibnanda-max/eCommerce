package rateLimiter

import (
	"time"
)

type Rule struct {
	RequestsPerMinute float64
	Burst             int
}

type Config struct {
	CacheSize int
	TTL       time.Duration
	Rules     map[string]Rule
}
