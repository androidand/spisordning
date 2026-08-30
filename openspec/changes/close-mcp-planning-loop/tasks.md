# Tasks: close-mcp-planning-loop

## 1. New MCP tool DTOs (internal/mcptools/mcptools.go)

- [ ] 1.1 Define `PersistPlanInput` — fields: `week_start` (YYYY-MM-DD string, optional, defaults to next Monday), `days` (int, default 7), `slots` ([]string, optional, same values as `ListCandidatesInput.Slots`). Validation: `days` must be 1–31; `week_start` must parse as YYYY-MM-DD if provided.
- [ ] 1.2 Define `PersistPlanResult` — fields: `plan_id` (string), `week_start` (YYYY-MM-DD string), `days` (int), `slot_count` (int, number of planned slots), `persisted` (bool).
- [ ] 1.3 Define `GetPlanInput` — field: `plan_id` (string, required). Validation: must parse as a valid `MealPlanID`.
- [ ] 1.4 Define `GetPlanResult` — fields: `plan` (PlanSummary), `candidates` ([]PlanCandidate), `decisions` ([]PlanDecision), `shopping_requirements` ([]ShoppingRequirement). This is a flat view combining `dto.MealPlanView` and `dto.ShoppingRequirement`.
- [ ] 1.5 Define `SetPlanDecisionInput` — fields: `plan_id` (string, required), `decisions` ([]PlanDecisionInput), where each `PlanDecisionInput` has `slot_date` (YYYY-MM-DD), `slot_kind` (string, one of "dinner"/"breakfast"/"snack", defaults to "dinner"), `mealie_recipe_id` (string, required).
- [ ] 1.6 Define `PlanDecisionInput` — fields: `slot_date` (YYYY-MM-DD), `slot_kind` (string, default "dinner"), `mealie_recipe_id` (string). Validation: `mealie_recipe_id` must be non-empty; `slot_date` must parse.
- [ ] 1.7 Define `PlanDecisionResponse` — fields: `plan_id` (string), `slot_date` (YYYY-MM-DD), `slot_kind` (string), `mealie_recipe_id` (string), `decided_at` (string, ISO 8601, nullable).
- [ ] 1.8 Define `PlanCandidate` — fields: `id` (string), `slot_date` (YYYY-MM-DD), `slot_kind` (string), `mealie_recipe_id` (string), `title` (string), `score` (float64), `rank` (int), `feasible` (bool). Mirrors `dto.MealPlanCandidate` but with MCP-friendly types.
- [ ] 1.9 Define `PlanSummary` — fields: `id` (string), `week_start` (YYYY-MM-DD), `status` (string), `created_at` (string, ISO 8601). Mirrors `dto.MealPlan`.
- [ ] 1.10 Define `ShoppingRequirement` — fields: `id` (string), `ingredient_id` (string), `quantity` (float64), `unit` (string), `acceptable_forms` ([]string), `preferred_form` (string, nullable). Mirrors `dto.ShoppingRequirement`.

## 2. `record_meal_from_plan` DTO (internal/mcptools/mcptools.go)

- [ ] 2.1 Define `RecordMealFromPlanInput` with `recipe`, `served_on`, `person_id`, `sentiment`, optional `slot`, optional `plan_id`, and optional `plan_slot_date`.
- [ ] 2.2 Add `RecordMealFromPlan(ctx, in RecordMealFromPlanInput) (RecordReactionResult, error)` to the `MealReactionService` interface.
- [ ] 2.3 Leave the existing `RecordReactionInput` and `record_meal_reaction` handler unchanged.

## 3. `PlanService` interface (internal/mcptools/mcptools.go)

- [ ] 3.1 Define `PlanService` interface with methods: `ListPlans(ctx) ([]PlanSummary, error)`, `GetPlan(ctx, planID) (GetPlanResult, error)`, `SetDecisions(ctx, planID string, decisions []PlanDecisionInput) ([]PlanDecisionResponse, error)`, `PersistPlan(ctx, in PersistPlanInput) (PersistPlanResult, error)`.
- [ ] 3.2 Add `Plan PlanService` field to `Dependencies` struct in `mcptools.go`.
- [ ] 3.3 Wire `deps.Plan` into `RegisterTools` — if non-nil, register `persist_plan`, `get_plan`, `set_plan_decision`, `list_plans`.

## 4. MCP tool handlers (internal/mcptools/mcptools.go)

- [ ] 4.1 Implement `persistPlanHandler` — calls `deps.Plan.PersistPlan(ctx, in)`, returns `PersistPlanResult`. Maps service errors to MCP tool-call errors.
- [ ] 4.2 Implement `getPlanHandler` — calls `deps.Plan.GetPlan(ctx, in.PlanID)`, returns `GetPlanResult`. Maps service errors (including "not found") to MCP tool-call errors.
- [ ] 4.3 Implement `setPlanDecisionHandler` — calls `deps.Plan.SetDecisions(ctx, in.PlanID, in.Decisions)`, returns `[]PlanDecisionResponse`. Maps service errors to MCP tool-call errors.
- [ ] 4.4 Implement `listPlansHandler` — calls `deps.Plan.ListPlans(ctx)`, returns `[]PlanSummary`. Maps service errors to MCP tool-call errors.
- [ ] 4.5 Implement `recordMealFromPlanHandler` — validates `plan_slot_date` when `plan_id` is set, calls `deps.Reactions.RecordMealFromPlan(ctx, in)`, and returns `RecordReactionResult`. Maps service errors to MCP tool-call errors.
- [ ] 4.6 Register all five new tools in `RegisterTools` with descriptive names and descriptions matching the tool's purpose.

## 5. MCP adapter implementation (cmd/mcp-server/adapters.go)

- [ ] 5.1 Implement `PlanService` interface on `mcpStoreAdapter` — `ListPlans` delegates to `svc.ListPlans(ctx)`, `GetPlan` delegates to `svc.GetPlan(ctx, id)`, `SetDecisions` delegates to `svc.SetDecisions(ctx, planID, in)`, `PersistPlan` delegates to `svc.RunPlan(ctx, PlanRunInput{Week: in.WeekStart, Days: in.Days})`.
- [ ] 5.2 Map `httpapi.PlanResponse` → `PlanSummary` in `ListPlans`/`GetPlan` — extract id, week_start, status, created_at.
- [ ] 5.3 Map `httpapi.PlanCandidateResponse` → `PlanCandidate` in `GetPlan` — extract slot_date, slot_kind, mealie_recipe_id, title, score, rank, feasible.
- [ ] 5.4 Map `httpapi.PlanDecisionResponse` → `PlanDecisionResponse` in `GetPlan`/`SetDecisions` — extract plan_id, slot_date, slot_kind, mealie_recipe_id, decided_at.
- [ ] 5.5 Map `httpapi.ShoppingRequirementResponse` → `ShoppingRequirement` in `GetPlan` — extract id, ingredient_id, quantity, unit, acceptable_forms, preferred_form.
- [ ] 5.6 Map `PersistPlanInput` → `httpapi.PlanRunInput` — `week_start` → `Week`, `days` → `Days`, `create_wishlist` defaults to false.
- [ ] 5.7 Map `PersistPlanResult` from `httpapi.PlanRunResult` — extract plan_id (from the persisted plan's ID, read back via `GetPlan` after `RunPlan`), week_start, days, slot_count (from plan candidates), persisted.
- [ ] 5.8 Wire `PlanService` in the composition root (`cmd/mcp-server/adapters.go` or equivalent) — pass `service.NewPlanning(db, mealie)` or the existing `*service.Planning` instance.

## 6. `record_meal_from_plan` adapter (cmd/mcp-server/adapters.go)

- [ ] 6.1 Implement `RecordMealFromPlan` on `mcpStoreAdapter` using `mcptools.RecordMealFromPlanInput`.
- [ ] 6.2 When `plan_id` is set, parse it as `domain.MealPlanID` and parse `plan_slot_date` as `time.Time`; pass both to `CreateMealEvent`/`CreateMealEventWithSlot`.
- [ ] 6.3 When `plan_id` is set but `plan_slot_date` is not, reject with a clear error ("plan_slot_date is required when plan_id is set").
- [ ] 6.4 When `plan_id` is set, also pass `planSlotKind` (from `in.Slot`) to `CreateMealEventWithSlot` so the meal event carries the slot kind for future plan-linking.
- [ ] 6.5 When `plan_id` is not set, pass `nil` plan context to preserve unlinked behavior.

## 7. Tool registration and wiring

- [ ] 7.1 Update `cmd/mcp-server/adapters.go` to create the `mcptools.Dependencies` struct with the new `Plan` field populated.
- [ ] 7.2 Verify that `RegisterTools` conditionally registers the new tools only when `deps.Plan` is non-nil (same pattern as existing tools).
- [ ] 7.3 Ensure the new tool names are unique and don't collide with existing tools (no `persist_plan`, `get_plan`, `set_plan_decision`, `list_plans`, or `record_meal_from_plan` exists yet).

## 8. Verification

- [ ] 8.1 `go build ./...` succeeds — no compilation errors in `internal/mcptools` or `cmd/mcp-server`.
- [ ] 8.2 `go test ./internal/mcptools/...` passes — unit tests for the new handlers (at minimum, test that invalid inputs are rejected before reaching the service layer).
- [ ] 8.3 `openspec validate close-mcp-planning-loop` passes.
- [ ] 8.4 `openspec status --change close-mcp-planning-loop` shows all artifacts in place.
