package rateLimiter

import (
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
	err error
}

func (c *fakeClock) Now() (time.Time, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now, c.err
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func newTestLimiter(t *testing.T, rules map[string]Rule) (*limiter, *fakeClock) {
	t.Helper()
	clock := &fakeClock{now: time.Unix(1_700_000_000, 0)}
	l, err := newLimiter(Config{CacheSize: 100, TTL: 10 * time.Minute, Rules: rules}, clock.Now)
	if err != nil {
		t.Fatalf("newLimiter() error = %v", err)
	}
	return l, clock
}

func Test_NewLimiter_InvalidConfig(t *testing.T) {
	ok := map[string]Rule{"auth": {RequestsPerMinute: 60, Burst: 5}}
	tests := []struct {
		name string
		cfg  Config
	}{
		{name: "zero cache", cfg: Config{CacheSize: 0, TTL: time.Minute, Rules: ok}},
		{name: "zero ttl", cfg: Config{CacheSize: 10, TTL: 0, Rules: ok}},
		{name: "no rules", cfg: Config{CacheSize: 10, TTL: time.Minute}},
		{name: "empty name", cfg: Config{CacheSize: 10, TTL: time.Minute, Rules: map[string]Rule{"": {RequestsPerMinute: 1, Burst: 1}}}},
		{name: "zero rate", cfg: Config{CacheSize: 10, TTL: time.Minute, Rules: map[string]Rule{"a": {RequestsPerMinute: 0, Burst: 1}}}},
		{name: "zero burst", cfg: Config{CacheSize: 10, TTL: time.Minute, Rules: map[string]Rule{"a": {RequestsPerMinute: 1, Burst: 0}}}},
		{name: "ttl shorter than refill", cfg: Config{CacheSize: 10, TTL: time.Minute, Rules: map[string]Rule{"a": {RequestsPerMinute: 1, Burst: 5}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewLimiter(tt.cfg); !errors.Is(err, ErrConfigInvalid) {
				t.Fatalf("NewLimiter() error = %v, want %v", err, ErrConfigInvalid)
			}
		})
	}
}

func Test_Allow_BurstThenRefill(t *testing.T) {
	l, clock := newTestLimiter(t, map[string]Rule{"auth": {RequestsPerMinute: 60, Burst: 3}})
	for i := range 3 {
		ok, _, err := l.Allow("auth", "1.1.1.1")
		if err != nil || !ok {
			t.Fatalf("request %d: ok = %v, err = %v", i, ok, err)
		}
	}
	ok, retry, err := l.Allow("auth", "1.1.1.1")
	if err != nil || ok {
		t.Fatalf("over limit: ok = %v, err = %v", ok, err)
	}
	if retry <= 0 || retry > time.Second {
		t.Fatalf("retry = %v, want (0, 1s]", retry)
	}
	clock.Advance(retry)
	if ok, _, _ := l.Allow("auth", "1.1.1.1"); !ok {
		t.Fatalf("after waiting retry: not allowed")
	}
	if ok, _, _ := l.Allow("auth", "1.1.1.1"); ok {
		t.Fatalf("second request after one refill: allowed")
	}
}

func Test_Allow_DeniedRequestsDoNotConsume(t *testing.T) {
	l, clock := newTestLimiter(t, map[string]Rule{"auth": {RequestsPerMinute: 60, Burst: 1}})
	if ok, _, _ := l.Allow("auth", "k"); !ok {
		t.Fatalf("first request denied")
	}
	for range 20 {
		if ok, _, _ := l.Allow("auth", "k"); ok {
			t.Fatalf("request allowed while empty")
		}
	}
	clock.Advance(time.Second)
	if ok, _, _ := l.Allow("auth", "k"); !ok {
		t.Fatalf("denied requests consumed tokens")
	}
}

func Test_Allow_KeysAndGroupsAreIndependent(t *testing.T) {
	l, _ := newTestLimiter(t, map[string]Rule{
		"auth":    {RequestsPerMinute: 60, Burst: 1},
		"catalog": {RequestsPerMinute: 60, Burst: 1},
	})
	if ok, _, _ := l.Allow("auth", "a"); !ok {
		t.Fatalf("auth a denied")
	}
	if ok, _, _ := l.Allow("auth", "b"); !ok {
		t.Fatalf("auth b denied by a")
	}
	if ok, _, _ := l.Allow("catalog", "a"); !ok {
		t.Fatalf("catalog a denied by auth a")
	}
	if ok, _, _ := l.Allow("auth", "a"); ok {
		t.Fatalf("auth a allowed twice")
	}
}

func Test_Allow_Errors(t *testing.T) {
	l, clock := newTestLimiter(t, map[string]Rule{"auth": {RequestsPerMinute: 60, Burst: 1}})
	if _, _, err := l.Allow("missing", "k"); !errors.Is(err, ErrGroupUnknown) {
		t.Fatalf("unknown group error = %v", err)
	}
	if _, _, err := l.Allow("auth", ""); !errors.Is(err, ErrKeyRequired) {
		t.Fatalf("empty key error = %v", err)
	}
	boom := errors.New("clock")
	clock.err = boom
	if ok, _, err := l.Allow("auth", "k"); ok || !errors.Is(err, boom) {
		t.Fatalf("clock error: ok = %v, err = %v", ok, err)
	}
}

func Test_Allow_Concurrent(t *testing.T) {
	l, _ := newTestLimiter(t, map[string]Rule{"auth": {RequestsPerMinute: 60, Burst: 10}})
	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	for range 50 {
		wg.Go(func() {
			if ok, _, _ := l.Allow("auth", "k"); ok {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	if allowed != 10 {
		t.Fatalf("allowed = %d, want 10", allowed)
	}
}

func Test_NewLimiter_Public(t *testing.T) {
	l, err := NewLimiter(Config{CacheSize: 10, TTL: time.Minute, Rules: map[string]Rule{"a": {RequestsPerMinute: 60, Burst: 1}}})
	if err != nil {
		t.Fatalf("NewLimiter() error = %v", err)
	}
	if ok, _, err := l.Allow("a", "k"); err != nil || !ok {
		t.Fatalf("Allow() = %v, %v", ok, err)
	}
}

func Test_HasGroup(t *testing.T) {
	l, _ := newTestLimiter(t, map[string]Rule{"auth": {RequestsPerMinute: 60, Burst: 1}})
	if !l.HasGroup("auth") || l.HasGroup("cart") {
		t.Fatalf("HasGroup() mismatch")
	}
}
