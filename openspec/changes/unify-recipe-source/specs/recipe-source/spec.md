# recipe-source (delta)

## ADDED Requirements

### Requirement: The native recipe_family is the authoritative recipe source of truth

The system SHALL resolve recipes for planning, recommendation, shopping-requirement building, meal
reactions, and favorites through the native `recipe_family` model as the authoritative source of
truth. An external system (Mealie) SHALL be treated as an import/reference source, not as the
planning source of truth.

#### Scenario: The planner reads a recipe from the native model

- **WHEN** the weekly planner resolves a recipe for a plan slot
- **THEN** it reads the recipe content from `recipe_family` / `recipe_variant` / `recipe_revision`
- **AND** it does not require a live Mealie lookup to produce the plan

#### Scenario: A new structured recipe is written to the native model

- **WHEN** a household member structures a freeform recipe via `structure_recipe`
- **THEN** the recipe is created in `recipe_family` (family + variant + revision)
- **AND** it is not created in Mealie as the system of record

### Requirement: A canonical mapping links external recipe IDs to native recipe families

The system SHALL maintain a durable, bidirectional mapping between an external recipe identifier
(e.g. a Mealie recipe slug) and a native `recipe_family`. The mapping SHALL be unique in both
directions: an external recipe maps to at most one family, and a family maps to at most one
external recipe of a given source.

#### Scenario: An in-flight Mealie reference resolves to a native family

- **WHEN** an existing `meal_plan` slot or shopping list item references a `MealieRecipeID`
- **THEN** the system resolves it to its `recipe_family` via the mapping
- **AND** the reference continues to work after Mealie is demoted

#### Scenario: The same external recipe is never imported twice

- **WHEN** the import runs again for a Mealie recipe that already has a mapping
- **THEN** no second `recipe_family` is created
- **AND** the existing mapping is preserved

### Requirement: The source of truth is switchable and reversible during migration

The system SHALL expose a runtime source-of-truth setting that controls whether recipes are
resolved from the native model, from Mealie, or from the native model with a Mealie fallback.
Changing this setting SHALL be sufficient to roll back a migration phase without reverting code or
rewriting data.

#### Scenario: Rolling back a cutover is a config change

- **WHEN** a migration phase misbehaves after the write path has been cut over
- **THEN** an operator can set the source of truth back to Mealie (or dual) via configuration
- **AND** planning continues to work without a code revert or a data rewrite

### Requirement: Imported recipes carry provenance and are distinguishable from native recipes

Every recipe imported from an external source SHALL be linked to its source via the mapping and
carry a provenance marker so that imported recipes are distinguishable from natively created
`recipe_family` rows.

#### Scenario: An imported recipe is identifiable as imported

- **WHEN** a household member lists recipe families
- **THEN** families that were imported from Mealie are identifiable as such (via their source
  marker / mapping)
- **AND** natively created families are not marked as imported

### Requirement: The migration is phased, additive, and non-destructive

The migration from Mealie to the native source of truth SHALL proceed in independently shippable
phases (dual-read, import, write cutover, demotion). No phase SHALL destructively rewrite
in-flight plan, shopping, favorite, or reaction data, and every phase SHALL be reversible.

#### Scenario: A partially-migrated state is safe to run

- **WHEN** some recipes have been imported and mapped but others have not
- **THEN** the system continues to plan correctly, resolving unmapped recipes through the
  configured fallback
- **AND** no in-flight plan or shopping list is corrupted by the partial state
