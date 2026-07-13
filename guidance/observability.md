# Observability Guidance

## Observability Baseline

When changes touch logging, metrics, tracing, or error handling:

### Logging
- Use structured logs with stable keys (request/correlation ID, user/tenant, operation); avoid free-form concatenation.
- Log errors once at the boundary with stack traces; avoid swallowing exceptions or logging the same failure repeatedly.
- Redact secrets/PII (tokens, cookies, signed URLs) and bound logging in loops to prevent noise.

### Tracing
- Propagate context (MDC/OpenTelemetry) across executors/virtual threads; ensure new threads carry correlation IDs.
- Annotate spans or events at key boundaries (outbound calls, retries, queue operations) for debuggability.

### Metrics
- Add counters/timers for new outcomes (success, client error, server error, timeout, retry) with clear names and tags.
- Prefer histograms/timers over ad-hoc logging for latency; keep metric cardinality bounded.
