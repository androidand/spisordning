## 1. Set up sqlc

- [ ] 1.1 Add `sqlc.yaml` (migrator: `postgres`, schema: `db/migrations`, queries: `db/queries`)
- [ ] 1.2 Add the sqlc CLI and a `go:generate` / make target for `sqlc generate`
- [ ] 1.3 Create `db/queries/` with one file per repository

## 2. Confine database access to the adapter

- [ ] 2.1 Identify the Postgres adapter package; move all `pgx`/sqlc imports there
- [ ] 2.2 Define the repository interfaces in the domain/application layer
- [ ] 2.3 Implement the interfaces with sqlc-generated queries in the adapter

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
