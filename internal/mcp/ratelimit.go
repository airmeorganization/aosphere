package mcp

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrRateLimitExceeded   = errors.New("rate limit exceeded: max requests per minute threshold reached")
	ErrConcurrencyExceeded = errors.New("concurrency limit exceeded: too many simultaneous executions for agent")
)

type RateLimiter struct {
	mu           sync.Mutex
	maxPerMinute int
	maxInFlight  int
	history      map[string][]time.Time
	inFlight     map[string]int
}

func NewRateLimiter(maxPerMinute, maxInFlight int) *RateLimiter {
	if maxPerMinute <= 0 {
		maxPerMinute = 60
	}
	if maxInFlight <= 0 {
		maxInFlight = 5
	}
	return &RateLimiter{
		maxPerMinute: maxPerMinute,
		maxInFlight:  maxInFlight,
		history:      make(map[string][]time.Time),
		inFlight:     make(map[string]int),
	}
}

// Acquire reserves an execution slot. Returns a release function that must be called when done.
func (r *RateLimiter) Acquire(agentKey string) (func(), error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	oneMinAgo := now.Add(-1 * time.Minute)

	// 1. Prune timestamps older than 1 minute
	var recent []time.Time
	for _, t := range r.history[agentKey] {
		if t.After(oneMinAgo) {
			recent = append(recent, t)
		}
	}

	// 2. Check rate limit
	if len(recent) >= r.maxPerMinute {
		r.history[agentKey] = recent
		return nil, fmt.Errorf("%w: limit is %d/min", ErrRateLimitExceeded, r.maxPerMinute)
	}

	// 3. Check concurrency
	currentInFlight := r.inFlight[agentKey]
	if currentInFlight >= r.maxInFlight {
		return nil, fmt.Errorf("%w: active %d >= max %d", ErrConcurrencyExceeded, currentInFlight, r.maxInFlight)
	}

	// Reserve slot
	recent = append(recent, now)
	r.history[agentKey] = recent
	r.inFlight[agentKey] = currentInFlight + 1

	released := false
	release := func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		if !released {
			r.inFlight[agentKey]--
			if r.inFlight[agentKey] < 0 {
				r.inFlight[agentKey] = 0
			}
			released = true
		}
	}

	return release, nil
}
