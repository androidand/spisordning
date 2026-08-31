# Current state (as of 2026-08-28)

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
  config/               env-only config (Config, Load, Validate, MissingVars, Has* predicates);
                        single source for every env var — established by
                        establish-config-di-and-presentation-layer
  domain/               core types (Person, Preference, Candidate, Ingredient,
                        ShoppingRequirement, …)
  dto/                  application-layer data-transfer types (services depend on dto, not
                        httpapi — enforced by the architecture test)
   httpapi/              HTTP handlers + wiring (health.go, people.go, shopping.go,
                        helpers.go, progress.go, tests) — shopping.go adds the shopping-list
                        endpoints incl. POST /shopping-lists/from-checklist (Apple Notes
                        ingestion); progress.go adds POST /plans/run/stream (SSE)
  httpclient/           shared JSON-over-HTTP transport used by every backend client
  llm/                  AI provider abstraction (Provider interface); Olla (Client) is the
                        primary OpenAI-compatible implementation
  mealie/client.go       read-only Mealie sync client (real, tested)
  grocy/client.go        Grocy inventory client (real, tested); degrades to
                        ErrGrocyNotConfigured → 503 when GROCY_BASE_URL is unset
   mcptools/              MCP tool adapters (list_recipe_candidates, record_meal_reaction,
                        get_shopping_requirements, create_shopping_list,
                        compare_shopping_prices, push_shopping_wishlist); thin layer over the
                        application layer — the three shopping tools stop at the retailer wishlist
                        (no cart/checkout/payment)
  persistence/          pgx-v5 Postgres repositories (people, recipes, meals, meal_plan)
                        + Config/FromEnv/New/NewStore; integration-tested
   planning/              requirements.go, staples.go, week.go (PlanWeek planner loop),
                          snacks.go (snack/breakfast tag filters, fallback snacks, PlanSimpleSlot,
                          PlanWeekAllSlots)
   retailer/client.go     Go-side client for the willys/ica/hemkop-adapter HTTP services;
                           Resolution carries PriceValue/Price (price-in-resolution); Client
                           stores kind + authFile (WithAuthFile/AuthFile) for elevated creds
   retailer/auth.go       AuthTier (AuthBasic/AuthElevated) + Operation constants + TierFor;
                           ErrElevatedStale / IsStaleCredential / IsElevatedStale for 401/403
   retailer/compare.go    cross-retailer price comparison (Compare resolves a set of
                           requirements against Willys + ICA + Hemköp, reports per-item cheapest +
                           availability; a stale retailer degrades to available:false)
  scoring/scoring.go     deterministic candidate scorer
  service/              application-layer services (grocy.go, …) — depend on internal/dto
  skolmaten/client.go    school-lunch client
 migrations/0001-0020      Postgres schema (0001 first-slice … 0020 recipe_source_ref) —
                             applied by docker-compose; Go persistence wired for all (see below)
api/openapi.yaml          design-first OpenAPI 3.0.3 contract; server code generated from this
web/                      Vite + React 19 + TypeScript 7.0.2 multi-page frontend (hand-written
                            typed client in web/src/generated/spisordning.ts); see below
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

`AGENTS.md` (agent instructions) and `docs/` (research + ADRs + infrastructure) now exist and are
tracked.

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
this repo — its code is in `~/dev/store-clients/willys-client/apps/willys-adapter`, alongside the
TypeScript `willys-client` it wraps. `docker-compose.yml` builds it from that sibling repo's
`Dockerfile.adapter`. That repo has its **own** `openspec/` with changes
(`add-adapter-cache-layer`, `ensure-active-store`, archived `2026-07-17-migrate-api-v1`) — check
it before assuming spisordning's OpenSpec state is the complete picture for anything
retailer-related.

The sibling now lives inside a larger `~/dev/store-clients/` workspace (a `go.work` + shared
`openspec/` holding many retailer clients: `willys-client`, `ica-client`, `hemkop-client`,
`dabas-client`, `axfood-client`, etc.). A second, unrelated repo, `~/dev/store-clients/willys-mcp`,
exists there too (older Next.js + MCP-server exploration with its own puppeteer auth and SQLite
caching). It is not wired into spisordning and should not be confused with the adapter that is.

`expose-shopping-price-and-notes-bridge` also added a cross-retailer Apple Notes path: a stub at
`~/dev/store-clients/willys-client/apps/notes-sync/spisordning-bridge.ts`
(`npm run notes:spisordning[:apply]`) reuses the existing `notes.ts` osascript reader + `core.ts`
checklist parser and POSTs to spisordning's `POST /shopping-lists/from-checklist` (default
`http://localhost:8080`). It is dry-run by default and deployment-gated — activate it (point
`SPISORDNING_URL` at the real host, pass `--apply`) once `deploy-food-brain-to-proxmox` lands. The
existing Willys-only `bridge.ts` flow is untouched.

See `docs/research/willys-capabilities.md` for the full capability map.

## Mealie / Grocy / Directus

- **Mealie**: real, tested client (`internal/mealie/client.go`), talks to an already-deployed
  instance (`MEALIE_BASE_URL` defaults to `http://hlab-mealie:9000`). Includes tag
  normalization, Swedish effort-time parsing, and a fallback to Mealie's own ingredient parser.
  **Recipe source of truth** is controlled by the `RECIPE_SOURCE` env var (`native`/`dual`/`mealie`,
  default `native`). The `recipe_source_ref` table (migration 000020) maps native `recipe_family`
  rows to Mealie slugs. `food-brain sync import` runs the one-way Mealie → recipe_family import.
  `StructureFromText` writes natively when `RECIPE_SOURCE=native` or `dual`.
- **Grocy**: real, tested client (`internal/grocy/client.go`) + service (`internal/service/grocy.go`)
  + handler (`internal/httpapi/grocy.go`); reads `GROCY_BASE_URL` / `GROCY_USER` / `GROCY_PASSWORD`
  via `internal/config`. A nil client degrades to `ErrGrocyNotConfigured` → 503. Frontend
  `GrocyPage` consumes the endpoints.
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
- `0014_meal_plan_slots.sql` — adds `slot_kind` column (`dinner`|`breakfast`|`snack`,
  default `dinner`) to `meal_plan_candidate` and `meal_plan_decision`, and
  `meal_plan_slot_kind` to `meal_event` (complete-live-meal-planning).

All migrations are applied by docker-compose's Postgres. Go persistence is wired for all
new tables (pantry, meal-history, price, meal-plan slots). The shopping/order tables
(0006–0007) remain unwritten in Go — tracked in `implement-shopping-and-commerce`.

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
- **Tools**: `list_recipe_candidates` (full-day planning: dinner + breakfast + snack slots),
  `record_meal_reaction` (with optional `slot` field, defaults to dinner),
  `get_shopping_requirements`, plus the shopping trio added by
  `expose-shopping-price-and-notes-bridge`: `create_shopping_list` (persist a list from
  requirements), `compare_shopping_prices` (cross-retailer price comparison via
  `internal/retailer.Compare`), and `push_shopping_wishlist` (push chosen resolutions to a
  retailer's wishlist — the terminal safe step; no cart/checkout/payment). All call
  `internal/mcptools` service interfaces; never persistence or SQL directly (enforced by the same
  architecture test used by `establish-enforced-go-architecture`).
- **Binary**: `cmd/mcp-server/main.go` — wired via `storeAdapter` (same pattern as
  `cmd/food-brain/adapters.go`). Built by the main `Dockerfile` (which ships both
  `food-brain` and `mcp-server`); the compose `mcp-server` service runs it via an
  `entrypoint` override on port 8081.
- **ADR**: `docs/adr/mcp-protocol-2026-07-28-and-go-sdk.md`.

The MCP server is the infrastructure that makes "AI SHALL call application-layer tools.
Never expose unrestricted SQL" structurally true — AI providers (Epic G) will consume
this surface, not talk to Postgres directly.

## Presentation layer (config, auth tiers, SSE, web frontend)

`establish-config-di-and-presentation-layer` added the env-only config package, the ICA
auth-tier concept, an SSE progress endpoint, and the first `web/` frontend slice.

- **`internal/config`** — single source for every env var. `Config` + `Load()` read the
  environment once; `Validate` / `MissingVars` / `FormatMissing` report unset required vars;
  `Has*` predicates gate optional integrations. `cmd/food-brain` and `cmd/mcp-server` both call
  `config.Load()` in their composition roots instead of ad-hoc `os.Getenv` reads. `ICA_AUTH_FILE`
  → `Config.ICAAuthFile` (see auth tiers below).
- **Auth tiers (`internal/retailer/auth.go`)** — `AuthTier` (`AuthBasic` / `AuthElevated`) and
  `Operation` constants (`OpResolve`, `OpSearch`, `OpCreateList`, `OpSyncList`, `OpToCart`,
  `OpBarcode`, `OpBonus`, `OpOffers`). `Client.TierFor(op)` declares which tier each operation
  needs: ICA's anonymous ecom surface is basic; account-bound writes are elevated. Willys and
  Hemköp are single-tier. `ErrElevatedStale` / `IsStaleCredential(code)` (keyed to 401/403) /
  `IsElevatedStale(err)` detect a stale elevated credential. The elevated credential (ICA mobile
  OAuth2/PKCE Bearer session) is a human-run Playwright login on the TS `ica-adapter` side; the
  file path is injected via `Client.WithAuthFile` (never read from env by the client). See
  `docs/infrastructure/ica-elevated-login.md`.
- **SSE progress (`POST /plans/run/stream`)** — `internal/httpapi/progress.go` streams
  `text/event-stream` progress events as a plan run progresses (`started`, `planning`,
  `resolving`, `wishlist`, `done`). `RunPlanInput.Progress` in `cmd/food-brain/plan.go` emits the
  phase events. Per-item events are not possible today: `ResolveRequirements` is a single
  blocking adapter HTTP call, so the Go side only observes phase boundaries. The synchronous
  `POST /plans/run` is unaffected — SSE is additive. The event payload (`PlanProgress{phase,
  message, at}`) is intentionally minimal and not finalized until the frontend's SSE consumer
  needs it (task 3.4).
- **`web/` frontend** — Vite + React 19.2.8 + TypeScript 7.0.2 (the native/Go compiler) +
  TanStack Query + openapi-fetch. Multi-page (HashRouter) with a sidebar nav and pages for
  Planner, Shopping, Compare, Recipes, Preferences, Pantry, Orders, Tonight, Sync, Dashboard,
  RecipeFamily, Prices, StoreLocator, Barcode, Aliases, Inspiration, and Grocy — each a
  TanStack Query + openapi-fetch consumer of the real REST API at `VITE_API_URL` (default
  `http://localhost:8080`), no mock data. The typed client is hand-written in
  `web/src/generated/spisordning.ts` (not codegen'd — `openapi-typescript` calls the TS compiler
  API, which TS 7.0 does not expose as a JS module). `npm run build` type-checks with TS 7;
  `npm run lint` uses the TS 6 toolchain.

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
