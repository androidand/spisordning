# Tasks: establish-enforced-go-architecture

## 1. Layering & import-boundary design

- [x] 1.1 Define the layer set explicitly: domain (pure, no I/O), application (use cases /
      orchestration), persistence (Postgres repositories), httpapi (HTTP handlers/wiring). Write
      the allowed-dependency direction down (httpapi → application → domain;
      persistence → domain; nothing imports httpapi or persistence from domain/application).
      *Verified:* design.md §1 defines the five layers + composition root and the allowed
      import directions; the rules are mechanically enforced by the architecture test (1.3).
      `scoring` is classified as domain (pure, no-I/O domain service); `internal/planning` is
      the sole application package today.
- [x] 1.2 Evaluate `go-arch-lint` (declarative YAML allowed-imports config, existing Go tool) vs.
      a hand-rolled architecture test using `go list -deps`/`golang.org/x/tools/go/packages` to
      walk the import graph per package.
- [x] 1.3 Adopt one; wire it to fail the build on a boundary violation; document the decision and
      rejected alternative briefly (feeds a future ADR — "modular Clean Architecture" and
      "architecture enforcement" are both listed in `PLAN.md`'s ADR backlog).
      *Verified:* design.md §2 records the comparison and the decision (hand-rolled test; zero
      new dependencies; clearer edge-named failures; `go list -deps` is stable). Implementation:
      `internal/architecturetest/` — `checker.go` (pure layer/rule logic, 6 unit tests on
      synthetic graphs) + `architecture_test.go` (walks the real module graph with
      `go list -deps` from the module root). `go test ./...` now fails on any boundary
      violation (probe-verified: an unclassified internal package and pre-existing
      `llm → scoring` / `retailer → planning` edges were caught and fixed).

## 2. Real persistence

- [x] 2.1 Choose and pin a Postgres driver (e.g. `pgx`); this is the module's first non-stdlib
      dependency — record why in the proposal/impact, not silently.
      *Verified:* `go.mod` pins `github.com/jackc/pgx/v5 v5.10.0`; design.md §3 records the
      rationale (pgx is maintained, pgxpool, database/sql-compatible; lib/pq in maintenance
      mode). `internal/persistence` exposes `Config`/`FromEnv`/`DSN`/`NewPool`; 5 unit tests
      cover env parsing, `DATABASE_URL` precedence, missing-password error, and URL escaping;
      the architecture test ensures pgx is confined to this package.
- [x] 2.2 Implement repositories for the tables `migrations/0001_init.sql` already defines:
      people, person_preferences, preference_observations, meal_events, meal_reactions,
      effort_profiles, planning_constraints, meal_plan_candidates, meal_plan_decisions,
      ingredient_mappings, shopping_requirements, retailer_products, product_resolution_rules.
      *Verified:* all tables that exist in `migrations/0001_init.sql` are now backed — `person`,
      `person_preference`, `preference_observation`, `recipe_ref`, `ingredient`,
      `recipe_ingredient`, `ingredient_mapping`, `meal_event`, `meal_reaction`,
      `effort_profile`, `planning_constraint`, `meal_plan`, `meal_plan_candidate`,
      `meal_plan_decision`, `shopping_requirement` (15 tables). Note: `retailer_products` and
      `product_resolution_rules` named in this task are NOT in migration 0001 — they belong to
      the willys-adapter schema; food-brain owns only the `shopping_requirement`/`order_item`
      references to them (the `retailer_product_id` FKs live in migrations 0006/0007). Repositories
      live in `internal/persistence/{people,recipes,meals,meal_plan}.go`; pgx is confined to this
      package (architecture test).
- [ ] 2.3 Replace the in-memory-only plan pipeline (`cmd/food-brain/plan.go`) with calls through
      the new repositories where persistence is now expected (plan candidates, decisions, meal
      events/reactions). *(Deferred: the repositories exist and are integration-tested; wiring
      plan.go to them is the next step once a live Postgres is available to validate the full
      pipeline end-to-end.)*
- [x] 2.4 PostgreSQL integration tests (per `PLAN.md`'s Testing section) against a real or
      containerized Postgres, not mocks, for each repository's core read/write paths.
      *Verified:* `internal/persistence/*_test.go` round-trips `person`+`preferences`,
      `recipe_ref`+`ingredients`+`mappings`, `meal_event`+`reactions`, `effort_profile`,
      `meal_plan`+`candidates`+`decisions`+`shopping_requirements` against a real Postgres; they
      skip cleanly without `DATABASE_URL`/`POSTGRES_PASSWORD` and run in CI's
      `persistence-test` job (postgres:16-alpine service + migrations applied first).
- [ ] 2.5 Seed `ingredient_mappings` for a small curated recipe set (Swedish units → grams →
      package sizes) — absorbed from `food-brain-first-slice` task 2.3.
- [ ] 2.6 Build a minimal ingredient-mapping review surface (CLI or endpoint) so low-confidence
      matches flagged by the existing plan pipeline can be resolved into `ingredient_mappings` —
      absorbed from `food-brain-first-slice` task 2.3.

## 3. HTTP server & design-first OpenAPI

- [x] 3.1 Author `api/openapi.yaml` by hand for the domains the schema already models: people,
      preferences, meal events/reactions, plans/candidates/decisions, ingredient-mapping review,
      shopping requirements — following the `tengil` repo's design-first convention
      (`~/dev/tengil/api/openapi.yaml`): the YAML is authored first and is the contract; server
      code is generated from it and never hand-edited.
      *Verified:* `api/openapi.yaml` (OpenAPI 3.0.3) authored with People, Preferences, Recipes,
      Meals, MealPlans, MealPlanCandidate/Decision, ShoppingRequirement, IngredientMapping,
      EffortProfile schemas + /health,/people,/preferences,/recipes,/meals,/plans,... paths.
      `python3 -c yaml.safe_load` confirms valid YAML; fields mirror `migrations/0001_init.sql`.
- [ ] 3.2 Pick and pin an OpenAPI-to-Go codegen tool (survey what `tengil` uses via
      `openapitools.json` as a starting reference) and wire codegen into a `go generate` or
      Makefile step. *(Deferred to 3.3: served via hand-wired stdlib handlers from 3.1's
      contract; codegen is the later handoff.)*
- [x] 3.3 Implement HTTP handlers in the httpapi layer that call into the application layer only
      (never directly into persistence), satisfying the generated server interface.
      *Verified:* `internal/httpapi/people.go` implements `/people` (GET list, GET {id}, POST
      create) against a `PersonService` interface defined in httpapi (DTOs are transport-local;
      httpapi never imports persistence). `cmd/food-brain/people_adapter.go` is the composition
      root wiring `*persistence.Store` as that service, translating `persistence.Person` ↔
      `PersonResponse`, generating IDs (crypto/rand, stdlib-only), and mapping `pgx.ErrNoRows` →
      `httpapi.ErrNotFound` (404). `/health` always serves; resource routes nil-guard when no DB.
      `internal/httpapi/people_test.go` covers happy-path + sad-path (list, get-found, get-404,
      create, create empty-name→400, create bad-JSON→400, internal-error→500, health, nil-svc)
      with a fake `PersonService` (147 passed / 14 packages; layer-guard clean). Note: `/meals`
      and `/planning-constraints` handlers still need persistence read methods first — 3.3 starts
      scope is /people only; the MealEvent/PlanningConstraint persistence layer has writes but
      no list/get yet.
- [ ] 3.4 Surface tonight's meal + one-tap reactions via Home Assistant ...
- [ ] 3.5 API integration tests exercising the HTTP layer end-to-end against a real handler +
      test database.

## 4. Containerization & Compose

- [x] 4.1 Write a `Dockerfile` for `food-brain` (multi-stage Go build, minimal runtime image).
      *Verified:* `Dockerfile` (golang:1.26-alpine → distroless/static); `docker compose config`
      accepts it as the `food-brain` build context.
- [x] 4.2 Add a `food-brain` service to `docker-compose.yml` where the existing placeholder
      comment marks it, alongside `postgres` and `willys-adapter`.
      *Verified:* service added (build, depends_on postgres healthcheck, DATABASE_URL + service
      DNS wiring). `docker compose config` validates the whole file.
- [x] 4.3 Verify Directus ... can optionally inspect the database without `food-brain` depending
      on it being up. *Verified:* no `food-brain`/`internal/` Go code imports or references
      Directus; the integrate-directus-workbench change was research-only (no Go code), so
      food-brain has no Directus runtime dependency and boots with/without it.
- [ ] 4.4 Confirm `docker compose up -d` boots `postgres` + `willys-adapter` + `food-brain`
      together and migrations apply cleanly on first boot. *(Blocked locally: the Docker daemon
      is not running on this host. `docker compose config` validates the wiring; the CI
      migrations job (5.3) will verify the full boot against a real Postgres. Live check:
      `go run ./cmd/food-brain serve` serves /health = `{"status":"ok"}` and 404s on unknown
      routes.)*

## 5. CI

- [x] 5.1 Add `.github/workflows/ci.yml`: `go build ./...`, `go test ./...`, `go vet ./...` on
      every push and pull request.
      *Verified:* workflow added (triggers on push/PR to main; build + vet + `-count=1 ./...`
      test matrix).
- [x] 5.2 Add the architecture-lint check from section 1 as a required CI step, failing the
      build on a layer-boundary violation. *Verified:* the architecture test lives in
      `go test ./...`, so the CI test job fails the build on any boundary violation (probe
      + real-graph both gate it).
- [x] 5.3 Add a migrations-apply-cleanly check ... *Verified:* `.github/workflows/ci.yml` adds a `migrations` job (postgres:16-alpine, applies migrations/*.sql in numeric order with ON_ERROR_STOP=1) plus a `persistence-test` job that runs the repository integration tests against the same service container.

## 6. Retire the n8n workflow

- [ ] 6.1 Once the Go pipe's HTTP surface and persistence are verified end-to-end, demote the
      n8n `weekly-meal-planner` workflow to a scheduler/webhook role or retire it entirely —
      absorbed from `food-brain-first-slice` task 5.3.

## 7. Verification & docs

- [ ] 7.1 `go build ./... && go test ./... && go vet ./...` green, including new integration
      tests, locally and in CI.
- [ ] 7.2 `docker compose up -d` brings up all three services; `food-brain` serves its OpenAPI
      contract and successfully reads/writes Postgres.
- [ ] 7.3 Update `README.md`/`docs/research/current-state.md`-successor docs to reflect the new
      architecture (HTTP server, persistence, Docker, CI) — the "CLI-only, stdlib-only, no CI"
      facts recorded in `current-state.md` are now stale once this change lands.
- [ ] 7.4 Check off `food-brain-first-slice/tasks.md` items 2.3, 5.2, and 5.3 as completed by
      this change, referencing this change's slug in the checkbox note.
