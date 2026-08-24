# Tasks: implement-recipe-availability

## 1. Consume upstream models (don't redefine)

- [x] 1.1 Consume `IngredientSubstitution` (`EQUIVALENT`/`GOOD`/`ACCEPTABLE`/`FORM`/`DIETARY`/
      `EMERGENCY`, directional, explicit ratio) from `establish-household-and-catalog` without
      defining a parallel substitution taxonomy — added `SubstitutionTier`, `IngredientSubstitution`
      to `internal/domain/domain.go`; `SubstitutionTierOrder()` walks in decreasing preference.
- [x] 1.2 Consume `IngredientForm` (fresh/dried/canned/frozen) from
      `establish-household-and-catalog` — added `IngredientForm` + four constants to
      `internal/domain/domain.go`; consumed in `LotInfo.Form` and `RecipeLine.PreferredForm`.
- [x] 1.3 Consume `InventoryLot`/`InventoryLocation` (including confidence tiers) from
      `implement-pantry-inventory` as a read-only input — this capability never writes inventory
      — domain `Confidence` type already exists in `internal/domain/pantry.go`; consumed via
      `LotInfo.Confidence` in `internal/availability`.
- [x] 1.4 Consume existing structured recipe ingredients (`recipe_ingredient` in
      `migrations/0001_init.sql`, or its successor from `implement-recipe-family-and-revisions`)
      — `RecipeLine` in `internal/availability` mirrors the recipe ingredient shape; form
      fields carried through from `shopping_requirement`/`ingredient_mapping` at call site.

## 2. Vocabulary & scope

- [x] 2.1 Define the tri-state verdict: `feasible` / `feasible-with-substitution` / `infeasible`,
      both per-ingredient-line and recipe-level — `RecipeStatus` constants in
      `internal/availability/availability.go`; per-line `IngredientStatus` adds
      `on-hand-uncertain` and `missing` for explainability.
- [x] 2.2 Explicitly scope out: shopping-gap computation (owned by `shopping_requirement` /
      `implement-shopping-and-commerce`), pantry mutation (read-only consumer of inventory),
      recipe scaling/servings math beyond what the recipe ingredient line already encodes
      — `EvaluateRecipe` takes pre-fetched `RecipeLine`s with fixed quantities; no gap or
      mutation logic present. Doc comment in package godoc makes this explicit.

## 3. Per-ingredient feasibility

- [x] 3.1 Match a recipe ingredient line against on-hand `InventoryLot`s via
      `ProductIngredientMapping`/`ingredient_id` — `findBestDirectLot` matches on
      `LotInfo.IngredientID`; form and confidence are secondary sort keys.
- [x] 3.2 Apply `IngredientForm` matching: prefer an on-hand lot whose form matches the recipe's
      required/preferred form; treat a mismatched form as requiring a `FORM`-category
      substitution, not an automatic match — `formMatchesPreferred` in
      `findBestDirectLot`; mismatched-form lots are deprioritized (not auto-matched);
      `TestEvaluateRecipe_FormMismatchRequiresSubstitution` verifies FORM sub is triggered.
- [x] 3.3 When no direct match exists, walk `IngredientSubstitution` in decreasing preference
      order (`EQUIVALENT` → `GOOD` → `ACCEPTABLE` → `FORM` → `DIETARY` → `EMERGENCY`) and record
      which tier, if any, resolved the line — `domain.SubstitutionTierOrder()` drives the walk;
      `TestEvaluateRecipe_SubstitutionAtEachTier` and `TestEvaluateRecipe_SubstitutionTierOrder` verify.
- [x] 3.4 Apply each substitution's explicit quantity ratio when computing whether the
      substituted lot's quantity actually covers the requirement — never assume 1:1
      — `line.Quantity * sub.Ratio` used everywhere; `TestEvaluateRecipe_ExplicitRatioNotAssumed`
      verifies 100g fresh → 33g dried with 20g on-hand → missing with shortfall.
- [x] 3.5 Decide and document the partial-quantity policy: partial = unmet (not partially
      satisfied) — documented in `findBestDirectLot` comment and `TestEvaluateRecipe_PartialQuantityIsUnmet`.
      Feeds shopping-gap cleanly: shortfall is reported on the `missing` line.
- [x] 3.6 A lot with `UNKNOWN` confidence SHALL NOT silently satisfy a requirement —
      satisfied-but-flagged: `StatusOnHandUncertain` with reason "on-hand-uncertain";
      recipe-level verdict downgrades from `feasible` to `feasible-with-substitution`
      when any line is uncertain. `TestEvaluateRecipe_UnknownConfidenceFlagged` verifies.

## 4. Overall recipe feasibility

- [x] 4.1 Aggregate per-ingredient verdicts into the recipe-level verdict — `computeRecipeStatus`
      in `availability.go` aggregates from per-line statuses.
- [x] 4.2 Define the aggregation rule precisely: any missing → `infeasible`; any substituted
      or uncertain → `feasible-with-substitution`; all on-hand confident → `feasible`.
      `TestEvaluateRecipe_RecipeLevelAggregation` and `TestEvaluateRecipe_AllOnHandFeasible` verify.

## 5. Explainability

- [x] 5.1 Every per-ingredient result carries a machine-readable reason: on-hand,
      `substituted-<tier>`, or missing (with the shortfall, if partial) — `IngredientVerdict.Reason`
      is always set; values: "on-hand", "on-hand-uncertain", "substituted-<TIER>",
      "substituted-<TIER>-uncertain", "missing", "missing-shortfall".
- [x] 5.2 No opaque scoring — this feeds `implement-recommendations`' pantry-availability and
      expiry inputs, which `PLAN.md` requires to be explainable — the recipe verdict is
      fully derivable from per-line results; no score or weight is computed here.

## 6. Expiry awareness

- [x] 6.1 Surface which on-hand lots a recipe's feasible verdict would consume, and which of
      those are near expiry (best-before soon) — `IngredientVerdict.ConsumedLotIDs` and
      `NearExpiryLotIDs` surface consumed lots; `RecipeVerdict.NearExpiryLotIDs` aggregates.
      Near-expiry threshold is 7 days; `Now` controls whether expiry is evaluated (zero =
      skip). `TestEvaluateRecipe_NearExpirySurface` and `TestEvaluateRecipe_NearExpiryNoSurfaceWhenNowZero` verify.

## 7. Persistence & interface

- [x] 7.1 Decide whether this stays pure on-demand domain logic (computed from
      `inventory_lot` + recipe ingredients + `ingredient_substitution` at call time) or whether a
      cache/materialized view is warranted for planning-time performance; default to no new
      persisted state unless a concrete performance need is shown — pure domain logic, no DB
      access, no new tables. `EvaluateRecipe` takes pre-fetched data.
- [x] 7.2 Expose as an internal Go package, with a (future) HTTP endpoint once
      `establish-enforced-go-architecture` lands an HTTP server — not new persisted domain state
      — `internal/availability` package; no HTTP, no persistence, no new tables.

## 8. Verification

- [x] 8.1 Domain unit tests: exact on-hand match, substitution match at each tier, quantity
      shortfall, missing ingredient with no substitute, `UNKNOWN`-confidence lot handling
      — 22 tests in `internal/availability/availability_test.go` covering all scenarios.
- [x] 8.2 Domain unit tests: recipe-level aggregation rule across mixed per-ingredient verdicts
      — `TestEvaluateRecipe_RecipeLevelAggregation` (mixed on-hand/sub/missing → infeasible),
      `TestEvaluateRecipe_AllOnHandFeasible`, `TestEvaluateRecipe_SubstitutionTierOrder`.
- [x] 8.3 `openspec validate implement-recipe-availability` — passes. All 211 repo tests pass.
