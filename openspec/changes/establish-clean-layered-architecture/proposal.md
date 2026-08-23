# Proposal: establish-clean-layered-architecture

## Problem

Spisordning currently has no application service layer. HTTP handlers in
`internal/httpapi` call `persistence.Store` methods directly through a thin
adapter in `cmd/food-brain/adapters.go`. Domain logic for people, recipes,
meals, and pantry operations lives in persistence (raw SQL) or in ad-hoc
adapter methods — there is no place for validation, orchestration, or
cross-cutting concerns.

Meanwhile, the planning pipeline (`cmd/food-brain/plan.go`) and the ambient
surface (`cmd/food-brain/tonight.go`) duplicate logic that should live in a
service. The MCP server (future) would need the same service abstractions
the HTTP API uses.

## Proposed solution

Introduce a `service` layer between `httpapi` and `persistence`:

```
cmd/food-brain          — composition root (DI wiring, CLI commands)
  ↓
internal/httpapi        — HTTP handlers + service interfaces (DI contracts)
  ↓
internal/service        — application services (use-case orchestration)
  ↓
internal/persistence    — Postgres repositories
  ↓
internal/domain         — pure value types
```

With infrastructure clients on the side:

```
internal/httpclient     — shared HTTP transport
internal/mealie         — Mealie client
internal/skolmaten      — school lunch client
internal/retailer       — retailer adapter client
internal/llm            — LLM provider client
internal/recipeimport   — web recipe import client
internal/ingredients    — SLV + Dabas clients
internal/matpriskollen  — price comparison client
```

## What changes

1. **New `service` layer** in `internal/service/` — every service implements
   an interface defined in `httpapi` (the DI contract). Services know about
   persistence (via a `Store` interface they depend on) and about other
   services they need (e.g., `Meals` needs `Preferences`).

2. **Move adapter logic** from `cmd/food-brain/adapters.go` into
   `internal/service/`. The cmd layer keeps only wiring code.

3. **Update architecture test** to classify `internal/service` as a new
   layer with its own import rules.

4. **Add missing endpoints** to the OpenAPI spec and implement them behind
   the new service layer.

## Service interfaces (defined in httpapi)

| Interface | Purpose |
|-----------|---------|
| `PersonService` | CRUD for household persons |
| `PreferencesService` | Read preferences (reaction learning handled by Meals) |
| `RecipesService` | List/get recipe refs, sync from Mealie |
| `MealsService` | Record meals + reactions, learn preferences |
| `PantryService` | Inventory locations, lots, events |
| `PlanningService` | Meal plans, candidates, decisions, shopping reqs |
| `IngredientsService` | Food lookup, nutrition data from SLV/Dabas |
| `StoresService` | Store search, product search, price offers |

## Dependencies

- Depends on `establish-enforced-go-architecture` (architecture test baseline).
- Depends on `establish-household-and-catalog` (domain types).
- All domain service changes are separate OpenSpec changes, cross-linked to
  this epic.

## Out of scope

- MCP server v2 implementation (tracked as a separate sub-change).
- Retailer adapter refactoring (tracked separately).
- Recipe family persistence (tracked separately).
