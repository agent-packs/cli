---
name: load-testing
description: Design and run load, stress, and soak tests to validate system performance under realistic and peak traffic. Use before major releases, after architecture changes, or when setting SLOs.
license: Apache-2.0
compatibility: Works with coding agents that support Agent Skills.
metadata:
  agentpacks.version: "0.1.0"
---

# Load Testing

Load tests are experiments, not one-time checks. Define a hypothesis, measure, and iterate.

## Test Design

- Test against production-like infrastructure, not a scaled-down staging environment. Results from smaller environments don't transfer.
- Use realistic request distributions from production access logs — not uniform random traffic. 80% reads / 20% writes is a different test than 50/50.
- Warm up the system before measuring: send traffic at 10% of target load for 2–5 minutes before ramping to full load.
- Include think time between requests (1–5 seconds per virtual user) to match browser and API client behavior.
- Script tests at the scenario level: login, browse, add-to-cart, checkout — not individual endpoints in isolation.

## Test Types

- **Load test** — ramp to expected peak, hold for 10–30 minutes, observe steady-state behavior. Goal: confirm SLO compliance under normal load.
- **Stress test** — increase load beyond peak until the system degrades or fails. Goal: identify the breaking point and failure mode.
- **Soak test** — run at sustained normal load for 8–24 hours. Goal: catch memory leaks, connection pool exhaustion, log fill, and gradual degradation.
- **Spike test** — sudden 5–10× traffic increase over 30–60 seconds. Goal: verify auto-scaling reacts in time and the system recovers cleanly.

## Tooling

- **k6** — JavaScript-based, CI-friendly, outputs InfluxDB/Prometheus/CSV metrics natively.
- **Locust** — Python, distributed, good for complex user journeys.
- **Gatling** — Scala/DSL, high throughput, HTML reports.
- **Artillery** — YAML/JS, good for HTTP and WebSocket.

Pick one tool per project and commit to it — mixing tools makes trend comparison impossible.

## Metrics and Assertions

- Assert on p95 and p99 latency, not just average. Averages hide tail latency.
- Assert on error rate: `error_rate < 0.1%` under load test, `error_rate < 1%` under stress test.
- Capture throughput (requests/second), active virtual users, and saturation signals (CPU, connection pool usage).
- Export raw metrics to a time-series store (InfluxDB, Prometheus) so you can overlay with deployment markers.

## CI Integration

- Run a smoke load test (2–5 minutes, 10% of peak) in CI on every merge to main.
- Run full load tests in a pre-production environment before production deploys.
- Gate deploys on load test results — a 30% p99 latency regression should block release.
- Store test results and baselines in version control or a metrics store. Regression is meaningless without a baseline.

## Checklist

- [ ] Test runs against production-like infrastructure.
- [ ] Realistic traffic distribution derived from production logs.
- [ ] p95 and p99 assertions defined, not just averages.
- [ ] At least: load test, soak test, and spike test scenarios defined.
- [ ] Metrics exported to a time-series backend.
- [ ] Baseline results committed and compared on each run.
