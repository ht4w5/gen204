package throttle_test

import (
	"math"
	"strings"
	"testing"

	"github.com/ht4w5/gen204/internal/throttle"
)

// TestNewRejectsInvalidTrafficLimit covers the startup contract: a
// negative traffic limit is invalid and must fail fast with a clear
// error, so the service can refuse to start instead of running with a
// silent misbehaving pacer.
func TestNewRejectsInvalidTrafficLimit(t *testing.T) {
	for _, rate := range []float64{-1, -0.5, -math.MaxFloat64, math.NaN()} {
		th, err := throttle.New(rate)
		if err == nil {
			t.Errorf("New(%g) = %v, want error", rate, th)
			continue
		}
		if th != nil {
			t.Errorf("New(%g) returned both a Throttle and an error", rate)
		}
		if !strings.Contains(err.Error(), "zero or positive") {
			t.Errorf("New(%g) error = %q, want it to name the valid range", rate, err)
		}
	}
}
