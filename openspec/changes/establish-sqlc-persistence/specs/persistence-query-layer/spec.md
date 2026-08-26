## ADDED Requirements

### Requirement: sqlc is the default query layer

The persistence layer SHALL use sqlc-generated queries as the default. Named queries SHALL be written
in `db/queries/*.sql` and generated from `db/migrations`.

#### Scenario: A new query is added
- **WHEN** a new database query is needed
- **THEN** it is added to `db/queries/*.sql` and generated via sqlc

#### Scenario: Generated code is not hand-edited
- **WHEN** the schema or a query changes
- **THEN** the sqlc output is regenerated, not edited by hand

### Requirement: Database access is confined to the adapter

Only the Postgres adapter package SHALL import sqlc output and `pgx`. Domain and application packages
SHALL depend on repository interfaces and SHALL NOT import the database driver or generated query
code.

#### Scenario: Domain does not import pgx
- **WHEN** the architecture tests run
- **THEN** no domain package imports `pgx` or sqlc output

#### Scenario: Application does not import pgx
- **WHEN** the architecture tests run
- **THEN** no application package imports `pgx` or sqlc output

### Requirement: Handwritten pgx is a documented exception

Handwritten `pgx` SHALL be permitted only inside the Postgres adapter and only where sqlc cannot
express the query, with a comment citing the reason.

#### Scenario: An exception is documented
- **WHEN** a query uses handwritten `pgx`
- **THEN** it is inside the adapter and carries a comment explaining why sqlc cannot express it

### Requirement: Typed IDs are mapped at the repository boundary

The adapter SHALL map sqlc-generated UUID columns to the Go typed ID types at the repository boundary,
so code outside the adapter never handles a raw driver UUID type.

#### Scenario: A repository returns a typed ID
- **WHEN** a repository method returns a recipe or person
- **THEN** it returns the Go typed ID, not a `pgtype.UUID`
