# Tasks: implement-meal-planning

## 1. Foundations

- [ ] 1.1 Depends on `establish-enforced-go-architecture` landing a real HTTP server and
      persistence layer (`food-brain` is currently CLI-only via `cmd/food-brain/plan.go`)
- [ ] 1.2 Reuse `migrations/0001_init.sql`'s existing `meal_plan`/`meal_plan_candidate`/
      `meal_plan_decision`/`planning_constraint`/`effort_profile` tables as-is — do not redesign
      this schema in this change
- [ ] 1.3 Reuse `internal/scoring/scoring.go`'s deterministic `Rank()` and
      `internal/planning/requirements.go`'s `BuildRequirements()` as-is — do not fork a parallel
      scorer or requirements builder

## 2. Weekly planning API surface

- [ ] 2.1 `POST /plans` — create a draft weekly plan for a given `week_start`
- [ ] 2.2 `GET /plans/:id` — ranked candidates per `slot_date` with score breakdown and
      feasibility, from `meal_plan_candidate`
- [ ] 2.3 `POST /plans/:id/decisions` — record the chosen recipe per `slot_date` into
      `meal_plan_decision`
- [ ] 2.4 `POST /plans/:id/approve` — draft → approved status transition on `meal_plan`
- [ ] 2.5 `GET`/`PUT` `planning_constraint` and `effort_profile` (weekday kitchen energy, active
      constraints)
- [ ] 2.6 `POST /plans/:id/shopping-requirements` — materialize `shopping_requirement` rows from
      decisions via `internal/planning.BuildRequirements`

## 3. Recommendation Domain input surface — wired vs. deferred

- [ ] 3.1 Document which `PLAN.md` Recommendation Domain inputs (people eating, allergies,
      preferences, ratings, meal history, recent meals, pantry availability, expiry,
      substitutions, effort, time, price, shopping requirements) are already wired through the
      existing scorer: preferences+confidence, effort vs. day energy, recent-meal repetition
      penalty, Skolmaten school-lunch dedup, Willys campaign bias
- [ ] 3.2 Document which remain unwired and why: pantry availability / expiry / substitutions
      (blocked on `implement-pantry-inventory` + `implement-recipe-availability`), allergies as
      a hard filter (blocked on `establish-household-and-catalog`'s `PersonRestriction`), ratings
      and per-slot people-eating (blocked on `implement-meals-and-preferences`), price (blocked
      on a later price-intelligence epic)
- [ ] 3.3 Leave explicit extension seams (interface points, not implementations) in the API/
      domain layer for each deferred input, so later changes wire them without reshaping this
      one

## 4. Explainability surface

- [ ] 4.1 Expose `scoring.Breakdown` per candidate over the API so a client can show why a
      candidate ranked where it did
- [ ] 4.2 Expose Olla-generated explanations (`internal/llm`) where present, clearly marked as
      additive/non-authoritative, consistent with `food-brain-first-slice`'s existing rule that
      the LLM never gates feasibility or overrides the scorer

## 5. OpenAPI & UX

- [ ] 5.1 Author the weekly-planning endpoints in `api/openapi.yaml`
      (`establish-enforced-go-architecture`'s design-first contract)
- [ ] 5.2 Decide on minimal UX for this slice: a server-rendered review page (akin to the
      existing `GET /review` picker in the retailer adapter) vs. API-only for now

## 6. Verification

- [ ] 6.1 API integration tests against the real Postgres schema (no schema changes introduced
      by this task)
- [ ] 6.2 Decide and implement: `food-brain plan` CLI continues to work unchanged, or is
      reimplemented as a client of the new HTTP API
- [ ] 6.3 `openspec validate implement-meal-planning`
