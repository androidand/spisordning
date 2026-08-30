# Unify the recipe source of truth

## Why

Spisordning currently runs two parallel recipe systems, and the one that is *live* is external.
The planner, recommender, shopping-requirement builder, meal reactions, and favorites all resolve
recipes through the external **Mealie** instance — `recipe_ref` keys on `MealieRecipeID`, and
`internal/service/planweek.go` / `planning.go` read recipe content from Mealie. Meanwhile the
native **`recipe_family` / `recipe_variant` / `recipe_revision`** model (the "git-like" household
cookbook) is fully built — schema (`migrations/000003_recipe_family.sql`), service
(`internal/service/recipe_family.go`), persistence, and frontend — but it is **not** what the
planner reads. The two systems are not unified: the household-owned cookbook is decorative, while
an external Mealie instance is the de-facto planning source of truth.

This undercuts the core mission in `PLAN.md` (a self-hosted, household-owned food knowledge
system) and creates a real maintenance hazard: `structure_recipe`
(`internal/service/recipe_structuring.go` `StructureFromText`) writes *new* structured recipes
into Mealie, so the cookbook keeps growing in the external system rather than the native one.
Every feature that "works" today does so by reaching across the process boundary into Mealie.

This change makes the native `recipe_family` the planning source of truth and demotes Mealie to an
import/reference source. It is the largest and riskiest of the current batch, so it is designed as
a **phased migration with a safe rollback path**, not a big-bang cutover.

## What Changes

- **Canonical recipe identity + mapping.** A durable, bidirectional mapping between a Mealie
  recipe (`MealieRecipeID` / slug) and a native `recipe_family`, so any existing reference
  (in-flight `meal_plan`, `shopping_list`, favorites, reactions) can be resolved in either
  direction. New additive migration.
- **Phased source-of-truth migration**, each phase independently shippable and reversible:
  - **P1 — Dual-read resolution.** Planner / recommender / shopping / reactions / favorites resolve
    a recipe via native `recipe_family` first, falling back to Mealie, behind an explicit
    resolution rule and a runtime source-of-truth flag.
  - **P2 — One-way data import.** An idempotent, resumable import of Mealie recipes into
    `recipe_family` (family + default variant + revision), each imported row carrying a
    provenance/source marker so imported recipes are distinguishable from native ones.
  - **P3 — Write cutover.** `structure_recipe` and any other recipe write paths target
    `recipe_family` instead of Mealie; Mealie becomes import-only.
  - **P4 — Demote Mealie.** Mealie is no longer consulted as the planning source; the Mealie client
    is retained for import/reference only.
- **A runtime source flag** (config) controlling which source is authoritative during migration,
  so any phase can be rolled back by flipping the flag rather than reverting code.

## Capabilities

### New Capabilities

- `recipe-source`: which system is the authoritative source of truth for recipes at runtime, the
  canonical Mealie↔recipe_family identity mapping, and the phased import/cutover/demotion of
  Mealie — with a reversible source flag.

### Modified Capabilities

<!-- none at the spec level. The behavioral changes to meal-planning (planner reads) and
     recipe-structuring (write target) are delivered through the new recipe-source capability and
     its tasks; those capabilities' existing specs remain valid and unchanged. -->

## Impact

- Affected code: `internal/service/planweek.go`, `planning.go`, `recipe_structuring.go`, the
  recommender, the shopping-requirement builder, and meal reaction/favorite resolution; a new
  `recipe_source_ref` mapping migration; a new import service + CLI command; config for the
  source-of-truth flag.
- Depends on `rebaseline-recipe-domain` (the `recipe_family` model — already complete) and the
  `recipe-family` spec's invariants.
- Cross-references (not dependencies): `activate-recipe-discovery` promotes discovered recipes
  into `recipe_family` and works regardless of this change; `close-mcp-planning-loop` persists
  plans that reference recipes by ID and is orthogonal to which source backs that ID.
- Riskiest seams: (1) the Mealie↔recipe_family ID mapping must be total and stable for every
  recipe the household actually plans; (2) in-flight `meal_plan` / `shopping_list` / favorites /
  reactions that reference `MealieRecipeID` must keep resolving across the cutover; (3) a
  partially-migrated state (some recipes imported, some not) must be safe to run. All three are
  mitigated by the phased design + source flag + per-phase rollback (see `design.md`).
- No change to the `recipe_family` domain model itself (owned by `rebaseline-recipe-domain`).
