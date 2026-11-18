# TCP-level HTTP without net/http, no request parsing

gen204 emits one fixed 204 response and exists to be timed, so we implement the wire protocol directly on `net`: accept a connection, drain-and-discard whatever the client sends (until the request terminator or a byte cap — never parsed), write a truly fixed byte sequence, close. We deliberately use no `net/http` and no request parsing at all.

Considered: `net/http` with an empty handler. Rejected — it brings request parsing, header handling, and dynamic `Date` generation we explicitly don't want, and the requirement is stdlib-only minus `net/http`.

Consequences worth remembering:

- The response is truly fixed bytes with **no `Date` header**, deviating from RFC 7231's SHOULD. All mainstream clients accept this; don't "fix" it.
- Connections close per request, so every RTT measurement includes the TCP handshake. This is intentional for probe semantics; don't add keep-alive without revisiting what probes measure.
- We must drain the request before responding: closing with unread bytes in the socket buffer makes the kernel RST, and the client can lose the 204.
