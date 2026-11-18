// Package server answers every accepted connection with one fixed 204
// Response and then closes it. Requests are drained and discarded, never
// parsed (ADR-0001).
package server

import (
	"bytes"
	"context"
	"net"
)

// response204 is the fixed 204 Response: a 204 status line, Connection:
// close, and an empty line — no Date header (deliberate RFC 7231
// deviation, ADR-0001). It is sent verbatim on every connection.
var response204 = []byte("HTTP/1.1 204 No Content\r\nConnection: close\r\n\r\n")

// requestTerminator marks the end of a request head. The stream is never
// parsed; draining only scans for these bytes.
var requestTerminator = []byte("\r\n\r\n")

// drainCap bounds how many client bytes are read and discarded per
// connection before responding anyway.
const drainCap = 8 << 10

// Serve accepts connections on l and answers each one with a single 204
// Response followed by a close. It serves until ctx is cancelled or Accept
// fails; on cancellation Serve closes l and returns nil.
func Serve(ctx context.Context, l net.Listener) error {
	served := make(chan struct{})
	defer close(served)
	go func() {
		select {
		case <-ctx.Done():
			l.Close()
		case <-served:
		}
	}()
	for {
		c, err := l.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go serveConn(c)
	}
}

// serveConn drains the client's bytes and then sends the 204 Response.
func serveConn(c net.Conn) {
	defer c.Close()
	drain(c)
	c.Write(response204)
}

// drain reads and discards client bytes until the request terminator, the
// byte cap, or a read error (including EOF) — never parsing.
func drain(c net.Conn) {
	buf := make([]byte, 1024)
	var drained []byte
	for len(drained) < drainCap {
		n, err := c.Read(buf)
		if n > 0 {
			drained = append(drained, buf[:n]...)
			if bytes.Contains(drained, requestTerminator) {
				return
			}
		}
		if err != nil {
			return
		}
	}
}
