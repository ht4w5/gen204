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
	listen := flag.String("listen", ":8080", "listen address (host:port)")
	rate := flag.Float64("rate", 0, "traffic limit in responses per second (0 = unlimited)")
	flag.Parse()

	// Validate the traffic limit before listening, so a bad -rate fails
	// fast instead of leaving a misbehaving server behind.
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
	ctx := server.WithThrottle(context.Background(), th)
	if err := server.Serve(ctx, l); err != nil {
		fmt.Fprintln(os.Stderr, "gen204:", err)
		os.Exit(1)
	}
}
