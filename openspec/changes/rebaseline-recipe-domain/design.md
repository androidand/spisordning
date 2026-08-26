# Design: re-baseline the recipe domain

## Context

- `rebaseline-identity-and-schema-types` (change 2) establishes the canonical identity and value-type
  model for the non-recipe tables (including the recipe-discovery staging tables
  `recipe_import_candidate`/`recipe_import_candidate_ingredient` and the `external_recipe_source`
  registry).
- The recipe **hierarchy** — `recipe_ref`, `recipe_ingredient` (`000001`), and
  `recipe_family`/`recipe_variant`/`recipe_revision`/`recipe_revision_parent` (`000003`) — plus every
  recipe reference still uses the old identity (`mealie_recipe_id` PK, slug PK, `BIGSERIAL`, `TEXT`
  refs) and `float` quantities.
- Pre-release, disposable dev data → fresh-bootstrap re-baseline on the change-1/2 baseline.

## Decisions

### D1: `recipe_ref` gets a UUIDv7 PK; `mealie_recipe_id` becomes a unique external column

`recipe_ref` is a Mealie-specific local mirror. It gets `id UUIDv7 PK` and keeps
`mealie_recipe_id TEXT NOT NULL UNIQUE` as the Mealie external id. It does not become a generic
external-ref table because the entity is Mealie-specific. All recipe references point at
`recipe_ref.id`.

### D2: `recipe_family` and `recipe_variant` get UUIDv7 + slug

Both are human-addressable domain entities → `id UUIDv7 PK` + `slug TEXT NOT NULL UNIQUE`.

### D3: `recipe_revision` gets UUIDv7; the parent edge stays composite

`recipe_revision`: `id UUIDv7 PK`. `recipe_revision_parent`: composite PK
`(revision_id UUID, parent_revision_id UUID)`.

### D4: `recipe_ingredient` stays composite, re-typed

Composite PK → `(recipe_ref_id UUID, ingredient_id)`. `quantity DOUBLE PRECISION` → `numeric(12,3)`.

### D5: recipe references re-typed to `recipe_ref_id UUID`

`meal_event`, `meal_plan_candidate`, `meal_plan_decision`, and `favorite`:
`mealie_recipe_id TEXT` → `recipe_ref_id UUID REFERENCES recipe_ref(id)`. `favorite`'s unique
constraints become `(person_id, recipe_ref_id)` and `(household_id, recipe_ref_id)`.

### D6: `promoted_variant_id` re-typed

`recipe_import_candidate.promoted_variant_id TEXT` → `recipe_variant_id UUID REFERENCES
recipe_variant(id)` (the deferred FK from `000002` now binds to the UUID).

### D7: Go typed IDs

`RecipeRefID`, `RecipeFamilyID`, `RecipeVariantID`, `RecipeRevisionID` (UUIDv7-backed). Recipe
references use `RecipeRefID`; the promoted-variant back-reference uses `RecipeVariantID`.

## Schema diff (recipe tables and references)

| table / column              | before                                   | after |
|-----------------------------|------------------------------------------|-------|
| `recipe_ref`                | `mealie_recipe_id TEXT` PK               | `id UUIDv7` PK + `mealie_recipe_id TEXT NOT NULL UNIQUE` |
| `recipe_ingredient`         | PK `(mealie_recipe_id, ingredient_id)`; `quantity DOUBLE` | PK `(recipe_ref_id UUID, ingredient_id)`; `quantity numeric(12,3)` |
| `recipe_family`             | `id TEXT` PK (slug)                      | `id UUIDv7` PK + `slug TEXT NOT NULL UNIQUE` |
| `recipe_variant`            | `id TEXT` PK (slug)                      | `id UUIDv7` PK + `slug TEXT NOT NULL UNIQUE` |
| `recipe_revision`           | `id BIGSERIAL` PK                        | `id UUIDv7` PK |
| `recipe_revision_parent`    | PK `(revision_id, parent_revision_id)` BIGINT | PK `(revision_id, parent_revision_id)` UUID |
| `meal_event`                | `mealie_recipe_id TEXT`                  | `recipe_ref_id UUID` |
| `meal_plan_candidate`       | `mealie_recipe_id TEXT`                  | `recipe_ref_id UUID` |
| `meal_plan_decision`        | `mealie_recipe_id TEXT`                  | `recipe_ref_id UUID` |
| `favorite`                  | `mealie_recipe_id TEXT`                  | `recipe_ref_id UUID` (unique constraints re-keyed) |
| `recipe_import_candidate`   | `promoted_variant_id TEXT`               | `recipe_variant_id UUID` |

## Open questions

- Should favorites target `recipe_ref` (a Mealie mirror) or `recipe_variant` (a household fork)?
  Currently `favorite → recipe_ref`. Keep `recipe_ref` for now; note that favorites may later target a
  variant once the household has promoted its own forks.
