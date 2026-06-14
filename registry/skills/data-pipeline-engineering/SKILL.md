---
name: data-pipeline-engineering
description: Design and build data pipelines, ETL workflows, and streaming systems. Use when ingesting, transforming, or delivering data at scale.
license: Apache-2.0
compatibility: Works with coding agents that support Agent Skills.
metadata:
  agentpacks.version: "0.1.0"
---

# Data Pipeline Engineering

Pipelines fail silently and accumulate debt quickly. Build in observability, idempotency, and backfill support from day one.

## Pipeline Design

- Make every pipeline step idempotent: re-running with the same inputs produces the same output without duplicates or side effects.
- Prefer append-only output over in-place updates. Updating existing records makes reruns destructive.
- Separate ingestion, transformation, and serving layers. Each layer has a clear schema contract with the next.
- Version your schemas explicitly. Breaking schema changes require a migration path, not silent in-place alteration.

## Batch vs. Streaming

- Choose batch when latency requirements are hourly or longer and source systems provide complete partitions.
- Choose streaming (Kafka, Kinesis, Pub/Sub) when freshness requirements are sub-minute or data arrives continuously.
- For micro-batch (Spark Structured Streaming, Flink), size windows to match downstream SLAs — not to reduce resource cost.
- When switching from batch to streaming, plan for replay: your streaming system must be able to reprocess historical events.

## Data Quality

- Validate row counts, null rates, and value distributions at source and after each transform step. Assert expectations, don't just log them.
- Use a data quality framework (Great Expectations, dbt tests, Soda) rather than ad-hoc `COUNT(*)` checks.
- Define freshness SLAs and alert when a table hasn't been updated within `N` minutes of the expected schedule.
- Quarantine bad records to a dead-letter location for manual review rather than silently dropping them.

## Orchestration

- Model pipelines as DAGs with explicit dependencies (Airflow, Prefect, Dagster, dbt). Avoid cron-based chains where failures are silent.
- Parameterize pipeline runs with execution date and partition key so backfills are reproducible: `run --start-date 2024-01-01 --end-date 2024-01-31`.
- Retry failed tasks automatically with backoff. Set `max_retries` and alert after exhaustion — don't swallow errors silently.
- Use sensors (dataset sensors, file sensors) instead of `sleep()` waits to gate downstream tasks on upstream completion.

## Storage and Partitioning

- Partition large tables by date (event day, load date). Never scan full tables in production queries.
- Use columnar formats (Parquet, ORC) for analytics workloads. Row formats (CSV, JSON) are for ingestion interfaces only.
- Compress at the block level (`snappy` for speed, `zstd` for ratio). Uncompressed Parquet is an antipattern at scale.

## Checklist

- [ ] Every pipeline step is idempotent and safe to rerun.
- [ ] Schema is versioned and backward-compatible changes are documented.
- [ ] Row counts and null rates validated after each transform.
- [ ] Dead-letter queue or quarantine location for invalid records.
- [ ] Pipeline modeled as a DAG with retry logic and alerts on exhaustion.
- [ ] Partitioning and columnar storage used for large tables.
