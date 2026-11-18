# 03: Drain contract vs. partial input held open (story 5 tension)

**What to build:** A decision, then (if warranted) a change. Story 5 says "garbage, partial, or empty input to still yield exactly one 204, so that I can probe with arbitrary payloads." The implemented drain contract (spec Implementation Decisions + ADR-0001) ends draining only at the request terminator (`\r\n\r\n`), the small byte cap, or EOF — never a timeout. Consequence: a probe that sends garbage or a partial request and then holds its write side open (i.e. never half-closes) waits indefinitely and never receives its 204. The ticket-01 test table masks this by ending every input variant with a write-side close. Resolving this needs a spec decision: either (a) story 5 is satisfied by "input ends with the client finishing its send" (current behavior is correct — document that probes must half-close when probing with arbitrary payloads), or (b) drain gets a bounded idle deadline so wait-then-read probes with arbitrary payloads get their 204 (at the cost of adding timing variability to such measurements and amending the drain contract in the spec and ADR-0001). Do not silently "fix" the ADR deviations either way.

**Blocked by:** None

**Status:** needs-triage

## Comments

- Surfaced by the spec axis of the code review of `integration/gen204-service` (diff `main...HEAD`). Left unfixed deliberately: the drain end-conditions are binding in the spec's Implementation Decisions and ADR-0001 ("do not 'fix' these"), so changing them is a maintainer decision. The current contract is stated in a lean comment at the `drain` site in `internal/server/server.go`.
