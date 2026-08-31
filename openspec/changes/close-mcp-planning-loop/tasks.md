# Tasks: close-mcp-planning-loop

## 1. New MCP tool DTOs (internal/mcptools/mcptools.go)

- [x] 1.1 Define `PersistPlanInput` — fields: `week_start` (YYYY-MM-DD string, optional, defaults to next Monday), `days` (int, default 7), `slots` ([]string, optional, same values as `ListCandidatesInput.Slots`). Validation: `days` must be 1–31; `week_start` must parse as YYYY-MM-DD if provided.
- [x] 1.2 Define `PersistPlanResult` — fields: `plan_id` (string), `week_start` (YYYY-MM-DD string), `days` (int), `slot_count` (int, number of planned slots), `persisted` (bool).
- [x] 1.3 Define `GetPlanInput` — field: `plan_id` (string, required). Validation: must parse as a valid `MealPlanID`.
- [x] 1.4 Define `GetPlanResult` — fields: `plan` (PlanSummary), `candidates` ([]PlanCandidate), `decisions` ([]PlanDecision), `shopping_requirements` ([]ShoppingRequirement). This is a flat view combining `dto.MealPlanView` and `dto.ShoppingRequirement`.
- [x] 1.5 Define `SetPlanDecisionInput` — fields: `plan_id` (string, required), `decisions` ([]PlanDecisionInput), where each `PlanDecisionInput` has `slot_date` (YYYY-MM-DD), `slot_kind` (string, one of "dinner"/"breakfast"/"snack", defaults to "dinner"), `mealie_recipe_id` (string, required).
- [x] 1.6 Define `PlanDecisionInput` — fields: `slot_date` (YYYY-MM-DD), `slot_kind` (string, default "dinner"), `mealie_recipe_id` (string). Validation: `mealie_recipe_id` must be non-empty; `slot_date` must parse.
- [x] 1.7 Define `PlanDecisionResponse` — fields: `plan_id` (string), `slot_date` (YYYY-MM-DD), `slot_kind` (string), `mealie_recipe_id` (string), `decided_at` (string, ISO 8601, nullable).
- [x] 1.8 Define `PlanCandidate` — fields: `id` (string), `slot_date` (YYYY-MM-DD), `slot_kind` (string), `mealie_recipe_id` (string), `title` (string), `score` (float64), `rank` (int), `feasible` (bool). Mirrors `dto.MealPlanCandidate` but with MCP-friendly types.
- [x] 1.9 Define `PlanSummary` — fields: `id` (string), `week_start` (YYYY-MM-DD), `status` (string), `created_at` (string, ISO 8601). Mirrors `dto.MealPlan`.
- [x] 1.10 Define `ShoppingRequirement` — fields: `id` (string), `ingredient_id` (string), `ingredient` (string, canonical name), `quantity` (float64), `unit` (string), `acceptable_forms` ([]string), `preferred_form` (string, nullable). Mirrors `dto.ShoppingRequirement` while keeping the MCP-facing `ingredient` name field.

## 2. `record_meal_from_plan` DTO (internal/mcptools/mcptools.go)

- [x] 2.1 Define `RecordMealFromPlanInput` with `recipe`, `served_on`, `person_id`, `sentiment`, optional `slot`, optional `plan_id`, and optional `plan_slot_date`.
- [x] 2.2 Add `RecordMealFromPlan(ctx, in RecordMealFromPlanInput) (RecordReactionResult, error)` to the `MealReactionService` interface.
- [x] 2.3 Leave the existing `RecordReactionInput` and `record_meal_reaction` handler unchanged.

## 3. `PlanService` interface (internal/mcptools/mcptools.go)

- [x] 3.1 Define `PlanService` interface with methods: `ListPlans(ctx) ([]PlanSummary, error)`, `GetPlan(ctx, planID) (GetPlanResult, error)`, `SetDecisions(ctx, planID string, decisions []PlanDecisionInput) ([]PlanDecisionResponse, error)`, `PersistPlan(ctx, in PersistPlanInput) (PersistPlanResult, error)`.
- [x] 3.2 Add `Plan PlanService` field to `Dependencies` struct in `mcptools.go`.
- [x] 3.3 Wire `deps.Plan` into `RegisterTools` — if non-nil, register `persist_plan`, `get_plan`, `set_plan_decision`, `list_plans`.

## 4. MCP tool handlers (internal/mcptools/mcptools.go)

- [x] 4.1 Implement `persistPlanHandler` — calls `deps.Plan.PersistPlan(ctx, in)`, returns `PersistPlanResult`. Maps service errors to MCP tool-call errors.
- [x] 4.2 Implement `getPlanHandler` — calls `deps.Plan.GetPlan(ctx, in.PlanID)`, returns `GetPlanResult`. Maps service errors (including "not found") to MCP tool-call errors.
- [x] 4.3 Implement `setPlanDecisionHandler` — calls `deps.Plan.SetDecisions(ctx, in.PlanID, in.Decisions)`, returns `[]PlanDecisionResponse`. Maps service errors to MCP tool-call errors.
- [x] 4.4 Implement `listPlansHandler` — calls `deps.Plan.ListPlans(ctx)`, returns `[]PlanSummary`. Maps service errors to MCP tool-call errors.
- [x] 4.5 Implement `recordMealFromPlanHandler` — validates `plan_slot_date` when `plan_id` is set, calls `deps.Reactions.RecordMealFromPlan(ctx, in)`, and returns `RecordReactionResult`. Maps service errors to MCP tool-call errors.
- [x] 4.6 Register all five new tools in `RegisterTools` with descriptive names and descriptions matching the tool's purpose.

## 5. MCP adapter implementation (cmd/mcp-server/adapters.go)

- [x] 5.1 Implement `PlanService` interface on `mcpStoreAdapter` — `ListPlans` delegates to `service.Planning.ListPlans`, `GetPlan` delegates to `service.Planning.GetPlan` + `ListShoppingRequirements`, `SetDecisions` delegates to `service.Planning.SetDecisions`, and `PersistPlan` delegates to `service.Planning.RunPlan` with `service.PlanWeekInput{WeekStart, Days, People, Preferences, EnergyFor, SchoolTagsFor, Slots}`.
- [x] 5.2 Map `dto.MealPlan` → `PlanSummary` in `ListPlans`/`GetPlan` — extract id, week_start, status, created_at.
- [x] 5.3 Map `dto.MealPlanCandidate` → `PlanCandidate` in `GetPlan` — extract slot_date, slot_kind, mealie_recipe_id, title, score, rank, feasible.
- [x] 5.4 Map `dto.MealPlanDecision` → `PlanDecisionResponse` in `GetPlan`/`SetDecisions` — extract plan_id, slot_date, slot_kind, mealie_recipe_id, decided_at.
- [x] 5.5 Map `dto.ShoppingRequirement` → `ShoppingRequirement` in `GetPlan` — extract id, ingredient_id, ingredient name, quantity, unit, acceptable_forms, preferred_form.
- [x] 5.6 Map `PersistPlanInput` → `service.PlanWeekInput` — `week_start` → `WeekStart`, `days` → `Days`, `slots` → `Slots`, and load household/effort/school context from persistence.
- [x] 5.7 Map `service.PlanRunResult` → `PersistPlanResult` — extract plan_id, week_start, days, slot_count, and persisted.
- [x] 5.8 Wire `PlanService` in the composition root (`cmd/mcp-server/adapters.go` or equivalent) — pass `service.NewPlanning(db, mealie)` or the existing `*service.Planning` instance.

## 6. `record_meal_from_plan` adapter (cmd/mcp-server/adapters.go)

- [x] 6.1 Implement `RecordMealFromPlan` on `mcpStoreAdapter` using `mcptools.RecordMealFromPlanInput`.
- [x] 6.2 When `plan_id` is set, parse it as `domain.MealPlanID` and parse `plan_slot_date` as `time.Time`; pass both to `CreateMealEvent`/`CreateMealEventWithSlot`.
- [x] 6.3 When `plan_id` is set but `plan_slot_date` is not, reject with a clear error ("plan_slot_date is required when plan_id is set").
- [x] 6.4 When `plan_id` is set, also pass `planSlotKind` (from `in.Slot`) to `CreateMealEventWithSlot` so the meal event carries the slot kind for future plan-linking.
- [x] 6.5 When `plan_id` is not set, pass `nil` plan context to preserve unlinked behavior.

## 7. Tool registration and wiring

- [x] 7.1 Update `cmd/mcp-server/adapters.go` to create the `mcptools.Dependencies` struct with the new `Plan` field populated.
- [x] 7.2 Verify that `RegisterTools` conditionally registers the new tools only when `deps.Plan` is non-nil (same pattern as existing tools).
- [x] 7.3 Ensure the new tool names are unique and don't collide with existing tools (no `persist_plan`, `get_plan`, `set_plan_decision`, `list_plans`, or `record_meal_from_plan` exists yet).

## 8. Verification

- [x] 8.1 `go build ./...` succeeds — no compilation errors in `internal/mcptools` or `cmd/mcp-server`.
- [x] 8.2 `go test ./internal/mcptools/...` passes — unit tests for the new handlers (at minimum, test that invalid inputs are rejected before reaching the service layer).
- [x] 8.3 `openspec validate close-mcp-planning-loop` passes.
- [x] 8.4 `openspec status --change close-mcp-planning-loop` shows all artifacts in place.

## 9. Shopping-requirement ingredient names

- [x] 9.1 Add `Slug` to `persistence.Ingredient` and update `UpsertIngredient` to store a slug, defaulting from `Display` when absent and preserving an existing non-empty slug on conflict.
- [x] 9.2 Update `ListRecipeIngredients` and `ListAllRecipeIngredients` to join `ingredient` and expose `COALESCE(i.slug, i.display, '')` as `IngredientName`.
- [x] 9.3 Update `persistence.ListShoppingRequirements` to join `ingredient` and expose `IngredientName`.
- [x] 9.4 Update `service.Recipes.syncIngredients` to upsert ingredients with `Slug: domain.CanonicalIngredientID(line.FoodName)`.
- [x] 9.5 Add `IngredientName` to `dto.ShoppingRequirement`, `httpapi.ShoppingRequirementResponse`, and the corresponding adapter mappings.
- [x] 9.6 Update `api/openapi.yaml`, `internal/openapi/types.gen.go`, and `web/src/generated/spisordning.ts` so `ShoppingRequirement.id` is a string and `ingredient_name` is an optional string.
- [x] 9.7 Update `cmd/mcp-server/adapters.go` so recipe-derived and plan-derived shopping requirements populate `mcptools.ShoppingRequirement.Ingredient` from the canonical ingredient name.
- [x] 9.8 Update `cmd/mcp-server/adapters.go` so `create_shopping_list` resolves each item by explicit `ingredient_id`, UUID-valued `ingredient`, or deterministic canonical-name-derived ingredient ID.
- [x] 9.9 Add tests for `resolveShoppingIngredientID` and `toMCPShoppingRequirement` and re-run `go build ./...`, `go vet ./...`, `go test ./...`, and the web build.
