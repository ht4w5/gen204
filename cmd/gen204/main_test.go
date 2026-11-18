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

// TestNegativeRateExitsBeforeListening builds the binary and starts it
// with a negative -rate on an address that is already bound: the traffic
// limit error must surface first. Had the service validated after
// listening, the failure would be "address already in use" instead.
func TestNegativeRateExitsBeforeListening(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "gen204")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer occupied.Close()

	cmd := exec.Command(bin, "-listen", occupied.Addr().String(), "-rate", "-1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err == nil {
		t.Fatal("exit status 0, want nonzero")
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("run: %v", err)
	}

	out := stderr.String()
	if !strings.Contains(out, "traffic limit must be zero or positive") || !strings.Contains(out, "-1") {
		t.Errorf("stderr = %q, want a clear error naming the bad traffic limit", out)
	}
	if strings.Contains(out, "address already in use") {
		t.Errorf("stderr = %q, want the traffic limit error before any listen attempt", out)
	}
}
