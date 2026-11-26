package web

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type loginRateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	window   time.Duration
	max      int
	now      func() time.Time
}

func newLoginRateLimiter(maxAttempts int, window time.Duration) *loginRateLimiter {
	if maxAttempts <= 0 {
		maxAttempts = 10
	}
	if window <= 0 {
		window = time.Minute
	}
	return &loginRateLimiter{
		attempts: make(map[string][]time.Time),
		window:   window,
		max:      maxAttempts,
		now:      time.Now,
	}
}

func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if len(parts) > 0 {
				ip := strings.TrimSpace(parts[0])
				if ip != "" {
					return ip
				}
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (rl *loginRateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := rl.now()
	cutoff := now.Add(-rl.window)
	times := rl.attempts[key]
	n := 0
	for _, t := range times {
		if t.After(cutoff) {
			times[n] = t
			n++
		}
	}
	times = times[:n]
	if len(times) >= rl.max {
		rl.attempts[key] = times
		return false
	}
	times = append(times, now)
	rl.attempts[key] = times
	return true
}

func (rl *loginRateLimiter) reset(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.attempts, key)
}
