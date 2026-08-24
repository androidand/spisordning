package dto

import (
	"context"
	"time"
)

// MealPlan mirrors api/openapi.yaml components/schemas/MealPlan.
type MealPlan struct {
	ID        int64     `json:"id"`
	WeekStart string    `json:"week_start"` // date (YYYY-MM-DD)
	Status    string    `json:"status"`     // draft | approved | archived
	CreatedAt time.Time `json:"created_at"`
}

// MealPlanNew is the POST /plans request body (api/openapi.yaml MealPlanNew).
type MealPlanNew struct {
	WeekStart string `json:"week_start"`
}

// MealPlanUpdate is the PATCH /plans/{id} request body.
type MealPlanUpdate struct {
	Status string `json:"status"`
}

// MealPlanCandidate mirrors api/openapi.yaml components/schemas/MealPlanCandidate.
type MealPlanCandidate struct {
	ID        int64              `json:"id"`
	Recipe    RecipeRefResponse  `json:"recipe"`
	SlotDate  string             `json:"slot_date"` // date (YYYY-MM-DD)
	Score     float64            `json:"score"`
	Breakdown map[string]float64 `json:"breakdown"`
	Feasible  bool               `json:"feasible"`
	Rank      int                `json:"rank"`
}

// MealPlanDecision mirrors api/openapi.yaml components/schemas/MealPlanDecision.
type MealPlanDecision struct {
	PlanID         int64     `json:"plan_id"`
	SlotDate       string    `json:"slot_date"`
	MealieRecipeID string    `json:"mealie_recipe_id"`
	DecidedAt      time.Time `json:"decided_at,omitempty"`
}

// MealPlanView mirrors api/openapi.yaml components/schemas/MealPlanView.
type MealPlanView struct {
	Plan       MealPlan            `json:"plan"`
	Candidates []MealPlanCandidate `json:"candidates"`
	Decisions  []MealPlanDecision  `json:"decisions"`
}

// ShoppingRequirement mirrors api/openapi.yaml components/schemas/ShoppingRequirement.
type ShoppingRequirement struct {
	ID              int64    `json:"id"`
	IngredientID    string   `json:"ingredient_id"`
	Quantity        float64  `json:"quantity"`
	Unit            string   `json:"unit"`
	AcceptableForms []string `json:"acceptable_forms"`
	PreferredForm   *string  `json:"preferred_form,omitempty"`
}

// PlanningService is the surface the /plans handlers need.
type PlanningService interface {
	ListPlans(ctx context.Context) ([]MealPlan, error)
	CreatePlan(ctx context.Context, weekStart string) (MealPlan, error)
	GetPlan(ctx context.Context, id int64) (MealPlanView, error)
	UpdatePlan(ctx context.Context, id int64, in MealPlanUpdate) (MealPlan, error)
	SetDecisions(ctx context.Context, planID int64, in []MealPlanDecision) ([]MealPlanDecision, error)
	ListShoppingRequirements(ctx context.Context, planID int64) ([]ShoppingRequirement, error)
}
