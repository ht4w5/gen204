# gen204

A minimal HTTP service whose only job is to answer with a fixed 204, built for measuring request round-trip time against it.

## Language

**204 Response**:
The fixed byte sequence the service sends in reply to every connection it accepts. Content-less, header-less beyond the essentials, identical every time.
_Avoid_: reply, page, payload

**RTT probe**:
A client that measures round-trip time by timing a request against gen204. Since connections are never kept alive, a probe's measurement includes the TCP handshake.
_Avoid_: health check, pinger, monitor

**Traffic limit**:
An optional ceiling, expressed in responses per second, on the aggregate rate at which the service emits 204s across all clients at once.
_Avoid_: rate limit, bandwidth cap, quota

**Throttling**:
Holding a response until traffic-limit budget frees up, then sending the 204 anyway. The service throttles; it never sheds requests.
_Avoid_: shedding, dropping (when meaning "delay until affordable")
