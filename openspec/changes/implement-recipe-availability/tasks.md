# Tasks: implement-recipe-availability

## 1. Consume upstream models (don't redefine)

- [ ] 1.1 Consume `IngredientSubstitution` (`EQUIVALENT`/`GOOD`/`ACCEPTABLE`/`FORM`/`DIETARY`/
      `EMERGENCY`, directional, explicit ratio) from `establish-household-and-catalog` without
      defining a parallel substitution taxonomy
- [ ] 1.2 Consume `IngredientForm` (fresh/dried/canned/frozen) from
      `establish-household-and-catalog`
- [ ] 1.3 Consume `InventoryLot`/`InventoryLocation` (including confidence tiers) from
      `implement-pantry-inventory` as a read-only input — this capability never writes inventory
- [ ] 1.4 Consume existing structured recipe ingredients (`recipe_ingredient` in
      `migrations/0001_init.sql`, or its successor from `implement-recipe-family-and-revisions`)

## 2. Vocabulary & scope

- [ ] 2.1 Define the tri-state verdict: `feasible` / `feasible-with-substitution` / `infeasible`,
      both per-ingredient-line and recipe-level
- [ ] 2.2 Explicitly scope out: shopping-gap computation (owned by `shopping_requirement` /
      `implement-shopping-and-commerce`), pantry mutation (read-only consumer of inventory),
      recipe scaling/servings math beyond what the recipe ingredient line already encodes

## 3. Per-ingredient feasibility

- [ ] 3.1 Match a recipe ingredient line against on-hand `InventoryLot`s via
      `ProductIngredientMapping`/`ingredient_id`
- [ ] 3.2 Apply `IngredientForm` matching: prefer an on-hand lot whose form matches the recipe's
      required/preferred form; treat a mismatched form as requiring a `FORM`-category
      substitution, not an automatic match
- [ ] 3.3 When no direct match exists, walk `IngredientSubstitution` in decreasing preference
      order (`EQUIVALENT` → `GOOD` → `ACCEPTABLE` → `FORM` → `DIETARY` → `EMERGENCY`) and record
      which tier, if any, resolved the line
- [ ] 3.4 Apply each substitution's explicit quantity ratio when computing whether the
      substituted lot's quantity actually covers the requirement — never assume 1:1
- [ ] 3.5 Decide and document the partial-quantity policy: does an on-hand quantity smaller than
      required count as unmet, or as feasible-with-a-noted-shortfall (feeds
      `implement-shopping-and-commerce`'s gap, but that computation is out of this change's
      scope — only the policy for *this* capability's verdict needs to be decided here)
- [ ] 3.6 A lot with `UNKNOWN` confidence SHALL NOT silently satisfy a requirement — decide
      whether it counts as unmet, or as satisfied-but-flagged, and make the choice explicit in
      the per-line reason

## 4. Overall recipe feasibility

- [ ] 4.1 Aggregate per-ingredient verdicts into the recipe-level verdict
- [ ] 4.2 Define the aggregation rule precisely (e.g. any unmet line with no viable substitute
      ⇒ recipe `infeasible`; any line resolved via substitution ⇒ recipe
      `feasible-with-substitution`; all lines on-hand with matching form ⇒ recipe `feasible`)

## 5. Explainability

- [ ] 5.1 Every per-ingredient result carries a machine-readable reason: on-hand,
      `substituted-<tier>`, or missing (with the shortfall, if partial)
- [ ] 5.2 No opaque scoring — this feeds `implement-recommendations`' pantry-availability and
      expiry inputs, which `PLAN.md` requires to be explainable

## 6. Expiry awareness

- [ ] 6.1 Surface which on-hand lots a recipe's feasible verdict would consume, and which of
      those are near expiry (best-before soon) — a fact this change computes and exposes, not a
      scoring decision it makes (scoring/weighting is `implement-recommendations`' job)

## 7. Persistence & interface

- [ ] 7.1 Decide whether this stays pure on-demand domain logic (computed from
      `inventory_lot` + recipe ingredients + `ingredient_substitution` at call time) or whether a
      cache/materialized view is warranted for planning-time performance; default to no new
      persisted state unless a concrete performance need is shown
- [ ] 7.2 Expose as an internal Go package, with a (future) HTTP endpoint once
      `establish-enforced-go-architecture` lands an HTTP server — not new persisted domain state

## 8. Verification

- [ ] 8.1 Domain unit tests: exact on-hand match, substitution match at each tier, quantity
      shortfall, missing ingredient with no substitute, `UNKNOWN`-confidence lot handling
- [ ] 8.2 Domain unit tests: recipe-level aggregation rule across mixed per-ingredient verdicts
- [ ] 8.3 `openspec validate implement-recipe-availability`
