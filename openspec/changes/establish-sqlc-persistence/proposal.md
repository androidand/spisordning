# Establish sqlc as the default persistence query layer

## Why

Persistence is currently handwritten (raw `pgx` / ad-hoc SQL), and the Go layer has no compile-time
guarantee about which packages may touch the database. The re-baselined schema (changes 1–3) needs a
query layer that is generated from the migrations, type-safe, and confined to the Postgres adapter so
the domain and application layers stay storage-agnostic.

## What Changes

- Adopt sqlc as the default query layer, generating Go from `db/migrations` (the single source of
  truth) plus named queries in `db/queries/*.sql`.
- Confine all database access to the Postgres adapter; the domain and application layers depend on
  repository interfaces, not on `pgx` or sqlc output.
- Keep a narrow, documented exception for handwritten `pgx` inside the adapter where sqlc cannot
  express the query.
- Add architecture tests that fail on `domain → pgx`, `domain → sqlc`, `application → pgx`, and
  `application → sqlc`.
- Map sqlc's generated UUID columns to the Go typed ID types (from changes 2 and 3) at the repository
  boundary.

## Impact

- Affected specs: `persistence-query-layer` (new).
- Affected code: `db/queries/`, `sqlc.yaml`, the Postgres adapter, repository implementations, and the
  architecture tests.
- Depends on `establish-migration-and-postgres-19` (migrations as the source of truth) and the
  identity re-baseline (typed IDs).
