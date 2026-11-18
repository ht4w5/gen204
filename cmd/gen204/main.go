// Command gen204 serves one fixed 204 Response per connection on the
// address given by -listen.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/ht4w5/gen204/internal/server"
)

func main() {
	listen := flag.String("listen", ":8080", "listen address (host:port)")
	flag.Parse()

	l, err := net.Listen("tcp", *listen)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen204:", err)
		os.Exit(1)
	}
	if err := server.Serve(context.Background(), l); err != nil {
		fmt.Fprintln(os.Stderr, "gen204:", err)
		os.Exit(1)
	}
}
