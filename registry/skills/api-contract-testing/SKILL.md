---
name: api-contract-testing
description: Verify API correctness through contract tests, integration tests, and schema validation. Use when building or consuming REST or GraphQL APIs, or when integrating services that must agree on a shared interface.
license: Apache-2.0
compatibility: Works with coding agents that support Agent Skills.
metadata:
  agentpacks.version: "0.1.0"
---

# API Contract Testing

Consumer-driven contract tests prevent integration failures without requiring a live environment.

## Contract Testing Fundamentals

- A contract defines what a consumer expects from a provider: request shape, response shape, and status codes for each interaction.
- Consumer-driven contract tests (Pact, Spring Cloud Contract) let consumers publish expectations and providers verify them independently.
- A provider that passes its own unit tests but fails consumer contracts has a breaking change — even if the provider "works".
- Store contracts in version control and version them with the consumer's release.

## REST API Testing

- Test every status code path, not just 200. A 400 with the wrong body breaks clients as surely as a 500.
- Validate response bodies against a JSON Schema or OpenAPI specification, not just field presence.
- Test pagination: first page, last page, empty page, and page beyond the total.
- Test idempotency for PUT and DELETE: calling twice must produce the same result.
- Test error responses: malformed body, missing required fields, invalid enum values, wrong auth token.

## GraphQL API Testing

- Test all query, mutation, and subscription shapes that clients use — not just the happy path.
- Validate that non-nullable fields are never null and that union types return the correct `__typename`.
- Test depth and complexity limits to verify DoS protection is in place.
- Contract test each operation the consuming app uses — field additions are safe, removals are breaking.

## Schema and OpenAPI Validation

- Generate OpenAPI specs from code (not by hand) to keep spec and implementation in sync.
- Validate request and response bodies against the spec in tests using a spec-aware validator (dredd, schemathesis).
- Run `schemathesis` property-based tests against your spec: it generates edge-case inputs automatically.
- Publish the spec to a developer portal or Confluence after each release so consumers can self-serve.

## Integration Test Patterns

- Use recorded HTTP interactions (VCR, WireMock, Polly.js) for third-party APIs to remove flakiness from external dependencies.
- Run integration tests against a real database and real queues, not in-memory replacements — they behave differently under transaction isolation and message ordering.
- Reset test state between runs: truncate tables or restore from a snapshot. Never let integration tests share persistent state.
- Tag integration tests separately (`@Integration`) and exclude them from unit test runs to keep the feedback loop fast.

## Checklist

- [ ] Consumer contracts published and provider CI verifies them on every build.
- [ ] All documented status codes (2xx, 4xx, 5xx) have corresponding tests.
- [ ] Response bodies validated against schema, not just HTTP status.
- [ ] Idempotency tested for PUT and DELETE endpoints.
- [ ] Third-party API calls mocked with recorded cassettes in unit/integration tests.
- [ ] OpenAPI spec generated from code and validated in CI.
