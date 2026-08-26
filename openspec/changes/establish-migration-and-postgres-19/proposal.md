# Establish migration tooling and PostgreSQL 19 re-baseline

## Why

The migration chain is currently broken and unmanaged. A recent merge concatenated two independent
implementations of the household/catalog schema with incompatible table shapes, so a fresh database
fails to bootstrap (it dies at the `household_membership.ended_by` foreign key, which has no column
to bind to). Beyond that, the chain has no migration tooling (raw `psql` via
`docker-entrypoint-initdb.d`), uses 4-digit numbering with a three-way `0010` collision and a missing
`0012`, targets PostgreSQL 16, has no migration runner, and no CI test that a fresh database actually
bootstraps. Pre-release is the only low-cost window to re-baseline all of this before the schema is
depended on.

## What Changes

- Reconcile the conflicting household/catalog schema by keeping the composite-PK design
  (`household_membership`, `person_restriction`, and `unit_conversion` with composite PKs; `unit(code)`;
  `account` with auth fields and an `ended_by` FK) and dropping the conflicting
  `0010_household_catalog.sql`.
- Renumber the chain to a clean, contiguous 6-digit sequence `000001`–`000013`.
- Adopt Goose as the migration tool (`-- +goose Up` / `-- +goose Down` headers, Goose version table).
- Relocate migrations from `migrations/` to `db/migrations/`.
- Bump the database to PostgreSQL 19 (beta for dev/CI, GA expected for production, PostgreSQL 18 as a
  documented fallback).
- Add a migration runner: `food-brain migrate up` / `migrate status` with embedded SQL; a one-shot
  Compose/CI migration service; application startup must not mutate the schema.
- Add a CI job that bootstraps a fresh database from the migration chain and fails on any error.
- Give the seed file (`migrations/seed/ingredient_mappings.sql`) a defined home and apply path.

## Impact

- Affected specs: `database-migrations` (new).
- Affected code: `migrations/` → `db/migrations/`, `docker-compose.yml`,
  `.github/workflows/ci.yml`, `cmd/food-brain` (new `migrate` command), `go.mod` (Goose dependency).
- This change is the substrate for `rebaseline-identity-and-schema-types` (change 2), which re-models
  identity and value types on top of the re-baselined chain.
