// Command gen204 serves one fixed 204 Response per connection on the
// address given by -listen, paced to the traffic limit given by -rate.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/ht4w5/gen204/internal/server"
	"github.com/ht4w5/gen204/internal/throttle"
)

func main() {
	listen := flag.String("listen", "", "listen address (host:port)")
	rate := flag.Float64("rate", 0, "traffic limit in responses per second (0 = unlimited)")
	flag.Parse()

	// Misconfiguration fails fast: a missing -listen or a bad -rate
	// exits before anything is bound.
	if *listen == "" {
		fmt.Fprintln(os.Stderr, "gen204: -listen is required (host:port)")
		os.Exit(1)
	}
	th, err := throttle.New(*rate)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen204:", err)
		os.Exit(1)
	}

	l, err := net.Listen("tcp", *listen)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen204:", err)
		os.Exit(1)
	}
	ctx := throttle.InContext(context.Background(), th)
	if err := server.Serve(ctx, l); err != nil {
		fmt.Fprintln(os.Stderr, "gen204:", err)
		os.Exit(1)
	}
}
