# Spec: gen204 service

Status: resolved

## Problem Statement

I measure request round-trip time to endpoints, but real HTTP servers add noise I don't want: dynamic headers, request parsing, keep-alive semantics, and unpredictable responses. I need a reference endpoint whose reply is so fixed and minimal that every millisecond I measure is network time, not server time — and I need to be able to cap how much traffic the endpoint generates so it can't flood a link during load tests.

## Solution

gen204 is a small Go service that answers every connection with one byte-identical `204 No Content` and then closes. It never parses requests, so nothing the client sends can change what it gets back. An optional global traffic limit throttles responses (delays them, never rejects them) to a configured responses-per-second ceiling. It builds with `make build` into a single static binary.

## User Stories

1. As an RTT probe, I want a 204 response no matter what bytes I send, so that my timing measurement never depends on crafting a valid request.
2. As an RTT probe, I want the response bytes to be identical every time (no `Date`, no dynamic headers), so that response size and server-side work are constant across samples.
3. As an RTT probe, I want the connection cleanly closed after each response, so that my measurement ends at a well-defined point and never hangs.
4. As an RTT probe, I want my request bytes drained before the response is sent, so that I reliably receive the 204 instead of a connection reset.
5. As an RTT probe, I want garbage, partial, or empty input to still yield exactly one 204, so that I can probe with arbitrary payloads.
6. As an RTT probe, I want the service to accept any method, path, or HTTP version without complaint, so that I never have to care about request shape.
7. As an RTT probe, I want a new connection per measurement to include the TCP handshake, so that my sample reflects the full client-visible round trip.
8. As an RTT probe, I want throttled responses to still arrive as 204s (just later), so that throttling never corrupts my samples with failures.
9. As a service operator, I want to configure the listen address and port, so that I can run the service on whichever interface and port the deployment requires.
10. As a service operator, I want an optional global traffic limit in responses per second, so that aggregate output across all clients stays within a known budget.
11. As a service operator, I want the traffic limit absent or `0` to mean unlimited, so that I can run without a cap without changing anything but the flag.
12. As a service operator, I want a negative limit rejected at startup with a clear error, so that misconfiguration fails fast instead of producing a silent misbehaving server.
13. As a service operator, I want fractional limits (e.g. `2.5`) accepted, so that I can express very low probe budgets.
14. As a service operator, I want excess demand to be throttled — responses held until budget frees up — and never shed, so that every client eventually gets its 204 and the cap only affects latency.
15. As a service operator, I want one shared budget across all clients rather than per-client budgets, so that total generated traffic is what's capped.
16. As a service operator, I want responses paced smoothly (never faster than the ceiling in any window) rather than in bursts, so that observed latency under load is uniform and the cap is honest.
17. As a service operator, I want the service to keep accepting connections while others wait on budget, so that a full budget queue never turns into dropped connections.
18. As a service operator, I want the service to listen on the address I give it and nothing else, so that I control its network exposure.
19. As a build engineer, I want `make build` to produce a binary, so that I don't need to know Go toolchain invocations.
20. As a build engineer, I want the binary written to a gitignored build directory, so that build artifacts never pollute the repository.
21. As a build engineer, I want `make test` to run the test suite and `make clean` to remove build output, so that the standard workflow is three make targets and no more.
22. As a build engineer, I want a CGO-free static binary, so that it runs on minimal/distroless targets without libc concerns.
23. As a maintainer, I want the service built on the Go standard library only (no `net/http`, no third-party deps), so that the codebase stays minimal and auditable.
24. As a maintainer, I want request handling to never parse requests, so that there is no request-shaped attack surface at all.
25. As a maintainer, I want the entrypoint to be thin (flags and wiring only), so that all behavior is testable in library packages.
26. As a maintainer, I want the documented deviations (no `Date` header, close-per-connection) recorded as an ADR, so that future readers don't "fix" them.
27. As a maintainer, I want tests to assert only externally visible behavior over a real socket, so that internal refactors don't break the suite.

## Implementation Decisions

- Three modules: a thin entrypoint (flag parsing and wiring only), a server package (listener loop, drain, respond, throttle integration), and a throttle package (the rate pacer).
- The server's public surface is a single serve function taking a context and a `net.Listener` and returning an error. The entrypoint owns `net.Listen`; the server owns everything after accept. This is the one test seam.
- The wire response is a truly fixed byte sequence — status line 204 with `Connection: close` and empty line — with no `Date` header (deliberate RFC 7231 deviation, per ADR-0001). It is sent verbatim to every accepted connection.
- Request handling is drain-then-respond: read and discard client bytes until the request terminator or a small byte cap is reached, never parsing, then write the response and close. Draining first is required — closing with unread socket-buffer bytes causes an RST that can destroy the 204 before the client sees it.
- Connections are one response then close. Keep-alive is deliberately absent so each RTT probe sample includes the TCP handshake (ADR-0001).
- The traffic limit is a single global ceiling on emitted responses per second (the traffic limit in the glossary), applied by throttling: a response is held until budget frees up, then the 204 is sent anyway. The service never sheds requests. There is no cap on waiting connections.
- Pacing is smooth: one response slot per 1/N seconds when the limit is N. No burst allowance.
- Flags: `-listen <addr:port>` for the bind address, `-rate <responses/sec>` as a float for the traffic limit. `-rate` absent or `0` means unlimited; negative exits with an error before listening.
- Build via Makefile with exactly three targets: `build` (static CGO-free binary into a gitignored build directory), `test`, `clean`.
- The traffic limit is expressed in responses per second, not bytes, because the response size is constant; the byte math is trivial and derived.

## Testing Decisions

- Tests assert external behavior only: bytes on a socket and elapsed time, never internal call structure or timing internals.
- All behavior tests go through the single serve seam, over loopback TCP with an ephemeral port and a raw client that writes arbitrary bytes and asserts on the exact response bytes and connection close.
- Throttle tests use real time with tolerant bounds — lower bounds that prove pacing (e.g. N requests at rate R span at least ~(N-1)/R seconds) and generous upper bounds — never tick-exact assertions. If these prove flaky, a deterministic slot seam is the escalation, not before.
- Table-driven style for input variations (HTTP request, garbage, partial write, empty) asserting the same fixed response.
- No prior art exists in this repo (greenfield); these conventions become the reference for future tests.

## Out of Scope

- Keep-alive or connection reuse of any kind
- TLS/HTTPS, HTTP/2, or any protocol upgrade
- Request parsing, routing, methods, status codes other than the fixed 204
- Per-client or per-IP rate limiting (the traffic limit is aggregate-only)
- Shedding/rejection modes (429s, drops) — throttling is the only overload behavior
- Metrics, logging, admin endpoints, health endpoints
- Config files or environment-variable configuration (flags only)
- Lint/install/publish make targets
- Graceful shutdown beyond context cancellation of the accept loop

## Further Notes

- ADR-0001 (TCP-level HTTP without net/http) records the deliberate deviations: no `Date`, close-per-connection, drain-before-respond. Do not "fix" these.
- Glossary vocabulary applies: the fixed reply is the **204 Response**; the ceiling is the **traffic limit** (never "rate limit" in docs or identifiers); overload behavior is **throttling**, never shedding; the service's consumers are **RTT probes**.
- The whole point of the service is measurement purity: any future change that adds per-response variability (dynamic headers, variable server work, keep-alive) defeats its purpose and needs a new ADR at minimum.

## Comments

- Implemented in full on integration branch `integration/gen204-service` (tickets 01 and 02 resolved there; code-review fix pass on both axes merged as `0bb532d`). Follow-up: `.scratch/gen204-service/issues/03-partial-input-drain-contract.md` records the one open spec tension (story 5 vs. the drain contract for a client that holds its write side open).
