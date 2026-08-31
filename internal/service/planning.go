// Package service — meal planning service.
package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/androidand/spisordning/internal/dto"
	"github.com/androidand/spisordning/internal/mealie"
	"github.com/androidand/spisordning/internal/persistence"
)

// Planning implements the PlanningService interface defined in dto and the
// PlanWeek orchestration used by the `food-brain plan` CLI and the MCP
// plan_week tool.
type Planning struct {
	db        Store
	mealie    *mealie.Client
	resolver  *ResolveRecipeResolver
}

// NewPlanning returns a Planning service backed by db. mc may be nil when no
// Mealie instance is configured; PlanWeek then reports an error.
func NewPlanning(db Store, mc *mealie.Client) *Planning {
	return &Planning{db: db, mealie: mc, resolver: NewResolveRecipeResolver(db, RecipeSourceModeFromEnv())}
}

func (s *Planning) ListPlans(ctx context.Context) ([]dto.MealPlan, error) {
	plans, err := s.db.ListMealPlans(ctx)
	if err != nil {
		return nil, fmt.Errorf("service: list plans: %w", err)
	}
	out := make([]dto.MealPlan, 0, len(plans))
	for _, p := range plans {
		out = append(out, dto.MealPlan{
			ID: p.ID.String(), WeekStart: p.WeekStart.Format("2006-01-02"),
			Status: p.Status, CreatedAt: p.CreatedAt,
		})
	}
	return out, nil
}

func (s *Planning) CreatePlan(ctx context.Context, weekStart string) (dto.MealPlan, error) {
	weekStartTime, err := time.Parse("2006-01-02", weekStart)
	if err != nil {
		return dto.MealPlan{}, fmt.Errorf("service: create plan: invalid week_start %q: %w", weekStart, err)
	}
	plan, err := s.db.GetOrCreateMealPlan(ctx, weekStartTime)
	if err != nil {
		return dto.MealPlan{}, fmt.Errorf("service: create plan: %w", err)
	}
	return dto.MealPlan{
		ID: plan.ID.String(), WeekStart: plan.WeekStart.Format("2006-01-02"),
		Status: plan.Status, CreatedAt: plan.CreatedAt,
	}, nil
}

func (s *Planning) GetPlan(ctx context.Context, id string) (dto.MealPlanView, error) {
	planID, err := domain.ParseMealPlanID(id)
	if err != nil {
		return dto.MealPlanView{}, fmt.Errorf("service: get plan: %w", err)
	}
	plan, err := s.db.GetMealPlan(ctx, planID)
	if err != nil {
		return dto.MealPlanView{}, fmt.Errorf("service: get plan: %w", err)
	}
	candidates, err := s.db.ListCandidates(ctx, planID)
	if err != nil {
		return dto.MealPlanView{}, fmt.Errorf("service: get plan: list candidates: %w", err)
	}
	decisions, err := s.db.ListDecisions(ctx, planID)
	if err != nil {
		return dto.MealPlanView{}, fmt.Errorf("service: get plan: list decisions: %w", err)
	}

	candidatesDTO, err := s.toPlanCandidates(ctx, candidates)
	if err != nil {
		return dto.MealPlanView{}, err
	}
	decisionsDTO, err := s.toPlanDecisions(ctx, decisions)
	if err != nil {
		return dto.MealPlanView{}, err
	}
	view := dto.MealPlanView{
		Plan: dto.MealPlan{
			ID: plan.ID.String(), WeekStart: plan.WeekStart.Format("2006-01-02"),
			Status: plan.Status, CreatedAt: plan.CreatedAt,
		},
		Candidates: candidatesDTO,
		Decisions:  decisionsDTO,
	}
	return view, nil
}

func (s *Planning) UpdatePlan(ctx context.Context, id string, in dto.MealPlanUpdate) (dto.MealPlan, error) {
	if in.Status == "" {
		return dto.MealPlan{}, fmt.Errorf("service: update plan: status is required")
	}
	planID, err := domain.ParseMealPlanID(id)
	if err != nil {
		return dto.MealPlan{}, fmt.Errorf("service: update plan: %w", err)
	}
	if err := s.db.SetMealPlanStatus(ctx, planID, in.Status); err != nil {
		return dto.MealPlan{}, fmt.Errorf("service: update plan: %w", err)
	}
	plan, err := s.db.GetMealPlan(ctx, planID)
	if err != nil {
		return dto.MealPlan{}, fmt.Errorf("service: update plan: read back: %w", err)
	}
	return dto.MealPlan{
		ID: plan.ID.String(), WeekStart: plan.WeekStart.Format("2006-01-02"),
		Status: plan.Status, CreatedAt: plan.CreatedAt,
	}, nil
}

func (s *Planning) SetDecisions(ctx context.Context, planID string, in []dto.MealPlanDecision) ([]dto.MealPlanDecision, error) {
	parsedPlanID, err := domain.ParseMealPlanID(planID)
	if err != nil {
		return nil, fmt.Errorf("service: set decisions: %w", err)
	}
	plan, err := s.db.GetMealPlan(ctx, parsedPlanID)
	if err != nil {
		return nil, fmt.Errorf("service: set decisions: plan not found: %w", err)
	}
	if plan.Status != "approved" {
		return nil, fmt.Errorf("service: set decisions: plan must be approved before setting decisions")
	}
	for _, d := range in {
		slotDate, err := time.Parse("2006-01-02", d.SlotDate)
		if err != nil {
			return nil, fmt.Errorf("service: set decisions: invalid slot_date %q: %w", d.SlotDate, err)
		}
		kind := strings.TrimSpace(d.SlotKind)
		if kind == "" {
			kind = string(domain.SlotDinner)
		}
		switch kind {
		case string(domain.SlotDinner), string(domain.SlotBreakfast), string(domain.SlotSnack):
		default:
			return nil, fmt.Errorf("service: set decisions: invalid slot_kind %q", d.SlotKind)
		}
		ref, err := s.resolver.ResolveRecipeRef(ctx, d.MealieRecipeID)
		if err != nil {
			return nil, fmt.Errorf("service: set decisions: resolve recipe %q: %w", d.MealieRecipeID, err)
		}
		decision := persistence.MealPlanDecision{
			PlanID: parsedPlanID, SlotDate: slotDate, SlotKind: kind, RecipeRefID: ref.ID,
		}
		if err := s.db.SetDecision(ctx, decision); err != nil {
			return nil, fmt.Errorf("service: set decisions: %w", err)
		}
	}
	decisions, err := s.db.ListDecisions(ctx, parsedPlanID)
	if err != nil {
		return nil, fmt.Errorf("service: set decisions: read back: %w", err)
	}
	return s.toPlanDecisions(ctx, decisions)
}

func (s *Planning) ListShoppingRequirements(ctx context.Context, planID string) ([]dto.ShoppingRequirement, error) {
	parsedPlanID, err := domain.ParseMealPlanID(planID)
	if err != nil {
		return nil, fmt.Errorf("service: list shopping requirements: %w", err)
	}
	reqs, err := s.db.ListShoppingRequirements(ctx, parsedPlanID)
	if err != nil {
		return nil, fmt.Errorf("service: list shopping requirements: %w", err)
	}
	out := make([]dto.ShoppingRequirement, 0, len(reqs))
	for _, r := range reqs {
		out = append(out, dto.ShoppingRequirement{
			ID:              r.ID.String(),
			IngredientID:    r.IngredientID.String(),
			IngredientName:  r.IngredientName,
			Quantity:        r.Quantity,
			Unit:            r.Unit,
			AcceptableForms: r.AcceptableForms,
			PreferredForm:   r.PreferredForm,
		})
	}
	return out, nil
}

func (s *Planning) toPlanCandidates(ctx context.Context, cands []persistence.MealPlanCandidate) ([]dto.MealPlanCandidate, error) {
	out := make([]dto.MealPlanCandidate, 0, len(cands))
	for _, c := range cands {
		ref, err := s.db.GetRecipeRef(ctx, c.RecipeRefID)
		if err != nil {
			return nil, fmt.Errorf("service: get plan: resolve recipe: %w", err)
		}
		out = append(out, dto.MealPlanCandidate{
			ID:        c.ID.String(),
			Recipe:    dto.RecipeRefResponse{MealieRecipeID: ref.MealieRecipeID, Title: ref.Title, Tags: ref.Tags, Effort: ref.Effort},
			SlotDate:  c.SlotDate.Format("2006-01-02"),
			SlotKind:  c.SlotKind,
			Score:     c.Score,
			Breakdown: c.Breakdown,
			Feasible:  c.Feasible,
			Rank:      c.Rank,
		})
	}
	return out, nil
}

func (s *Planning) toPlanDecisions(ctx context.Context, decisions []persistence.MealPlanDecision) ([]dto.MealPlanDecision, error) {
	out := make([]dto.MealPlanDecision, 0, len(decisions))
	for _, d := range decisions {
		ref, err := s.db.GetRecipeRef(ctx, d.RecipeRefID)
		if err != nil {
			return nil, fmt.Errorf("service: get plan: resolve recipe: %w", err)
		}
		out = append(out, dto.MealPlanDecision{
			PlanID:         d.PlanID.String(),
			SlotDate:       d.SlotDate.Format("2006-01-02"),
			SlotKind:       d.SlotKind,
			MealieRecipeID: ref.MealieRecipeID,
			DecidedAt:      d.DecidedAt,
		})
	}
	return out, nil
}

// PlanRunResult is the outcome of an explicit plan run.
type PlanRunResult struct {
	PlanID    string
	WeekStart time.Time
	Days      int
	SlotCount int
	Persisted bool
}

// RunPlan plans a week, persists it, and approves the resulting plan so the
// persisted plan can immediately receive decisions.
func (s *Planning) RunPlan(ctx context.Context, in PlanWeekInput) (PlanRunResult, error) {
	res, err := s.PlanWeek(ctx, in)
	if err != nil {
		return PlanRunResult{}, err
	}
	if res.PersistError != nil {
		return PlanRunResult{}, res.PersistError
	}
	if !res.Persisted {
		return PlanRunResult{
			WeekStart: res.WeekStart,
			Days:      in.Days,
			SlotCount: len(res.Planned),
		}, nil
	}
	plan, err := s.db.GetOrCreateMealPlan(ctx, res.WeekStart)
	if err != nil {
		return PlanRunResult{}, fmt.Errorf("service: run plan: read plan after persist: %w", err)
	}
	if err := s.db.SetMealPlanStatus(ctx, plan.ID, "approved"); err != nil {
		return PlanRunResult{}, fmt.Errorf("service: run plan: approve plan: %w", err)
	}
	return PlanRunResult{
		PlanID:    plan.ID.String(),
		WeekStart: res.WeekStart,
		Days:      in.Days,
		SlotCount: len(res.Planned),
		Persisted: true,
	}, nil
}
