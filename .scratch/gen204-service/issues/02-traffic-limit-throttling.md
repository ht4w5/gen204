# 02: Global traffic limit with throttling

**What to build:** An operator starts the service with `-rate <responses/sec>` and the aggregate number of 204 Responses emitted across all clients never exceeds that ceiling. Excess demand is throttled — responses held until budget frees up, then the 204 is sent anyway — never shed, so every RTT probe eventually receives its response, just later. Pacing is smooth (no bursts): never more than one response slot per 1/N seconds. Waiting connections are uncapped and new connections keep being accepted while others wait. The rate flag is a float (fractional values like `2.5` work); absent or `0` means unlimited; a negative value exits with an error before listening.

**Blocked by:** 01: Fixed 204 Response for every connection

**Status:** ready-for-agent

- [ ] With a traffic limit of N per second, aggregate output across concurrent connections never exceeds the ceiling, enforced smoothly (no burst)
- [ ] Throttled probes are delayed, not rejected: every connection still receives its 204 Response
- [ ] Concurrent dialers share one aggregate budget rather than getting per-client budgets
- [ ] Connections waiting for budget are not capped; the service keeps accepting while throttled
- [ ] Fractional rates (e.g. `2.5`) are honored; absent or `0` produces no measurable pacing (unlimited)
- [ ] A negative rate exits with a clear error before listening
- [ ] Throttle tests use the serve seam over loopback TCP with tolerant real-time bounds (lower bounds proving pacing, generous upper bounds) — never tick-exact assertions
