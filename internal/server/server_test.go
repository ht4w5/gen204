package server_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/ht4w5/gen204/internal/server"
	"github.com/ht4w5/gen204/internal/throttle"
)

// want204Response is the fixed 204 Response defined by ADR-0001: a 204
// status line, Connection: close, no Date header, and nothing else. It is
// an independent literal: the tests must not derive it from the server.
const want204Response = "HTTP/1.1 204 No Content\r\nConnection: close\r\n\r\n"

const testTimeout = 2 * time.Second

// startServer runs server.Serve on an ephemeral loopback port and stops it
// when the test ends.
func startServer(t *testing.T) string {
	t.Helper()
	return startServerWithThrottle(t, nil)
}

// startServerWithThrottle is startServer with the aggregate traffic
// limit th in effect (nil: no traffic limit).
func startServerWithThrottle(t *testing.T, th *throttle.Throttle) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	ctx = throttle.InContext(ctx, th)
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, l) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Serve returned error after context cancellation: %v", err)
			}
		case <-time.After(testTimeout):
			t.Error("Serve did not return after context cancellation")
		}
	})
	return l.Addr().String()
}

// exchange runs one raw-client round trip: it dials addr, hands the open
// connection to send for writing, then reads until the server closes the
// connection and returns the bytes. A reset surfaces as an error, as does
// a response that does not arrive within timeout.
func exchange(addr string, timeout time.Duration, send func(net.Conn) error) ([]byte, error) {
	c, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	defer c.Close()
	if err := send(c); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}
	if err := c.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return nil, fmt.Errorf("set read deadline: %w", err)
	}
	got, err := io.ReadAll(c)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	return got, nil
}

func TestServeRespondsWith204ResponseToHTTPRequest(t *testing.T) {
	addr := startServer(t)

	got, err := exchange(addr, testTimeout, func(c net.Conn) error {
		_, err := io.WriteString(c, "GET /index.html HTTP/1.1\r\nHost: example.test\r\n\r\n")
		return err
	})
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if string(got) != want204Response {
		t.Errorf("response = %q, want %q", got, want204Response)
	}
}

func TestServeSendsIdentical204ResponseForAnyInput(t *testing.T) {
	// Each input is ended with a write-side close: draining ends at the
	// request terminator, the byte cap, or EOF. The empty case is the
	// immediate-close connection from the ticket.
	tests := []struct {
		name  string
		input []byte
	}{
		{
			name:  "http request",
			input: []byte("GET /index.html HTTP/1.1\r\nHost: example.test\r\n\r\n"),
		},
		{
			name:  "garbage",
			input: []byte("\x00\xff not http at all \x9cB"),
		},
		{
			name:  "partial write",
			input: []byte("GET / HTT"),
		},
		{
			name:  "empty",
			input: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := startServer(t)

			got, err := exchange(addr, testTimeout, func(c net.Conn) error {
				if len(tt.input) > 0 {
					if _, err := c.Write(tt.input); err != nil {
						return err
					}
				}
				return c.(*net.TCPConn).CloseWrite()
			})
			if err != nil {
				t.Fatalf("probe: %v", err)
			}
			if string(got) != want204Response {
				t.Errorf("response = %q, want %q", got, want204Response)
			}
		})
	}
}

func TestServeDrainsClientBytesBeforeResponding(t *testing.T) {
	addr := startServer(t)

	// Write the request head in chunks with the terminator split across
	// writes, and never close the write side: the response must still
	// arrive, and the close must be clean (a reset surfaces as an error).
	got, err := exchange(addr, testTimeout, func(c net.Conn) error {
		for _, chunk := range []string{"POST /probe HTTP/1.1\r\n", "Host: example.test\r\n", "\r", "\n"} {
			if _, err := io.WriteString(c, chunk); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if string(got) != want204Response {
		t.Errorf("response = %q, want %q", got, want204Response)
	}
}

func TestServeAnswersConcurrentConnectionsIdentically(t *testing.T) {
	addr := startServer(t)

	results := runProbes(t, addr, 3)
	checkResponses(t, results)
}
