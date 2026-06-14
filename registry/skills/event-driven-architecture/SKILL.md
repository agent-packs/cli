---
name: event-driven-architecture
description: Design and implement event-driven systems using message queues, event streams, and async patterns. Use when decoupling services, building reactive systems, or processing high-throughput data streams.
license: Apache-2.0
compatibility: Works with coding agents that support Agent Skills.
metadata:
  agentpacks.version: "0.1.0"
---

# Event-Driven Architecture

Events are facts about what happened. They cannot be undone — design for that.

## Event Design

- Name events in past tense: `OrderPlaced`, `PaymentFailed`, `UserEmailUpdated`. An event is a fact, not a command.
- Include all context needed to process the event without calling back to the source. A consumer that must re-query the producer to understand an event creates implicit coupling.
- Version events from day one: `{ "type": "OrderPlaced", "version": 2, "data": {...} }`. You will need it.
- Keep events small and focused. A `UserUpdated` event with 40 fields couples all consumers to all user data — prefer `UserEmailUpdated`, `UserAddressChanged`.
- Include `eventId` (UUID), `occurredAt` (ISO 8601 UTC), `correlationId`, and `causationId` on every event envelope.

## Producers

- Publish events atomically with the state change. Use the outbox pattern (write event to DB in the same transaction as the state change, then relay to the broker) to guarantee delivery.
- Never publish directly to a broker inside a database transaction — the transaction may roll back but the message is already sent.
- Guarantee at-least-once delivery. Consumers must be idempotent.

## Consumers

- Make every consumer idempotent: processing the same event twice must produce the same result. Use `eventId` as an idempotency key stored in a deduplication table.
- Use explicit consumer group names. Auto-generated group names lose their offset position on restart.
- Commit offsets only after successful processing, never before. Pre-committing loses events on consumer crash.
- Dead-letter failed messages after `N` retries. Alert on DLQ depth — a growing DLQ is a production incident.
- Process messages in order within a partition — do not parallelize within a partition unless the consumer is explicitly designed for it.

## Kafka-Specific

- Choose the partition key deliberately: same entity ID → same partition → ordered processing for that entity.
- Compact topics (log compaction) for "current state" use cases; retain-by-time for event logs.
- Monitor consumer lag per topic-partition. Lag growing over time indicates a consumer can't keep up.
- Set `enable.auto.commit=false` and commit manually after processing.

## Schema Registry and Compatibility

- Register event schemas in a schema registry (Confluent, AWS Glue). Enforce compatibility mode: `BACKWARD` for most cases (new fields optional), `FULL` when both sides evolve independently.
- `BACKWARD` compatibility: new schema can read messages written by old schema. Means: only add optional fields, never remove fields or change types.
- Generate producer and consumer code from schemas, not by hand. Manual schema-to-code mapping drifts.

## Checklist

- [ ] Events named in past tense with `eventId`, `occurredAt`, `correlationId`, and `causationId`.
- [ ] Outbox pattern used to guarantee at-least-once delivery.
- [ ] All consumers are idempotent with deduplication on `eventId`.
- [ ] Offsets committed only after successful processing.
- [ ] Dead-letter queue configured with alerts on depth.
- [ ] Event schemas registered and compatibility mode enforced.
