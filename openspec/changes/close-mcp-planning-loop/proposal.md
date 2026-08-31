# Close the MCP planning loop

## Why

Today the Spisordning MCP server can only **generate** a week of meal-plan candidates in memory — it cannot persist a plan, record the household's accept/swap/undo decisions, or link a recorded meal back to a plan slot. The REST API and CLI already do all of this (`POST /plans/run`, `PATCH /plans/{id}`, `POST /plans/{id}/decisions`, `food-brain plan`), but the MCP tool an AI chat actually calls (`list_recipe_candidates`) returns `PlannedSlot[]` that vanish when the tool call ends. This means an agent can propose a week but cannot commit it.

This change closes the loop: expose plan persistence + decision-recording over MCP so an agent can plan AND commit a week, not just propose one.

## What Changes

- **`persist_plan`** — runs the full weekly planner (via `service.Planning.RunPlan`, which wraps `PlanWeek`) and persists the result to `meal_plan`/`meal_plan_candidate`/`meal_plan_decision`/`shopping_requirement`. It supports `slots` for dinner, breakfast, and snack, and approves the persisted plan so subsequent decisions can be recorded.
- **`get_plan`** — returns the current week plan with its candidates, decisions, and shopping requirements (via the existing `service.Planning.GetPlan` and `ListShoppingRequirements`).
- **`set_plan_decision`** — records per-slot accept/swap decisions on a plan (via the existing `service.Planning.SetDecisions`).
- **`record_meal_from_plan`** — records a meal event linked to a specific plan slot. It is a separate tool from `record_meal_reaction`; its input accepts optional `plan_id`/`plan_slot_date` fields and wires them through to `CreateMealEvent`/`CreateMealEventWithSlot`.
- **`list_plans`** — lists all meal plans with their week start, status, and date range (via the existing `service.Planning.ListPlans`).
- **Shopping-requirement names** — plan-derived and recipe-derived shopping requirements expose a canonical ingredient name, and `create_shopping_list` accepts either an ingredient UUID or a canonical ingredient name.

All new MCP tools are thin adapters over existing service methods — no second planner, no second persistence path.

## Capabilities

### New Capabilities

- `mcp-planning`: plan persistence, decision recording, and plan-linked meal events over the MCP tool surface, exposing the existing `service.Planning` methods to MCP clients.

### Modified Capabilities

- `mcp-server`: extends the existing MCP tool set (currently 7 tools) with 5 new plan-management tools. The existing `record_meal_reaction` tool is unchanged.

## Impact

- **Affected code:**
  - `internal/mcptools/mcptools.go` — new tool input/output DTOs, new `PlanService` interface, new handlers.
  - `cmd/mcp-server/adapters.go` — `mcpStoreAdapter` implements the new `PlanService` methods by delegating to `service.Planning`.
  - `internal/mcptools/mcptools.go` — new `RecordMealFromPlanInput` DTO and `RecordMealFromPlan` method on the reaction service interface; existing `RecordReactionInput` is unchanged.
  - `cmd/mcp-server/adapters.go` — new `RecordMealFromPlan` adapter threads `plan_id`/`plan_slot_date` through to `CreateMealEvent`/`CreateMealEventWithSlot`.
  - `internal/persistence/recipes.go` — `Ingredient` upserts store `slug`; recipe-ingredient reads join `ingredient` to expose the canonical/display name.
  - `internal/persistence/meal_plan.go` — shopping-requirement reads join `ingredient` to expose the canonical/display name.
  - `internal/service/service.go`, `internal/service/planning.go`, `internal/dto/planning.go`, `internal/httpapi/plans.go`, `api/openapi.yaml`, `internal/openapi/types.gen.go`, and `web/src/generated/spisordning.ts` — expose `ingredient_name` on shopping requirements.
  - `cmd/mcp-server/adapters.go` — recipe-derived and plan-derived shopping requirements use canonical ingredient names; `create_shopping_list` resolves canonical names or ingredient UUIDs.
- **No new migrations** — all persistence already exists (`meal_plan`, `meal_plan_candidate`, `meal_plan_decision`, `shopping_requirement`, `meal_event`, `ingredient`).
- **Depends on:** `implement-mcp-server` (foundational MCP server), `complete-live-meal-planning` (breakfast/snack slot types consumed by planning).
- **Orthogonal to:** `unify-recipe-source` (plans reference recipes by Mealie ID; this change does not assume which source provides the recipe).
- **No changes to:** retailer adapters, pricing, Apple Notes, deployment, or the existing REST API surface.
