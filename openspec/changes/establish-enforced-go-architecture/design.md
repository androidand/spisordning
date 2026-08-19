# Design: establish-enforced-go-architecture

## 1. Layer set (task 1.1)

Four internal layers plus the composition root, mirroring `PLAN.md`'s Clean-Architecture
leanings and the existing package layout:

| Layer | Path prefix | Responsibility | May import (from `internal/`) |
|---|---|---|---|
| domain | `internal/domain`, `internal/recipefamily`, `internal/scoring` | Pure types + invariants, no I/O (scoring is a pure deterministic domain service) | domain only |
| application | `internal/planning` | Use-case orchestration over domain | domain |
| infrastructure clients | `internal/mealie`, `internal/skolmaten`, `internal/retailer`, `internal/llm`, `internal/httpclient`, `internal/recipeimport` | External-system ports (HTTP clients, parsers) | domain only |
| persistence | `internal/persistence` | Postgres repositories over `migrations/` | domain |
| httpapi | `internal/httpapi` | HTTP handlers + wiring | application, domain (never persistence) |
| composition root | `cmd/food-brain` | `main`, dependency wiring, CLI | everything |

Allowed dependency direction, written down: `httpapi → application → domain` and
`persistence → domain`; nothing imports `httpapi` or `persistence` from `domain`/`application`;
no `internal/` package imports `cmd/`.

These rules are **enforced mechanically**, not by review discipline (task 1.3), and the
enforcement test is part of `go test ./...`, so CI gets it for free.

## 2. Tool evaluation (task 1.2)

Compared:

- **go-arch-lint** — declarative YAML allowed-imports config; a real Go tool; adds a build-time
  dependency and its own rule DSL to learn; failure messages are decent but generic.
- **Hand-rolled architecture test** — a Go test under `internal/architecturetest` that runs
  `go list -deps -f '{{.ImportPath}} {{join .Imports " "}}'` once per `go test` invocation and
  applies the layer rules above to the resulting import graph.

**Decision: hand-rolled test.** Reasons:

1. Zero new dependencies — the module is deliberately stdlib-only today; this change's only
   justified dependency addition is the Postgres driver (2.1).
2. The rules are ~30 lines of Go, readable in the same repo as the code they police; a YAML
   config for a tool nobody else on the team runs is one more artifact to keep in sync.
3. Failure output names the exact violating `pkg → import` edge with the rule it broke —
   clearer than go-arch-lint's generic output for a graph this small.
4. `go list -deps` is a stable, documented interface; no external binary to install in CI.

Rejected alternative recorded here for the future ADR backlog (`PLAN.md` lists "architecture
enforcement"): if the rules ever grow beyond a dozen edges, re-evaluate go-arch-lint (or
`golangci-lint`'s `importas`/`depguard` passes) rather than extending the hand-rolled checker.

## 3. Postgres driver (task 2.1)

**Decision: `github.com/jackc/pgx/v5`** — the module's first non-stdlib dependency, chosen
deliberately:

- `pgx` is the de-facto standard Postgres driver for Go, actively maintained, with a native
  connection pool (`pgxpool`) and `database/sql` compatibility.
- The alternative, `lib/pq`, is in maintenance mode; `database/sql` alone would still need a
  driver.
- The dependency is confined to `internal/persistence`; no other package imports it (enforced
  by the architecture test's rule that `domain`/`application`/clients never import
  `persistence`, and persistence imports only domain + external).

Connection config comes from environment (`DATABASE_URL` or the individual
`POSTGRES_HOST/PORT/DB/USER/PASSWORD` vars used by docker-compose), parsed in
`internal/persistence` so callers and tests never handle connection strings by hand.

## 4. OpenAPI → Go codegen (task 3.2)

**Decision: `github.com/oapi-codegen/oapi-codegen/v2` (pinned `v2.8.0`), `-generate types` only.**

- Tengil (this org's convention) uses oapi-codegen; `openapitools.json` pins it
  (`generator-cli.version: 7.17.0` is the OpenAPI *Generator* CLI, a different tool —
  tengil's Go codegen is via oapi-codegen, per its Makefile `generate-tengil-go.sh`).
- Spisordning stays stdlib-only (`net/http`, no `chi`/`echo`/`gin`), so we generate
  **types only** — never the chi/echo server stubs that would pull a router dependency.
- The `oapi-codegen/v2/cmd/oapi-codegen` command is recorded in `tools/tools.go`
  (build-tagged `//go:build tools`), so `go mod tidy` pins it without it entering the
  normal build graph or `go list -deps` (the layer guard never sees it).
- Generated types live in `internal/openapi/` (`types.gen.go`, DO NOT EDIT), classified
  as `Domain` (pure data, no I/O) in the layer checker. The `//go:generate` directive in
  `internal/openapi/doc.go` and the `Makefile generate` target both reproduce it.
- `api/openapi.yaml` is the single authored contract; CI `codegen` job regenerates and
  `git diff --exit-code` so drift fails the build. Codegen surfaced two real spec bugs
  (3.1 `exclusiveMinimum` in a 3.0 doc; a property-level `$ref`) — both fixed.
- The hand-written stdlib handlers (`internal/httpapi`) satisfy this contract today;
  migrating them to consume the generated types is the follow-on handoff (still task 3.3).