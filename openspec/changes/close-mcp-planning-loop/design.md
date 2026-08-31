## Context

The MCP server (`cmd/mcp-server/adapters.go`) already delegates to the application-layer planner (`internal/planning.PlanWeek`) but only returns `PlannedSlot[]` in memory. The REST API (`internal/httpapi/plans.go`) already exposes the full plan lifecycle: create, run, get, update status, set decisions, list candidates, list shopping requirements. The `service.Planning` type (`internal/service/planning.go`, `internal/service/planweek.go`) implements all of these against `internal/persistence/meal_plan.go`.

This change's job is to expose the same service methods as MCP tools — a thin adapter layer, not a second planning system.

## Goals / Non-Goals

**Goals:**
- Expose plan persistence (`persist_plan`) so an MCP client can run the planner and persist the result in one call.
- Expose plan reading (`get_plan`, `list_plans`) so an MCP client can inspect existing plans.
- Expose decision recording (`set_plan_decision`) so an MCP client can record accept/swap decisions.
- Link recorded meals to plan slots through a separate `record_meal_from_plan` tool.
- Keep the MCP layer as thin as possible — every new MCP tool delegates to an existing `service.Planning` method.

**Non-Goals:**
- Not redesigning the planner, scorer, or persistence schema.
- Not adding new REST API endpoints (the MCP tools reuse the existing service methods).
- Not implementing a streaming SSE plan run over MCP (that is `sse-progress-streaming`'s concern, already complete for REST).
- Not adding plan-linked meal event reading (a future `get_meal_details` tool can add that).

## Decisions

### D1: Five new tools

The MCP tool surface gains five new tools:

| Tool | Service method | Description |
|---|---|---|
| `persist_plan` | `Planning.PlanWeek` (via `RunPlan`) | Run the planner and persist the week. |
| `get_plan` | `Planning.GetPlan` + `ListShoppingRequirements` | Read back a plan with candidates, decisions, requirements. |
| `set_plan_decision` | `Planning.SetDecisions` | Record accept/swap decisions for slots. |
| `record_meal_from_plan` | `MealReactionService.RecordMealFromPlan` + `persistence.CreateMealEvent` / `CreateMealEventWithSlot` | Record a meal event linked to a plan slot, plus reaction. |
| `list_plans` | `Planning.ListPlans` | List all plans with metadata. |

The existing `record_meal_reaction` tool is unchanged. `record_meal_from_plan` is a separate tool with its own input DTO; it reuses the same persistence path but accepts optional plan-linking fields.

### D2: `persist_plan` reuses `service.Planning.RunPlan`

The MCP adapter delegates to `service.Planning.RunPlan`, which wraps `service.Planning.PlanWeek`. `RunPlan` calls the planner, surfaces persistence errors, and — when persistence succeeds — fetches or creates the plan row and sets its status to `approved`. Approving the plan in the same call makes the MCP loop usable: an agent can call `persist_plan` and then immediately call `set_plan_decision` without a separate status-update tool.

`persist_plan` accepts `week_start`, `days`, and `slots`. Empty `slots` preserves dinner-only behavior; non-empty `slots` may include `dinner`, `breakfast`, and `snack`. Dinner uses the full weekly planner, while breakfast and snack use the simple slot planner.

### D3: `get_plan` returns a flat view, not a nested plan view

`dto.MealPlanView` already wraps `Plan` + `Candidates` + `Decisions`. The MCP tool returns this as a single JSON object, not as separate API calls. Shopping requirements are included in the same response for convenience (the MCP client rarely needs to call a separate endpoint for them).

### D4: `set_plan_decision` requires the plan to be in `approved` status

The existing `service.Planning.SetDecisions` already enforces that the plan must be in `approved` status before decisions can be set. The MCP tool inherits this constraint without modification — if the plan is `draft`, the call fails with a clear error.

### D5: `record_meal_from_plan` is a separate tool that threads plan context

The existing `record_meal_reaction` tool already accepts `recipe`, `served_on`, `person_id`, `sentiment`, and optional `slot`. It remains unchanged. `record_meal_from_plan` accepts the same core fields plus two additional optional fields:

- `plan_id` (string, optional): the plan this meal was served from. When set, the meal event is linked to the plan.
- `plan_slot_date` (string, YYYY-MM-DD, optional): the slot date within the plan. Required when `plan_id` is set.

The adapter threads these through to `CreateMealEvent`/`CreateMealEventWithSlot`, which already accept `planID`/`planSlotDate`/`planSlotKind` parameters. The meal event's `meal_plan_id` and `meal_plan_slot_date` columns are populated, connecting the meal to its origin plan.

### D6: `list_plans` returns a summary, not a full plan view

`list_plans` returns a list of `PlanResponse` objects (id, week_start, status, created_at) — not the full `PlanView` with candidates and decisions. A client that needs the full view calls `get_plan` with the specific plan ID. This keeps the response small and the tool fast.

### D7: The `PlanService` interface in `mcptools` wraps `httpapi.PlanService`

The existing `mcptools` package defines a `PlannerService` interface (for `PlanDinners`/`PlanSlots`). This change adds a new `PlanService` interface that mirrors `httpapi.PlanService`'s surface but returns MCP-friendly DTOs. The `mcpStoreAdapter` implements both interfaces.

The `Dependencies` struct in `mcptools.go` gains a `Plan` field of type `PlanService`, wired from the composition root.

### D8: Shopping requirements expose canonical ingredient names

MCP shopping tools operate on canonical ingredient names, not raw UUIDs. Therefore:

- `persistence.Ingredient` stores `slug`, and `UpsertIngredient` defaults `slug` from `display` when absent, preserving an existing non-empty slug on conflict.
- `ListRecipeIngredients` and `ListShoppingRequirements` join `ingredient` and expose `COALESCE(i.slug, i.display, '')` as the requirement's ingredient name.
- `dto.ShoppingRequirement`, `httpapi.ShoppingRequirementResponse`, OpenAPI, generated Go types, and web types expose `ingredient_name`.
- `cmd/mcp-server` maps `IngredientName` into `mcptools.ShoppingRequirement.Ingredient` for both recipe-derived requirements (`get_shopping_requirements`) and plan-derived requirements (`get_plan`).
- `create_shopping_list` resolves each item to a `domain.IngredientID` by preferring an explicit `ingredient_id`, then a UUID-valued `ingredient`, then a deterministic ID derived from the canonical name.

This keeps the MCP surface usable by LLM clients while preserving deterministic UUID links in `shopping_list_item.ingredient_id`.

## Risks / Trade-offs

- **`RunPlan` may be slow** — it runs candidate generation, scoring, and persistence in one call. An MCP client should be prepared for a multi-second response. No streaming is implemented (D2); if a future client needs progress events, a separate `persist_plan_stream` tool can be added later.
- **The plan lifecycle is simple** — `draft` → `approved` → `archived`. The MCP tools do not expose a `create_plan` or `archive_plan` endpoint; the plan is created automatically by `persist_plan`. If a client needs explicit control over plan creation (e.g. to set a custom week_start), a `create_plan` tool can be added later.
- **No plan-linked meal event reading** — this change only writes the link. Reading a meal event's plan context requires a separate tool (future `get_meal_details`). The `meal_event` table already has `meal_plan_id` and `meal_plan_slot_date` columns; the gap is purely in the MCP surface, not the schema.
- **`record_meal_from_plan` and `record_meal_reaction` are two tools** — this is intentional. The existing tool remains unchanged for clients that don't need plan linking; the new tool is for clients that do. They share the same persistence path but have different input shapes.
