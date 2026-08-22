# meal-planning (delta)

## ADDED Requirements

### Requirement: Weekly planning is reachable over HTTP without reimplementing scoring

The system SHALL expose weekly meal planning (creating a draft plan, viewing ranked candidates,
recording decisions, approving a plan, materializing shopping requirements) as an HTTP API
backed by the existing `meal_plan`/`meal_plan_candidate`/`meal_plan_decision`/
`planning_constraint`/`effort_profile` schema and the existing deterministic scorer. The API
SHALL NOT compute candidate rankings through logic parallel to or divergent from
`internal/scoring.Rank()`.

#### Scenario: Ranked candidates over the API match the CLI scorer's output

- **WHEN** the same plan inputs are scored via the HTTP API and via the existing `food-brain
  plan` CLI pipe
- **THEN** both produce the identical deterministic ranking, because both call the same
  `internal/scoring.Rank()` function

#### Scenario: A decision is recorded against an existing plan candidate

- **WHEN** a household approves a candidate for a given slot via `POST /plans/:id/decisions`
- **THEN** a `meal_plan_decision` row is created referencing that `meal_plan_candidate`
- **AND** the plan's shopping requirements can subsequently be materialized from that decision

### Requirement: Score explanations are surfaced, not hidden behind the API

Every ranked candidate returned by the API SHALL include its score breakdown and feasibility
reason. Where an LLM-generated explanation exists, it SHALL be included and clearly marked as
additive, never replacing or overriding the deterministic breakdown.

#### Scenario: A client can show why a candidate ranked where it did

- **WHEN** a client requests ranked candidates for a plan slot
- **THEN** each candidate's response includes its `Breakdown` (preference, effort, repetition,
  school dedup, campaign contributions) and feasibility
- **AND** an LLM explanation, if present, is a separate, clearly labeled field

### Requirement: Unwired Recommendation Domain inputs are documented extension seams, not silent gaps

The system SHALL document an explicit extension seam, not a silent omission, for every input in
`PLAN.md`'s Recommendation Domain list that this capability does not yet wire into scoring:
pantry availability, expiry, substitutions, allergies as a hard filter, ratings, and price.

#### Scenario: An unwired input is discoverable, not silently absent

- **WHEN** a developer or reviewer inspects the planning API's scoring inputs
- **THEN** pantry availability, expiry, substitutions, allergy filtering, ratings, and price are
  each documented as not-yet-wired, with the dependency that unblocks them named
- **AND** none of these inputs silently appear to already be handled

### Requirement: Extension seams are explicit for deferred Recommendation Domain inputs

The system SHALL expose interface points (not implementations) for every deferred input so
later changes can wire them without reshaping this one. The seams are:

- **Pantry availability / expiry / substitutions**: blocked on `implement-pantry-inventory`
  and `implement-recipe-availability`. Seam: `domain.Candidate` carries `Ingredients` and
  `Tags`; the scorer's `feasibility()` function can be extended to check pantry stock.
- **Allergies as a hard filter**: blocked on `establish-household-and-catalog`'s
  `PersonRestriction` (likes/dislikes-vs-allergies split). Seam: `domain.PlanContext.People`
  will carry restrictions; `feasibility()` can gate on allergy matches.
- **Ratings / per-slot people-eating**: blocked on `implement-meals-and-preferences`'s
  `MealReview`/`Favorite`/attendance model. Seam: `domain.PlanContext.People` will carry
  ratings; the scorer's `preferenceScore()` can weight by rating history.
- **Price**: blocked on a later price-intelligence epic. Seam: `domain.Candidate` can carry
  price data; a new `Price` weight in `scoring.Weights` can be added.

#### Scenario: A developer can identify the seam for adding pantry availability

- **WHEN** a developer wants to wire pantry availability into scoring
- **THEN** they can extend `domain.Candidate` with stock data and add a check in
  `scoring.feasibility()` without modifying the `PlanService` interface or the HTTP handlers
