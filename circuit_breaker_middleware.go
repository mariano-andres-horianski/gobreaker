// In a system, service A calls B which in turn calls C.
// This middleware monitors B, and it will return three states:
// Closed, Open, and HalfOpen.
// If the state is closed, it will forward the request to B.
// If the state is open, it will not forward the request to B.
// If the state is halfOpen, it will forward the request to B, but it will not forward the request to C.
// If it detects a number of consecutive failed requests, it switches to open.
// After some time being open, it will switch to halfOpen.
// If it detects a number of consecutive successful requests, it will switch to closed.
package circuit_breaker

import (
	"errors"
	"sync"
	"time"
)

// Generic value that means "anything an hypothetical service returns"
// since it's not like I'm gonna use this anyway
type Value struct{}
type circuit_breaker struct {
	mu               sync.Mutex
	state            string
	threshold        int
	successCount     int
	failureCount     int
	halfOpenCount    int //test request counter during half open state
	halfOpenDuration time.Duration
	timeout          time.Duration
	lastStateChange  time.Time
}

var (
	ErrCircuitOpen         = errors.New("circuit breaker is open")
	ErrTooManyTestRequests = errors.New("too many test requests in half-open state")
	ErrUnknownState        = errors.New("unknown circuit breaker state")
)

func NewCircuitBreaker(threshold int, timeout time.Duration, halfOpenDuration time.Duration) *circuit_breaker {
	return &circuit_breaker{
		mu:               sync.Mutex{},
		state:            "Closed",
		threshold:        threshold,
		successCount:     0,
		failureCount:     0,
		halfOpenCount:    0,
		halfOpenDuration: halfOpenDuration,
		timeout:          timeout,
	}
}

// This function seems pointless for now, might delete it since
// readability is not really improved and so called "good practices" are not necessary in learning projects
func (c *circuit_breaker) shouldTrip() bool {
	return c.failureCount >= c.threshold
}
func (c *circuit_breaker) trip() {
	c.mu.Lock()
	c.state = "Open"
	c.lastStateChange = time.Now()
	c.successCount = 0
	c.failureCount = 0
	c.mu.Unlock()
}

func (c *circuit_breaker) CheckService(operation func() (Value, error)) (Value, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == "Open" {
		if time.Since(c.lastStateChange) > c.timeout {
			c.state = "halfOpen"
			c.halfOpenCount = 0
			for i := 0; i < c.threshold; i++ {
				_, err := operation()
				if err != nil {
					c.failureCount++
				} else {
					c.successCount++
				}
			}
			if c.shouldTrip() {
				c.trip()
			}
		} else {
			return Value{}, ErrCircuitOpen
		}
	}
	if c.state == "Closed" {
		return operation()
	}

	return Value{}, nil
}
