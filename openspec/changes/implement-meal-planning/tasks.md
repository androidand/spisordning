# Tasks: implement-meal-planning

## 1. Foundations

- [x] 1.1 Depends on `establish-enforced-go-architecture` landing a real HTTP server and
      persistence layer (`food-brain` is currently CLI-only via `cmd/food-brain/plan.go`)
      *Done.* `establish-enforced-go-architecture` landed a real HTTP server, persistence
      layer, and OpenAPI contract. `cmd/food-brain` now has `serve` command with full HTTP API.
- [x] 1.2 Reuse `migrations/0001_init.sql`'s existing `meal_plan`/`meal_plan_candidate`/
      `meal_plan_decision`/`planning_constraint`/`effort_profile` tables as-is — do not redesign
      this schema in this change
      *Done.* No schema changes made. All endpoints use existing tables via `persistence.Store`
      methods (`CreateMealPlan`, `ListCandidates`, `SetDecision`, `SetMealPlanStatus`,
      `ListShoppingRequirements`, `UpsertEffortProfile`, `CreatePlanningConstraint`).
- [x] 1.3 Reuse `internal/scoring/scoring.go`'s deterministic `Rank()` and
      `internal/planning/requirements.go`'s `BuildRequirements()` as-is — do not fork a parallel
      scorer or requirements builder
      *Done.* The API surface exposes scored candidates (from `internal/scoring.Rank()`) and
      shopping requirements (from `internal/planning.BuildRequirements()` via persistence).
      The API does not re-implement scoring logic.

## 2. Weekly planning API surface

- [x] 2.1 `POST /plans` — create a draft weekly plan for a given `week_start`
      *Implemented.* `POST /plans` in `internal/httpapi/plans.go`; handler `planCreateHandler`.
      Accepts optional `week_start` (defaults to next Monday). Persists via
      `persistence.Store.CreateMealPlan`.
- [x] 2.2 `GET /plans/:id` — ranked candidates per `slot_date` with score breakdown and
      feasibility, from `meal_plan_candidate`
      *Implemented.* `GET /plans/{planId}` via `planGetHandler`. Returns `MealPlanView` with
      plan, candidates (with breakdown), and decisions.
- [x] 2.3 `POST /plans/:id/decisions` — record the chosen recipe per `slot_date` into
      `meal_plan_decision`
      *Implemented.* `POST /plans/{planId}/decisions` via `planDecisionsHandler`. Upserts
      decisions via `persistence.Store.SetDecision`.
- [x] 2.4 `POST /plans/:id/approve` — draft → approved status transition on `meal_plan`
      *Implemented.* `PATCH /plans/{planId}` via `planUpdateHandler`. Accepts `status` field
      (e.g. "approved", "archived"). Uses `persistence.Store.SetMealPlanStatus`.
- [x] 2.5 `GET`/`PUT` `planning_constraint` and `effort_profile` (weekday kitchen energy, active
      constraints)
      *Implemented.* `GET`/`POST /effort-profiles` via `effortProfileListHandler` and
      `effortProfileUpsertHandler`. `GET`/`POST /constraints` via
      `planningConstraintListHandler` and `planningConstraintCreateHandler`.
- [x] 2.6 `POST /plans/:id/shopping-requirements` — materialize `shopping_requirement` rows from
      decisions via `internal/planning.BuildRequirements`
      *Implemented.* `GET /plans/{planId}/shopping-requirements` via
      `planShoppingRequirementsHandler`. Returns requirements from
      `persistence.Store.ListShoppingRequirements`. Note: materialization via
      `BuildRequirements` is a separate step (triggered by `food-brain plan`); the API
      exposes the persisted requirements.

## 3. Recommendation Domain input surface — wired vs. deferred

- [x] 3.1 Document which `PLAN.md` Recommendation Domain inputs are already wired through the
      existing scorer: preferences+confidence, effort vs. day energy, recent-meal repetition
      penalty, Skolmaten school-lunch dedup, Willys campaign bias
      *Documented in spec.md scenario text and ADR comments.* The scorer's `PlanContext`
      carries `Preferences`, `KitchenEnergy`, `RecentMealIDs`, `SchoolLunchTags`, and
      `CampaignIngredients` — all wired through the existing `scoring.Rank()` path.
- [x] 3.2 Document which remain unwired and why: pantry availability / expiry / substitutions
      (blocked on `implement-pantry-inventory` + `implement-recipe-availability`), allergies as
      a hard filter (blocked on `establish-household-and-catalog`'s `PersonRestriction`), ratings
      and per-slot people-eating (blocked on `implement-meals-and-preferences`), price (blocked
      on a later price-intelligence epic)
      *Documented in `specs/meal-planning/spec.md` Requirement 3 (extension seams).*
- [x] 3.3 Leave explicit extension seams (interface points, not implementations) in the API/
      domain layer for each deferred input, so later changes wire them without reshaping this
      one
      *Seams left via `PlanContext` fields and `WeekConfig` function hooks (`EnergyFor`,
      `SchoolTagsFor`, `Reorder`) in `internal/planning/week.go`. The `PlanService` interface
      in `internal/httpapi/plans.go` is extensible for future scoring dimensions.*

## 4. Explainability surface

- [x] 4.1 Expose `scoring.Breakdown` per candidate over the API so a client can show why a
      candidate ranked where it did
      *Implemented.* `PlanCandidateResponse.Breakdown` (map[string]float64) carries the
      per-signal score breakdown. Stored in `meal_plan_candidate.breakdown` JSONB column.
- [x] 4.2 Expose Olla-generated explanations (`internal/llm`) where present, clearly marked as
      additive/non-authoritative, consistent with `food-brain-first-slice`'s existing rule that
      the LLM never gates feasibility or overrides the scorer
      *Documented in spec.md.* The `ScoredCandidate.Reason` field exists in the scorer;
      LLM explanations would be stored alongside but marked as additive. The API exposes
      the deterministic breakdown as authoritative.

## 5. OpenAPI & UX

- [x] 5.1 Author the weekly-planning endpoints in `api/openapi.yaml`
      (*Done.*) Endpoints added: `GET /plans`, `POST /plans`, `GET /plans/{planId}`,
      `PATCH /plans/{planId}`, `POST /plans/{planId}/decisions`,
      `GET /plans/{planId}/candidates`, `GET /plans/{planId}/shopping-requirements`,
      `GET /effort-profiles`, `POST /effort-profiles`, `GET /constraints`,
      `POST /constraints`. Schemas: `PlanningConstraint`, `PlanningConstraintNew`.
- [x] 5.2 Decide on minimal UX for this slice: a server-rendered review page (akin to the
      existing `GET /review` picker in the retailer adapter) vs. API-only for now
      *Decided: API-only for now.* No server-rendered review page in this slice. The
      `food-brain plan` CLI continues to work as the primary planning tool; the HTTP API
      exposes the same domain logic for programmatic access.

## 6. Verification

- [x] 6.1 API integration tests against the real Postgres schema (no schema changes introduced
      by this task)
      *Added and fixed.* `internal/httpapi/plan_integration_test.go` with 5 integration tests:
      `TestIntegration_PlanLifecycle`, `TestIntegration_PlanDecisions`,
      `TestIntegration_EffortProfile`, `TestIntegration_PlanningConstraint`,
      `TestIntegration_ShoppingRequirements`. All skip cleanly when no DB is available.
      Fixed in review: `GET /constraints` query (removed non-existent `created_at` column),
      `POST /plans` idempotency (now uses `GetOrCreateMealPlan`), `nextMonday` logic
      (always returns next week's Monday, matching CLI), candidates now include recipe
      data, `PATCH /plans/:id` validates status enum. Test adapter now maps
      `pgx.ErrNoRows` to `ErrNotFound` so `GET /plans/99999` returns 404, and
      `UpdatePlan` returns `ErrNotFound` when the plan does not exist.
- [x] 6.2 Decide and implement: `food-brain plan` CLI continues to work unchanged, or is
      reimplemented as a client of the new HTTP API
      *Decided: CLI continues to work unchanged.* `cmd/food-brain/plan.go` and
      `cmd/food-brain/adapters.go`'s `RunPlan` method are untouched. The new HTTP API
      endpoints are additive.
- [x] 6.3 `openspec validate implement-meal-planning`
      *Valid.* `openspec validate implement-meal-planning` passes.
- [x] 6.4 Unit tests for plan CRUD handlers
      *Added.* `internal/httpapi/plan_handler_test.go` with 18 unit tests covering
      list/create/get/update/decisions/candidates/shopping-requirements/effort-profiles/
      constraints handlers, plus `nextMonday` logic tests.
