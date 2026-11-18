# 01: Fixed 204 Response for every connection

**What to build:** A user runs `make build`, starts the binary with a listen address, and an RTT probe receives a byte-identical 204 Response followed by a clean connection close — whether it sends an HTTP request, garbage, a partial write, or nothing at all. Client bytes are drained (read and discarded, never parsed) before the response is written, so the probe reliably receives the 204 instead of a connection reset. The response carries no `Date` header and is truly fixed bytes (per ADR-0001). `make test` runs the suite and `make clean` removes build output; the binary lands in the gitignored build directory. Covers the thin entrypoint (flags and wiring), the server package (accept, drain, respond, close), the serve seam over a caller-supplied listener, and the Makefile.

**Blocked by:** None (can start immediately)

**Status:** ready-for-agent

- [ ] `make build` produces a CGO-free binary in the gitignored build directory; `make test` and `make clean` work
- [ ] The binary listens on the address given via the listen flag and serves exactly one response per connection, then closes
- [ ] The response bytes are byte-for-byte identical for an HTTP request, garbage input, a partial write, and an empty (immediate-close) connection
- [ ] The response is the fixed 204 Response with no `Date` header and `Connection: close`, matching ADR-0001
- [ ] Client request bytes are drained before responding; the probe observes the full response and clean EOF, never a reset
- [ ] Behavior tests run through the single serve seam over loopback TCP with an ephemeral port, asserting only external behavior
- [ ] Entrypoint stays thin: flag parsing and wiring only, all behavior testable in library packages
