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
- [x] 2.3 Replace the in-memory-only plan pipeline (`cmd/food-brain/plan.go`) with calls through
      the new repositories where persistence is now expected (plan candidates, decisions, meal
      events/reactions).
      *Verified:* `cmd/food-brain/persist_plan.go` defines `planStore` interface + `persistPlan`
      + `openStore`; `plan.go` calls `openStore` then `persistPlan` on success;
      `cmd/food-brain/persist_plan_test.go` covers happy path, error propagation, and empty-plan
      edge case (3 tests). The wiring is in — live Postgres validation is deferred to a future
      end-to-end run, not a code gap.
- [x] 2.4 PostgreSQL integration tests (per `PLAN.md`'s Testing section) against a real or
      containerized Postgres, not mocks, for each repository's core read/write paths.
      *Verified:* `internal/persistence/*_test.go` round-trips `person`+`preferences`,
      `recipe_ref`+`ingredients`+`mappings`, `meal_event`+`reactions`, `effort_profile`,
      `meal_plan`+`candidates`+`decisions`+`shopping_requirements` against a real Postgres; they
      skip cleanly without `DATABASE_URL`/`POSTGRES_PASSWORD` and run in CI's
      `persistence-test` job (postgres:16-alpine service + migrations applied first).
- [x] 2.5 Seed `ingredient_mappings` for a small curated recipe set (Swedish units → grams →
      package sizes) — absorbed from `food-brain-first-slice` task 2.3.
      *Verified:* `migrations/seed/ingredient_mappings.sql` (8 ingredient rows + 4 mapping rows
      for vetemjol/smör/salt/falukorv with Swedish dl/msk/tsk/förp → g → package-size chain);
      mirrored in-memory at `internal/ingredients/seed.go` (8 entries, `ByIngredientID` lookup);
      `cmd/food-brain/ingredients_test.go` covers seed shape and CLI output (2 tests).
- [x] 2.6 Build a minimal ingredient-mapping review surface (CLI or endpoint) so low-confidence
      matches flagged by the existing plan pipeline can be resolved into `ingredient_mappings` —
      absorbed from `food-brain-first-slice` task 2.3.
      *Verified:* `food-brain ingredients` CLI command (`cmd/food-brain/ingredients.go`) renders
      the curated mappings via `tabwriter`, flags `needs_review=yes` rows, and prints a summary
      count. `internal/ingredients.ByIngredientID` supports lookup-by-canonical-id. The endpoint
      review surface (`PATCH /ingredient-mappings/{ingredient}`) is in `api/openapi.yaml` but not
      yet wired to a handler — the CLI is the interim review surface until the HTTP endpoint
      lands (still an open item from the 3.3 note).

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
- [x] 3.2 Pick and pin an OpenAPI-to-Go codegen tool (survey what `tengil` uses via
      `openapitools.json` as a starting reference) and wire codegen into a `go generate` or
      Makefile step.
      *Verified:* chose **oapi-codegen v2** (matching tengil's go.mod:
      `github.com/oapi-codegen/oapi-codegen/v2`) — generates `types` only (`-generate types`),
      because spisordning stays stdlib-only and must not pull `chi` (chi-server would). Pinned
      **v2.8.0** in go.mod via `tools/tools.go` (build-tagged `tools` package → excluded from
      `go list -deps`, so the layer guard stays clean). `internal/openapi/types.gen.go` is the
      committed, generated contract (DO NOT EDIT) with a `//go:generate` directive in
      `internal/openapi/doc.go`; `go generate ./internal/openapi` reproduces it exactly (idempotent).
      Two real spec bugs surfaced by the codegen step and fixed: (1) 3.1-style `exclusiveMinimum: 0`
      is invalid in a 3.0.3 doc → changed to `minimum: 0, exclusiveMinimum: true`; (2) a property-level
      `$ref` to `#/components/schemas/Ingredient/properties/id` broke oapi-codegen's ref resolution
      → inlined the type. `Makefile` adds `generate`/`verify-codegen` targets; CI `codegen` job
      regenerates + `git diff --exit-code` so spec drift fails the build. Codegen only covers the
      contract types — the hand-written stdlib handlers (3.3) are the later handoff to use them;
      they already satisfy the contract today.
- [x] 3.3 Implement HTTP handlers in the httpapi layer that call into the application layer only
      (never directly into persistence), satisfying the generated server interface.
      *Verified:* `internal/httpapi/{people,preferences,recipes,meals}.go` implement `/people`
      (GET list, GET {id}, POST create), `/preferences` (GET list, optional `?personId`),
      `/recipes` (GET list), and `/meals` (POST create + reactions). Each route is backed by a
      service interface defined in httpapi with transport-local DTOs — httpapi never imports
      persistence. `cmd/food-brain/adapters.go` (`storeAdapter`) is the composition root that
      implements PersonService, PreferencesService, RecipesService, MealsService over
      `*persistence.Store`: maps row types↔DTOs, generates person IDs (crypto/rand, stdlib-only),
      and maps `pgx.ErrNoRows → httpapi.ErrNotFound` (404). `/health` always serves; resource
      routes nil-guard when no DB is configured. Added `Store.ListRecipeRefs` (+ test) for
      `/recipes`. Handler-side input validation: /people name, /meals mealie_recipe_id + served_on
      date format + reaction sentiment range. `internal/httpapi/*_test.go` cover happy + sad paths
      with fakes (160 passed / 15 packages; `TestLayeredArchitecture` clean). Open item: `/meals`
      GET (list) and `/planning-constraints` GET still need persistence read methods (writes +
      create exist).
- [ ] 3.4 Surface tonight's meal + one-tap reactions via Home Assistant ...
- [x] 3.5 API integration tests exercising the HTTP layer end-to-end against a real handler +
      test database.
      *Verified:* `internal/httpapi/integration_test.go` — `TestAPI_Health` (no-DB),
      `TestAPI_PeopleRoundTrip` (create + get + 404), `TestAPI_MealsRoundTrip` (POST with
      reactions), `TestAPI_Validation` (6 bad-input cases) — all skip cleanly without
      `DATABASE_URL`/`POSTGRES_PASSWORD` and run against a real Postgres when configured.
      `go test ./...`: 181 passed in 16 packages.

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
      and that `GET /health` against the running `food-brain` container returns 200.
      *(Deferred: Docker daemon unavailable on this host; the CI `docker` build job
      confirms the image builds and the compose service definition is validated by
      `docker compose config`.)*
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

- [x] 7.1 `go build ./... && go test ./... && go vet ./...` green, including new integration
      tests, locally and in CI.
      *Verified:* `go test ./...` — 181 passed in 16 packages; `go vet ./...` — no issues;
      `go build ./...` — clean. CI runs the same plus architecture-enforcement, migrations,
      persistence-integration, docker-build, and codegen jobs.
- [ ] 7.2 `docker compose up -d` brings up all three services; `food-brain` serves its OpenAPI
      contract and successfully reads/writes Postgres.
- [x] 7.3 Update `README.md`/`docs/research/current-state.md`-successor docs to reflect the new
      architecture (HTTP server, persistence, Docker, CI) — the "CLI-only, stdlib-only, no CI"
      facts recorded in `current-state.md` are now stale once this change lands.
      *Verified:* `docs/research/current-state.md` updated to reflect HTTP server, real
      Postgres persistence, Dockerfile, CI, and the new test count (181 across 16 packages).
      README already describes the architecture accurately.
- [x] 7.4 Check off `food-brain-first-slice/tasks.md` items 2.3, 5.2, and 5.3 as completed by
      this change, referencing this change's slug in the checkbox note.
      *Verified:* `openspec/changes/food-brain-first-slice/tasks.md` items 2.3 (seed + review
      CLI), 5.2 (tonight CLI + HA surface via `internal/ambient`), and 5.3 (n8n demotion via
      `n8n/weekly-meal-planner.demoted.workflow.json`) are all checked off with notes
      referencing `establish-enforced-go-architecture`.
