package mcptools_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/androidand/spisordning/internal/mcptools"
)

// ---- fakes ----

type fakePlanner struct {
	calls    int
	lastDate time.Time
	lastDays int
	dinners  []mcptools.PlannedSlot
	err      error
}

func (f *fakePlanner) PlanDinners(_ context.Context, date time.Time, days int) ([]mcptools.PlannedSlot, error) {
	f.calls++
	f.lastDate = date
	f.lastDays = days
	return f.dinners, f.err
}

func (f *fakePlanner) PlanSlots(_ context.Context, date time.Time, days int, _ []string) ([]mcptools.PlannedSlot, error) {
	f.calls++
	f.lastDate = date
	f.lastDays = days
	return f.dinners, f.err
}

type fakeReactions struct {
	calls         int
	last          mcptools.RecordReactionInput
	fromPlanCalls int
	lastFromPlan  mcptools.RecordMealFromPlanInput
	res           mcptools.RecordReactionResult
	fromPlanRes   mcptools.RecordReactionResult
	err           error
	fromPlanErr   error
}

func (f *fakeReactions) RecordReaction(_ context.Context, in mcptools.RecordReactionInput) (mcptools.RecordReactionResult, error) {
	f.calls++
	f.last = in
	return f.res, f.err
}

func (f *fakeReactions) RecordMealFromPlan(_ context.Context, in mcptools.RecordMealFromPlanInput) (mcptools.RecordReactionResult, error) {
	f.fromPlanCalls++
	f.lastFromPlan = in
	return f.fromPlanRes, f.fromPlanErr
}

type fakePlan struct {
	plans           []mcptools.PlanSummary
	plan            mcptools.GetPlanResult
	persist         mcptools.PersistPlanResult
	decisions       []mcptools.PlanDecisionResponse
	lastPlanID      string
	lastPersist     mcptools.PersistPlanInput
	lastDecisions   []mcptools.PlanDecisionInput
	listPlansCalls  int
	getPlanCalls    int
	persistCalls    int
	setDecisionsCalls int
	err             error
}

func (f *fakePlan) ListPlans(_ context.Context) ([]mcptools.PlanSummary, error) {
	f.listPlansCalls++
	return f.plans, f.err
}

func (f *fakePlan) GetPlan(_ context.Context, planID string) (mcptools.GetPlanResult, error) {
	f.getPlanCalls++
	f.lastPlanID = planID
	return f.plan, f.err
}

func (f *fakePlan) SetDecisions(_ context.Context, planID string, decisions []mcptools.PlanDecisionInput) ([]mcptools.PlanDecisionResponse, error) {
	f.setDecisionsCalls++
	f.lastPlanID = planID
	f.lastDecisions = decisions
	return f.decisions, f.err
}

func (f *fakePlan) PersistPlan(_ context.Context, in mcptools.PersistPlanInput) (mcptools.PersistPlanResult, error) {
	f.persistCalls++
	f.lastPersist = in
	return f.persist, f.err
}

type fakeRequirements struct {
	calls int
	last  []string
	reqs  []mcptools.ShoppingRequirement
	err   error
}

func (f *fakeRequirements) ShoppingRequirements(_ context.Context, recipeIDs []string) ([]mcptools.ShoppingRequirement, error) {
	f.calls++
	f.last = recipeIDs
	return f.reqs, f.err
}

// ---- helpers ----

// connectServer registers the tools on a fresh server and connects an in-memory
// MCP client to it, returning the client session for tool calls.
func connectServer(t *testing.T, deps mcptools.Dependencies) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0"}, nil)
	mcptools.RegisterTools(server, deps)

	ct, st := mcp.NewInMemoryTransports()
	sess, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// structured decodes a tool result's StructuredContent into T.
func structured[T any](t *testing.T, res *mcp.CallToolResult) T {
	t.Helper()
	var out T
	b, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal structured content: %v", err)
	}
	return out
}

// ---- tests ----

func TestListRecipeCandidates(t *testing.T) {
	planner := &fakePlanner{dinners: []mcptools.PlannedSlot{
		{Date: "2026-08-20", Slot: "dinner", Recipe: "r1", Title: "Pasta", Score: 0.9},
		{Date: "2026-08-21", Slot: "dinner", Recipe: "r2", Title: "Curry", Score: 0.7},
	}}
	cs := connectServer(t, mcptools.Dependencies{Planner: planner})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "list_recipe_candidates",
		Arguments: map[string]any{"date": "2026-08-20", "days": 2},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}
	got := structured[[]mcptools.PlannedSlot](t, res)
	if len(got) != 2 || got[0].Recipe != "r1" || got[1].Title != "Curry" {
		t.Fatalf("unexpected dinners: %+v", got)
	}
	if planner.calls != 1 {
		t.Fatalf("planner called %d times, want 1", planner.calls)
	}
	if !planner.lastDate.Equal(time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("planner got date %v", planner.lastDate)
	}
	if planner.lastDays != 2 {
		t.Fatalf("planner got days %d, want 2", planner.lastDays)
	}
}

func TestListRecipeCandidates_Defaults(t *testing.T) {
	planner := &fakePlanner{dinners: []mcptools.PlannedSlot{{Date: "2026-08-20", Slot: "dinner", Recipe: "r1", Title: "X", Score: 1}}}
	cs := connectServer(t, mcptools.Dependencies{Planner: planner})

	if _, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_recipe_candidates"}); err != nil {
		t.Fatalf("call: %v", err)
	}
	if planner.calls != 1 {
		t.Fatalf("planner called %d times, want 1", planner.calls)
	}
	if planner.lastDays != 1 {
		t.Fatalf("planner got days %d, want default 1", planner.lastDays)
	}
	if planner.lastDate.IsZero() {
		t.Fatal("planner got zero date, want today")
	}
}

func TestListRecipeCandidates_InvalidDate(t *testing.T) {
	planner := &fakePlanner{}
	cs := connectServer(t, mcptools.Dependencies{Planner: planner})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "list_recipe_candidates",
		Arguments: map[string]any{"date": "not-a-date"},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected tool error for invalid date")
	}
	if planner.calls != 0 {
		t.Fatalf("planner called %d times, want 0", planner.calls)
	}
}

func TestRecordMealReaction(t *testing.T) {
	reactions := &fakeReactions{res: mcptools.RecordReactionResult{
		MealEventID: "meal-42", Recipe: "r1", ServedOn: "2026-08-20", PersonID: "p1", Sentiment: 2,
	}}
	cs := connectServer(t, mcptools.Dependencies{Reactions: reactions})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "record_meal_reaction",
		Arguments: map[string]any{
			"recipe": "r1", "served_on": "2026-08-20", "person_id": "p1", "sentiment": 2,
		},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}
	got := structured[mcptools.RecordReactionResult](t, res)
	if got.MealEventID != "meal-42" || got.Sentiment != 2 {
		t.Fatalf("unexpected result: %+v", got)
	}
	if reactions.calls != 1 {
		t.Fatalf("reactions called %d times, want 1", reactions.calls)
	}
	if reactions.last.Recipe != "r1" || reactions.last.PersonID != "p1" || reactions.last.Sentiment != 2 {
		t.Fatalf("unexpected input passed to service: %+v", reactions.last)
	}
}

func TestRecordMealReaction_Validation(t *testing.T) {
	cases := []struct {
		name string
		args map[string]any
	}{
		{"missing recipe", map[string]any{"served_on": "2026-08-20", "person_id": "p1", "sentiment": 1}},
		{"missing person", map[string]any{"recipe": "r1", "served_on": "2026-08-20", "sentiment": 1}},
		{"bad date", map[string]any{"recipe": "r1", "served_on": "nope", "person_id": "p1", "sentiment": 1}},
		{"sentiment too high", map[string]any{"recipe": "r1", "served_on": "2026-08-20", "person_id": "p1", "sentiment": 9}},
		{"sentiment too low", map[string]any{"recipe": "r1", "served_on": "2026-08-20", "person_id": "p1", "sentiment": -9}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reactions := &fakeReactions{}
			cs := connectServer(t, mcptools.Dependencies{Reactions: reactions})

			res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
				Name: "record_meal_reaction", Arguments: tc.args,
			})
			// A malformed call may be rejected at the protocol level (err) or as a
			// tool error (IsError); either way the application layer must not run.
			if err == nil && !res.IsError {
				t.Fatalf("expected rejection, got success: %+v", res)
			}
			if reactions.calls != 0 {
				t.Fatalf("service called %d times, want 0", reactions.calls)
			}
		})
	}
}

func TestGetShoppingRequirements(t *testing.T) {
	reqs := &fakeRequirements{reqs: []mcptools.ShoppingRequirement{
		{Ingredient: "tomato", Quantity: 4, Unit: "pcs"},
		{Ingredient: "flour", Quantity: 1.5, Unit: "kg"},
	}}
	cs := connectServer(t, mcptools.Dependencies{Requirements: reqs})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_shopping_requirements",
		Arguments: map[string]any{"recipes": []any{"r1", "r2"}},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}
	got := structured[[]mcptools.ShoppingRequirement](t, res)
	if len(got) != 2 || got[0].Ingredient != "tomato" {
		t.Fatalf("unexpected requirements: %+v", got)
	}
	if reqs.calls != 1 {
		t.Fatalf("requirements called %d times, want 1", reqs.calls)
	}
	if len(reqs.last) != 2 || reqs.last[0] != "r1" || reqs.last[1] != "r2" {
		t.Fatalf("unexpected recipe ids passed: %v", reqs.last)
	}
}

func TestGetShoppingRequirements_Empty(t *testing.T) {
	reqs := &fakeRequirements{}
	cs := connectServer(t, mcptools.Dependencies{Requirements: reqs})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_shopping_requirements",
		Arguments: map[string]any{"recipes": []any{}},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected tool error for empty recipes")
	}
	if reqs.calls != 0 {
		t.Fatalf("requirements called %d times, want 0", reqs.calls)
	}
}

// TestToolInputTypeMismatch verifies a type-mismatched argument is rejected by
// the SDK's input validation before the application layer runs.
func TestToolInputTypeMismatch(t *testing.T) {
	reactions := &fakeReactions{}
	cs := connectServer(t, mcptools.Dependencies{Reactions: reactions})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "record_meal_reaction",
		Arguments: map[string]any{
			"recipe": "r1", "served_on": "2026-08-20", "person_id": "p1", "sentiment": "not-a-number",
		},
	})
	if err != nil {
		t.Fatalf("unexpected protocol error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected tool error for a type-mismatched argument, got %+v", res)
	}
	if reactions.calls != 0 {
		t.Fatalf("service called %d times, want 0 (input must be rejected before the application layer)", reactions.calls)
	}
}

func TestListTools(t *testing.T) {
	cs := connectServer(t, mcptools.Dependencies{
		Planner: &fakePlanner{}, Reactions: &fakeReactions{}, Requirements: &fakeRequirements{},
	})
	res, err := cs.ListTools(context.Background(), &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	names := map[string]bool{}
	for _, tool := range res.Tools {
		names[tool.Name] = true
	}
	for _, want := range []string{"list_recipe_candidates", "record_meal_reaction", "get_shopping_requirements"} {
		if !names[want] {
			t.Fatalf("missing tool %q; got %v", want, names)
		}
	}
}

func TestRegisterTools_NilServiceOmitsTool(t *testing.T) {
	cs := connectServer(t, mcptools.Dependencies{Planner: &fakePlanner{}})
	res, err := cs.ListTools(context.Background(), &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	for _, tool := range res.Tools {
		if tool.Name == "record_meal_reaction" || tool.Name == "get_shopping_requirements" ||
			tool.Name == "record_meal_from_plan" || tool.Name == "persist_plan" ||
			tool.Name == "get_plan" || tool.Name == "list_plans" || tool.Name == "set_plan_decision" {
			t.Fatalf("unexpected tool %q registered for a nil service", tool.Name)
		}
	}
}

func TestListPlans(t *testing.T) {
	plan := &fakePlan{plans: []mcptools.PlanSummary{
		{ID: "plan-1", WeekStart: "2026-08-24", Status: "approved", CreatedAt: "2026-08-20T12:00:00Z"},
	}}
	cs := connectServer(t, mcptools.Dependencies{Plan: plan})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_plans"})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}
	got := structured[[]mcptools.PlanSummary](t, res)
	if len(got) != 1 || got[0].ID != "plan-1" || got[0].Status != "approved" {
		t.Fatalf("unexpected plans: %+v", got)
	}
	if plan.listPlansCalls != 1 {
		t.Fatalf("service called %d times, want 1", plan.listPlansCalls)
	}
}

func TestGetPlan(t *testing.T) {
	plan := &fakePlan{plan: mcptools.GetPlanResult{
		Plan: mcptools.PlanSummary{ID: "plan-1", WeekStart: "2026-08-24", Status: "approved"},
		Candidates: []mcptools.PlanCandidate{
			{ID: "c1", SlotDate: "2026-08-24", SlotKind: "dinner", MealieRecipeID: "r1", Title: "Pasta", Score: 0.9, Feasible: true, Rank: 0},
		},
		Decisions: []mcptools.PlanDecisionResponse{
			{PlanID: "plan-1", SlotDate: "2026-08-24", SlotKind: "dinner", MealieRecipeID: "r1"},
		},
		ShoppingRequirements: []mcptools.ShoppingRequirement{
			{ID: "req-1", IngredientID: "tomato", Ingredient: "tomato", Quantity: 4, Unit: "pcs"},
		},
	}}
	cs := connectServer(t, mcptools.Dependencies{Plan: plan})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_plan",
		Arguments: map[string]any{"plan_id": "plan-1"},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}
	got := structured[mcptools.GetPlanResult](t, res)
	if got.Plan.ID != "plan-1" || len(got.Candidates) != 1 || len(got.Decisions) != 1 || len(got.ShoppingRequirements) != 1 {
		t.Fatalf("unexpected plan: %+v", got)
	}
	if plan.getPlanCalls != 1 || plan.lastPlanID != "plan-1" {
		t.Fatalf("service got planID %q after %d calls", plan.lastPlanID, plan.getPlanCalls)
	}
}

func TestGetPlan_RequiresPlanID(t *testing.T) {
	plan := &fakePlan{}
	cs := connectServer(t, mcptools.Dependencies{Plan: plan})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_plan"})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected tool error for missing plan_id, got %+v", res)
	}
	if plan.getPlanCalls != 0 {
		t.Fatalf("service called %d times, want 0", plan.getPlanCalls)
	}
}

func TestPersistPlan_Defaults(t *testing.T) {
	plan := &fakePlan{persist: mcptools.PersistPlanResult{
		PlanID: "plan-1", WeekStart: "2026-08-24", Days: 7, SlotCount: 7, Persisted: true,
	}}
	cs := connectServer(t, mcptools.Dependencies{Plan: plan})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "persist_plan"})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}
	got := structured[mcptools.PersistPlanResult](t, res)
	if got.PlanID != "plan-1" || !got.Persisted {
		t.Fatalf("unexpected result: %+v", got)
	}
	if plan.persistCalls != 1 {
		t.Fatalf("service called %d times, want 1", plan.persistCalls)
	}
	if plan.lastPersist.WeekStart == "" {
		t.Fatal("expected default week_start to be filled")
	}
	if plan.lastPersist.Days != 7 {
		t.Fatalf("expected default days 7, got %d", plan.lastPersist.Days)
	}
}

func TestPersistPlan_InvalidSlot(t *testing.T) {
	plan := &fakePlan{}
	cs := connectServer(t, mcptools.Dependencies{Plan: plan})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "persist_plan",
		Arguments: map[string]any{"slots": []any{"brunch"}},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected tool error for invalid slot, got %+v", res)
	}
	if plan.persistCalls != 0 {
		t.Fatalf("service called %d times, want 0", plan.persistCalls)
	}
}

func TestSetPlanDecision(t *testing.T) {
	plan := &fakePlan{decisions: []mcptools.PlanDecisionResponse{
		{PlanID: "plan-1", SlotDate: "2026-08-24", SlotKind: "dinner", MealieRecipeID: "r1"},
	}}
	cs := connectServer(t, mcptools.Dependencies{Plan: plan})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "set_plan_decision",
		Arguments: map[string]any{
			"plan_id": "plan-1",
			"decisions": []any{
				map[string]any{"slot_date": "2026-08-24", "mealie_recipe_id": "r1"},
			},
		},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}
	got := structured[[]mcptools.PlanDecisionResponse](t, res)
	if len(got) != 1 || got[0].SlotKind != "dinner" {
		t.Fatalf("unexpected decisions: %+v", got)
	}
	if plan.setDecisionsCalls != 1 || plan.lastPlanID != "plan-1" {
		t.Fatalf("service got planID %q after %d calls", plan.lastPlanID, plan.setDecisionsCalls)
	}
	if len(plan.lastDecisions) != 1 || plan.lastDecisions[0].SlotKind != "dinner" {
		t.Fatalf("expected default slot_kind dinner, got %+v", plan.lastDecisions)
	}
}

func TestSetPlanDecision_Validation(t *testing.T) {
	cases := []struct {
		name string
		args map[string]any
	}{
		{"missing plan", map[string]any{"decisions": []any{map[string]any{"slot_date": "2026-08-24", "mealie_recipe_id": "r1"}}}},
		{"empty decisions", map[string]any{"plan_id": "plan-1", "decisions": []any{}}},
		{"missing recipe", map[string]any{"plan_id": "plan-1", "decisions": []any{map[string]any{"slot_date": "2026-08-24"}}}},
		{"bad slot date", map[string]any{"plan_id": "plan-1", "decisions": []any{map[string]any{"slot_date": "nope", "mealie_recipe_id": "r1"}}}},
		{"bad slot kind", map[string]any{"plan_id": "plan-1", "decisions": []any{map[string]any{"slot_date": "2026-08-24", "mealie_recipe_id": "r1", "slot_kind": "brunch"}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan := &fakePlan{}
			cs := connectServer(t, mcptools.Dependencies{Plan: plan})

			res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
				Name: "set_plan_decision", Arguments: tc.args,
			})
			if err == nil && !res.IsError {
				t.Fatalf("expected rejection, got success: %+v", res)
			}
			if plan.setDecisionsCalls != 0 {
				t.Fatalf("service called %d times, want 0", plan.setDecisionsCalls)
			}
		})
	}
}

func TestRecordMealFromPlan(t *testing.T) {
	reactions := &fakeReactions{fromPlanRes: mcptools.RecordReactionResult{
		MealEventID: "meal-42", Recipe: "r1", ServedOn: "2026-08-20", PersonID: "p1", Sentiment: 2,
	}}
	cs := connectServer(t, mcptools.Dependencies{Reactions: reactions})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "record_meal_from_plan",
		Arguments: map[string]any{
			"recipe": "r1", "served_on": "2026-08-20", "person_id": "p1", "sentiment": 2,
			"plan_id": "plan-1", "plan_slot_date": "2026-08-20", "plan_slot_kind": "dinner",
		},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}
	got := structured[mcptools.RecordReactionResult](t, res)
	if got.MealEventID != "meal-42" || got.Sentiment != 2 {
		t.Fatalf("unexpected result: %+v", got)
	}
	if reactions.fromPlanCalls != 1 {
		t.Fatalf("service called %d times, want 1", reactions.fromPlanCalls)
	}
	if reactions.lastFromPlan.PlanID != "plan-1" || reactions.lastFromPlan.PlanSlotDate != "2026-08-20" || reactions.lastFromPlan.PlanSlotKind != "dinner" {
		t.Fatalf("unexpected plan fields: %+v", reactions.lastFromPlan)
	}
}

func TestRecordMealFromPlan_UnlinkedDefaultsSlot(t *testing.T) {
	reactions := &fakeReactions{fromPlanRes: mcptools.RecordReactionResult{MealEventID: "meal-43"}}
	cs := connectServer(t, mcptools.Dependencies{Reactions: reactions})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "record_meal_from_plan",
		Arguments: map[string]any{
			"recipe": "r1", "served_on": "2026-08-20", "person_id": "p1", "sentiment": 1,
		},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}
	if reactions.fromPlanCalls != 1 {
		t.Fatalf("service called %d times, want 1", reactions.fromPlanCalls)
	}
	if reactions.lastFromPlan.Slot != "dinner" {
		t.Fatalf("expected default slot dinner, got %q", reactions.lastFromPlan.Slot)
	}
}

func TestRecordMealFromPlan_Validation(t *testing.T) {
	cases := []struct {
		name string
		args map[string]any
	}{
		{"missing recipe", map[string]any{"served_on": "2026-08-20", "person_id": "p1", "sentiment": 1}},
		{"missing person", map[string]any{"recipe": "r1", "served_on": "2026-08-20", "sentiment": 1}},
		{"bad date", map[string]any{"recipe": "r1", "served_on": "nope", "person_id": "p1", "sentiment": 1}},
		{"sentiment too high", map[string]any{"recipe": "r1", "served_on": "2026-08-20", "person_id": "p1", "sentiment": 9}},
		{"bad slot", map[string]any{"recipe": "r1", "served_on": "2026-08-20", "person_id": "p1", "sentiment": 1, "slot": "brunch"}},
		{"plan without slot date", map[string]any{"recipe": "r1", "served_on": "2026-08-20", "person_id": "p1", "sentiment": 1, "plan_id": "plan-1"}},
		{"bad plan slot kind", map[string]any{"recipe": "r1", "served_on": "2026-08-20", "person_id": "p1", "sentiment": 1, "plan_id": "plan-1", "plan_slot_date": "2026-08-20", "plan_slot_kind": "brunch"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reactions := &fakeReactions{}
			cs := connectServer(t, mcptools.Dependencies{Reactions: reactions})

			res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
				Name: "record_meal_from_plan", Arguments: tc.args,
			})
			if err == nil && !res.IsError {
				t.Fatalf("expected rejection, got success: %+v", res)
			}
			if reactions.fromPlanCalls != 0 {
				t.Fatalf("service called %d times, want 0", reactions.fromPlanCalls)
			}
		})
	}
}
