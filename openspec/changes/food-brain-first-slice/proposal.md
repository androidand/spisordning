# Food Brain — first vertical slice

## Why

The family needs weekly dinner planning that respects each person's preferences, the cook's
energy that day, what's on sale at their Willys store, and what the kids already ate at
school. A working v1 exists as an n8n workflow, but its logic lives in workflow nodes and has
no durable model of preferences or meal reactions, so it can't learn. This slice builds the
thin end-to-end pipe through a tested Go service that owns those durable domains, proving the
whole flow (recipe → plan → shopping list) before any single part is deepened.

## What Changes

- New Go service `food-brain` with a PostgreSQL schema for the domains no existing app owns:
  people, preferences, preference observations, meal events, meal reactions, effort profiles,
  planning constraints, plan candidates/decisions, ingredient mappings, shopping requirements.
- Read-only sync from the already-deployed **Mealie** (recipe ids + normalized ingredients);
  Mealie stays the recipe authority — we store references and snapshots, not copies.
- A deterministic Go scorer (preferences, effort, repetition penalty, Skolmaten school-lunch
  dedup, Willys campaign bias) that emits ranked dinner candidates. **Olla** (local LLM) is
  used only to vary candidates and write human-readable explanations — never to decide
  feasibility.
- Canonical, retailer-independent **shopping requirements** as the planner's output.
- A `willys-adapter` HTTP service wrapping the existing TypeScript `willys-client`, exposing a
  retailer interface (`SearchProducts`, `GetProduct`, `resolveRequirements`,
  `CreateShoppingList`). Its primary output is a **per-week wishlist**; cart-filling and
  payment stay manual (see design). It owns session/CSRF/retry so Go never touches cookies.
- Local docker-compose for Postgres + food-brain + willys-adapter (deployment baseline;
  tengil packaging is a separate change in the tengil repo).

## Capabilities

### New Capabilities

- `meal-planning`: durable family food domains, the deterministic+LLM suggestion engine, and
  the weekly-plan → canonical shopping-requirements flow.
- `retailer-adapter`: retailer-independent product resolution and shopping-list output,
  wrapping the Willys client behind a stable interface.

### Modified Capabilities

<!-- none — new repo -->

## Impact

- New repo `~/dev/spisordning`: Go module, PostgreSQL schema/migrations, docker-compose,
  config. The `willys-adapter` service's *code* lives in the willys-client repo
  (`apps/willys-adapter`, next to its TypeScript dependency); its deployment config lives here.
- Integrates with existing Mealie, Olla, Skolmaten (read-only) and the `willys-client` repo
  (wrapped, unmodified).
- Absorbs the responsibilities of the existing n8n `weekly-meal-planner` workflow, which is
  demoted to a scheduler/webhook or retired once this slice is proven.
- No automated checkout: BankID/Klarna payment and slot booking remain human actions.
