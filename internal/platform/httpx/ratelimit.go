package httpx

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter limita por clave (IP, celular) en memoria. Decisión provisional AQ-07:
// en memoria al inicio; con varias instancias el límite es por instancia.
type RateLimiter struct {
	mu      sync.Mutex
	limit   rate.Limit
	burst   int
	entries map[string]*entry
	ttl     time.Duration
	lastGC  time.Time
}

type entry struct {
	lim  *rate.Limiter
	seen time.Time
}

// NewRateLimiter permite `perMinute` eventos por minuto con ráfaga `burst`.
func NewRateLimiter(perMinute float64, burst int) *RateLimiter {
	return &RateLimiter{
		limit:   rate.Limit(perMinute / 60),
		burst:   burst,
		entries: map[string]*entry{},
		ttl:     10 * time.Minute,
	}
}

// Allow indica si la clave puede hacer otra solicitud ahora.
func (l *RateLimiter) Allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if now.Sub(l.lastGC) > l.ttl {
		for k, e := range l.entries {
			if now.Sub(e.seen) > l.ttl {
				delete(l.entries, k)
			}
		}
		l.lastGC = now
	}
	e, ok := l.entries[key]
	if !ok {
		e = &entry{lim: rate.NewLimiter(l.limit, l.burst)}
		l.entries[key] = e
	}
	e.seen = now
	return e.lim.AllowN(now, 1)
}
