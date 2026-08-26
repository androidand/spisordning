# Re-baseline the recipe domain

## Why

The recipe hierarchy uses the same inconsistent identity as the rest of the schema: `recipe_ref` is
keyed by `mealie_recipe_id TEXT` (a foreign-system id as the primary key), `recipe_family` and
`recipe_variant` use slug-as-PK, `recipe_revision` uses a `BIGSERIAL` surrogate, and every recipe
reference (`meal_event`, `meal_plan_candidate`, `meal_plan_decision`, `favorite`, `recipe_ingredient`)
stores a `mealie_recipe_id TEXT` that does not encode what it references. Recipe quantities are IEEE
`float`. This change brings the recipe domain onto the canonical identity and value-type model
established by `rebaseline-identity-and-schema-types`.

## What Changes

- `recipe_ref`: `mealie_recipe_id TEXT` PK → `id UUIDv7` PK + `mealie_recipe_id TEXT NOT NULL UNIQUE`
  (the Mealie external id stays on this Mealie-specific entity, not a generic external-ref table).
- `recipe_family`, `recipe_variant`: slug-as-PK → `id UUIDv7` PK + `slug TEXT NOT NULL UNIQUE`.
- `recipe_revision`: `BIGSERIAL` → `id UUIDv7`.
- `recipe_ingredient`: composite PK re-typed to `(recipe_ref_id UUID, ingredient_id)`; `quantity` →
  `numeric(12,3)`.
- `recipe_revision_parent`: composite PK re-typed to UUID.
- Recipe references re-typed from `mealie_recipe_id TEXT` to `recipe_ref_id UUID`: `meal_event`,
  `meal_plan_candidate`, `meal_plan_decision`, `favorite` (and `favorite`'s unique constraints).
- `recipe_import_candidate.promoted_variant_id` re-typed from `TEXT` to `recipe_variant_id UUID`.
- Go: strongly-typed recipe ID types (`RecipeRefID`, `RecipeFamilyID`, `RecipeVariantID`,
  `RecipeRevisionID`).

## Impact

- Affected specs: `recipe-domain` (new).
- Affected code: recipe migrations (`000001` recipe_ref/recipe_ingredient, `000003` family/variant/
  revision/parent, `000002` promoted_variant_id), meal tables (`000001` meal_event/meal_plan_candidate/
  meal_plan_decision, `000012` favorite), Go recipe types and repositories.
- Depends on `rebaseline-identity-and-schema-types` (the canonical identity/value-type model). Feeds
  `establish-sqlc-persistence`.
