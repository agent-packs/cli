---
name: observability-and-monitoring
description: Instrument code with structured logs, metrics, and distributed traces. Use when adding observability to services, debugging production issues, or setting up monitoring.
license: Apache-2.0
compatibility: Works with coding agents that support Agent Skills.
metadata:
  agentpacks.version: "0.2.0"
---

# Observability and Monitoring

Instrument first, debug second. Good observability is built into code, not bolted on after incidents.

## Structured Logging

- Use structured (JSON) log output with consistent field names: `level`, `timestamp`, `service`, `trace_id`, `span_id`, `message`.
- Log at the boundary of operations: request received, external call made, result returned.
- Never log secrets, PII, or tokens — redact at the log site, not in a post-processor.
- Use log levels correctly: DEBUG for trace/context, INFO for state transitions, WARN for degraded but recoverable, ERROR for failures needing attention.

## Metrics

- Instrument services with the RED method: Request rate, Error rate, Duration (latency p50/p95/p99).
- For workers and queues use the USE method: Utilization, Saturation, Errors.
- Name metrics with `service_subsystem_unit_total` or `service_subsystem_unit_seconds` conventions.
- Add cardinality-bounded labels: status code, method, route template (not full URL), region. Never use user IDs or request IDs as labels.

## Distributed Tracing

- Propagate trace context (W3C TraceContext or B3) across all service boundaries: HTTP headers, message queue attributes, gRPC metadata.
- Name spans with `verb.noun` conventions: `db.query`, `cache.get`, `http.post`.
- Add span attributes at the point where the context is known.
- Mark spans with error status and record the exception when errors propagate out.

## Alerting and SLOs

- Define SLOs before adding alerts. Alert on SLO burn rate, not raw thresholds.
- Every alert must have a runbook link in its annotation.
- Use multi-window burn rate alerts: short window for fast burn, long window for slow burn.

## Health Endpoints

- `/healthz` returns 200 only when the process is alive (liveness).
- `/readyz` returns 200 only when all critical dependencies (database, cache) are reachable (readiness).
- Log service startup and shutdown with version, commit SHA, and config hash.

## Checklist

- [ ] All external calls wrapped in a span with error and latency attributes.
- [ ] Metrics include RED signals on every service boundary.
- [ ] Trace context propagated across every async boundary.
- [ ] `/healthz` and `/readyz` endpoints implemented and tested.
- [ ] No secrets or PII in log lines.
- [ ] Every alert has a runbook link.
