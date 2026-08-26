## 1. Reconcile the household/catalog conflict

- [x] 1.1 Drop `migrations/0010_household_catalog.sql` (ours: conflicting `BIGSERIAL`
      `household_membership`, `unit(id)`, no `ended_by`)
- [x] 1.2 Keep theirs: `0010_migrate_persons_to_household.sql` and `0011_household_and_catalog.sql`
      (composite PKs, `unit(code)`, `account` auth fields, `ended_by` FK)
- [x] 1.3 Audit Go code for references to dropped columns/shape (`unit.id`, `household_membership.id`,
      `person_restriction.id`); update to `unit.code` / composite keys

## 2. Renumber to a contiguous 6-digit sequence

- [x] 2.1 Relocate `migrations/*.sql` → `db/migrations/`
- [x] 2.2 Rename to `000001`–`000013` per the `design.md` mapping (drop the `0010` collision, fill the
      missing `0012`)
- [x] 2.3 Move `migrations/seed/` → `db/seeds/`

## 3. Adopt Goose

- [x] 3.1 Add `-- +goose Up` / `-- +goose Down` headers to every migration
- [x] 3.2 Wrap multi-statement bodies (`DO $$ … END $$`, `CREATE FUNCTION`) in
      `-- +goose StatementBegin` / `StatementEnd` where required
- [x] 3.3 Add the Goose Go dependency to `go.mod`

## 4. Bump to PostgreSQL 19

- [x] 4.1 `docker-compose.yml`: `postgres:19beta3-alpine`
- [x] 4.2 CI: PostgreSQL 19 service for the bootstrap test
- [x] 4.3 Document the PG19 GA target and the PG18 fallback
- [x] 4.4 Verify the chain applies on both PG19 and PG18

## 5. Migration runner

- [x] 5.1 Add `food-brain migrate up` / `migrate status` with embedded SQL (`embed.FS`)
- [x] 5.2 Add a one-shot Compose/CI migration service that runs `migrate up` before the app
- [x] 5.3 `serve` reads the Goose version table and refuses to start (clear message) when migrations are
      pending; it never mutates the schema
- [x] 5.4 Seed step: apply `db/seeds/` idempotently after migrations (`migrate up --seed` or `seed`)

## 6. CI fresh-bootstrap test

- [x] 6.1 CI job: fresh PG19 → `food-brain migrate up` → assert success + expected table count
- [x] 6.2 Fail the pipeline on any migration error

## 7. Verify

- [x] 7.1 Fresh PG19 bootstrap: 13 migrations, expected table count, no errors
- [x] 7.2 `go build ./...` and `go test ./...` pass
- [x] 7.3 `openspec validate establish-migration-and-postgres-19` passes
