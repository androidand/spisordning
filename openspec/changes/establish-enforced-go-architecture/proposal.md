# Establish enforced Go architecture

## Why

`food-brain` is currently a CLI-only, stdlib-only Go module (`docs/research/current-state.md`):
it has a real PostgreSQL schema (`migrations/0001_init.sql` — people, preferences, meal
events/reactions, effort profiles, planning constraints, plan candidates/decisions, ingredient
mappings, shopping requirements) that **nothing writes to yet**, no HTTP server, no Dockerfile,
and no CI. `docker-compose.yml` runs `postgres` and `willys-adapter` today; `food-brain` itself
has an explicit placeholder comment — "food-brain joins here once it grows an HTTP server" — it
cannot join until this change exists. There is also no mechanism, beyond code review discipline,
stopping domain code from importing persistence code or persistence code from importing HTTP
handlers.

This is also where three specific, already-tracked, still-open tasks from
`food-brain-first-slice/tasks.md` belong, so they are absorbed here rather than left stranded in
a change that is otherwise done:

- **2.3** — "Seed `ingredient_mappings` for a small curated recipe set (Swedish units → grams →
  package sizes) + a minimal review surface (CLI or endpoint)." Today the plan pipeline uses
  lowercased Mealie food names as canonical ids and only flags low-confidence matches; this
  change gives `ingredient_mappings` a real persisted home and a review surface, which requires
  the real persistence layer and (for an endpoint) the HTTP server this change adds.
- **5.2** — "Surface tonight's meal + one-tap reactions via Home Assistant (through homeops MCP
  / HA API)." Meal reactions are a write path into the schema this change wires up; the HA
  surface needs an API to call.
- **5.3** — "Demote the n8n `weekly-meal-planner` workflow to scheduler/webhook (or retire) once
  the Go pipe is verified." This is only safe to do once the Go pipe is a real service with an
  HTTP surface and persistence, i.e. after this change lands.

This change does not duplicate that work — it is the vehicle that finally makes 2.3, 5.2, and
5.3 possible, and closes them out.

Without this change, `PLAN.md`'s "Initial Definition of Done" cannot be met: it requires the Go
backend to boot, Postgres to boot, migrations to work, an OpenAPI contract to exist,
architecture to be enforced by CI, a Docker image to build, and Compose to work with Directus as
an optional, non-load-bearing observer. All of `PLAN.md`'s later vertical-slice changes
(`establish-household-and-catalog`, `implement-recipe-family-and-revisions`, etc.) assume this
foundation exists.

**Cautionary evidence from `establish-reference-lab`'s Grocy findings (2026-08-16):**
`docs/research/grocy-api-and-database.md` found that Grocy — software with years of real
production use across a large user base — has **zero declared foreign keys anywhere in its
schema** (`PRAGMA foreign_keys=0`, confirmed empty on every table checked) and **no automated
test suite at all** (no `tests/` directory, no CI workflow, no PHPUnit dependency). It works
despite this, but the same investigation separately reproduced a live data-integrity bug (a
unit-conversion trigger that silently inserts a wrong default and then collides with an
explicit value — see `establish-household-and-catalog` design.md invariant 11) of exactly the
kind FK constraints and test coverage exist to catch early rather than discover live. This
change's CI-enforced layering, real FKs throughout (`PLAN.md`'s own repeated guidance), and
test-suite requirements are not process for its own sake — they are the concrete difference
between finding that class of bug in a test run versus finding it, as this project did, by
hand while manually exercising a reference system.

## What Changes

- **Real persistence**: a Postgres repository layer (`internal/persistence` or similar) that
  reads and writes the tables `migrations/0001_init.sql` already defines, replacing the current
  in-memory-only plan pipeline. `go.mod` gains its first non-stdlib dependency (a Postgres
  driver, e.g. `pgx`), a deliberate break from "stdlib-only" that this proposal calls out
  explicitly rather than letting slide in silently.
- **HTTP server**: `food-brain` grows an HTTP server exposing the domains the schema already
  models (people, preferences, meal events/reactions, plans, shopping requirements,
  ingredient-mapping review).
- **Design-first OpenAPI**: an `api/openapi.yaml` hand-authored contract with generated
  server/client code, mirroring the `tengil` repo's own convention (`~/dev/tengil/api/openapi.yaml`
  plus `openapitools.json`-driven codegen) — the spec is the source of truth; generated code is
  never hand-edited.
- **Dockerfile**: a `food-brain` container image so it can join `docker-compose.yml` where the
  existing comment marks its place, alongside `postgres` and `willys-adapter`.
- **CI**: GitHub Actions workflows that did not exist before — build + `go test ./...` + `go vet`
  on every push/PR, plus an architecture-enforcement lint step (see below).
- **Mechanically enforced Clean Architecture boundaries**: domain / application / persistence
  (and HTTP as a fourth, outermost layer) are separated not just by convention but by a checked
  import graph — evaluate `go-arch-lint` (declarative allowed-imports config) against a
  hand-rolled Go test that walks `go list -deps` per package; adopt whichever gives clearer
  failure messages and lower maintenance, and wire the choice into CI.
- Absorbs `food-brain-first-slice` tasks 2.3, 5.2, and 5.3 (see Why).

## Capabilities

### New Capabilities

- `architecture-foundation`: the foundational, always-true invariants of the `food-brain`
  service as a piece of infrastructure — persistence is real, the API is versioned and
  contract-first, layer boundaries are mechanically enforced, and the service is containerized
  and CI-tested. This capability does not describe any food/recipe/meal domain behavior; those
  live in `meal-planning` and future domain capabilities.

### Modified Capabilities

<!-- none directly — meal-planning's persistence moves from "planned" to "real" as a side
     effect, but its requirements (food-brain-first-slice/specs/meal-planning/spec.md) do not
     change in substance -->

## Impact

- `go.mod`: first non-stdlib dependencies (Postgres driver, OpenAPI codegen tool, chosen
  architecture-lint tool).
- New: `internal/persistence` (or equivalent), `internal/httpapi` (or equivalent),
  `api/openapi.yaml`, `Dockerfile`, `.github/workflows/ci.yml`, an architecture-lint config
  (`archlint.yml` or a Go test under e.g. `internal/architecturetest`).
- `docker-compose.yml`: gains a `food-brain` service where the existing placeholder comment
  says it will (this proposal does not edit that file itself; the task list below tracks it as
  implementation work).
- `food-brain-first-slice/tasks.md`: items 2.3, 5.2, and 5.3 are superseded by this change's
  task list and should be checked off there, not duplicated.
- Unblocks every later vertical-slice change in `PLAN.md`'s likely sequence
  (`establish-household-and-catalog` onward), all of which assume a real HTTP+persistence
  service to extend.
