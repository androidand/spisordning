## ADDED Requirements

### Requirement: Canonical identity is UUIDv7 for domain entities

Every non-recipe domain entity SHALL have a `UUIDv7` primary key generated in Go. The database
SHALL NOT be the source of canonical identity (no `BIGSERIAL` surrogate, no slug-as-PK) for a
domain entity.

#### Scenario: A domain entity row has a UUIDv7 primary key
- **WHEN** a row is inserted into `person`, `product`, `meal_event`, or `shopping_requirement`
- **THEN** its `id` is a `UUIDv7` value assigned by Go, not a serial or slug

#### Scenario: No domain entity uses a surrogate serial
- **WHEN** the schema is inspected after the re-baseline
- **THEN** no non-recipe domain entity table uses `BIGSERIAL` as its primary key

### Requirement: Human-addressable entities keep a unique slug

Entities that humans address by name SHALL carry a `slug TEXT NOT NULL UNIQUE` column in addition
to their UUIDv7 primary key. The slugged entities are `person`, `ingredient`, `household`,
`product`, and `inventory_location`; the slug SHALL be stable and human-readable, and the UUID
SHALL be the referential identity.

#### Scenario: A product is referenced by UUID but shown by slug
- **WHEN** a relationship column references a product
- **THEN** it stores the product's UUID, and the product's `slug` is used for display

#### Scenario: Slugs are unique
- **WHEN** two rows attempt to use the same slug on `ingredient`
- **THEN** the second insert is rejected by the unique constraint

### Requirement: Foreign systems are external references, not identity

References to foreign systems SHALL be stored in domain-specific external-reference tables keyed
by `(provider, external_id)` with a foreign key to the canonical entity. The schema SHALL NOT use
a generic polymorphic `entity_type`/`entity_id` shape for external references.

#### Scenario: A Mealie food id maps to an ingredient
- **WHEN** a Mealie food is matched to an ingredient
- **THEN** a row exists in `ingredient_external_ref` with `provider='mealie'`, the
  `external_id`, and an `ingredient_id` foreign key to the canonical ingredient

#### Scenario: External references are unique per provider
- **WHEN** the same `(provider, external_id)` is inserted twice
- **THEN** the second insert is rejected by the primary key

### Requirement: Pure relationships use composite primary keys

Tables that represent a pure relationship with no independent lifecycle SHALL use a composite
primary key of the parent keys and SHALL NOT carry a surrogate serial.

#### Scenario: Meal attendance is composite
- **WHEN** a person is recorded as present at a meal
- **THEN** the `meal_participant` row is keyed by `(meal_event_id, person_id)` with no surrogate id

#### Scenario: A relationship row cannot duplicate
- **WHEN** the same `(meal_event_id, person_id)` is inserted twice into `meal_participant`
- **THEN** the second insert is rejected by the composite primary key

### Requirement: Stable codes remain stable codes

Small named registries keyed by a human code (`effort_profile` by weekday) SHALL keep their stable
code as the primary key. `external_recipe_source` SHALL be validated and either kept as a
stable-code primary key or converted to UUIDv7 + slug, with the decision recorded in this change.

#### Scenario: Effort profile is keyed by weekday
- **WHEN** an effort profile is stored
- **THEN** its primary key is the `weekday` code (0-6), not a UUID

### Requirement: Strongly-typed Go IDs

The domain layer SHALL define named types wrapping `uuid.UUID` for each entity (for example
`PersonID`, `ProductID`, `IngredientID`). Repository interfaces and domain services SHALL use
these types so that a call passing the wrong entity's id type does not compile.

#### Scenario: A repository rejects the wrong id type at compile time
- **WHEN** code calls a person repository method with a `ProductID`
- **THEN** the program fails to compile
