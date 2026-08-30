## 1. Re-baseline the recipe hierarchy

- [x] 1.1 `recipe_ref`: `mealie_recipe_id TEXT` PK → `id UUIDv7` PK + `mealie_recipe_id TEXT NOT NULL
      UNIQUE`
- [x] 1.2 `recipe_family`, `recipe_variant`: slug PK → `id UUIDv7` + `slug TEXT NOT NULL UNIQUE`
- [x] 1.3 `recipe_revision`: `BIGSERIAL` → `id UUIDv7`
- [x] 1.4 `recipe_ingredient`: composite PK → `(recipe_ref_id UUID, ingredient_id)`; `quantity` →
      `numeric(12,3)`
- [x] 1.5 `recipe_revision_parent`: composite PK re-typed to UUID

## 2. Re-type recipe references

- [x] 2.1 `meal_event.mealie_recipe_id` → `recipe_ref_id UUID`
- [x] 2.2 `meal_plan_candidate.mealie_recipe_id` → `recipe_ref_id UUID`
- [x] 2.3 `meal_plan_decision.mealie_recipe_id` → `recipe_ref_id UUID`
- [x] 2.4 `favorite.mealie_recipe_id` → `recipe_ref_id UUID` (+ re-key the two unique constraints)
- [x] 2.5 `recipe_import_candidate.promoted_variant_id` → `recipe_variant_id UUID` (bind the deferred FK)

## 3. Go typed IDs

- [x] 3.1 Add `RecipeRefID`, `RecipeFamilyID`, `RecipeVariantID`, `RecipeRevisionID` (UUIDv7-backed)
- [x] 3.2 Update the recipe repositories and call sites to use the typed IDs

## 4. Verify

- [x] 4.1 Fresh PG19 bootstrap applies cleanly with the recipe migrations included
- [x] 4.2 `go build ./...` and `go test ./...` pass
- [x] 4.3 `openspec validate rebaseline-recipe-domain` passes
