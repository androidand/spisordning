# Tasks: unify-recipe-source

## 1. Identity mapping

- [ ] 1.1 New additive migration creating `recipe_source_ref` (`recipe_family_id` FK, `source`,
      `source_recipe_id`, `imported_at`, `imported_by`; UNIQUE (`source`,`source_recipe_id`) and
      UNIQUE (`recipe_family_id`)) — `design.md` D1.
- [ ] 1.2 Persistence: `GetRecipeSourceRefByFamily`, `GetRecipeSourceRefBySource`,
      `UpsertRecipeSourceRef`, `ListUnmappedMealieRecipes` (for import progress).
- [ ] 1.3 Integration tests: bidirectional uniqueness, upsert idempotency.

## 2. P1 — Dual-read resolution

- [ ] 2.1 Add the source-of-truth config flag (`native`/`dual`/`mealie`), read at startup; default
      `mealie` until the import lands — `design.md` D2.
- [ ] 2.2 Implement `ResolveRecipe(ref)` used by planner, recommender, shopping, reactions,
      favorites — native-first with mapping + Mealie fallback per the flag.
- [ ] 2.3 Wire `planweek.go` / `planning.go` to resolve via `ResolveRecipe` instead of direct
      Mealie reads.
- [ ] 2.4 Wire recommender, shopping-requirement builder, meal reactions, and favorites to
      `ResolveRecipe`.
- [ ] 2.5 Tests: each consumer resolves correctly under `native`, `dual`, and `mealie`; an unmapped
      recipe under `native` is surfaced, not dropped.

## 3. P2 — One-way import (Mealie → recipe_family)

- [ ] 3.1 Import service: for each Mealie recipe, upsert family + default variant + revision and
      insert `recipe_source_ref` — idempotent, resumable, stable order — `design.md` D3.
- [ ] 3.2 Provenance: mark imported families/variants (variant `source_attribution` + mapping row).
- [ ] 3.3 Conflict policy: link to an existing same-slug native family rather than duplicating.
- [ ] 3.4 CLI command (`food-brain import-recipes` or similar) to run/inspect the import, with a
      dry-run and a progress report.
- [ ] 3.5 Tests: idempotency (re-run creates no duplicates), resumability, conflict-linking,
      provenance markers.

## 4. P3 — Write cutover

- [ ] 4.1 Change `StructureFromText` to create the recipe in `recipe_family` (family + variant +
      revision) + `recipe_source_ref`, instead of `mealie.CreateRecipe/SetIngredients/
      SetInstructions/SetTags` — `design.md` D4.
- [ ] 4.2 Preserve low-confidence reporting mapped onto the native revision's ingredient
      raw-text/flags.
- [ ] 4.3 Audit for any other recipe write paths still targeting Mealie; cut them over.
- [ ] 4.4 Tests: a structured recipe lands in `recipe_family` with correct ingredients/steps and a
      source ref; no Mealie write occurs.

## 5. P4 — Demote Mealie

- [ ] 5.1 Set the source flag default to `native` once the import is complete and verified.
- [ ] 5.2 Remove Mealie from the hot planning path (reads now resolve natively); retain the client
      for import/reference only — `design.md` D5.
- [ ] 5.3 Document the end-state and the deferred follow-ups (in-flight reference rewrite, client
      removal) — `design.md` D6/D8.
- [ ] 5.4 Integration test: a full week plans end-to-end with the flag at `native` and no live
      Mealie dependency.

## 6. Verification & docs

- [ ] 6.1 `openspec validate unify-recipe-source` passing.
- [ ] 6.2 Update `docs/research/current-state.md` to reflect the new source of truth.
- [ ] 6.3 Rollback runbook: the exact flag values to revert each phase — `design.md` D7.
