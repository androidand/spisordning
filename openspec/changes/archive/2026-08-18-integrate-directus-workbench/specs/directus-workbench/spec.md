# directus-workbench (delta)

## ADDED Requirements

### Requirement: Directus SHALL remain optional and removable

The system SHALL continue to function with Directus stopped or entirely absent. No Spisordning service SHALL depend on Directus being running for its own correctness, and no data SHALL exist only inside Directus's own metadata store.

#### Scenario: Stopping Directus does not affect Spisordning

- **WHEN** the Directus instance is stopped
- **THEN** Spisordning's own API and tests continue to pass unaffected
- **AND** no Spisordning request handler returns an error attributable to Directus being down

#### Scenario: Directus absence at first boot is not a failure

- **WHEN** Spisordning's stack is started without Directus configured at all
- **THEN** migrations apply, the service boots, and the API is fully usable

### Requirement: Spisordning SHALL remain the sole migration owner

All schema changes SHALL originate from Spisordning's own `migrations/` directory. Directus SHALL NOT be used to create, alter, or drop tables, columns, or constraints, regardless of its own schema-editing UI capability.

#### Scenario: A schema change is proposed through migrations, not Directus's UI

- **WHEN** a new table or column is needed
- **THEN** it is added via a new file under `migrations/`
- **AND** it is not created or altered through Directus's collection-editing UI

#### Scenario: Directus's own metadata tables do not count as schema drift

- **WHEN** Directus adds its own internal metadata tables (e.g. `directus_*`) to the database
- **THEN** these are recognized as Directus's own bookkeeping, distinct from Spisordning's owned
  schema
- **AND** they are excluded from Spisordning's own migration history

### Requirement: Every Directus-exposed collection SHALL be explicitly classified

Before any table is exposed through Directus, it SHALL be classified as exactly one of `SAFE_DIRECT_CRUD`, `READ_ONLY`, `DOMAIN_CONTROLLED`, or `HIDDEN`. A table classified `DOMAIN_CONTROLLED` or `HIDDEN` SHALL NOT permit direct write access through Directus's generic CRUD UI.

#### Scenario: An unclassified table is not exposed

- **WHEN** a new table exists in `migrations/` with no recorded classification
- **THEN** it is treated as `HIDDEN` by default until explicitly classified otherwise

#### Scenario: A DOMAIN_CONTROLLED table rejects direct Directus writes

- **WHEN** a table classified `DOMAIN_CONTROLLED` (e.g. one backing an append-only history
  invariant) is accessed through Directus
- **THEN** database permissions prevent Directus from writing to it directly
- **AND** any mutation to that table must go through Spisordning's own application layer

#### Scenario: A SAFE_DIRECT_CRUD table permits direct Directus writes

- **WHEN** a table is classified `SAFE_DIRECT_CRUD` (no derived state, no cross-table invariant to
  protect)
- **THEN** Directus may read and write it directly without going through Spisordning's API
