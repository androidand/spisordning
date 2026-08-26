# Design: sqlc as the default persistence query layer

## Context

- Changes 1–3 re-baseline the schema, identity, and value types. The query layer must be generated
  from the migrations and confined to the Postgres adapter.
- Today persistence is handwritten; there is no compile-time boundary on which packages may import the
  database driver.

## Decisions

### D1: sqlc generates from `db/migrations` + `db/queries`

- `sqlc.yaml` sets the migrator to `postgres`, the schema to `db/migrations`, and the queries to
  `db/queries/*.sql`.
- Generated Go is a build artifact (not hand-edited); a `go:generate` / make target runs `sqlc
  generate`.
- `db/queries/*.sql` holds named queries, one file per repository.

### D2: All database access is confined to the Postgres adapter

- The adapter package is the only package that imports sqlc output and `pgx`.
- The domain and application layers depend on repository interfaces defined in (or consumed by) those
  layers; they never name a concrete table, query, or driver type.

### D3: Handwritten `pgx` is a narrow, documented exception

- Where sqlc cannot express a query (dynamic SQL, complex multi-statement transactions), handwritten
  `pgx` is allowed only inside the adapter, with a comment citing the reason.

### D4: Architecture tests enforce the boundary

- Tests fail when a `domain` package imports `pgx` or sqlc output, or when an `application` package
  imports `pgx` or sqlc output. The adapter is exempt.

### D5: Typed IDs flow through the repository boundary

- sqlc generates `UUID` columns as `pgtype.UUID` / `[16]byte`. The adapter maps these to the Go typed
  ID types (from changes 2 and 3) at the repository boundary, so the rest of the code never sees a raw
  `pgtype.UUID`.

## Risks / trade-offs

- sqlc adds a codegen step to the build; the generated output must be committed or generated in CI.
- Some dynamic queries (e.g. the per-run Mealie re-sync in `food-brain plan`) may require the `pgx`
  exception.

## Open questions

- Exact adapter package path and where the repository interfaces live (domain vs application).
- Whether sqlc uses its built-in `postgres` migrator against `db/migrations` or a flattened schema
  snapshot.
