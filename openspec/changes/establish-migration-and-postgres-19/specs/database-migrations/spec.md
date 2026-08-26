## ADDED Requirements

### Requirement: Migrations are managed by Goose

The schema SHALL be managed by Goose migrations under `db/migrations/`. Every migration file SHALL
carry `-- +goose Up` and `-- +goose Down` headers. The application SHALL NOT apply raw SQL to the
database at startup.

#### Scenario: A new migration is added
- **WHEN** a schema change is needed
- **THEN** a new `NNNNNN_slug.sql` file with Goose headers is added to `db/migrations/`

#### Scenario: Applied migrations are tracked
- **WHEN** migrations have been applied to a database
- **THEN** the applied versions are recorded in the Goose version table

### Requirement: Migration files use a contiguous 6-digit sequence

Migration files SHALL be named `NNNNNN_slug.sql` with a strictly increasing, contiguous 6-digit prefix
and no gaps or collisions.

#### Scenario: No numbering collision
- **WHEN** the `db/migrations/` directory is inspected
- **THEN** every file has a unique 6-digit prefix and the sequence is contiguous with no gaps

### Requirement: A fresh database bootstraps cleanly

A fresh PostgreSQL database SHALL be fully bootstrappable by applying the migration chain in order with
no errors.

#### Scenario: Fresh bootstrap
- **WHEN** `food-brain migrate up` runs against an empty database
- **THEN** all migrations apply and the expected tables exist

### Requirement: The database targets PostgreSQL 19

The development and CI databases SHALL run PostgreSQL 19 (beta for dev/CI, GA for production), with
PostgreSQL 18 as a documented fallback. The migration chain SHALL apply on both.

#### Scenario: CI bootstraps on PostgreSQL 19
- **WHEN** the CI pipeline runs the migration bootstrap test
- **THEN** it uses a PostgreSQL 19 service and the chain applies without error

### Requirement: A migration runner applies the chain

The CLI SHALL provide `food-brain migrate up` and `food-brain migrate status` that apply embedded Goose
migrations to the configured database. A one-shot Compose/CI migration service SHALL run the runner
before the app starts. Application startup SHALL NOT mutate the schema.

#### Scenario: Runner applies pending migrations
- **WHEN** `food-brain migrate up` is run against a database with pending migrations
- **THEN** the pending Goose migrations are applied in order

#### Scenario: Startup does not migrate
- **WHEN** `food-brain serve` starts while migrations are pending
- **THEN** it does not apply them and reports that `food-brain migrate up` is required

### Requirement: Seeds are applied separately from migrations

Seed data SHALL live under `db/seeds/` and be applied by the runner as an idempotent post-migration
step. Seeds SHALL NOT be numbered migrations and SHALL NOT participate in the Goose version table.

#### Scenario: Seed is idempotent
- **WHEN** the seed step runs twice against the same database
- **THEN** the second run changes nothing (`ON CONFLICT DO NOTHING`)
