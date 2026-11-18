package server_test

import (
	"context"
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
	ctx = server.WithThrottle(ctx, th)
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

// readToEnd reads until the server closes the connection and returns the
// bytes it received. A clean close ends the read; a reset surfaces as an
// error.
func readToEnd(t *testing.T, c net.Conn) []byte {
	t.Helper()
	if err := c.SetReadDeadline(time.Now().Add(testTimeout)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	got, err := io.ReadAll(c)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	return got
}

func TestServeRespondsWith204ResponseToHTTPRequest(t *testing.T) {
	addr := startServer(t)

	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	if _, err := io.WriteString(c, "GET /index.html HTTP/1.1\r\nHost: example.test\r\n\r\n"); err != nil {
		t.Fatalf("write request: %v", err)
	}

	got := readToEnd(t, c)
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

			c, err := net.Dial("tcp", addr)
			if err != nil {
				t.Fatalf("dial: %v", err)
			}
			defer c.Close()

			if len(tt.input) > 0 {
				if _, err := c.Write(tt.input); err != nil {
					t.Fatalf("write input: %v", err)
				}
			}
			if err := c.(*net.TCPConn).CloseWrite(); err != nil {
				t.Fatalf("close write side: %v", err)
			}

			got := readToEnd(t, c)
			if string(got) != want204Response {
				t.Errorf("response = %q, want %q", got, want204Response)
			}
		})
	}
}

func TestServeDrainsClientBytesBeforeResponding(t *testing.T) {
	addr := startServer(t)

	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	// Write the request head in chunks with the terminator split across
	// writes, and never close the write side: the response must still
	// arrive, and the close must be clean (readToEnd fails on a reset).
	for _, chunk := range []string{"POST /probe HTTP/1.1\r\n", "Host: example.test\r\n", "\r", "\n"} {
		if _, err := io.WriteString(c, chunk); err != nil {
			t.Fatalf("write request chunk: %v", err)
		}
	}

	got := readToEnd(t, c)
	if string(got) != want204Response {
		t.Errorf("response = %q, want %q", got, want204Response)
	}
}

func TestServeAnswersConcurrentConnectionsIdentically(t *testing.T) {
	addr := startServer(t)

	const dialers = 3
	results := make(chan string, dialers)
	for range dialers {
		go func() {
			c, err := net.Dial("tcp", addr)
			if err != nil {
				results <- "dial failed: " + err.Error()
				return
			}
			defer c.Close()
			if _, err := io.WriteString(c, "GET / HTTP/1.1\r\nHost: example.test\r\n\r\n"); err != nil {
				results <- "write failed: " + err.Error()
				return
			}
			if err := c.SetReadDeadline(time.Now().Add(testTimeout)); err != nil {
				results <- "set deadline failed: " + err.Error()
				return
			}
			got, err := io.ReadAll(c)
			if err != nil {
				results <- "read failed: " + err.Error()
				return
			}
			results <- string(got)
		}()
	}

	for range dialers {
		if got := <-results; got != want204Response {
			t.Errorf("response = %q, want %q", got, want204Response)
		}
	}
}
