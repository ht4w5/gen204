// Package throttle paces 204 Responses to a traffic limit: one response
// slot per 1/N seconds at a limit of N responses per second. Excess
// demand is throttled — held until a slot frees up — never shed, and the
// budget is aggregate: one Throttle covers every connection at once.
package throttle

import (
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

// Wait blocks until the traffic limit releases the next response slot.
// Slots are spaced one interval apart with no burst allowance: idle
// time never banks extra slots. Wait never rejects — it only delays —
// so every caller eventually proceeds. With an unlimited Throttle
// (rate 0) Wait returns immediately.
func (t *Throttle) Wait() {
	if t.interval <= 0 {
		return
	}
	t.mu.Lock()
	slot := time.Now()
	if t.next.After(slot) {
		slot = t.next
	}
	t.next = slot.Add(t.interval)
	t.mu.Unlock()
	if d := time.Until(slot); d > 0 {
		time.Sleep(d)
	}
}
