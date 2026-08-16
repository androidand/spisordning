# Tasks: establish-enforced-go-architecture

## 1. Layering & import-boundary design

- [ ] 1.1 Define the layer set explicitly: domain (pure, no I/O), application (use cases /
      orchestration), persistence (Postgres repositories), httpapi (HTTP handlers/wiring). Write
      the allowed-dependency direction down (httpapi → application → domain;
      persistence → domain; nothing imports httpapi or persistence from domain/application).
- [ ] 1.2 Evaluate `go-arch-lint` (declarative YAML allowed-imports config, existing Go tool) vs.
      a hand-rolled architecture test using `go list -deps`/`golang.org/x/tools/go/packages` to
      walk the import graph per package.
- [ ] 1.3 Adopt one; wire it to fail the build on a boundary violation; document the decision and
      rejected alternative briefly (feeds a future ADR — "modular Clean Architecture" and
      "architecture enforcement" are both listed in `PLAN.md`'s ADR backlog).

## 2. Real persistence

- [ ] 2.1 Choose and pin a Postgres driver (e.g. `pgx`); this is the module's first non-stdlib
      dependency — record why in the proposal/impact, not silently.
- [ ] 2.2 Implement repositories for the tables `migrations/0001_init.sql` already defines:
      people, person_preferences, preference_observations, meal_events, meal_reactions,
      effort_profiles, planning_constraints, meal_plan_candidates, meal_plan_decisions,
      ingredient_mappings, shopping_requirements, retailer_products, product_resolution_rules.
- [ ] 2.3 Replace the in-memory-only plan pipeline (`cmd/food-brain/plan.go`) with calls through
      the new repositories where persistence is now expected (plan candidates, decisions, meal
      events/reactions).
- [ ] 2.4 PostgreSQL integration tests (per `PLAN.md`'s Testing section) against a real or
      containerized Postgres, not mocks, for each repository's core read/write paths.
- [ ] 2.5 Seed `ingredient_mappings` for a small curated recipe set (Swedish units → grams →
      package sizes) — absorbed from `food-brain-first-slice` task 2.3.
- [ ] 2.6 Build a minimal ingredient-mapping review surface (CLI or endpoint) so low-confidence
      matches flagged by the existing plan pipeline can be resolved into `ingredient_mappings` —
      absorbed from `food-brain-first-slice` task 2.3.

## 3. HTTP server & design-first OpenAPI

- [ ] 3.1 Author `api/openapi.yaml` by hand for the domains the schema already models: people,
      preferences, meal events/reactions, plans/candidates/decisions, ingredient-mapping review,
      shopping requirements — following the `tengil` repo's design-first convention
      (`~/dev/tengil/api/openapi.yaml`): the YAML is authored first and is the contract; server
      code is generated from it and never hand-edited.
- [ ] 3.2 Pick and pin an OpenAPI-to-Go codegen tool (survey what `tengil` uses via
      `openapitools.json` as a starting reference) and wire codegen into a `go generate` or
      Makefile step.
- [ ] 3.3 Implement HTTP handlers in the httpapi layer that call into the application layer only
      (never directly into persistence), satisfying the generated server interface.
- [ ] 3.4 Surface tonight's meal + one-tap reactions via Home Assistant (through homeops MCP / HA
      API), now that a real HTTP surface and persisted meal_reactions exist — absorbed from
      `food-brain-first-slice` task 5.2.
- [ ] 3.5 API integration tests exercising the HTTP layer end-to-end against a real handler +
      test database.

## 4. Containerization & Compose

- [ ] 4.1 Write a `Dockerfile` for `food-brain` (multi-stage Go build, minimal runtime image).
- [ ] 4.2 Add a `food-brain` service to `docker-compose.yml` where the existing placeholder
      comment marks it, alongside `postgres` and `willys-adapter`.
- [ ] 4.3 Verify Directus (once introduced by the separate `integrate-directus-workbench`
      change) can optionally inspect the database without `food-brain` depending on it being
      up — not this change's job to add Directus, but this change's job to not block it.
- [ ] 4.4 Confirm `docker compose up -d` boots `postgres` + `willys-adapter` + `food-brain`
      together and migrations apply cleanly on first boot.

## 5. CI

- [ ] 5.1 Add `.github/workflows/ci.yml`: `go build ./...`, `go test ./...`, `go vet ./...` on
      every push and pull request (none of this exists today).
- [ ] 5.2 Add the architecture-lint check from section 1 as a required CI step, failing the
      build on a layer-boundary violation.
- [ ] 5.3 Add a migrations-apply-cleanly check (spin up Postgres in CI, apply
      `migrations/0001_init.sql`, fail on error) per `PLAN.md`'s "migration tests" testing
      category.

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
