package mcptools

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ---- Meal-plan tool output ----

// PlanSummary is one meal plan row.
type PlanSummary struct {
	ID        string `json:"id"`
	WeekStart string `json:"week_start"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// PlanCandidate is one planned candidate for a plan slot.
type PlanCandidate struct {
	ID             string             `json:"id"`
	SlotDate       string             `json:"slot_date"`
	SlotKind       string             `json:"slot_kind"`
	MealieRecipeID string             `json:"mealie_recipe_id"`
	Title          string             `json:"title"`
	Score          float64            `json:"score"`
	Breakdown      map[string]float64 `json:"breakdown,omitempty"`
	Feasible       bool               `json:"feasible"`
	Rank           int                `json:"rank"`
}

// PlanDecisionResponse is one persisted decision for a plan slot.
type PlanDecisionResponse struct {
	PlanID         string  `json:"plan_id"`
	SlotDate       string  `json:"slot_date"`
	SlotKind       string  `json:"slot_kind"`
	MealieRecipeID string  `json:"mealie_recipe_id"`
	DecidedAt      *string `json:"decided_at,omitempty"`
}

// GetPlanResult is the structured output of the get_plan tool.
type GetPlanResult struct {
	Plan               PlanSummary           `json:"plan"`
	Candidates         []PlanCandidate       `json:"candidates"`
	Decisions          []PlanDecisionResponse `json:"decisions"`
	ShoppingRequirements []ShoppingRequirement `json:"shopping_requirements"`
}

// PersistPlanResult is the structured output of the persist_plan tool.
type PersistPlanResult struct {
	PlanID    string `json:"plan_id"`
	WeekStart string `json:"week_start"`
	Days      int    `json:"days"`
	SlotCount int    `json:"slot_count"`
	Persisted bool   `json:"persisted"`
}

// ---- Meal-plan tool inputs ----

// GetPlanInput is the input for the get_plan tool.
type GetPlanInput struct {
	PlanID string `json:"plan_id"`
}

// ListPlansInput is the input for the list_plans tool.
type ListPlansInput struct{}

// PersistPlanInput is the input for the persist_plan tool.
type PersistPlanInput struct {
	// WeekStart is the Monday of the week to plan, YYYY-MM-DD. Empty means next Monday.
	WeekStart string `json:"week_start,omitempty"`
	// Days is how many days to plan starting at WeekStart. <=0 means 7.
	Days int `json:"days,omitempty"`
	// Slots is the set of slot kinds to plan. Empty means dinner only.
	// Valid values: "dinner", "breakfast", "snack".
	Slots []string `json:"slots,omitempty"`
}

// SetPlanDecisionInput is the input for the set_plan_decision tool.
type SetPlanDecisionInput struct {
	PlanID    string              `json:"plan_id"`
	Decisions []PlanDecisionInput `json:"decisions"`
}

// PlanDecisionInput is one slot decision inside SetPlanDecisionInput.
type PlanDecisionInput struct {
	// SlotDate is the date of the slot, YYYY-MM-DD.
	SlotDate string `json:"slot_date"`
	// SlotKind is the slot kind. Defaults to "dinner" when omitted.
	SlotKind string `json:"slot_kind,omitempty"`
	// MealieRecipeID is the Mealie recipe id chosen for the slot.
	MealieRecipeID string `json:"mealie_recipe_id"`
}

// RecordMealFromPlanInput is the input for the record_meal_from_plan tool.
type RecordMealFromPlanInput struct {
	// Recipe is the Mealie recipe id that was served.
	Recipe string `json:"recipe"`
	// ServedOn is the date the meal was served, YYYY-MM-DD.
	ServedOn string `json:"served_on"`
	// PersonID is the household member who reacted.
	PersonID string `json:"person_id"`
	// Sentiment is -2 (hates) .. 2 (loves).
	Sentiment int `json:"sentiment"`
	// Slot is the slot kind the meal belongs to. Defaults to "dinner" when omitted.
	Slot string `json:"slot,omitempty"`
	// PlanID optionally links the meal event to a meal plan.
	PlanID string `json:"plan_id,omitempty"`
	// PlanSlotDate is the plan slot date the meal event belongs to. Required when PlanID is set.
	PlanSlotDate string `json:"plan_slot_date,omitempty"`
	// PlanSlotKind is the plan slot kind. Defaults to Slot when omitted.
	PlanSlotKind string `json:"plan_slot_kind,omitempty"`
}

// ---- Meal-plan service interface ----

// PlanService is the application surface the meal-plan tools call. The
// composition root implements it against the planning service and persistence.
type PlanService interface {
	ListPlans(ctx context.Context) ([]PlanSummary, error)
	GetPlan(ctx context.Context, planID string) (GetPlanResult, error)
	SetDecisions(ctx context.Context, planID string, decisions []PlanDecisionInput) ([]PlanDecisionResponse, error)
	PersistPlan(ctx context.Context, in PersistPlanInput) (PersistPlanResult, error)
}

// ---- Meal-plan tool handlers ----

func listPlansHandler(p PlanService) mcp.ToolHandlerFor[ListPlansInput, []PlanSummary] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ ListPlansInput) (*mcp.CallToolResult, []PlanSummary, error) {
		plans, err := p.ListPlans(ctx)
		if err != nil {
			return nil, nil, err
		}
		return nil, plans, nil
	}
}

func getPlanHandler(p PlanService) mcp.ToolHandlerFor[GetPlanInput, GetPlanResult] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in GetPlanInput) (*mcp.CallToolResult, GetPlanResult, error) {
		if in.PlanID == "" {
			return nil, GetPlanResult{}, fmt.Errorf("get_plan: plan_id is required")
		}
		out, err := p.GetPlan(ctx, in.PlanID)
		if err != nil {
			return nil, GetPlanResult{}, err
		}
		return nil, out, nil
	}
}

func persistPlanHandler(p PlanService) mcp.ToolHandlerFor[PersistPlanInput, PersistPlanResult] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in PersistPlanInput) (*mcp.CallToolResult, PersistPlanResult, error) {
		weekStart := in.WeekStart
		if weekStart == "" {
			weekStart = nextMonday(time.Now()).Format("2006-01-02")
		}
		if _, err := time.Parse("2006-01-02", weekStart); err != nil {
			return nil, PersistPlanResult{}, fmt.Errorf("persist_plan: invalid week_start %q: want YYYY-MM-DD", in.WeekStart)
		}
		days := in.Days
		if days <= 0 {
			days = 7
		}
		if days > 31 {
			return nil, PersistPlanResult{}, fmt.Errorf("persist_plan: days must be <= 31, got %d", days)
		}
		for _, slot := range in.Slots {
			if !isValidSlotKind(slot) {
				return nil, PersistPlanResult{}, fmt.Errorf("persist_plan: invalid slot %q: want dinner, breakfast, or snack", slot)
			}
		}
		out, err := p.PersistPlan(ctx, PersistPlanInput{WeekStart: weekStart, Days: days, Slots: in.Slots})
		if err != nil {
			return nil, PersistPlanResult{}, err
		}
		return nil, out, nil
	}
}

func setPlanDecisionHandler(p PlanService) mcp.ToolHandlerFor[SetPlanDecisionInput, []PlanDecisionResponse] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in SetPlanDecisionInput) (*mcp.CallToolResult, []PlanDecisionResponse, error) {
		if in.PlanID == "" {
			return nil, nil, fmt.Errorf("set_plan_decision: plan_id is required")
		}
		if len(in.Decisions) == 0 {
			return nil, nil, fmt.Errorf("set_plan_decision: at least one decision is required")
		}
		for i := range in.Decisions {
			d := &in.Decisions[i]
			if d.MealieRecipeID == "" {
				return nil, nil, fmt.Errorf("set_plan_decision: decisions[%d].mealie_recipe_id is required", i)
			}
			if _, err := time.Parse("2006-01-02", d.SlotDate); err != nil {
				return nil, nil, fmt.Errorf("set_plan_decision: decisions[%d].slot_date: %w", i, err)
			}
			if d.SlotKind == "" {
				d.SlotKind = "dinner"
			} else if !isValidSlotKind(d.SlotKind) {
				return nil, nil, fmt.Errorf("set_plan_decision: decisions[%d].slot_kind %q: want dinner, breakfast, or snack", i, d.SlotKind)
			}
		}
		out, err := p.SetDecisions(ctx, in.PlanID, in.Decisions)
		if err != nil {
			return nil, nil, err
		}
		return nil, out, nil
	}
}

func recordMealFromPlanHandler(r MealReactionService) mcp.ToolHandlerFor[RecordMealFromPlanInput, RecordReactionResult] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in RecordMealFromPlanInput) (*mcp.CallToolResult, RecordReactionResult, error) {
		if in.Recipe == "" {
			return nil, RecordReactionResult{}, fmt.Errorf("record_meal_from_plan: recipe is required")
		}
		if in.PersonID == "" {
			return nil, RecordReactionResult{}, fmt.Errorf("record_meal_from_plan: person_id is required")
		}
		if _, err := parseRequiredDate(in.ServedOn); err != nil {
			return nil, RecordReactionResult{}, fmt.Errorf("record_meal_from_plan: %w", err)
		}
		if in.Sentiment < -2 || in.Sentiment > 2 {
			return nil, RecordReactionResult{}, fmt.Errorf("record_meal_from_plan: sentiment must be in [-2, 2], got %d", in.Sentiment)
		}
		if in.Slot == "" {
			in.Slot = "dinner"
		} else if !isValidSlotKind(in.Slot) {
			return nil, RecordReactionResult{}, fmt.Errorf("record_meal_from_plan: invalid slot %q: want dinner, breakfast, or snack", in.Slot)
		}
		if in.PlanID != "" {
			if _, err := parseRequiredDate(in.PlanSlotDate); err != nil {
				return nil, RecordReactionResult{}, fmt.Errorf("record_meal_from_plan: %w", err)
			}
			if in.PlanSlotKind != "" && !isValidSlotKind(in.PlanSlotKind) {
				return nil, RecordReactionResult{}, fmt.Errorf("record_meal_from_plan: invalid plan_slot_kind %q: want dinner, breakfast, or snack", in.PlanSlotKind)
			}
		}
		res, err := r.RecordMealFromPlan(ctx, in)
		if err != nil {
			return nil, RecordReactionResult{}, err
		}
		return nil, res, nil
	}
}

// ---- helpers ----

func parseRequiredDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("date is required")
	}
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q: want YYYY-MM-DD", s)
	}
	return d, nil
}

func isValidSlotKind(s string) bool {
	switch s {
	case "dinner", "breakfast", "snack":
		return true
	default:
		return false
	}
}

// nextMonday returns the Monday of the week after the one containing t.
func nextMonday(t time.Time) time.Time {
	next := t.AddDate(0, 0, 7)
	daysSinceMonday := (int(next.Weekday()) - 1 + 7) % 7
	return next.AddDate(0, 0, -daysSinceMonday)
}
