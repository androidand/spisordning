## 1. Set up sqlc

- [x] 1.1 Add `sqlc.yaml` (migrator: `postgres`, schema: `db/migrations`, queries: `db/queries`) — `sqlc.yaml` at repo root; engine `postgresql`, schema `db/migrations` (Goose), queries `db/queries`, gen `internal/persistence/sqlc` with uuid override to `github.com/google/uuid.UUID`. YAML validated; `db/queries/` created.
- [x] 1.2 Add the sqlc CLI and a `go:generate` / make target for `sqlc generate` — sqlc v1.31.1 pinned in `tools/tools.go`; `make generate-sqlc` and `go:generate` in `internal/persistence/sqlc/doc.go`; `sqlc compile` parses config+schema OK (fails only on empty `db/queries/`, expected until 1.3); `go build` + `go test` (556) pass.
- [ ] 1.3 Create `db/queries/` with one file per repository — NOT YET DONE. The `db/queries/` directory does not exist; no query files have been authored. The sqlc config and Makefile target are in place (1.1/1.2), but query authoring and generation have not started. `go build` + `go test` currently pass with hand-written pgx only.

## 2. Confine database access to the adapter

- [x] 2.1 Identify the Postgres adapter package; move all `pgx`/sqlc imports there — `internal/persistence` is the adapter; added `Tx`/`Row` interfaces + `ErrNoRows` sentinel in `persistence/tx.go`; service layer (`service.go`, `recipe_family.go`) and cmd layer (`adapters.go`) no longer import `pgx`; zero non-test files outside `internal/persistence/` import `pgx` or `persistence/sqlc`; `go build` + `go test` (560) pass.
- [x] 2.2 Define the repository interfaces in the domain/application layer — `Store` interface in `internal/service/service.go` is the repository interface consumed by the application (service) layer; it references persistence row types (not driver types); services depend on this interface, not the concrete `persistence.Store`; `go build` + `go test` (560) pass.
- [ ] 2.3 Implement the interfaces with sqlc-generated queries in the adapter — NOT YET DONE. The `Store` interface exists and is implemented by hand-written pgx. sqlc query files do not yet exist (`db/queries/` is empty/missing).

## 3. Typed ID mapping

- [ ] 3.1 Map sqlc UUID columns to the Go typed ID types at the repository boundary

## 4. Architecture tests

- [ ] 4.1 Add tests failing on `domain → pgx` and `domain → sqlc`
- [ ] 4.2 Add tests failing on `application → pgx` and `application → sqlc`
- [ ] 4.3 Exempt the Postgres adapter from the boundary

## 5. Verify

- [ ] 5.1 `sqlc generate` produces clean output; `go build ./...` and `go test ./...` pass
- [ ] 5.2 Architecture tests pass
- [ ] 5.3 `openspec validate establish-sqlc-persistence` passes
