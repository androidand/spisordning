# Design: unify-recipe-source

## D1 — Canonical recipe identity and the Mealie↔recipe_family mapping

A native `recipe_family` is identified by a slug (`RecipeFamilyID`); a Mealie recipe by its slug
(`MealieRecipeID`). These are different namespaces and neither is globally unique across the two
systems. We introduce a mapping table `recipe_source_ref` (new additive migration):

- `recipe_family_id` (FK → `recipe_family.id`, NOT NULL)
- `source` (TEXT, e.g. `'mealie'` — extensible to other sources)
- `source_recipe_id` (TEXT, the Mealie slug; NOT NULL)
- `imported_at` (timestamptz), `imported_by` (nullable)
- UNIQUE (`source`, `source_recipe_id`) and UNIQUE (`recipe_family_id`) — a family maps to at most
  one external source recipe, and an external recipe maps to at most one family of a given source.

This is the single source of truth for cross-system identity. Every resolution path (planner,
recommender, shopping, reactions, favorites) resolves through it. It is bidirectional by query
(one FK each way), so an in-flight `MealieRecipeID` can be mapped to its `recipe_family_id` and
vice versa.

Why a mapping table rather than storing the Mealie slug on `recipe_family`: it keeps the native
model clean (no external-namespace column on the domain aggregate) and leaves room for other
sources (`activate-recipe-discovery`'s promoted recipes, future sources) to register here too.

## D2 — The resolution rule (P1 dual-read)

Define one function, `ResolveRecipe(ref) → (recipe_family_id, content, source)`, used by every
consumer. Behavior is controlled by the source flag:

- Flag = `native` (target end-state): resolve via `recipe_family`; if a `recipe_ref` / plan slot
  carries only a `MealieRecipeID`, map it through `recipe_source_ref` to the family; if no mapping
  exists, the recipe is treated as not-yet-imported (surfaced, never silently dropped).
- Flag = `dual` (migration default): try `recipe_family` first (direct or via mapping); fall back
  to Mealie when the family is absent; record which source served it (for observability).
- Flag = `mealie` (rollback / pre-migration): current behavior unchanged.

The flag is a single config value (env), read once at startup. This makes any phase reversible by
config change, not code revert.

## D3 — The import (P2)

A one-way, idempotent, resumable import `Mealie → recipe_family`:

- For each Mealie recipe: upsert a `recipe_family` (slug = Mealie slug, or a namespaced slug if it
  collides with an existing native family), a default `recipe_variant`, and one `recipe_revision`
  carrying the ingredients/steps/servings. Insert a `recipe_source_ref` row.
- Idempotency: keyed on `recipe_source_ref (source, source_recipe_id)`; re-running skips
  already-imported recipes (or re-imports only if explicitly forced). Never creates a second family
  for the same Mealie recipe.
- Provenance: imported families/variants carry a source marker (variant
  `source_attribution = 'mealie:<slug>'` plus the `recipe_source_ref` row) so imported rows are
  distinguishable from native ones.
- Resumable: processes recipes in a stable order, records progress, and can be interrupted and
  re-run without duplication.
- Conflict policy: if a native `recipe_family` with the same slug already exists, the import links
  the Mealie recipe to the existing family via `recipe_source_ref` rather than creating a
  duplicate (the household's native version wins; the import is a link, not an overwrite).

## D4 — Write cutover (P3)

`StructureFromText` (and any other write path) is changed to create the recipe in `recipe_family`
(family + variant + revision) and register a `recipe_source_ref` row with `source='structured'`
(or the relevant source), instead of calling `mealie.CreateRecipe/SetIngredients/SetInstructions/
SetTags`. The Mealie write calls are removed from this path. The low-confidence reporting
(`LowConfidence`) is preserved, mapped onto the native revision's ingredient raw-text/flags.

## D5 — Demotion (P4)

With the flag at `native` and the import complete (every plannable Mealie recipe has a mapping),
Mealie is no longer consulted for planning. The `internal/mealie` client is retained for
import/reference (and any explicit "sync from Mealie" action) but is no longer on the hot planning
path. Removing the client entirely is a separate, later cleanup (out of scope here).

## D6 — In-flight references across the cutover

`meal_plan`, `meal_plan_candidate`, `meal_plan_decision`, `shopping_list(_item)`, `favorite`, and
`meal_reaction` reference recipes by `MealieRecipeID` today. Decision:

- Keep those columns as-is during migration; resolve them through `recipe_source_ref` at read time
  (D2). This avoids a risky data rewrite of every in-flight reference.
- After P4 (Mealie demoted), a follow-up (out of scope) can rewrite those references to
  `recipe_family_id` and drop the Mealie columns.

This keeps each phase low-risk: no phase requires rewriting in-flight plan/shopping data.

## D7 — Rollback

Each phase rolls back by config, not code:

- P1: flag `native`/`dual` → `mealie`.
- P2: the import is additive; rolling back means ignoring the imported families (flag `mealie`) —
  the data can be left in place or pruned via the `recipe_source_ref` marker.
- P3: revert the write-path change (code) — only needed if P3 misbehaves; the flag still controls
  reads.
- P4: flag back to `dual`/`mealie`.

No phase is destructive or irreversible.

## D8 — What is explicitly out of scope

- Rewriting in-flight `meal_plan`/`shopping_list`/`favorite`/`reaction` rows to native IDs (D6
  defers this).
- Removing the `internal/mealie` client (kept for import).
- The `recipe_family` domain model (owned by `rebaseline-recipe-domain`).
- Discovery promotion (owned by `activate-recipe-discovery`).
