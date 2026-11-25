package frontendDependencyInjection

import (
	"time"
)

type configStruct struct {
	Server struct {
		Port uint `yaml:"port"`
	} `yaml:"server"`
	Backend struct {
		APIPort uint `yaml:"apiPort"`
	} `yaml:"backend"`
	RateLimit rateLimitConfig `yaml:"rateLimit"`
}

type rateLimitConfig struct {
	CacheSize int                      `yaml:"cacheSize"`
	TTL       time.Duration            `yaml:"ttl"`
	Rules     map[string]rateLimitRule `yaml:"rules"`
}

type rateLimitRule struct {
	RequestsPerMinute float64 `yaml:"requestsPerMinute"`
	Burst             int     `yaml:"burst"`
}
