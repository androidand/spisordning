# Tasks: implement-recommendations

## 1. Consume the existing scorer, don't replace it

- [x] 1.1 Extend `internal/scoring/scoring.go`'s `Weights`/`Breakdown` with novelty/familiarity
      dimensions rather than forking a parallel scorer
- [x] 1.2 Preserve existing dimensions and their current weights/tests unchanged: preferences+
      confidence, effort budget, repetition penalty, Skolmaten dedup, Willys campaign bias
      (verified: all 5 original dimensions + weights intact in Weights/Breakdown — Preference 1.0,
      Effort 0.6, Repetition 0.8, SchoolDedup 0.7, Campaign 0.4; scoring_test.go diff is
      additions-only so the 9 original tests are byte-for-byte unchanged; go test ./... = 87 pass)

## 2. Familiarity vs. novelty vocabulary

- [x] 2.1 Define "known favorite": derived from aggregate `person_preference` sentiment/
      confidence plus `meal_event` frequency for a recipe
      (implemented `scoring.IsKnownFavorite`: net-positive confidence-weighted preferenceScore AND
      cookCount >= favoriteMinCooks(2); extracted `cookCount` helper reused by familiarityScore;
      4 tests — liked+cooked=favorite, liked+never-cooked=no, cooked+disliked=no, deterministic;
      go test ./... = 91 pass)
 - [x] 2.2 Define "discovery/novelty": recipes with no or minimal `meal_event` history for the
      household, or recently added via a future recipe-discovery capability
      (verified: `scoring.IsDiscovery` — novel iff `cookCount <= discoveryMaxCooks`(1), the
      preference-agnostic mirror of `favoriteMinCooks`; 5 tests incl. never/once/regularly-cooked
      and preference-independence; `go test ./...` 96 pass, `openspec validate` valid)
- [x] 2.3 Define a novelty score dimension analogous to the existing `Breakdown` fields —
      deterministic, and explainable in the same `Breakdown`/`Reason` shape as today's scorer
      (verified: `Familiarity` Breakdown field is the novelty dimension; added `familiarityReason`
      — a deterministic, LLM-free note (novel (never/rarely cooked) / known favorite / familiar) —
      wired into `ScoredCandidate.Reason` (feasibility note prefixed when infeasible); 6 tests;
      `go test ./...` 102 pass, `openspec validate` valid)

## 3. User-facing control modes

- [x] 3.1 Model the four candidate modes PLAN.md names: `safe choice`, `something similar`,
      `surprise me`, `something completely new`
      (implemented `scoring.Mode` type + `ModeSafeChoice`/`ModeSomethingSimilar`/`ModeSurpriseMe`/
      `ModeCompletelyNew` constants and `Modes()`; go test ./... = 124 pass)
- [x] 3.2 Define each mode as a deterministic transformation of scorer weights and/or candidate-
      pool filtering — never a separate scoring algorithm and never an LLM decision
      (implemented `Mode.WeightsFor()`: each mode is a pure re-weighting of `DefaultWeights` over
      the same scorer — safe choice raises Preference/Effort/Familiarity and lowers Repetition;
      surprise me/completely new flip Familiarity negative to pull toward novelty; no separate
      algorithm, no LLM; `TestMode_WeightsDistinct` proves the four transformations are distinct)
- [x] 3.3 Decide the default mode and how the selected mode is threaded through the
      `implement-meal-planning` API into the scorer
      (default is `DefaultMode` = `something similar`; `RankWithMode(candidates, ctx, mode)` is the
      single entry point that applies the mode's weights — an empty mode falls back to
      `DefaultMode`. The meal-planning API threads the user-selected mode here; until that lands
      the CLI passes `DefaultMode`. `TestRankWithMode_EmptyUsesDefault` pins the fallback)

## 4. Balance guarantee

- [x] 4.1 Decide whether a recommendation batch includes a minimum mix of favorites and novel
      candidates by default, and how `safe choice` vs. `surprise me` shift that ratio
      (implemented `SelectBatch(ranked, ctx, n)`: when the pool has both known favorites and
      discovery candidates, the batch includes at least one of each — it reorders (never
      re-scores) the mode-ranked list, so the deterministic ranking is preserved. The mix ratio is
      driven by the mode's weights (safe choice leans familiar, surprise me leans novel); the
      guarantee only prevents a degenerate all-one-group batch. `TestSelectBatch_BalanceGuarantee`,
      `TestSelectBatch_AllFavorites`, `TestSelectBatch_SmallBatch` cover the guarantee, the honest
      all-favorites case, and the 1-slot edge case)
- [x] 4.2 Explainability: each candidate's reason SHALL state whether it was surfaced for
      familiarity or for novelty
      (verified: `familiarityReason` — a deterministic, LLM-free note (novel (never/rarely cooked) /
      known favorite / familiar) — is wired into `ScoredCandidate.Reason` (feasibility note
      prefixed when infeasible); `TestScore_ReasonKeepsFeasibilityWhenInfeasible` pins the
      combined note)

## 5. Full Recommendation Domain input surface (PLAN.md list)

- [x] 5.1 Audit the full input list: people eating, allergies, preferences, ratings, meal
      history, recent meals, pantry availability, expiry, substitutions, effort, time, price,
      shopping requirements — record which this change wires and which remain deferred
      (audited against `domain.PlanContext`. WIRED by this change: people eating (People),
      preferences (Preferences), meal history + recent meals (RecentMealIDs -> cookCount/repetition),
      effort (KitchenEnergy + candidate.Effort), time (Day -> repetition window), Skolmaten dedup
      (SchoolLunchTags), Willys campaign bias (CampaignIngredients). DEFERRED: allergies (5.3),
      ratings/favorites (5.4), pantry availability + expiry + substitutions (5.2), price (5.5),
      shopping requirements (implement-shopping-and-commerce). No input is silently dropped — each
      deferred input has a named owning change)
- [x] 5.2 Wire pantry availability, expiry, and substitutions once `implement-recipe-
      availability` / `implement-pantry-inventory` land. **Done 2026-08-22:** `implement-recipe-
      availability` and `implement-pantry-inventory` now exist. Wired by: (1) adding
      `AvailabilityVerdicts map[string]string` to `domain.PlanContext` — a recipe-ID → verdict
      string map populated by the caller once the availability capability has been evaluated;
      (2) extending `scoring.feasibility()` to check the map and return `false` with reason
      "ingredients not available in pantry" when the verdict is `infeasible`; (3) 6 new tests
      covering: infeasible verdict blocks, feasible/feasible-with-sub don't block, missing
      data is ignored (backward compat), both constraints block, infeasible ranks last,
      feasible-with-sub still passes. Verdict strings match `availability.VerdictFeasible`,
      `availability.VerdictFeasibleWithSub`, `availability.VerdictInfeasible`. Expiry and
      substitutions are surfaced through the availability verdict's per-line `Reason` and
      `ConsumedLotIDs` fields — the scorer uses the recipe-level verdict as the hard gate,
      while the detailed line breakdown feeds the `ScoredCandidate.Reason` when a future
      consumer joins the verdict.
- [ ] 5.3 Wire allergies as a hard filter — never a scored, negotiable dimension — once
      `establish-household-and-catalog`'s `PersonRestriction` model lands — **DEFERRED** (out of
      scope here; the restriction model does not exist yet)
- [ ] 5.4 Wire ratings/favorites once `implement-meals-and-preferences`'s `MealReview`/
      `Favorite` model lands — **DEFERRED** (out of scope here; the review/favorite model does not
      exist yet)
- [ ] 5.5 Wire price once a future price-intelligence capability exists — **DEFERRED** (later
      epic; no price-intelligence capability exists yet)

## 6. Never merely an LLM response

- [x] 6.1 Reaffirm and extend `food-brain-first-slice`'s existing rule (its D2/D3 design
      decisions): the LLM may vary within the deterministic candidate set and generate prose
      explanations, but MUST NOT decide feasibility, novelty classification, or ranking
      (encoded in `integrate-ai` design/spec; enforced by `TestProviderError_Propagates`,
      `TestScore_ReasonKeepsFeasibilityWhenInfeasible`, and the Olla-down E2E test)
- [x] 6.2 Assert this with a test analogous to the existing scorer reproducibility test: ranking
      and mode selection are identical across repeated runs with Olla unavailable
      (`TestRunPlan_EndToEnd_OllaUnavailable` in cmd/food-brain/plan_test.go)

## 7. Verification

- [x] 7.1 Unit tests: novelty/familiarity scoring is deterministic and reproducible without the
      LLM present (`TestFamiliarity_Deterministic`, `TestIsKnownFavorite_Deterministic`,
      `TestIsDiscovery_Deterministic`, `TestRank_DeterministicAndReproducible`)
- [x] 7.2 Unit tests: each control mode produces a distinct, deterministic weight/filter
      transformation from the same candidate pool
      (`TestMode_WeightsDistinct` — the four modes yield four distinct `Weights`;
      `TestMode_RankingDistinct` — familiarity-seeking modes top the known favorite, novelty-seeking
      modes top the discovery candidate; `TestMode_Deterministic` — each mode is reproducible across
      5 runs; go test ./... = 124 pass)
- [x] 7.3 Unit tests: the balance guarantee (task 4.1) holds across a representative candidate
      pool mix
      (`TestSelectBatch_BalanceGuarantee` — a 3-slot batch over a 4-favorite/2-novel pool includes at
      least one of each; `TestSelectBatch_AllFavorites` — an all-favorites pool yields an honest
      all-favorites batch; `TestSelectBatch_SmallBatch` — a 1-slot batch returns the top candidate
      as-is)
- [x] 7.4 `openspec validate implement-recommendations`
