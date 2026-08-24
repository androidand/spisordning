# Current state (as of 2026-08-19)

This is the Phase 0 "inspect existing Spisordning" deliverable `PLAN.md` asks for. It records
what actually exists today, distinct from what `PLAN.md` envisions, so new OpenSpec changes
build on reality rather than re-deriving or contradicting it.

## Layout

```
cmd/food-brain/        main.go (serve/demo/plan), demo.go, plan.go, plan_test.go,
                        people_adapter.go (composition root: wires persistence.Store into
                        httpapi's PersonService interface)
cmd/mcp-server/        main.go (stdio + stateless Streamable HTTP on :8081), adapters.go
                        (composition root: wires persistence.Store + planning into
                        mcptools' service interfaces); see
                        docs/adr/mcp-protocol-2026-07-28-and-go-sdk.md
internal/
  architecturetest/     layer-boundary guard (TestLayeredArchitecture via `go list -deps`)
  domain/               core types (Person, Preference, Candidate, Ingredient,
                        ShoppingRequirement, …)
  httpapi/              HTTP handlers + wiring (health.go, people.go, helpers.go, tests)
  httpclient/           shared JSON-over-HTTP transport used by every backend client
  llm/                  AI provider abstraction (Provider interface); Olla (Client) is the
                        primary OpenAI-compatible implementation
  mealie/client.go       read-only Mealie sync client (real, tested)
  mcptools/              MCP tool adapters (list_recipe_candidates, record_meal_reaction,
                        get_shopping_requirements); thin layer over the application layer
  persistence/          pgx-v5 Postgres repositories (people, recipes, meals, meal_plan)
                        + Config/FromEnv/New/NewStore; integration-tested
  planning/              requirements.go, staples.go, week.go (PlanWeek planner loop)
  retailer/client.go     Go-side client for the willys-adapter HTTP service
  scoring/scoring.go     deterministic candidate scorer
  skolmaten/client.go    school-lunch client
migrations/0001-0013      Postgres schema (0001 first-slice … 0013 price intelligence) —
                          applied by docker-compose; Go persistence wired for all (see below)
api/openapi.yaml          design-first OpenAPI 3.0.3 contract; server code generated from this
openspec/                 see below
```

`go.mod`: `github.com/androidand/spisordning`, Go 1.26.1, **stdlib-only except `pgx/v5`**
(`github.com/jackc/pgx/v5 v5.10.0`) and the MCP Go SDK
(`github.com/modelcontextprotocol/go-sdk v1.7.0`) — the latter added by
`implement-mcp-server`; the protocol/SDK/binary-placement rationale is recorded in
`docs/adr/mcp-protocol-2026-07-28-and-go-sdk.md`. The architecture test confines pgx to
`internal/persistence` and forbids `internal/mcptools` from importing clients, persistence,
httpapi, or cmd; `establish-enforced-go-architecture` further added `service`/`contract`
layers (`internal/service`, `internal/dto`) enforcing that services depend on `internal/dto`
rather than `internal/httpapi` importing `internal/service` directly. `go build ./... &&
go test ./...` passes (exact count drifts as changes land — check CI for the current number;
persistence integration tests skip locally without a Postgres and run in CI's
`persistence-test` job). CI also runs architecture-enforcement, migrations-apply,
docker-build, and codegen-verify jobs.

No `AGENTS.md`/`CLAUDE.md` existed before this change. No `docs/` existed before this change.

## `PLAN.md` vs. `README.md`

`PLAN.md` was untracked in git until this change (never committed). `README.md` is the
original, tracked description (2026-07-18/19) and is **narrower and inconsistent** with
`PLAN.md`: README describes a thin "Food Brain" service wrapping Mealie/Grocy/Willys/HA/Olla/
Skolmaten as-is; `PLAN.md` describes a much larger from-scratch relational rebuild
(RecipeFamily/Variant/Revision, Ingredient vs Product vs RetailerProduct, an inventory ledger,
etc.). `PLAN.md` is now the mission document going forward (see `AGENTS.md`); README should be
read as historical context for the first slice, not the current target architecture.

## OpenSpec state (before this change)

`openspec/specs/` contained only one merged capability, `retailer-adapter`, and only the
`pinned-product-resolution` requirements within it (from the one archived change). Five other
changes existed but were **not reflected in `openspec/specs/`**:

| Change | Status | What it actually does |
|---|---|---|
| `food-brain-first-slice` | In progress (2 tasks open: ingredient-mapping review UI, HA surfacing) | The whole first vertical slice — Go service, Postgres schema, Mealie sync, scorer, shopping requirements, willys-adapter wiring. |
| `archive/2026-07-19-pinned-product-resolution` | Archived, synced | Household "pin store" consulted before fuzzy search. |
| `review-and-pick` | Done, unarchived | Review queue + `GET /review` picker page for needs-review terms; promo-variant expansion (`GET /promotions/:code/products`, "Visa fler sorter"). |
| `size-aware-matching` | Done, unarchived, live-verified | Splits a shopping term into name + size hint; matches on name only, prefers size-matching candidates. Fixed the "1.5L resolves to a 50cl can" bug. |
| `name-vs-quantity-confidence` | Done, unarchived, live-verified | Split `confidence` (name match) from a new `quantityUncertain` flag. Fixed a real list resolving 0/9 → 7/9. |
| `notes-sync-via-adapter` | Done, unarchived, live-verified | Apple Notes bridge (`bridge.ts` + `notes.ts`) reads a mapped note, posts to the adapter's `/resolve` and `/shopping-lists`, replacing a standalone sync stack. Supports multiple note↔wishlist mappings. |

These four unarchived-but-done changes were archived and synced into `openspec/specs/` as part
of this restructuring (see `openspec/changes/archive/` for their new archive entries).

## The retailer adapter lives in a sibling repo

`willys-adapter` (the HTTP service spisordning's `internal/retailer` talks to) is **not** in
this repo — its code is in `~/dev/willys/willys-client/apps/willys-adapter`, alongside the
TypeScript `willys-client` it wraps. `docker-compose.yml` builds it from that sibling repo's
`Dockerfile.adapter`. That repo has its **own** `openspec/` with three changes
(`add-adapter-cache-layer`, `ensure-active-store`, archived `2026-07-17-migrate-api-v1`) — check
it before assuming spisordning's OpenSpec state is the complete picture for anything
retailer-related.

A second, unrelated sibling repo, `~/dev/willys/willys-mcp`, exists (older Next.js + MCP-server
exploration with its own puppeteer auth and SQLite caching). It is not wired into spisordning
and should not be confused with the adapter that is.

See `docs/research/willys-capabilities.md` for the full capability map.

## Mealie / Grocy / Directus

- **Mealie**: real, tested client (`internal/mealie/client.go`), talks to an already-deployed
  instance (`MEALIE_BASE_URL` defaults to `http://hlab-mealie:9000`). Includes tag
  normalization, Swedish effort-time parsing, and a fallback to Mealie's own ingredient parser.
- **Grocy**: no code, config, or schema anywhere in this repo or its siblings. Pure research
  target.
- **Directus**: no code or config anywhere. Pure research target (the "Directus Research
  Spike" in `PLAN.md` has not been performed).

## Database

`migrations/0001_init.sql` defines the full first-slice schema (person, preferences,
recipe_ref, ingredient, ingredient_mapping, meal_event/reaction, effort_profile,
planning_constraint, meal_plan/candidate/decision, shopping_requirement). Later migrations
extend it:

- `0002_recipe_discovery.sql` — `external_recipe_source`, `recipe_import_candidate`,
  `recipe_import_candidate_ingredient`.
- `0003_recipe_family.sql` — `recipe_family`, `recipe_variant`, `recipe_revision`,
  `recipe_revision_parent`.
- `0004_shopping_list.sql` — `shopping_list` + `shopping_list_item` (household shopping list,
  seeded from `meal_plan`'s `shopping_requirement`s; no retailer product id per D1; a
  `CHECK` enforces at least one of requirement/ingredient/label per item).
- `0005_retailer_list_binding.sql` — `retailer_list_binding` (outbound-only binding of a
  `shopping_list` to a retailer wishlist; push is additive, not idempotent).
- `0006_shopping_cart.sql` — `shopping_cart` + `shopping_cart_item` (a checkpoint record of a
  to-cart call, not a mirror of live retailer cart state).
- `0007_order.sql` — `order` + `order_item` (actual purchase record; `source` is an explicit
  enum `'manual'|'retailer_api'|'receipt_import'`; `order_item.substituted_for_item_id` is a
  self-reference; forward extension point for a future `inventory_event(kind='PURCHASE')`).
- `0008_household_catalog_minimal.sql` — `household`, `product`, `product_identifier`,
  `product_ingredient_mapping` (minimal slice for pantry and price; full catalog in 0011).
- `0009_pantry_inventory.sql` — `inventory_location`, `inventory_lot`, `inventory_event`
  (pantry inventory ledger, implement-pantry-inventory).
- `0010_migrate_persons_to_household.sql` — backfills `household` + `household_membership`
  for existing flat-person data.
- `0011_household_and_catalog.sql` — full catalog: `account`, `person_restriction`,
  `ingredient_form`, `ingredient_substitution`, `unit`/`unit_conversion`/`ingredient_unit_conversion`.
- `0012_meal_history.sql` — `meal_participant`, `meal_review`, `favorite`, `meal_event`→`meal_plan`
  link (implement-meals-and-preferences).
- `0013_price_intelligence.sql` — `retailer`, `store`, `retailer_product`,
  `store_product_offer`, `price_observation`, `current_store_product_price` view
  (implement-price-intelligence).

All migrations are applied by docker-compose's Postgres. Go persistence is wired for all
new tables (pantry, meal-history, price). The shopping/order tables (0006–0007) remain
unwritten in Go — tracked in `implement-shopping-and-commerce`.

## CI / Docker

CI exists: `.github/workflows/ci.yml` runs `go build`/`go test`/`go vet` on every
push/PR, plus `migrations` (apply `migrations/*.sql` against postgres:16) and
`persistence-test` (repository integration tests) jobs. No Makefile yet (CI uses
`go` directly). `food-brain` now ships a multi-stage `Dockerfile` and joins
`docker-compose.yml` as a `food-brain` service; `docker compose up -d` brings up
`postgres`, `willys-adapter`, and `food-brain` (`food-brain serve` on `:8080`,
exposing the OpenAPI contract, reading/writing Postgres). `/health` always serves
even without a database, and `/people` provides the first persistence-backed
endpoints. `docker-compose.yml` today: `postgres` (stock image) + `willys-adapter`
(built from the sibling repo) + `food-brain` (built from this repo).

## MCP Server

An MCP (Model Context Protocol) server is implemented as a separate `cmd/mcp-server`
binary alongside `cmd/food-brain` (`implement-mcp-server`, completed 2026-08-22).

- **Protocol**: MCP 2026-07-28, stateless — no `initialize` handshake, no
  `Mcp-Session-Id` header. Streamable HTTP (POST /mcp) and stdio transports.
- **SDK**: `github.com/modelcontextprotocol/go-sdk` v1.7.0.
- **Tools**: `list_recipes`, `record_reaction`, `get_tonight_meal`, `list_people`.
  All call `internal/httpapi` service interfaces; never persistence or SQL directly
  (enforced by the same architecture test used by `establish-enforced-go-architecture`).
- **Binary**: `cmd/mcp-server/main.go` — wired via `storeAdapter` (same pattern as
  `cmd/food-brain/adapters.go`). Dockerfile at `Dockerfile.mcp`; compose service in
  `docker-compose.yml` on port 8401.
- **ADR**: `docs/adr/mcp-protocol-2026-07-28-and-go-sdk.md`.

The MCP server is the infrastructure that makes "AI SHALL call application-layer tools.
Never expose unrestricted SQL" structurally true — AI providers (Epic G) will consume
this surface, not talk to Postgres directly.

## What this means for new OpenSpec changes

- Don't re-propose the retailer resolution pipeline (pinning, review-and-pick, size-aware
  matching, confidence splitting) — it exists and is live. New retailer-adjacent work
  (`implement-shopping-and-commerce`, `research-and-integrate-ica`,
  `implement-price-intelligence`) should build on top of it, not around it.
- `establish-enforced-go-architecture` is the change responsible for wiring the existing schema
  to real persistence, adding an HTTP server, a Dockerfile, and CI/architecture enforcement —
  it absorbs `food-brain-first-slice`'s remaining open tasks.
- Deploying `food-brain` itself via Tengil is gated on that change producing a real container
  image; it is not part of the initial reference-lab deploy (see Epic H).
