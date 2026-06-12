---
name: database-schema-design
description: Design, evolve, and migrate database schemas safely. Use when creating tables, changing columns, writing migrations, or reviewing data model changes.
license: Apache-2.0
compatibility: Works with coding agents that support Agent Skills.
metadata:
  agentpacks.version: "0.2.0"
---

# Database Schema Design

Migrations are production operations. Design schemas for safety and evolution, not just the happy path.

## Schema Design

- Use surrogate keys (`BIGINT GENERATED ALWAYS AS IDENTITY` or UUID v7) over natural keys unless the natural key is genuinely immutable and globally unique.
- Enforce NOT NULL at the database level for required fields. Nullable means "unknown", not "optional in the app".
- Store money as `NUMERIC(19,4)` or integer cents — never `FLOAT` or `DOUBLE`.
- Store timestamps in UTC: `TIMESTAMPTZ` in PostgreSQL; `DATETIME(6)` + explicit UTC handling elsewhere.
- Normalize to 3NF by default; denormalize only with a written rationale and compensating constraints or triggers.

## Migrations

- Every migration must be reversible (have a rollback) unless data destruction is intentional and documented.
- Never lock large tables under concurrent load. For PostgreSQL:
  - Add nullable columns first (`ADD COLUMN ... DEFAULT NULL`) — lock-free.
  - Backfill in batches with `UPDATE ... WHERE id BETWEEN ...`.
  - Then add the NOT NULL constraint with a `DEFAULT` or check.
- Run migrations before deploying code that depends on the new schema (expand-contract pattern).
- Decouple schema changes and code changes into separate deploys.
- Test migrations on a production-sized data copy before the release window.

## Indexes

- Index every foreign key column (not automatic in most databases).
- Create indexes `CONCURRENTLY` in PostgreSQL to avoid table locks.
- Use partial indexes for low-cardinality conditions: `CREATE INDEX ... WHERE deleted_at IS NULL`.
- Review `EXPLAIN ANALYZE` for every new query on a table with >100k rows before shipping.

## Naming Conventions

- Tables: plural snake_case (`user_accounts`, `order_items`).
- Columns: singular snake_case (`created_at`, `user_id`).
- Foreign keys: `<referenced_table_singular>_id` (`user_id`, `order_id`).
- Constraints: descriptive names (`fk_orders_user_id`, `uq_users_email`, `ck_orders_status`).
- Indexes: `ix_<table>_<columns>` (`ix_orders_user_id_created_at`).

## Checklist

- [ ] Migration has a tested rollback.
- [ ] No column rename in a single step — add new, backfill, deprecate old.
- [ ] All new NOT NULL columns have a database-level default.
- [ ] Large-table changes use lock-free patterns.
- [ ] Every foreign key column is indexed.
- [ ] Migration tested with realistic data volume and query plan reviewed.
