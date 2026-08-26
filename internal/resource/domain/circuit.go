package domain

import (
	"errors"
	"sync"
	"time"
)

var ErrCircuitOpen = errors.New("runtime circuit is open")

type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"
	CircuitOpen     CircuitState = "open"
	CircuitHalfOpen CircuitState = "half_open"
)

type CircuitBreaker struct {
	mu        sync.Mutex
	state     CircuitState
	failures  int
	threshold int
	cooldown  time.Duration
	openedAt  time.Time
}

func NewCircuitBreaker(threshold int, cooldown time.Duration) *CircuitBreaker {
	if threshold < 1 {
		threshold = 1
	}
	return &CircuitBreaker{state: CircuitClosed, threshold: threshold, cooldown: cooldown}
}

func (b *CircuitBreaker) Allow(now time.Time) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == CircuitOpen {
		if now.Sub(b.openedAt) < b.cooldown {
			return ErrCircuitOpen
		}
		b.state = CircuitClosed
	}
	return nil
}

func (b *CircuitBreaker) Success() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.state = CircuitClosed
}

func (b *CircuitBreaker) Failure(now time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == CircuitHalfOpen {
		b.state = CircuitClosed
		b.failures = 0
		return
	}
	b.failures++
	if b.state == CircuitHalfOpen || b.failures >= b.threshold {
		b.state = CircuitOpen
		b.openedAt = now
	}
}

func (b *CircuitBreaker) State() CircuitState {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}
