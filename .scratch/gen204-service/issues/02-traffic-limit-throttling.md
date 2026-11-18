# 02: Global traffic limit with throttling

**What to build:** An operator starts the service with `-rate <responses/sec>` and the aggregate number of 204 Responses emitted across all clients never exceeds that ceiling. Excess demand is throttled — responses held until budget frees up, then the 204 is sent anyway — never shed, so every RTT probe eventually receives its response, just later. Pacing is smooth (no bursts): never more than one response slot per 1/N seconds. Waiting connections are uncapped and new connections keep being accepted while others wait. The rate flag is a float (fractional values like `2.5` work); absent or `0` means unlimited; a negative value exits with an error before listening.

**Blocked by:** 01: Fixed 204 Response for every connection

**Status:** resolved

- [x] With a traffic limit of N per second, aggregate output across concurrent connections never exceeds the ceiling, enforced smoothly (no burst)
- [x] Throttled probes are delayed, not rejected: every connection still receives its 204 Response
- [x] Concurrent dialers share one aggregate budget rather than getting per-client budgets
- [x] Connections waiting for budget are not capped; the service keeps accepting while throttled
- [x] Fractional rates (e.g. `2.5`) are honored; absent or `0` produces no measurable pacing (unlimited)
- [x] A negative rate exits with a clear error before listening
- [x] Throttle tests use the serve seam over loopback TCP with tolerant real-time bounds (lower bounds proving pacing, generous upper bounds) — never tick-exact assertions

## Comments

- Implemented on `ticket/02-traffic-limit-throttling`. Throttle package: `throttle.New(rate)` (negative/NaN rejected, `0` = unlimited) and a smooth slot reservation (one response slot per 1/N s, no burst, one aggregate budget). The server seam is unchanged: `server.Serve(ctx, net.Listener) error`; `serveConn` throttles after drain and before writing the 204 Response. Entrypoint adds the float `-rate` flag and validates it via `throttle.New` before `net.Listen`. Throttle tests go through the serve seam over loopback TCP with real time and tolerant bounds (0.9× lower pacing bounds, generous upper bounds). The negative-rate checkbox is covered by a test that runs the built binary against an already-bound address, proving the error surfaces before any listen attempt.
- Code-review fix pass (see `fix/code-review-fixes`): pacing now tied to actual emission (`Throttle.Do(f)` serializes sleep→write, next slot from actual emission time), so the ceiling holds in any window even under scheduler oversleep; throttle travels via `throttle.InContext`/`FromContext` so `internal/server` exports only `Serve`; off-seam `throttle.New` unit test removed in favor of binary-level coverage.

## Answer

Resolved on integration branch `integration/gen204-service` (commit `0f6bd5d`, "feat(throttle): global traffic limit with throttling"; review fixes `0bb532d`). All acceptance boxes checked and verified: aggregate smooth traffic limit (no burst, consecutive 204 Responses ≥ 1/N s apart), throttling delays but never sheds, one shared budget across all clients, uncapped waiting connections with the accept loop never blocked, fractional rates honored, absent/`0` unlimited, negative rate exits with a clear error before listening. Throttle tests use the serve seam over loopback TCP with tolerant real-time bounds only.
