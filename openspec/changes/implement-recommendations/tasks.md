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

- [ ] 3.1 Model the four candidate modes PLAN.md names: `safe choice`, `something similar`,
      `surprise me`, `something completely new`
- [ ] 3.2 Define each mode as a deterministic transformation of scorer weights and/or candidate-
      pool filtering — never a separate scoring algorithm and never an LLM decision
- [ ] 3.3 Decide the default mode and how the selected mode is threaded through the
      `implement-meal-planning` API into the scorer

## 4. Balance guarantee

- [ ] 4.1 Decide whether a recommendation batch includes a minimum mix of favorites and novel
      candidates by default, and how `safe choice` vs. `surprise me` shift that ratio
- [ ] 4.2 Explainability: each candidate's reason SHALL state whether it was surfaced for
      familiarity or for novelty

## 5. Full Recommendation Domain input surface (PLAN.md list)

- [ ] 5.1 Audit the full input list: people eating, allergies, preferences, ratings, meal
      history, recent meals, pantry availability, expiry, substitutions, effort, time, price,
      shopping requirements — record which this change wires and which remain deferred
- [ ] 5.2 Wire pantry availability, expiry, and substitutions once `implement-recipe-
      availability` / `implement-pantry-inventory` land — not implemented in this change until
      those exist
- [ ] 5.3 Wire allergies as a hard filter — never a scored, negotiable dimension — once
      `establish-household-and-catalog`'s `PersonRestriction` model lands
- [ ] 5.4 Wire ratings/favorites once `implement-meals-and-preferences`'s `MealReview`/
      `Favorite` model lands
- [ ] 5.5 Wire price once a future price-intelligence capability exists (deferred, later epic)

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
- [ ] 7.2 Unit tests: each control mode produces a distinct, deterministic weight/filter
      transformation from the same candidate pool
- [ ] 7.3 Unit tests: the balance guarantee (task 4.1) holds across a representative candidate
      pool mix
- [x] 7.4 `openspec validate implement-recommendations`
