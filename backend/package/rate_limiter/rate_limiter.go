package rateLimiter

import (
	"sync"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
	"golang.org/x/time/rate"
)

type Limiter interface {
	Allow(group string, key string) (bool, time.Duration, error)
	HasGroup(group string) bool
}

type bucketGroup struct {
	mu      sync.Mutex
	limit   rate.Limit
	burst   int
	buckets *expirable.LRU[string, *rate.Limiter]
}

type limiter struct {
	groups map[string]*bucketGroup
	now    func() (time.Time, error)
}

func NewLimiter(cfg Config) (Limiter, error) {
	return newLimiter(cfg, utils.TimeNow)
}

func (l *limiter) Allow(group string, key string) (bool, time.Duration, error) {
	g, ok := l.groups[group]
	if !ok {
		return false, 0, ErrGroupUnknown
	}
	if key == "" {
		return false, 0, ErrKeyRequired
	}
	now, err := l.now()
	if err != nil {
		return false, 0, err
	}
	reservation := g.bucket(key).ReserveN(now, 1)
	if !reservation.OK() {
		return false, retryCeiling, nil
	}
	delay := reservation.DelayFrom(now)
	if delay > 0 {
		reservation.CancelAt(now)
		return false, delay, nil
	}
	return true, 0, nil
}

func (l *limiter) HasGroup(group string) bool {
	_, ok := l.groups[group]
	return ok
}
