package circuitbreaker

import (
	"time"

	"github.com/sony/gobreaker/v2"
)

type Breaker struct {
	cb *gobreaker.CircuitBreaker[any]
}

func New(name string) *Breaker {
	settings := gobreaker.Settings{
		Name:        name,
		MaxRequests: 3,
		Interval:    60 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},
	}

	return &Breaker{
		cb: gobreaker.NewCircuitBreaker[any](settings),
	}
}

func (b *Breaker) Execute(fn func() (any, error)) (any, error) {
	return b.cb.Execute(fn)
}

func (b *Breaker) State() string {
	return b.cb.State().String()
}
