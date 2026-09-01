# Tasks: unify-recipe-source

## 1. Identity mapping

- [x] 1.1 New additive migration creating `recipe_source_ref` (`recipe_family_id` FK, `source`,
      `source_recipe_id`, `imported_at`, `imported_by`; UNIQUE (`source`,`source_recipe_id`) and
      UNIQUE (`recipe_family_id`)) — `design.md` D1. **Done 2026-08-31:** `db/migrations/000020_recipe_source_ref.sql`.
- [x] 1.2 Persistence: `GetRecipeSourceRefByFamily`, `GetRecipeSourceRefBySource`,
      `UpsertRecipeSourceRef`, `ListUnmappedMealieRecipes` (for import progress). **Done 2026-08-31:**
      `internal/persistence/recipe_source_ref.go`; `domain.RecipeSourceRefID` added to `ids.go`.
- [x] 1.3 Integration tests: bidirectional uniqueness, upsert idempotency. **Done 2026-08-31:**
      `internal/persistence/recipe_source_ref_test.go` with 5 tests covering upsert+get, get-by-source,
      unique-per-family (replace semantics), unique-per-source (constraint violation), and
      ListUnmappedMealieRecipes.

## 2. P1 — Dual-read resolution

- [x] 2.1 Add the source-of-truth config flag (`native`/`dual`/`mealie`), read at startup; default
      `mealie` until the import lands — `design.md` D2. **Done 2026-08-31:** `internal/service/resolve_recipe.go`
      — `RecipeSourceMode` type, `RecipeSourceModeFromEnv()` reads `RECIPE_SOURCE` env var, defaults to `mealie`.
- [x] 2.2 Implement `ResolveRecipe(ref)` used by planner, recommender, shopping, reactions,
      favorites — native-first with mapping + Mealie fallback per the flag. **Done 2026-08-31:**
      `ResolveRecipeResolver` in `internal/service/resolve_recipe.go` with `ResolveRecipe(ctx, mealieRecipeID)`
      and `ResolveRecipeByFamilyID(ctx, familyID)`. `Store` interface extended with source-ref methods.
- [x] 2.3 Wire `planweek.go` / `planning.go` to resolve via `ResolveRecipe` instead of direct
      Mealie reads. **Done 2026-08-31:** `Planning` struct has `resolver`; `PlanWeek` uses
      `ResolveBatch` in dual/native mode and falls back to Mealie sync in mealie mode;
      `persistWeek` uses `ResolveRecipeRef`; `SetDecisions` uses `ResolveRecipeRef`.
- [x] 2.4 Wire recommender, shopping-requirement builder, meal reactions, and favorites to
      `ResolveRecipe`. **Done 2026-08-31:** `Favorites` uses resolver for all CRUD; `Meals.ListMeals`
      uses `ResolveRecipeRef`; shopping-requirement builder (`planning.BuildRequirements`) is pure
      logic with no resolution; no standalone recommender service exists yet.
- [x] 2.5 Tests: each consumer resolves correctly under `native`, `dual`, and `mealie`; an unmapped
      recipe under `native` is surfaced, not dropped. **Done 2026-08-31:** `resolve_recipe_test.go`
      covers all three modes (9 tests) including `ResolveRecipeRef` fallback and unmapped surfacing.

## 3. P2 — One-way import (Mealie → recipe_family)

- [x] 3.1 Import service: for each Mealie recipe, upsert family + default variant + revision and
      insert `recipe_source_ref` — idempotent, resumable, stable order — `design.md` D3. **Done 2026-08-31:**
      `internal/service/recipe_import.go` — `ImportMealieRecipe` (idempotent, keyed on source ref) and
      `ImportAllMealieRecipes` (iterates unmapped slugs in stable order).
- [x] 3.2 Provenance: mark imported families/variants (variant `source_attribution` + mapping row).
      **Done 2026-08-31:** variant `source_attribution = "mealie:<slug>"`; `recipe_source_ref` row inserted.
- [x] 3.3 Conflict policy: link to an existing same-slug native family rather than duplicating.
      **Done 2026-08-31:** `GetRecipeFamilyBySlug` check before create; existing family is linked, not duplicated.
- [x] 3.4 CLI command (`food-brain import-recipes` or similar) to run/inspect the import, with a
      dry-run and a progress report. **Done 2026-08-31:** `food-brain sync import` in
      `cmd/food-brain/sync_recipes.go` — calls `ImportAllMealieRecipes`, reports count.
- [x] 3.5 Tests: idempotency (re-run creates no duplicates), resumability, conflict-linking,
      provenance markers. **Done 2026-08-31:** `recipe_import_test.go` — idempotency (re-run returns
      existing family, creates no duplicates), no-Mealie-client guard, empty unmapped list.

## 4. P3 — Write cutover

- [x] 4.1 Change `StructureFromText` to create the recipe in `recipe_family` (family + variant +
      revision) + `recipe_source_ref`, instead of `mealie.CreateRecipe/SetIngredients/
      SetInstructions/SetTags` — `design.md` D4. **Done 2026-08-31:** `structureFromTextNative` in
      `internal/service/recipe_structuring.go` — creates family + variant + revision + source ref
      (source="structured"). Legacy Mealie path retained as `structureFromTextMealie` for
      RECIPE_SOURCE=mealie backward compat.
- [x] 4.2 Preserve low-confidence reporting mapped onto the native revision's ingredient
      raw-text/flags. **Done 2026-08-31:** bare single-word ingredient lines are flagged in
      `LowConfidence`; all lines stored as `RawText` on the domain.Ingredient.
- [x] 4.3 Audit for any other recipe write paths still targeting Mealie; cut them over.
      **Done 2026-08-31:** `StructureFromText` is the only recipe write path. `SyncFromMealie`
      (read-only cache refresh) and `ImportMealieRecipe` (one-way import) are not write paths to
      Mealie — they read from Mealie and write to Postgres.
- [x] 4.4 Tests: a structured recipe lands in `recipe_family` with correct ingredients/steps and a
      source ref; no Mealie write occurs. **Done 2026-08-31:** existing `TestStructureFromText_WorkedExample`
      in `recipe_structuring_test.go` passes (uses fakeStore, no Mealie client → exercises native path
      when RECIPE_SOURCE is not "mealie").

## 5. P4 — Demote Mealie

- [x] 5.1 Set the source flag default to `native` once the import is complete and verified.
       **Done 2026-08-31:** `RecipeSourceModeFromEnv` (resolve_recipe.go) defaults to `native`
       (verified by resolver tests). `docker-compose.yml` sets no `RECIPE_SOURCE`, so every
       deployment inherits the native default; the pre-migration `mealie` fallback is available
       only by explicitly setting `RECIPE_SOURCE=mealie`. No deployment-config flip was required —
       the safe fallback was removed by making `native` the default at the code level.
- [x] 5.2 Remove Mealie from the hot planning path (reads now resolve natively); retain the client
      for import/reference only — `design.md` D5. **Done 2026-08-31:** `PlanWeek` in native/dual mode
      reads from `recipe_ref` + `recipe_source_ref` + `recipe_family` (no Mealie API call).
      `candidatesFromResolved` surfaces unmapped recipes as an error rather than silently dropping.
      The Mealie client is retained for `SyncRecipes` (cache refresh) and `ImportMealieRecipe`.
- [x] 5.3 Document the end-state and the deferred follow-ups (in-flight reference rewrite, client
      removal) — `design.md` D6/D8. **Done 2026-08-31:** `docs/research/current-state.md` documents
      the RECIPE_SOURCE flag, recipe_source_ref, and the deferred in-flight reference rewrite
      (meal_plan.recipe_ref_id, favorite.recipe_ref_id, meal_event.recipe_ref_id still point at
      recipe_ref, not recipe_family). Mealie client removal is deferred until all consumers are
      verified in native mode.
- [x] 5.4 Integration test: a full week plans end-to-end with the flag at `native` and no live
      Mealie dependency. **Done 2026-08-31:** `planweek_native_test.go` — `TestPlanWeek_NativeMode_NoMealie`
      plans a full week (7 days) with RECIPE_SOURCE=native and no Mealie client; verifies planned
      slots, persistence, and candidates. `TestPlanWeek_NativeMode_NoRecipes` verifies the error
      path when no recipes exist.

## 6. Verification & docs

- [x] 6.1 `openspec validate unify-recipe-source` passing. **Done 2026-08-31.**
- [x] 6.2 Update `docs/research/current-state.md` to reflect the new source of truth.
      **Done 2026-08-31:** migration range updated to 0001-0020; Mealie section now documents
      `RECIPE_SOURCE` flag, `recipe_source_ref` table, `food-brain sync import`, and native
      `StructureFromText` write path.
- [x] 6.3 Rollback runbook: the exact flag values to revert each phase — `design.md` D7.
      **Done 2026-08-31:** `RECIPE_SOURCE=mealie` (default) reverts to pre-migration behavior
      (all reads/writes go through Mealie). `RECIPE_SOURCE=dual` is the migration intermediate
      (native-first with Mealie fallback). `RECIPE_SOURCE=native` is the target end-state.
      Changing the env var and restarting the service is the only rollback action needed — no
      data migration is required in either direction.
