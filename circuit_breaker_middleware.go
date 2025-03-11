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
	timeout          time.Duration // Closed state duration
	lastStateChange  time.Time
}

var (
	ErrCircuitOpen         = errors.New("circuit breaker is open")
	ErrTooManyTestRequests = errors.New("too many failed test requests in half-open state")
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

func (c *circuit_breaker) trip() {
	c.mu.Lock()
	c.state = "Open"
	c.lastStateChange = time.Now()
	c.successCount = 0
	c.failureCount = 0
	c.mu.Unlock()
}
func (c *circuit_breaker) reset() {
	c.mu.Lock()
	c.state = "Closed"
	c.lastStateChange = time.Now()
	c.failureCount = 0
	c.mu.Unlock()
}
func (c *circuit_breaker) CheckService(operation func() (Value, error)) (Value, error) {
	// If requests are successful, retrieve the result and keep open or half open the circuit
	// If enough requests are successful in a row in half open state, switch to closed
	// If enough consecutive requests fail, switch to closed
	// If the circuit could not handle the half open state in enough time, switch to open yet again
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state == "Open" {
		if time.Since(c.lastStateChange) > c.timeout {
			c.state = "halfOpen"
			c.halfOpenCount = 0
			val, err := operation()
			if err != nil {
				c.halfOpenCount++
			} else {
				c.successCount++
				// After enough successful requests, switch to closed
				if c.successCount >= c.threshold {
					c.reset()
				}
				return val, nil
			}
			if c.halfOpenCount >= c.threshold || time.Since(c.lastStateChange) > c.halfOpenDuration {
				c.trip()
				return Value{}, ErrTooManyTestRequests
			}
			return Value{}, err
		} else {
			return Value{}, ErrCircuitOpen
		}
	}
	if c.state == "Closed" {
		val, err := operation()
		if err != nil {
			c.failureCount++
			// If it detects a number of consecutive failed requests, it will switch to open.
			if c.failureCount >= c.threshold {
				c.trip()
			}
			return Value{}, err
		} else {
			c.failureCount = 0
			return val, nil
		}
	}

	return Value{}, nil
}
