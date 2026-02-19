package utils

import (
	"sync"
	"time"
)

// RateLimiter controls the rate of outgoing requests per domain
type RateLimiter struct {
	mu       sync.Mutex
	lastCall time.Time
	delay    time.Duration
}

// NewRateLimiter creates a new RateLimiter with the given delay in milliseconds
func NewRateLimiter(delayMs int) *RateLimiter {
	return &RateLimiter{
		delay: time.Duration(delayMs) * time.Millisecond,
	}
}

// Wait blocks until enough time has passed since the last request
func (r *RateLimiter) Wait() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastCall)
	if elapsed < r.delay {
		time.Sleep(r.delay - elapsed)
	}
	r.lastCall = time.Now()
}