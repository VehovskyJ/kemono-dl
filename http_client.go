package main

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type RateLimiter struct {
	lastRequestTime time.Time
	minInterval     time.Duration
	mu              sync.Mutex
}

func NewRateLimiter(requestsPerSecond int) *RateLimiter {
	return &RateLimiter{
		minInterval: time.Second / time.Duration(requestsPerSecond),
	}
}

// Wait blocks execution until the minimum interval between requests has elapsed to respect the configured rate limit.
func (rl *RateLimiter) Wait() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	elapsed := time.Since(rl.lastRequestTime)
	if elapsed < rl.minInterval {
		time.Sleep(rl.minInterval - elapsed)
	}
	rl.lastRequestTime = time.Now()
}

var httpClient = &http.Client{
	Timeout: time.Minute * 2,
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	},
}

// Global rate limiter: 1 request per second
var rateLimiter = NewRateLimiter(1)
