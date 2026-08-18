# Spisordning

Family meal planning, grocery orchestration, and preference-aware menu generation for a
self-hosted homelab. Swedish for roughly "stove order" / "kitchen rota" — the system that
decides what the family eats this week and gets it into the shopping basket.

> Status: **design phase**. A working v1 already exists as an n8n workflow (see below);
> spisordning absorbs and replaces it with a tested Go service. Architecture and the first
> vertical slice are planned as OpenSpec changes under `openspec/`.
>
> **The current mission document is [`PLAN.md`](PLAN.md)** — a much larger relational
> food-knowledge-graph vision than this README's original "thin wrapper" framing below. Read
> `PLAN.md` and [`docs/research/current-state.md`](docs/research/current-state.md) first; treat
> this README as historical context for the first slice, not the target architecture.

## What already exists (reconciled 2026-07, do not rebuild)

A homelab recon found much of the generic layer already running:

- **A working v1 meal planner** — `~/dev/homelab/n8n/weekly-meal-planner.workflow.json`:
  Skolmaten school-lunch → LLM dinner plan (via local **Olla** proxy) → shopping list →
  optional Mealie write → optional Willys enrichment. **Spisordning absorbs this as its v1**:
  the durable domains and scoring move into tested Go; n8n is demoted to a thin
  scheduler/webhook (or retired). Two overlapping planners is exactly the integration
  nightmare this project's rule forbids.
- **Mealie** — already deployed (tengil app `hlab-mealie`). Reuse it; don't stand up another.
- **Olla** — local Ollama/OpenAI-compatible LLM proxy (`192.168.1.240:40114`). Food Brain uses
  it for candidate variation and human-readable explanations (see suggestion engine below).
- **Skolmaten** — school-lunch API (`~/dev/skolmaten`, `192.168.1.120:8787`). First-class
  input: don't propose for dinner what the kids already ate at school.
- **Home Assistant** (`homeassistant.local:8123`) + **homeops MCP** (~57 HA tools,
  `~/dev/homeops-mcp-research`) — the ambient interface and HA control already exist; Food
  Brain drives HA through these rather than reimplementing.
- **No reusable Postgres confirmed** — spisordning stands up its own.

## Suggestion engine: LLM-assisted, Go-scored

Hard constraints and scoring (preferences, effort, repetition penalty, pantry-awareness,
campaign bias, school-lunch dedup) are **deterministic, tested Go** — reproducible and
explainable. **Olla** proposes and varies candidates and writes the natural-language
"why this meal" text. The LLM never decides feasibility; Go does. Windmill/n8n decides *when*
to run; Food Brain decides *what* the answer is.

## Guiding principle

**One owner per domain.** Every domain has exactly one source of truth. Integrations talk
over APIs — never by writing into another application's database. The custom code we build is
only the family-specific intelligence that no off-the-shelf app models well; everything
generic (recipes, pantry, product data) is an existing open-source component.

## Composition

```
┌─────────────────────────────────────────────┐
│ Family UI / AI chat / Home Assistant         │
└───────────────────┬─────────────────────────┘
                    │
          ┌─────────▼─────────┐
          │  Food Brain       │  ← custom Go service + PostgreSQL
          │  (this repo)      │     the unique ~20%
          └─────────┬─────────┘
                    │
    ┌───────────────┼────────────────┐
    │               │                │
┌───▼────┐     ┌────▼────┐     ┌─────▼────────┐
│ Mealie │     │ Grocy   │     │ Willys       │
│Recipes │     │ Pantry  │     │ adapter      │
└────────┘     └─────────┘     └──────┬───────┘
                                      │
                              ┌───────▼───────┐
                              │ willys-client │ (existing TS repo,
                              │ wrapped as    │  wrapped, not ported)
                              │ HTTP service  │
                              └───────────────┘
```

## Domain ownership

| Domain | Source of truth |
|---|---|
| Recipe instructions & ingredients | Mealie |
| Recipe images, categories, tags | Mealie |
| Pantry stock, expiry, storage | Grocy (later) |
| Family preferences & observations | Food Brain |
| Cooking effort / kitchen energy | Food Brain |
| Meal history & reactions | Food Brain |
| Weekly-plan decisions | Food Brain |
| Canonical shopping requirements | Food Brain |
| Retailer product mappings | Food Brain / Willys adapter |
| Final retailer cart & payment | Willys (human-in-the-loop) |
| Supplemental product metadata | Open Food Facts (later) |

## Off-the-shelf components

- **Mealie** (AGPL) — canonical recipe store, editor, import, REST API, webhooks. The recipe authority.
- **Grocy** (MIT) — physical inventory only: stock, expiry, storage. *Not* a second recipe system. Added once a pantry workflow has proven itself.
- **Windmill** — operational workflow engine (schedules, approvals, retries) in real code (Go/TS/Python), not low-code spaghetti. Orchestrates *when* things run; the Food Brain decides *what* the answer is.
- **Home Assistant** — ambient family-facing surface: tonight's dinner, thaw reminders, one-tap post-meal reactions.
- **Open Food Facts** — supplemental barcode/nutrition enrichment. Never authoritative for Willys price/availability/article-id.
- **Authentik** — SSO across the family-facing services. Optional until multiple UIs exist.

## The Willys handoff boundary

The Willys API has **no standalone basket to create** — only the per-session cart and durable
**wishlists**. Checkout requires BankID/Klarna, which we never automate. So the boundary is:

> Food Brain writes a per-week **wishlist** → human reviews/adjusts it in the Willys app on
> their phone → one call converts the wishlist to the session cart → human books the slot and pays.

The wishlist is durable, reviewable on mobile, one API call from becoming a cart, and degrades
gracefully to an in-store shopping list. The retailer adapter's primary output is a shopping
list; cart-filling is an optional second step. **Payment and slot booking always stay manual.**

The Food Brain emits retailer-independent requirements; the adapter resolves them:

```jsonc
// Food Brain emits:
{ "ingredientId": "cauliflower", "quantity": 500, "unit": "g",
  "acceptableForms": ["fresh", "frozen"], "preferredForm": "fresh" }

// Willys adapter resolves to:
{ "retailerProductId": "willys-123456", "packages": 1,
  "resolvedQuantity": 650, "matchType": "exact", "confidence": 0.94 }
```

A Mealie recipe must **never** carry a permanent Willys article number as its ingredient
identity — that coupling is the adapter's job, and it can change store to store and week to week.

### Campaign-aware planning

The Willys v1 API exposes store-scoped campaigns (`/v1/search/campaigns/online`) and
promotions (`/v1/promotionproduct/*`). A planner that biases the week's menu toward what's on
sale at the family's own store is a first-class feature, not an afterthought — it's the thing
that makes the household feel the system paying for itself. (Store scoping is why the Willys
client's `ensureHomeStore()` matters here.)

## Pantry realism (the ADHD clause)

Exact pantry inventory becomes more work than it saves and dies within a fortnight. Inventory
is classed, and Grocy tracks only the first class initially:

- **TRACKED_EXACTLY** — frozen meat, expensive items, long-expiry goods, leftovers. → Grocy.
- **TRACKED_APPROXIMATELY** — milk, cheese, eggs, vegetables. → Food Brain, coarse.
- **ASSUMED_STAPLE** — salt, pepper, flour, common spices. → assumed present, never tracked.

## Hard-won constraints (from building willys-client)

- **No basket object.** Session cart (set-absolute quantities) + wishlists. See handoff above.
- **Store-scoped everything.** Campaigns, prices, slots depend on the active store; the adapter must pin it.
- **Swedish ingredient parsing is the grind.** dl/msk/tsk/förp → grams → package sizes. The
  `ingredient_mappings` table is first-class and needs a review UI early, or every downstream
  quantity is garbage.
- **The Willys client is TypeScript and stays that way.** It's wrapped as an HTTP service that
  owns session caching, CSRF, retries and rate-limiting so the Go brain never touches cookies.

## Deployment

**Baseline now: plain LXC + docker-compose.** Fastest reliable path to serving the family;
matches how these apps are distributed. New services this needs: PostgreSQL, `food-brain`
(this repo), `willys-adapter`. Reuse the already-deployed Mealie; reuse Olla, Skolmaten, HA.

**Strategic target: tengil.** The work to make tengil's single-app workflow handle full
multi-container stacks (API + DB + auth) — so spisordning, Mealie+Postgres, and brick-now all
deploy through it — is planned as an OpenSpec change **in the tengil repo**, not here. Once
those `package` manifests survive an update/restart cycle with data intact, deployment
migrates onto tengil. Known friction to resolve there: OCI installs currently require
`root@pam` password auth (API tokens don't work), which blocks a Go orchestrator driving it.

Order once building: PostgreSQL → `food-brain` → `willys-adapter` → wire Windmill/n8n
scheduling → HA surfacing via homeops.

Later: `grocy` (proven pantry workflow), `open-food-facts` (enrichment), `authentik` (SSO).

## First vertical slice

Add a recipe in Mealie → sync id + normalized ingredients to Food Brain → add preference/effort
metadata → ask for 5 dinner suggestions → approve the menu → produce canonical shopping
requirements → resolve through the Willys adapter → review uncertain matches → create a Willys
wishlist → show tonight's meal in Home Assistant → collect one-tap reactions after dinner.

That builds the unique ~20% (preference learning, effort-aware planning, retailer resolution)
while existing apps provide the generic 80%.

## Running the core (no infra needed)

The deterministic planning core is built and tested without any database, LLM, or network:

```bash
go test ./...                # 114 tests incl. an end-to-end plan test against fake services
go run ./cmd/food-brain demo # in-memory demo: ranks a sample week + prints requirements
```

The live weekly pipe is `food-brain plan` (dry-run by default):

```bash
cp family.example.json family.json   # people, preferences, weekday kitchen energy
export MEALIE_BASE_URL=... MEALIE_API_TOKEN=...          # required
export SKOLMATEN_BASE_URL=... SKOLMATEN_SCHOOL=skolan    # optional: school-lunch dedup
export OLLA_OPENAI_BASE_URL=... OLLA_MODEL=...           # optional: explanations
export ADAPTER_URL=http://localhost:8402                 # willys-adapter

go run ./cmd/food-brain plan                    # prints plan + requirements, changes nothing
go run ./cmd/food-brain plan --create-wishlist  # resolves products, creates "Vecka NN" wishlist
```

Confidently-resolved products go on the wishlist; anything below the review threshold is
printed as needing review and **never silently added**. Payment and slot booking stay in the
Willys app.

The demo shows all six scoring signals at work — preferences (confidence-weighted, per-person
weight), effort vs. the day's kitchen energy, a repetition penalty, school-lunch dedup,
campaign bias, and novelty/familiarity (whether the household has cooked it before) — plus
hard-constraint feasibility. Layout:

- `internal/domain` — core value types (people, preferences, candidates, canonical ingredients, plan context)
- `internal/scoring` — the pure, deterministic scorer (`Rank`)
- `internal/planning` — the weekly planner loop (`PlanWeek`) + canonical shopping-requirement aggregation
- `internal/mealie` — read-only Mealie recipe sync (references + snapshots, never copies)
- `internal/skolmaten` — school-lunch menu client + meal-name tokenizer for dedup
- `internal/llm` — AI provider abstraction: Olla is the primary implementation; explanations + feasible-set-only reordering (never load-bearing)
- `internal/retailer` — client for the willys-adapter (resolve, create wishlist)
- `internal/recipefamily` — in-memory recipe-family/revision DAG core (immutable revisions)
- `internal/recipeimport` — external recipe JSON-LD import into review candidates
- `internal/httpclient` — shared JSON-over-HTTP transport for every backend client
- `cmd/food-brain` — the CLI (`demo`, `plan`)
- `migrations/0001-0007` — the durable PostgreSQL schema

The **willys-adapter** is built and lives with its TypeScript dependency in the willys-client
repo (`apps/willys-adapter`, `npm run adapter`, port 8402); the Go side talks to it through
`internal/retailer`. `docker-compose.yml` here runs Postgres (schema auto-applied) + the
adapter — copy `.env.example` to `.env` first.

Still to wire: Postgres persistence (the schema is ready; `plan` currently syncs per run),
the ingredient-mapping review surface, and Home Assistant surfacing. Tracked in
`openspec/changes/food-brain-first-slice/tasks.md`.

## Related repos

- [`willys`](../willys) — the Willys API client (TypeScript) this system wraps.
- [`tengil`](../tengil) — Proxmox app/container control plane; candidate deployment mechanism.
- [`homelab`](../homelab) — existing homelab stack (n8n, Home Assistant, etc.).
