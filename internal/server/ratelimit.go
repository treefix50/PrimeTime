package server

import (
	"sync"
	"time"
)

type RateLimiter struct {
	mu          sync.Mutex
	minInterval time.Duration
	lastSeen    map[string]time.Time
}

func NewRateLimiter(minInterval time.Duration) *RateLimiter {
	return &RateLimiter{
		minInterval: minInterval,
		lastSeen:    make(map[string]time.Time),
	}
}

func (r *RateLimiter) Allow(key string) (bool, time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	last, ok := r.lastSeen[key]
	if !ok {
		r.lastSeen[key] = now
		return true, 0
	}
	elapsed := now.Sub(last)
	if elapsed < r.minInterval {
		return false, r.minInterval - elapsed
	}
	r.lastSeen[key] = now
	return true, 0
}
