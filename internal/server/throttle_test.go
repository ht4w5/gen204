package server_test

import (
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/ht4w5/gen204/internal/throttle"
)

// probeTimeout is how long a raw client waits for its 204 Response. It
// is deliberately generous: throttled responses arrive late by design,
// and the tests bound lateness from above only loosely.
const probeTimeout = 30 * time.Second

// probeResult is what one raw client observed: the exact bytes it
// received and when its response arrived, measured from the test's
// start. A non-nil err records a client-side failure.
type probeResult struct {
	response string
	arrived  time.Duration
	err      error
}

// probe dials one connection, sends a plain request, and records what
// came back. A throttled probe blocks until its response slot opens.
func probe(addr string, start time.Time) probeResult {
	got, err := exchange(addr, probeTimeout, sendProbeRequest)
	return probeResult{response: string(got), arrived: time.Since(start), err: err}
}

// sendProbeRequest writes one plain request head, the way an RTT probe
// does.
func sendProbeRequest(c net.Conn) error {
	_, err := io.WriteString(c, "GET / HTTP/1.1\r\nHost: example.test\r\n\r\n")
	return err
}

// runProbes dials n connections concurrently and returns what each
// observed, in dial order.
func runProbes(t *testing.T, addr string, n int) []probeResult {
	t.Helper()
	start := time.Now()
	results := make([]probeResult, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = probe(addr, start)
		}()
	}
	wg.Wait()
	return results
}

// checkResponses asserts every probe got exactly the 204 Response: the
// service throttles, it never sheds.
func checkResponses(t *testing.T, results []probeResult) {
	t.Helper()
	for i, r := range results {
		if r.err != nil {
			t.Errorf("probe %d: %v", i, r.err)
			continue
		}
		if r.response != want204Response {
			t.Errorf("probe %d response = %q, want %q", i, r.response, want204Response)
		}
	}
}

// arrivalSpan reports the earliest and latest response arrivals.
func arrivalSpan(results []probeResult) (earliest, latest time.Duration) {
	earliest = results[0].arrived
	latest = results[0].arrived
	for _, r := range results[1:] {
		if r.arrived < earliest {
			earliest = r.arrived
		}
		if r.arrived > latest {
			latest = r.arrived
		}
	}
	return earliest, latest
}

// spanFor is the pacing span of n responses at the given traffic limit:
// one response slot per 1/rate seconds means n responses span
// ~(n-1)/rate seconds.
func spanFor(n int, rate float64) time.Duration {
	return time.Duration(float64(n-1) / rate * float64(time.Second))
}

// checkPacing asserts the arrivals honor the traffic limit: the
// responses span at least 0.9×(n-1)/rate seconds — one response slot per
// 1/rate seconds, no burst allowance — and the last arrives within
// upper× that span of the start. The 0.9 factor keeps the lower bound
// tolerant of arrival jitter; upper is generous: throttling may only
// delay, never stall.
func checkPacing(t *testing.T, results []probeResult, rate, upper float64) {
	t.Helper()
	earliest, latest := arrivalSpan(results)
	wantSpan := spanFor(len(results), rate)
	if span := latest - earliest; span < wantSpan*9/10 {
		t.Errorf("responses span %v, want at least %v (0.9 * %v pacing bound)", span, wantSpan*9/10, wantSpan)
	}
	if max := time.Duration(upper * float64(wantSpan)); latest > max {
		t.Errorf("latest response arrived after %v, want at most %v (%g * %v pacing bound)", latest, max, upper, wantSpan)
	}
}

func TestTrafficLimitPacesConcurrentProbesSmoothly(t *testing.T) {
	const rate = 4.0 // responses per second
	const probes = 5
	th, err := throttle.New(rate)
	if err != nil {
		t.Fatalf("New(%g): %v", rate, err)
	}
	addr := startServerWithThrottle(t, th)

	results := runProbes(t, addr, probes)
	checkResponses(t, results)

	// N probes at a limit of R span ~(N-1)/R seconds: one response slot
	// per 1/R seconds with no burst allowance, even though the server
	// had been idle.
	checkPacing(t, results, rate, 5)
}

func TestTrafficLimitCapsSequentialProbesToTheCeiling(t *testing.T) {
	const rate = 4.0 // responses per second
	const probes = 5
	th, err := throttle.New(rate)
	if err != nil {
		t.Fatalf("New(%g): %v", rate, err)
	}
	addr := startServerWithThrottle(t, th)

	start := time.Now()
	results := make([]probeResult, probes)
	for i := range probes {
		results[i] = probe(addr, start)
	}
	checkResponses(t, results)

	// One client probing one after another is still held to the same
	// ceiling: N responses at a limit of R span ~(N-1)/R seconds.
	checkPacing(t, results, rate, 5)
}

func TestTrafficLimitHonorsFractionalRates(t *testing.T) {
	const rate = 2.5 // responses per second: one slot per 400ms
	const probes = 5
	th, err := throttle.New(rate)
	if err != nil {
		t.Fatalf("New(%g): %v", rate, err)
	}
	addr := startServerWithThrottle(t, th)

	results := runProbes(t, addr, probes)
	checkResponses(t, results)

	// The upper bound is where fractional rates prove themselves: a
	// floor to whole responses per second would pace at 1/s and take
	// 4s, well past the 2× bound.
	checkPacing(t, results, rate, 2)
}

func TestThrottledProbesAreDelayedNotRejected(t *testing.T) {
	const rate = 3.0 // responses per second
	const probes = 6
	th, err := throttle.New(rate)
	if err != nil {
		t.Fatalf("New(%g): %v", rate, err)
	}
	addr := startServerWithThrottle(t, th)

	results := runProbes(t, addr, probes)
	// Every connection gets its byte-identical 204 Response — nothing
	// shed — even though most probes have to wait for a slot.
	checkResponses(t, results)

	// Delayed, not rejected: the probes must actually have been held
	// back by throttling before their responses went out.
	checkPacing(t, results, rate, 5)
}

func TestTrafficLimitKeepsAcceptingWhileProbesWait(t *testing.T) {
	const rate = 20.0 // responses per second: one slot per 50ms
	const waiting = 24
	const late = 5
	th, err := throttle.New(rate)
	if err != nil {
		t.Fatalf("New(%g): %v", rate, err)
	}
	addr := startServerWithThrottle(t, th)

	start := time.Now()
	results := make([]probeResult, waiting+late)
	var wg sync.WaitGroup
	probeN := func(i int) {
		defer wg.Done()
		results[i] = probe(addr, start)
	}
	for i := range waiting {
		wg.Add(1)
		go probeN(i)
	}
	// Dial the late batch while the first batch is still queued for
	// budget: at one slot per 50ms the first batch needs ~1.2s. Waiting
	// connections are uncapped and the service keeps accepting, so the
	// late dials must connect and be served alongside the queue.
	time.Sleep(100 * time.Millisecond)
	for i := waiting; i < waiting+late; i++ {
		wg.Add(1)
		go probeN(i)
	}
	wg.Wait()

	// 29 connections under one aggregate budget, none capped or shed.
	checkResponses(t, results)
	checkPacing(t, results, rate, 5)
}

func TestTrafficLimitOfZeroIsUnlimited(t *testing.T) {
	tests := []struct {
		name        string
		newThrottle func(t *testing.T) *throttle.Throttle
	}{
		{
			name: "zero rate",
			newThrottle: func(t *testing.T) *throttle.Throttle {
				th, err := throttle.New(0)
				if err != nil {
					t.Fatalf("New(0): %v", err)
				}
				return th
			},
		},
		{
			name:        "no traffic limit at all",
			newThrottle: func(t *testing.T) *throttle.Throttle { return nil },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := startServerWithThrottle(t, tt.newThrottle(t))

			results := runProbes(t, addr, 8)
			checkResponses(t, results)

			_, latest := arrivalSpan(results)
			// Paced at even one per second, eight probes would span
			// seven seconds. The generous bound proves responses were
			// not held at all.
			if latest > 2*time.Second {
				t.Errorf("latest response arrived after %v, want unpaced (< 2s)", latest)
			}
		})
	}
}
