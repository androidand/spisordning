# Implement meal planning

## Why

`food-brain-first-slice` already shipped the durable weekly-planning schema
(`migrations/0001_init.sql`: `meal_plan`, `meal_plan_candidate`, `meal_plan_decision`,
`planning_constraint`, `effort_profile`, `shopping_requirement`) and a deterministic scorer
(`internal/scoring/scoring.go`) that ranks candidates on preferences+confidence, effort vs. the
day's kitchen energy, a repetition penalty, Skolmaten school-lunch dedup, and Willys campaign
bias — plus `internal/planning/requirements.go`, which aggregates chosen meals into canonical
shopping requirements. `docs/research/current-state.md` confirms this all works today only as a
CLI pipe (`food-brain plan`, `cmd/food-brain/plan.go`), with **nothing writing to Postgres yet**.

This change is primarily about exposing that already-existing domain logic through a real
weekly-planning HTTP API — **not** redesigning the underlying model. It depends on
`establish-enforced-go-architecture`, which is where real Postgres persistence, the HTTP server,
and the design-first OpenAPI contract land; this change cannot ship an API before that
foundation exists, and should be sequenced after it.

`PLAN.md`'s "Recommendation Domain" lists the full target input surface: people eating,
allergies, preferences, ratings, meal history, recent meals, pantry availability, expiry,
substitutions, effort, time, price, shopping requirements. Of these, the existing scorer already
wires: **preferences+confidence, effort vs. day energy, recent-meal repetition, Skolmaten dedup,
Willys campaign bias**. Not yet wired: **pantry availability / expiry / substitutions** (depend
on `implement-pantry-inventory` and `implement-recipe-availability` landing first),
**allergies as a hard filter** (depends on `establish-household-and-catalog`'s
`PersonRestriction` — likes/dislikes-vs-allergies split — landing first), **ratings / people
eating per slot** (depend on `implement-meals-and-preferences`'s `MealReview`/`Favorite`/
attendance model), and **price** (a later price-intelligence epic). This change surfaces what
already exists and leaves explicit seams for the rest; it does not implement the deferred
inputs.

## What Changes

- A weekly-planning HTTP API layer over the existing `meal_plan`/`meal_plan_candidate`/
  `meal_plan_decision`/`planning_constraint`/`effort_profile` tables and the existing
  `internal/scoring.Rank()` / `internal/planning.BuildRequirements()` functions — reused as-is,
  not forked or redesigned.
- Endpoints to create a draft weekly plan, list ranked candidates per slot (with score
  breakdown), record a decision per slot, approve a plan, manage `planning_constraint`/
  `effort_profile`, and materialize `shopping_requirement`s from decisions.
- Expose the scorer's `Breakdown`/`Reason` fields over the API so a client can show *why* a
  candidate ranked where it did, and expose Olla-generated explanations where present — clearly
  marked additive/non-authoritative, per `food-brain-first-slice`'s existing D2/D3 decisions
  (the LLM never gates feasibility).
- Document, in `tasks.md`, exactly which `PLAN.md` Recommendation Domain inputs are wired today
  vs. deferred, and leave interface seams (not implementations) for the deferred ones.
- Explicitly **not** in scope: new scoring dimensions (novelty/familiarity balance and
  user-facing control modes are `implement-recommendations`), pantry/allergy/rating integration
  (blocked on the dependencies above), any change to `migrations/0001_init.sql`'s existing
  planning tables.

## Capabilities

### Modified Capabilities

- `meal-planning`: extends the capability `food-brain-first-slice` introduced (durable family
  food model, deterministic scoring, canonical shopping requirements) with a weekly-planning
  HTTP API surface. The underlying domain model and scorer are unchanged; this change adds how
  they're reached.

## Impact

- Affected code: new `internal/httpapi` (or equivalent, shared with
  `establish-enforced-go-architecture`) handlers for the planning domain; `api/openapi.yaml`
  gains the weekly-planning endpoints. No changes to `internal/scoring`, `internal/planning`, or
  `migrations/0001_init.sql`.
- Depends on `establish-enforced-go-architecture` (HTTP server + real persistence + OpenAPI
  contract) — this change should land after it.
- Depends on, but does not implement, `implement-pantry-inventory`,
  `implement-recipe-availability`, `establish-household-and-catalog` (allergies), and
  `implement-meals-and-preferences` (ratings/attendance) for the deferred input surface.
- `cmd/food-brain/plan.go`'s CLI pipe either continues to work unchanged or is reimplemented as
  a client of the new API — decided during implementation, tracked in `tasks.md`.
