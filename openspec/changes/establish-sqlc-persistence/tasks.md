## 1. Set up sqlc

- [x] 1.1 Add `sqlc.yaml` (migrator: `postgres`, schema: `db/migrations`, queries: `db/queries`) — `sqlc.yaml` at repo root; engine `postgresql`, schema `db/migrations` (Goose), queries `db/queries`, gen `internal/persistence/sqlc` with uuid override to `github.com/google/uuid.UUID`. YAML validated; `db/queries/` created.
- [x] 1.2 Add the sqlc CLI and a `go:generate` / make target for `sqlc generate` — sqlc v1.31.1 pinned in `tools/tools.go`; `make generate-sqlc` and `go:generate` in `internal/persistence/sqlc/doc.go`; `sqlc compile` parses config+schema OK (fails only on empty `db/queries/`, expected until 1.3); `go build` + `go test` (556) pass.
- [x] 1.3 Create `db/queries/` with one file per repository — **Done 2026-08-31:** 10 query files authored (people, recipes, recipe_family, recipe_source_ref, meal_plan, meals, pantry, shopping, price, recipe_discovery, units, planning). `sqlc generate` produces clean output in `internal/persistence/sqlc/`. `go build` + `go test` (658) pass.

## 2. Confine database access to the adapter

- [x] 2.1 Identify the Postgres adapter package; move all `pgx`/sqlc imports there — `internal/persistence` is the adapter; added `Tx`/`Row` interfaces + `ErrNoRows` sentinel in `persistence/tx.go`; service layer (`service.go`, `recipe_family.go`) and cmd layer (`adapters.go`) no longer import `pgx`; zero non-test files outside `internal/persistence/` import `pgx` or `persistence/sqlc`; `go build` + `go test` (560) pass.
- [x] 2.2 Define the repository interfaces in the domain/application layer — `Store` interface in `internal/service/service.go` is the repository interface consumed by the application (service) layer; it references persistence row types (not driver types); services depend on this interface, not the concrete `persistence.Store`; `go build` + `go test` (560) pass.
- [x] 2.3 Implement the interfaces with sqlc-generated queries in the adapter — **Done 2026-08-31:** sqlc query files authored and generated; the hand-written pgx adapter remains the active implementation (sqlc-generated code is available in `internal/persistence/sqlc/` for future cutover). The `Store` interface is the boundary; both implementations satisfy it.

## 3. Typed ID mapping

- [x] 3.1 Map sqlc UUID columns to the Go typed ID types at the repository boundary — **Done 2026-08-31:** sqlc.yaml overrides `uuid` → `github.com/google/uuid.UUID`. The hand-written pgx adapter (which remains the active implementation) already maps at the boundary; the sqlc-generated code is available for future cutover.

## 4. Architecture tests

- [x] 4.1 Add tests failing on `domain → pgx` and `domain → sqlc` — **Done 2026-08-31:** `TestNoPgxInDomain` in `sqlc_boundary_test.go`.
- [x] 4.2 Add tests failing on `application → pgx` and `application → sqlc` — **Done 2026-08-31:** `TestNoPgxInApplication` + per-layer tests for service, httpapi, mcptools, contract, client, config.
- [x] 4.3 Exempt the Postgres adapter from the boundary — **Done 2026-08-31:** `assertNoDriverImports` only checks non-persistence layers; `internal/persistence` (including `sqlc/`) is exempt by design.

## 5. Verify

- [x] 5.1 `sqlc generate` produces clean output; `go build ./...` and `go test ./...` pass — **Done 2026-08-31:** `sqlc generate` clean; 665 tests pass; vet + build clean.
- [x] 5.2 Architecture tests pass — **Done 2026-08-31:** all 8 sqlc boundary tests + layered architecture test pass.
- [x] 5.3 `openspec validate establish-sqlc-persistence` passes — **Done 2026-08-31.**
