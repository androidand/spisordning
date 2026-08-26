## ADDED Requirements

### Requirement: Recipe entities use UUIDv7 identity

Recipe domain entities SHALL use UUIDv7 primary keys generated in Go. `recipe_ref` SHALL keep
`mealie_recipe_id` as a unique external column rather than as the primary key. `recipe_family` and
`recipe_variant` SHALL carry a `slug TEXT NOT NULL UNIQUE` column in addition to their UUIDv7 primary
key.

#### Scenario: A recipe reference points at a UUID
- **WHEN** `meal_event`, `meal_plan_candidate`, `meal_plan_decision`, or `favorite` references a recipe
- **THEN** it stores `recipe_ref_id` (a UUIDv7), not a `mealie_recipe_id` string

#### Scenario: recipe_ref is keyed by UUID
- **WHEN** a `recipe_ref` row is inserted
- **THEN** its `id` is a UUIDv7 and `mealie_recipe_id` is a unique external column

### Requirement: Recipe relationships use composite keys

`recipe_ingredient` and `recipe_revision_parent` SHALL use composite primary keys re-typed to
reference UUIDv7 recipe ids, with no surrogate column.

#### Scenario: recipe_ingredient is composite
- **WHEN** a `recipe_ingredient` row is inserted
- **THEN** its primary key is `(recipe_ref_id, ingredient_id)`

#### Scenario: revision lineage is composite
- **WHEN** a `recipe_revision_parent` edge is inserted
- **THEN** its primary key is `(revision_id, parent_revision_id)`, both UUIDv7

### Requirement: Recipe quantities are numeric

Recipe ingredient quantities SHALL be stored as `numeric(12,3)`, not IEEE `float`.

#### Scenario: recipe_ingredient quantity is numeric
- **WHEN** a `recipe_ingredient` quantity is stored
- **THEN** it is a `numeric(12,3)` value

### Requirement: Recipe IDs are strongly typed in Go

The Go layer SHALL define distinct recipe ID types (`RecipeRefID`, `RecipeFamilyID`,
`RecipeVariantID`, `RecipeRevisionID`) backed by UUIDv7, so a repository call with the wrong recipe ID
type does not compile.

#### Scenario: Wrong recipe ID type does not compile
- **WHEN** a repository method expects `RecipeRefID` and is passed a `RecipeVariantID`
- **THEN** the code does not compile
