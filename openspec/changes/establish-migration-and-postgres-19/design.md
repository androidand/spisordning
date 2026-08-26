# Design: migration tooling and PostgreSQL 19 re-baseline

## Context

- The migration chain is broken. A merge concatenated two independent implementations of the
  household/catalog schema:
  - `0010_household_catalog.sql` (ours): `household_membership` with `BIGSERIAL id` PK, `unit(id TEXT)`,
    `person_restriction` with `BIGSERIAL id` PK, no `ended_by`.
  - `0010_migrate_persons_to_household.sql` + `0011_household_and_catalog.sql` (theirs):
    `household_membership` with composite PK `(household_id, person_id)` and an `ended_by` column,
    `unit(code TEXT)`, `person_restriction` with composite PK, `account` with auth fields.
- Verified against a fresh PostgreSQL 16: the merged chain **fails** at
  `0011_household_and_catalog.sql` — `column "ended_by" referenced in foreign key constraint does not
  exist`.
- Verified: the **reconciled** chain (drop ours `0010_household_catalog.sql`, keep theirs) applies
  cleanly — 13 migrations, 53 tables, no errors.
- No migration tooling, 4-digit numbering with a `0010` collision and missing `0012`, PostgreSQL 16,
  no migration runner, no CI fresh-bootstrap test.

## Decisions

### D1: Reconcile the household/catalog conflict by keeping the composite-PK design

Keep theirs (`0010_migrate_persons_to_household.sql` + `0011_household_and_catalog.sql`):
- `household_membership` composite PK `(household_id, person_id)` with `joined_at`/`ended_at`/`ended_by`.
- `person_restriction` composite PK `(person_id, tag, kind)`.
- `unit(code TEXT PK)`, `unit_conversion` composite PK `(from_unit, to_unit)`.
- `account` with `username`/`email`/`password_hash`/`auth_method`/`person_id`/`last_login_at`.
- `household_membership.ended_by` FK to `account(id)`.

Drop ours (`0010_household_catalog.sql`): `BIGSERIAL` `household_membership`, `unit(id)`, no `ended_by`.

Rationale: theirs is more complete (auth fields, `ended_by`, `same_dimension` CHECK) and its
composite-PK style aligns with the no-surrogate identity model that change 2 establishes. Change 1 only
makes the chain coherent; change 2 re-models identity further (UUIDv7 where appropriate).

### D2: Target renumbering (6-digit, contiguous)

| target                    | source                              | content |
|---------------------------|-------------------------------------|---------|
| `000001_init`             | `0001_init`                         | core domain (person, ingredient, recipe_ref, meal_plan, meal_event, …) |
| `000002_recipe_discovery` | `0002_recipe_discovery`             | recipe import candidate + ingredients |
| `000003_recipe_family`    | `0003_recipe_family`                | recipe family / effort profile |
| `000004_shopping_list`    | `0004_shopping_list`                | shopping list + items |
| `000005_retailer_list_binding` | `0005_retailer_list_binding`   | retailer list binding |
| `000006_shopping_cart`    | `0006_shopping_cart`                | shopping cart + items |
| `000007_order`            | `0007_order`                        | order + order items |
| `000008_household_catalog_minimal` | `0008_household_catalog_minimal` | household, product, product_identifier, product_ingredient_mapping |
| `000009_pantry_inventory` | `0009_pantry_inventory`             | inventory_location, inventory_lot, inventory_event |
| `000010_household_membership` | `0010_migrate_persons_to_household` | household_membership + default household backfill |
| `000011_household_and_catalog` | `0011_household_and_catalog`    | account, person_restriction, unit, unit_conversion, ingredient_form, ingredient_substitution, `same_dimension` |
| `000012_meals_and_preferences` | `0010_meals_and_preferences`  | meal_participant, meal_review, favorite, meal_event plan columns |
| `000013_price_intelligence` | `0013_price_intelligence`         | retailer, store, retailer_product, store_product_offer, price_observation, current-price view |

Ordering constraint: `000010_household_membership` must precede `000011_household_and_catalog` (the
latter adds the `ended_by` FK). `000012_meals_and_preferences` is independent and slots after `000011`.

### D3: Adopt Goose

- Every migration carries `-- +goose Up` and `-- +goose Down` headers.
- Multi-statement bodies use `-- +goose StatementBegin` / `-- +goose StatementEnd` where required
  (e.g. the `DO $$ … END $$` backfill blocks, `CREATE FUNCTION`).
- Applied versions are tracked in Goose's `goose_db_version` table.
- Down migrations are best-effort for a pre-release re-baseline: the supported path is fresh bootstrap.
  Provide `Down` where straightforward; mark genuinely non-invertible steps `-- +goose NOROLLBACK`.

### D4: Relocate to `db/migrations`

- `migrations/*.sql` → `db/migrations/*.sql`.
- `migrations/seed/` → `db/seeds/` (see D8).
- No second hand-written `schema.sql`; sqlc (change 4) consumes `db/migrations` as the source of truth.

### D5: PostgreSQL 19

- `docker-compose.yml`: `postgres:19beta3-alpine` for local dev.
- CI: a `postgres:19beta3` service for the bootstrap test.
- Production target: PostgreSQL 19 GA; PostgreSQL 18 is the documented fallback.
- The chain must apply on both PG19 and PG18 (no PG19-only features are required by the current schema).

### D6: Migration runner

- `food-brain migrate up` and `food-brain migrate status`: embedded SQL via `embed.FS`, applying Goose
  migrations to the database named by `POSTGRES_*` / `DATABASE_URL`.
- A one-shot Compose/CI migration service runs `food-brain migrate up` before the app starts.
- `food-brain serve` must not mutate the schema at startup; it may read `goose_db_version` and refuse to
  start (with a clear message) when migrations are pending.

### D7: CI fresh-bootstrap test

- A CI job spins up a fresh PostgreSQL 19, runs `food-brain migrate up`, and asserts success plus an
  expected table count. Any migration error fails the pipeline.

### D8: Seed handling

- `db/seeds/ingredient_mappings.sql` is applied by the runner as a post-migration seed step
  (`food-brain migrate up --seed`, or a separate `food-brain seed`), idempotently (`ON CONFLICT DO
  NOTHING`). Seeds are not numbered migrations and do not participate in `goose_db_version`.

## Risks / trade-offs

- Dropping `0010_household_catalog.sql` removes the `unit(id)` shape; any code referencing `unit.id`
  must move to `unit.code` (task 1.3 audits this).
- Goose adds a Go dependency; accepted as the standard migration tooling.
- PG19 beta in dev/CI: pin the exact beta tag and document the GA cutover so the bump is a one-line change.

## Open questions

- Exact PG19 beta tag to pin (currently `19beta3`).
- Whether `migrate` is a `food-brain` subcommand (leaning yes) or a separate `cmd/migrate` binary.
