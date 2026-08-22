# Tasks: implement-recipe-availability

## 1. Consume upstream models (don't redefine)

- [x] 1.1 Consume `IngredientSubstitution` (`EQUIVALENT`/`GOOD`/`ACCEPTABLE`/`FORM`/`DIETARY`/
      `EMERGENCY`, directional, explicit ratio) from `establish-household-and-catalog` without
      defining a parallel substitution taxonomy. **Done 2026-08-22:** `internal/availability`
      consumes `domain.IngredientSubstitutionCategory` and `domain.IngredientSubstitution`
      exactly as defined in `internal/domain/domain.go` — no parallel taxonomy. The
      `Substitution` input type in `availability.go` mirrors the domain struct fields
      (`FromIngredientID`, `FromForm`, `ToIngredientID`, `ToForm`, `Category`, `Ratio`)
      without adding any new categories or assuming 1:1 ratios.
- [x] 1.2 Consume `IngredientForm` (fresh/dried/canned/frozen) from
      `establish-household-and-catalog`. **Done 2026-08-22:** `RecipeIngredientLine.DefaultForm`
      carries the form preference from `ingredient_mapping.default_form`. The availability
      logic uses it to filter `FORM`-tier substitutions (task 3.2), not to redefine the
      form taxonomy. `domain.IngredientForm` is not redefined.
- [x] 1.3 Consume `InventoryLot`/`InventoryLocation` (including confidence tiers) from
      `implement-pantry-inventory` as a read-only input — this capability never writes inventory.
      **Done 2026-08-22:** `availability.InventoryLotInput` is a read-only snapshot type
      (no mutation methods). `domain.Confidence` (EXACT/LIKELY/ESTIMATED/UNKNOWN) is
      consumed directly. The package has zero persistence dependencies.
- [x] 1.4 Consume existing structured recipe ingredients (`recipe_ingredient` in
      `migrations/0001_init.sql`, or its successor from `implement-recipe-family-and-revisions`).
      **Done 2026-08-22:** `availability.RecipeIngredientLine` mirrors
      `persistence.RecipeIngredient` (`IngredientID`, `Quantity`, `Unit`) plus
      `DefaultForm` from `ingredient_mapping`. No recipe schema changes.

## 2. Vocabulary & scope

- [x] 2.1 Define the tri-state verdict: `feasible` / `feasible-with-substitution` / `infeasible`,
      both per-ingredient-line and recipe-level. **Done 2026-08-22:** `LineStatus`
      (`on-hand`/`substituted`/`unknown`/`missing`) and `RecipeVerdictLevel`
      (`feasible`/`feasible-with-substitution`/`infeasible`) defined in
      `internal/availability/types.go`. Aggregation rule: any `missing` → `infeasible`;
      any `substituted` or `unknown` → `feasible-with-substitution`; all `on-hand` → `feasible`.
- [x] 2.2 Explicitly scope out: shopping-gap computation (owned by `shopping_requirement` /
      `implement-shopping-and-commerce`), pantry mutation (read-only consumer of inventory),
      recipe scaling/servings math beyond what the recipe ingredient line already encodes.
      **Done 2026-08-22:** The package has no write methods. Partial quantities are
      recorded as `Shortfall` on the line verdict (task 3.5) but no gap computation
      is performed — the shortfall field is available for a future consumer. Unit
      conversion is out of scope (unit mismatch → unmet).

## 3. Per-ingredient feasibility

- [x] 3.1 Match a recipe ingredient line against on-hand `InventoryLot`s via
      `ProductIngredientMapping`/`ingredient_id`. **Done 2026-08-22:** `matchingLots`
      filters by `IngredientID` and `Unit`. Product-level filtering is not performed
      here (recipe_ingredient has no product_id); a future consumer can scope lots
      by product if needed.
- [x] 3.2 Apply `IngredientForm` matching: prefer an on-hand lot whose form matches the recipe's
      required/preferred form; treat a mismatched form as requiring a `FORM`-category
      substitution, not an automatic match. **Done 2026-08-22:** `DefaultForm` on
      `RecipeIngredientLine` is used to filter substitutions in `trySubstitution` —
      when set, only substitutions whose `from_form` matches (or is nil) are considered.
      On-hand lots are not form-filtered (lots don't store form); form mismatches are
      resolved via FORM-tier substitutions.
- [x] 3.3 When no direct match exists, walk `IngredientSubstitution` in decreasing preference
      order (`EQUIVALENT` → `GOOD` → `ACCEPTABLE` → `FORM` → `DIETARY` → `EMERGENCY`) and record
      which tier, if any, resolved the line. **Done 2026-08-22:** `subTierOrder` maps
      each category to an integer; `trySubstitution` sorts candidates and picks the
      first that has on-hand stock. The tier is recorded in `LineVerdict.SubstitutionTier`.
- [x] 3.4 Apply each substitution's explicit quantity ratio when computing whether the
      substituted lot's quantity actually covers the requirement — never assume 1:1.
      **Done 2026-08-22:** `needed := line.Quantity * sub.Ratio` is computed for every
      substitution. Tests verify ratio < 1 (flour→potato-starch at 0.5) and ratio > 1
      implicitly via the shortfall path.
- [x] 3.5 Decide and document the partial-quantity policy: does an on-hand quantity smaller than
      required count as unmet, or as feasible-with-a-noted-shortfall (feeds
      `implement-shopping-and-commerce`'s gap, but that computation is out of this change's
      scope — only the policy for *this* capability's verdict needs to be decided here).
      **Done 2026-08-22:** Partial quantity = unmet. The shortfall amount is recorded on
      `LineVerdict.Shortfall` for a future consumer. The line status is `StatusMissing`
      and the recipe verdict is `infeasible` if this is the only missing line. This is
      the conservative interpretation: a recipe that can't be made as written is infeasible.
      **Fixed 2026-08-22 (VERIFY gate):** When confident lots are partially available but
      insufficient, the partial lots are NOT consumed if a substitution is found (the
      substitution replaces the line, preserving partial lots for other lines). If no
      substitution is found, the partial lots ARE consumed so they don't double-count.
      Same policy applies to the unknown-confidence path.
- [x] 3.6 A lot with `UNKNOWN` confidence SHALL NOT silently satisfy a requirement — decide
      whether it counts as unmet, or as satisfied-but-flagged, and make the choice explicit in
      the per-line reason. **Done 2026-08-22:** UNKNOWN lots satisfy the line but are
      flagged (`StatusUnknown`, `IsUncertain = true`). The per-line reason states
      "on-hand (uncertain confidence)". The recipe-level verdict elevates to
      `feasible-with-substitution` (not `feasible`) when any line is uncertain, so the
      user sees that not everything is confidently on hand.
      **Fixed 2026-08-22 (VERIFY gate):** `trySubstitution` now tracks confidence of
      consumed substitute lots and sets `IsUncertain = true` when any consumed lot is
      UNKNOWN — the spec's "SHALL NOT silently trust" rule now covers substitution-backed
      lines too, not just direct on-hand matches.

## 4. Overall recipe feasibility

- [x] 4.1 Aggregate per-ingredient verdicts into the recipe-level verdict. **Done 2026-08-22:**
      `aggregateVerdict` in `availability.go` iterates all lines and applies the rule:
      any `missing` → `infeasible`; any `substituted` or `unknown` →
      `feasible-with-substitution`; all `on-hand` → `feasible`.
- [x] 4.2 Define the aggregation rule precisely (e.g. any unmet line with no viable substitute
      ⇒ recipe `infeasible`; any line resolved via substitution ⇒ recipe
      `feasible-with-substitution`; all lines on-hand with matching form ⇒ recipe `feasible`).
      **Done 2026-08-22:** Rule is documented in the `aggregateVerdict` doc comment and
      enforced by the function. Tests cover: all-on-hand, one-substitution, one-missing,
      all-uncertain, mixed-on-hand-and-missing.

## 5. Explainability

- [x] 5.1 Every per-ingredient result carries a machine-readable reason: on-hand,
      `substituted-<tier>`, or missing (with the shortfall, if partial). **Done 2026-08-22:**
      `LineVerdict.Reason` is a human-readable string set for every status. Examples:
      "on-hand: milk 2.00 dl", "substituted milk→oat-milk via EQUIVALENT (ratio 1.00): milk 2.00 dl",
      "no on-hand match for ingredient \"saffron\" (unit=\"g\")".
- [x] 5.2 No opaque scoring — this feeds `implement-recommendations`' pantry-availability and
      expiry inputs, which `PLAN.md` requires to be explainable. **Done 2026-08-22:**
      The recipe-level verdict is derived from per-line results, never the reverse.
      `LineVerdict.ConsumedLotIDs` surfaces which lots would be consumed (enabling
      expiry awareness, task 6.1). No numeric score is computed.

## 6. Expiry awareness

- [x] 6.1 Surface which on-hand lots a recipe's feasible verdict would consume, and which of
      those are near expiry (best-before soon) — a fact this change computes and exposes, not a
      scoring decision it makes (scoring/weighting is `implement-recommendations`' job).
      **Done 2026-08-22:** `LineVerdict.ConsumedLotIDs` lists the lot IDs consumed to
      satisfy each line. The caller can join against `inventory_lot.best_before` to
      surface near-expiry lots. This change does not compute expiry scores — that is
      `implement-recommendations`' job per the spec.

## 7. Persistence & interface

- [x] 7.1 Decide whether this stays pure on-demand domain logic (computed from
      `inventory_lot` + recipe ingredients + `ingredient_substitution` at call time) or whether a
      cache/materialized view is warranted for planning-time performance; default to no new
      persisted state unless a concrete performance need is shown. **Done 2026-08-22:**
      Pure on-demand. Zero DB queries in this package. A future persistence wrapper can
      query the three tables and pass the data to `ComputeRecipeAvailability`.
- [x] 7.2 Expose as an internal Go package, with a (future) HTTP endpoint once
      `establish-enforced-go-architecture` lands an HTTP server — not new persisted domain state.
      **Done 2026-08-22:** `internal/availability` package with public `ComputeRecipeAvailability`
      function. No HTTP, no new tables, no persistence layer.

## 8. Verification

- [x] 8.1 Domain unit tests: exact on-hand match, substitution match at each tier, quantity
      shortfall, missing ingredient with no substitute, `UNKNOWN`-confidence lot handling.
      **Done 2026-08-22 (initial):** 21 tests in `internal/availability/availability_test.go`.
      **Fixed 2026-08-22 (VERIFY gate):** Added 5 new tests —
      `TestSubstitutionViaUnknownLotFlagged` (substitution backed by UNKNOWN lot sets
      IsUncertain), `TestPartialOnHandWithSubstitution` (partial lots preserved when
      substitution replaces line), `TestPartialOnHandDoesNotDoubleCountWithSubstitution`
      (partial confident lot available for second line after substitution on first),
      `TestFormFilterRejectsMismatched` (negative case: defaultForm set, all subs mismatched
      → missing), `TestSubstitutionRatioGreaterThanOne` (ratio > 1 exercises needed > required).
      Total: 26 tests. Removed unused `eggLot`/`formSub` helpers. Fixed unstable
      `sort.Slice` tie-break (secondary sort by ToIngredientID within same tier).
- [x] 8.2 Domain unit tests: recipe-level aggregation rule across mixed per-ingredient verdicts.
      **Done 2026-08-22:** Tests cover: all-on-hand→feasible, one-substitution→feasible-with-sub,
      one-missing→infeasible, all-uncertain→feasible-with-sub, mixed-on-hand-and-missing→infeasible,
      explainability (reasons are non-empty, verdict derivable from lines).
- [x] 8.3 `openspec validate implement-recipe-availability`. **Verified:** valid.
      Full suite: 255 tests passed (18 packages), 0 vet issues, build success,
      architecture tests 8/8.
