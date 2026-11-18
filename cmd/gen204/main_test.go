package main_test

import (
	"bytes"
	"errors"
	"net"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildBinary compiles the gen204 binary into a temp dir and returns its
// path.
func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "gen204")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

// runGenFails runs the binary with args and returns its stderr. The run
// must exit non-zero — a spawn failure is not an exit and fails the
// test.
func runGenFails(t *testing.T, bin string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatal("exit status 0, want nonzero")
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("run: %v", err)
	}
	return stderr.String()
}

// TestInvalidRateExitsBeforeListening starts the service with a traffic
// limit that cannot be honored — negative or not-a-number — on an
// address that is already bound: the misconfiguration must surface
// before any listen attempt. Had the service validated after listening,
// the failure would be "address already in use" instead. The assertions
// are wording-free: non-zero exit, an error naming the offending value,
// and no listen error.
func TestInvalidRateExitsBeforeListening(t *testing.T) {
	bin := buildBinary(t)

	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer occupied.Close()

	for _, rate := range []string{"-1", "NaN"} {
		t.Run(rate, func(t *testing.T) {
			stderr := runGenFails(t, bin, "-listen", occupied.Addr().String(), "-rate", rate)
			if !strings.Contains(stderr, rate) {
				t.Errorf("stderr = %q, want it to name the offending value %q", stderr, rate)
			}
			if strings.Contains(stderr, "address already in use") {
				t.Errorf("stderr = %q, want the traffic limit error before any listen attempt", stderr)
			}
		})
	}
}

// TestMissingListenExitsBeforeListening: -listen has no default — the
// service listens on the given address and nothing else — so a run
// without it must fail fast with an error naming the missing flag.
func TestMissingListenExitsBeforeListening(t *testing.T) {
	bin := buildBinary(t)

	stderr := runGenFails(t, bin)
	if !strings.Contains(stderr, "listen") {
		t.Errorf("stderr = %q, want an error naming the missing -listen flag", stderr)
	}
}
