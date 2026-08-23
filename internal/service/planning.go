// Package service — meal planning service.
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/androidand/spisordning/internal/httpapi"
	"github.com/androidand/spisordning/internal/persistence"
)

// PlanningService is the surface the /plans handlers need.
type PlanningService interface {
	ListPlans(ctx context.Context) ([]httpapi.MealPlan, error)
	CreatePlan(ctx context.Context, weekStart string) (httpapi.MealPlan, error)
	GetPlan(ctx context.Context, id int64) (httpapi.MealPlanView, error)
	UpdatePlan(ctx context.Context, id int64, in httpapi.MealPlanUpdate) (httpapi.MealPlan, error)
	SetDecisions(ctx context.Context, planID int64, in []httpapi.MealPlanDecision) ([]httpapi.MealPlanDecision, error)
	ListShoppingRequirements(ctx context.Context, planID int64) ([]httpapi.ShoppingRequirement, error)
}

// Planning implements PlanningService.
type Planning struct{ db Store }

// NewPlanning returns a Planning service backed by db.
func NewPlanning(db Store) *Planning { return &Planning{db: db} }

func (s *Planning) ListPlans(ctx context.Context) ([]httpapi.MealPlan, error) {
	// List all plans — persistence doesn't have a ListMealPlans method yet;
	// this is a gap to fill when the endpoint is needed.
	return nil, fmt.Errorf("service: list plans: not yet implemented (needs ListMealPlans in persistence)")
}

func (s *Planning) CreatePlan(ctx context.Context, weekStart string) (httpapi.MealPlan, error) {
	weekStartTime, err := time.Parse("2006-01-02", weekStart)
	if err != nil {
		return httpapi.MealPlan{}, fmt.Errorf("service: create plan: invalid week_start %q: %w", weekStart, err)
	}
	plan, err := s.db.GetOrCreateMealPlan(ctx, weekStartTime)
	if err != nil {
		return httpapi.MealPlan{}, fmt.Errorf("service: create plan: %w", err)
	}
	return httpapi.MealPlan{
		ID: plan.ID, WeekStart: plan.WeekStart.Format("2006-01-02"),
		Status: plan.Status, CreatedAt: plan.CreatedAt,
	}, nil
}

func (s *Planning) GetPlan(ctx context.Context, id int64) (httpapi.MealPlanView, error) {
	plan, err := s.db.GetMealPlan(ctx, id)
	if err != nil {
		return httpapi.MealPlanView{}, fmt.Errorf("service: get plan: %w", err)
	}
	candidates, err := s.db.ListCandidates(ctx, id)
	if err != nil {
		return httpapi.MealPlanView{}, fmt.Errorf("service: get plan: list candidates: %w", err)
	}
	decisions, err := s.db.ListDecisions(ctx, id)
	if err != nil {
		return httpapi.MealPlanView{}, fmt.Errorf("service: get plan: list decisions: %w", err)
	}

	view := httpapi.MealPlanView{
		Plan: httpapi.MealPlan{
			ID: plan.ID, WeekStart: plan.WeekStart.Format("2006-01-02"),
			Status: plan.Status, CreatedAt: plan.CreatedAt,
		},
		Candidates: toPlanCandidates(candidates),
		Decisions:  toPlanDecisions(decisions),
	}
	return view, nil
}

func (s *Planning) UpdatePlan(ctx context.Context, id int64, in httpapi.MealPlanUpdate) (httpapi.MealPlan, error) {
	if in.Status == "" {
		return httpapi.MealPlan{}, fmt.Errorf("service: update plan: status is required")
	}
	if err := s.db.SetMealPlanStatus(ctx, id, in.Status); err != nil {
		return httpapi.MealPlan{}, fmt.Errorf("service: update plan: %w", err)
	}
	plan, err := s.db.GetMealPlan(ctx, id)
	if err != nil {
		return httpapi.MealPlan{}, fmt.Errorf("service: update plan: read back: %w", err)
	}
	return httpapi.MealPlan{
		ID: plan.ID, WeekStart: plan.WeekStart.Format("2006-01-02"),
		Status: plan.Status, CreatedAt: plan.CreatedAt,
	}, nil
}

func (s *Planning) SetDecisions(ctx context.Context, planID int64, in []httpapi.MealPlanDecision) ([]httpapi.MealPlanDecision, error) {
	for _, d := range in {
		plan, err := s.db.GetMealPlan(ctx, planID)
		if err != nil {
			return nil, fmt.Errorf("service: set decisions: plan not found: %w", err)
		}
		if plan.Status != "approved" {
			return nil, fmt.Errorf("service: set decisions: plan must be approved before setting decisions")
		}
		slotDate, err := time.Parse("2006-01-02", d.SlotDate)
		if err != nil {
			return nil, fmt.Errorf("service: set decisions: invalid slot_date %q: %w", d.SlotDate, err)
		}
		decision := persistence.MealPlanDecision{
			PlanID: planID, SlotDate: slotDate, MealieRecipeID: d.MealieRecipeID,
		}
		if err := s.db.SetDecision(ctx, decision); err != nil {
			return nil, fmt.Errorf("service: set decisions: %w", err)
		}
	}
	decisions, err := s.db.ListDecisions(ctx, planID)
	if err != nil {
		return nil, fmt.Errorf("service: set decisions: read back: %w", err)
	}
	return toPlanDecisions(decisions), nil
}

func (s *Planning) ListShoppingRequirements(ctx context.Context, planID int64) ([]httpapi.ShoppingRequirement, error) {
	reqs, err := s.db.ListShoppingRequirements(ctx, planID)
	if err != nil {
		return nil, fmt.Errorf("service: list shopping requirements: %w", err)
	}
	out := make([]httpapi.ShoppingRequirement, 0, len(reqs))
	for _, r := range reqs {
		out = append(out, httpapi.ShoppingRequirement{
			ID:              r.ID,
			IngredientID:    r.IngredientID,
			Quantity:        r.Quantity,
			Unit:            r.Unit,
			AcceptableForms: r.AcceptableForms,
			PreferredForm:   r.PreferredForm,
		})
	}
	return out, nil
}

func toPlanCandidates(cands []persistence.MealPlanCandidate) []httpapi.MealPlanCandidate {
	out := make([]httpapi.MealPlanCandidate, 0, len(cands))
	for _, c := range cands {
		out = append(out, httpapi.MealPlanCandidate{
			ID:       c.ID,
			Recipe:   httpapi.RecipeRefResponse{MealieRecipeID: c.MealieRecipeID},
			SlotDate: c.SlotDate.Format("2006-01-02"),
			Score:    c.Score,
			Breakdown: c.Breakdown,
			Feasible: c.Feasible,
			Rank:     c.Rank,
		})
	}
	return out
}

func toPlanDecisions(decisions []persistence.MealPlanDecision) []httpapi.MealPlanDecision {
	out := make([]httpapi.MealPlanDecision, 0, len(decisions))
	for _, d := range decisions {
		out = append(out, httpapi.MealPlanDecision{
			PlanID:         d.PlanID,
			SlotDate:       d.SlotDate.Format("2006-01-02"),
			MealieRecipeID: d.MealieRecipeID,
			DecidedAt:      d.DecidedAt,
		})
	}
	return out
}
