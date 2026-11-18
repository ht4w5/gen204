// Package throttle paces 204 Responses to a traffic limit: one response
// slot per 1/N seconds at a limit of N responses per second. Excess
// demand is throttled — held until a slot frees up — never shed, and the
// budget is aggregate: one Throttle covers every connection at once.
package throttle

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// Throttle hands out response slots under a traffic limit.
type Throttle struct {
	interval time.Duration // spacing between response slots; 0 = unlimited
	mu       sync.Mutex
	next     time.Time // when the next response slot opens
}

// New returns a Throttle pacing 204 Responses to the traffic limit rate
// (responses per second). A rate of 0 means unlimited: the returned
// Throttle never delays a response. Rates that are negative or
// not-a-number are invalid.
func New(rate float64) (*Throttle, error) {
	if math.IsNaN(rate) || rate < 0 {
		return nil, fmt.Errorf("traffic limit must be zero or positive (got %g)", rate)
	}
	t := &Throttle{}
	if rate > 0 {
		switch ns := float64(time.Second) / rate; {
		case ns >= math.MaxInt64:
			t.interval = time.Duration(math.MaxInt64) // a limit this low paces effectively forever
		default:
			t.interval = time.Duration(ns)
		}
	}
	return t, nil
}

// Do runs f at the next response slot and opens the following slot one
// interval after f actually ran — not after its wake-up — so emitted
// responses are never closer than 1/N seconds, even if a waiter
// oversleeps its slot. No burst allowance: idle time never banks slots.
// Do never rejects — it only delays. A nil or unlimited Throttle just
// runs f.
func (t *Throttle) Do(f func() error) error {
	if t == nil || t.interval <= 0 {
		return f()
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if d := time.Until(t.next); d > 0 {
		time.Sleep(d)
	}
	err := f()
	t.next = time.Now().Add(t.interval)
	return err
}

// throttleKey carries the Throttle through a context. The Throttle
// travels this way so the serve seam keeps the single
// Serve(ctx, listener) error signature the spec fixes, instead of
// growing a throttle parameter.
type throttleKey struct{}

// InContext returns a copy of ctx carrying t. A nil t means no traffic
// limit.
func InContext(ctx context.Context, t *Throttle) context.Context {
	return context.WithValue(ctx, throttleKey{}, t)
}

// FromContext returns the Throttle carried by ctx, or nil for no
// traffic limit.
func FromContext(ctx context.Context) *Throttle {
	th, _ := ctx.Value(throttleKey{}).(*Throttle)
	return th
}
