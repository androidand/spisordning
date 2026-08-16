# Implement recipe availability

## Why

`PLAN.md` does not name this capability verbatim, but it is the connective tissue its own
diagrams imply: the "Final Goal" operational loop (Discover → Cookbook → **Plan** → Shop →
**Pantry** → Cook → Review → Learn → Recommend → Plan again) requires something that reads a
recipe's ingredients against what a household actually has on hand, and the "Recommendation
Domain" input list explicitly names `pantry availability`, `expiry`, and `substitutions` as
scoring inputs — none of which can be computed without first answering "can this recipe be made
right now, and how confidently."

This change depends on two other Epic D/B changes that own the models it consumes rather than
redefines:
- **`implement-pantry-inventory`** owns `InventoryLot`/`InventoryLocation` — the current physical
  state this capability reads. This change is a read-only consumer; it never mutates inventory.
- **`establish-household-and-catalog`** owns the `IngredientSubstitution` model
  (`EQUIVALENT`/`GOOD`/`ACCEPTABLE`/`FORM`/`DIETARY`/`EMERGENCY`, directional, explicit
  non-1:1 ratio) and `IngredientForm` (fresh/dried/canned/frozen). This change consumes that
  taxonomy exactly as defined — it does not invent a parallel substitution or form model.

`migrations/0001_init.sql` already has structured recipe ingredients (`recipe_ingredient`:
`ingredient_id`, `quantity`, `unit`, keyed to `recipe_ref`), so the recipe-side input this
capability needs already exists; this change does not touch recipe modeling itself (that is
`implement-recipe-family-and-revisions`' scope for the deeper RecipeFamily/Variant/Revision
model — this change works against whatever structured-ingredient shape is current at
implementation time, `recipe_ingredient` or its successor).

**Validated by `establish-reference-lab`'s Grocy findings (2026-08-16):** Grocy has a recipe
fulfillment/cost feature covering the same conceptual ground this change owns, but
`docs/research/grocy-units-and-planning.md` found it's implemented as ~100-line nested SQL
views computing a `possible_servings` figure with no per-ingredient breakdown surfaced anywhere
— a household member sees "you can make 0 servings" with no way to see which ingredient is
short or why. That's the concrete, real-world version of the failure this proposal already
commits to avoiding (line 42-43, "not an opaque score") — it's the single clearest example
found across both reference systems of a feature that exists but isn't explainable, and it
sharpens this change's non-negotiable: the per-ingredient reason breakdown is not a nice-to-have
UI layer bolted onto an aggregate number, it must be the actual computation's primary output,
with the recipe-level verdict derived from it — never the other way around.

## What Changes

- Given a recipe's structured ingredient lines and required quantities, and a household's
  current `InventoryLot`s, determine feasibility **per ingredient line** as one of: satisfied
  on-hand, satisfied via substitution (naming the tier used), or unmet.
- Aggregate per-ingredient results into a **recipe-level** verdict: `feasible` (everything
  on-hand, no substitution needed), `feasible-with-substitution` (at least one line required a
  substitution), or `infeasible` (at least one line unmet with no acceptable substitute).
- When an ingredient isn't directly available, walk `IngredientSubstitution` in decreasing
  preference order (`EQUIVALENT` → `GOOD` → `ACCEPTABLE` → `FORM` → `DIETARY` → `EMERGENCY`),
  applying each substitution's explicit quantity ratio — never assuming 1:1.
- Treat `InventoryLot` confidence honestly: a lot with `UNKNOWN` confidence SHALL NOT silently
  count as satisfying a requirement without being flagged as such in the explanation.
- Every verdict is explainable: each ingredient line's result carries a machine-readable reason
  (on-hand / substituted-`<tier>` / missing), not an opaque score.
- Deliberately tight scope: no shopping-gap computation (that's `shopping_requirement`'s job, see
  `implement-shopping-and-commerce`), no pantry mutation (read-only), no recipe scaling beyond
  what `recipe_ingredient` already encodes.

## Capabilities

### New Capabilities

- `recipe-availability`: per-ingredient and per-recipe feasibility determination against current
  household inventory, accounting for acceptable substitutions and ingredient forms,
  explainable by construction.

### Modified Capabilities

<!-- none — read-only consumer of pantry-inventory and ingredient-catalog, does not modify
     either -->

## Impact

- Affected code: new `internal/availability` (or equivalent) Go package — pure domain logic, no
  new tables expected unless caching proves necessary (see `tasks.md`).
- Depends on `implement-pantry-inventory` (`InventoryLot`/`InventoryLocation`) and
  `establish-household-and-catalog` (`IngredientSubstitution`, `IngredientForm`,
  `ProductIngredientMapping`) — this change should land after both.
- Feeds `implement-recommendations`' pantry-availability, expiry, and substitutions scoring
  inputs; this change computes the feasibility facts, `implement-recommendations` decides how
  they affect ranking.
- No changes to `internal/scoring`, `internal/planning`, or existing meal-planning tables.
