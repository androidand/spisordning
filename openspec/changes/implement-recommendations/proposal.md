# Implement recommendations

## Why

`PLAN.md`'s "Recommendation Domain" is explicit that recommendations must be **deterministic,
explainable candidate/ranking logic — not merely an LLM response**. `internal/scoring/scoring.go`
already delivers exactly that shape: a pure function of a candidate and a `PlanContext`,
producing a `Breakdown` per signal (preference, effort, repetition, school dedup, campaign) and
a feasibility flag, with the LLM (Olla) used only additively (varying candidates, writing prose
explanations) and never gating feasibility. This change **extends that scorer**, it does not
replace it: it adds the dimensions and modes `PLAN.md`'s "Recommendation Inspiration" section
asks for that the current scorer has no notion of at all — balancing **KNOWN FAVORITES** against
**DISCOVERY/NOVELTY**, and the future user controls PLAN.md names (`safe choice` / `something
similar` / `surprise me` / `something completely new`).

This change depends on work landing elsewhere for the fuller Recommendation Domain input surface
`PLAN.md` lists (people eating, allergies, preferences, ratings, meal history, recent meals,
pantry availability, expiry, substitutions, effort, time, price, shopping requirements):
`implement-recipe-availability` and `implement-pantry-inventory` for pantry
availability/expiry/substitutions, `establish-household-and-catalog` for allergies as a hard
filter, `implement-meals-and-preferences` for ratings/favorites, and `implement-meal-planning`
for the API surface these control modes are threaded through. This change's own scope — novelty/
familiarity balance and user-facing control modes — does not require any of those to land first,
since it operates on data the scorer already has access to (`meal_event` history, `person_
preference`); the fuller input surface is wired incrementally as each dependency lands.

## What Changes

- Extend `internal/scoring`'s `Weights`/`Breakdown` with a novelty/familiarity dimension,
  preserving every existing dimension and its current tests unchanged.
- Define **known favorite**: derived from aggregate `person_preference` sentiment/confidence
  plus `meal_event` frequency for a recipe. Define **discovery/novelty**: recipes with no or
  minimal `meal_event` history for the household.
- Define the four candidate modes PLAN.md names — `safe choice`, `something similar`, `surprise
  me`, `something completely new` — each as a deterministic transformation of scorer weights
  and/or candidate-pool filtering, never a separate scoring algorithm or an LLM decision.
- Decide and implement a default balance guarantee: whether a recommendation batch includes a
  minimum mix of favorites and novel candidates by default, and how each mode shifts that ratio.
- Reaffirm and extend `food-brain-first-slice`'s existing rule: the LLM may vary within the
  deterministic candidate set and generate prose explanations, but MUST NOT decide feasibility,
  novelty classification, or ranking.
- Document the full `PLAN.md` Recommendation Domain input list, marking which inputs this change
  wires now (novelty/familiarity, using existing preference/meal-history data) versus which
  remain deferred to their owning change (pantry/expiry/substitutions, allergies, ratings,
  price) — this change does not implement the deferred inputs.

## Capabilities

### New Capabilities

- `recommendations`: novelty/familiarity balance and explicit user-facing recommendation control
  modes, extending the existing deterministic scorer with the "Recommendation Inspiration"
  dimension `PLAN.md` calls for.

### Modified Capabilities

<!-- none listed here — internal/scoring.go itself is extended in place (see Impact), but the
     `meal-planning` capability's existing requirements (food-brain-first-slice) are unchanged
     in substance; this is new capability surface, not a redefinition of scoring determinism -->

## Impact

- Affected code: `internal/scoring/scoring.go` gains new `Weights`/`Breakdown` fields and a mode
  parameter to `Rank()` (or an equivalent seam); existing scorer tests continue to pass
  unmodified where they don't touch the new fields.
- Depends on `implement-meal-planning` for the API surface that threads a selected mode through
  to the scorer; depends on `implement-recipe-availability` + `implement-pantry-inventory` for
  pantry/expiry/substitution inputs; depends on `establish-household-and-catalog` for allergies
  as a hard filter; depends on `implement-meals-and-preferences` for ratings/favorites data.
  None of these block this change's core novelty/familiarity/mode work, which the existing
  `meal_event`/`person_preference` data already supports.
- No changes to `migrations/0001_init.sql` are required by this change's core scope; a future
  `implement-recipe-discovery` epic may later expand the pool of "novel" candidates beyond
  Mealie's synced recipes, but that is out of scope here.
