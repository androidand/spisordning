// Package main is the composition root. This file wires the in-memory planning
// pipeline (internal/planning) to Postgres persistence (internal/persistence)
// once a database is available (task 2.3): the scored week — its candidates,
// winning decisions and canonical shopping requirements — is anchored to a
// meal_plan row so later runs can read prior meals/reactions for the
// repetition penalty and so the plan survives the process.
//
// When no database is configured (POSTGRES_PASSWORD/DATABASE_URL unset) the
// pipeline stays in-memory exactly as before; this keeps `food-brain plan`
// runnable without Postgres and keeps plan_test.go green in CI's no-DB job.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/androidand/spisordning/internal/persistence"
	"github.com/androidand/spisordning/internal/planning"
	"github.com/androidand/spisordning/internal/scoring"
)

// planStore is the persistence edge runPlan writes the plan through (task 2.3).
// It is satisfied by *persistence.Store; defining it here (in the composition
// root) lets the wiring be unit-tested with a fake and keeps plan.go's plan
// loop decoupled from the concrete persistence API.
type planStore interface {
	GetOrCreateMealPlan(ctx context.Context, weekStart time.Time) (persistence.MealPlan, error)
	InsertCandidate(ctx context.Context, c persistence.MealPlanCandidate) error
	SetDecision(ctx context.Context, d persistence.MealPlanDecision) error
	InsertShoppingRequirement(ctx context.Context, r persistence.ShoppingRequirement) error
}

// openStore opens the Postgres store from the environment when one is configured.
// Returns (nil, nil) when no database is configured (in-memory path), and a
// non-nil error only when a database IS configured but unreachable.
func openStore(ctx context.Context) (*persistence.Store, error) {
	cfg, err := persistence.FromEnv(os.Getenv)
	if err != nil {
		return nil, nil // no database configured -> stay in-memory
	}
	store, err := persistence.New(ctx, cfg)
	if err != nil {
		return nil, err // configured but unreachable
	}
	return store, nil
}

// persistPlan writes a planned week to Postgres: one meal_plan row (get-or-create
// by week), one candidate + decision per planned slot, and one shopping
// requirement per canonical requirement.
func persistPlan(ctx context.Context, store planStore, weekStart time.Time,
	planned []planning.PlannedSlot, reqs []domain.ShoppingRequirement) error {

	plan, err := store.GetOrCreateMealPlan(ctx, weekStart)
	if err != nil {
		return fmt.Errorf("persist plan: meal_plan: %w", err)
	}

	for _, s := range planned {
		c := persistence.MealPlanCandidate{
			PlanID:         plan.ID,
			SlotDate:       s.Date,
			MealieRecipeID: s.Winner.Candidate.MealieRecipeID,
			Score:          s.Winner.Score,
			Breakdown:      breakdownToMap(s.Winner.Breakdown),
			Feasible:       s.Winner.Feasible,
			Rank:           0, // the persisted winner is the top-scored candidate
		}
		if err := store.InsertCandidate(ctx, c); err != nil {
			return fmt.Errorf("persist plan: insert candidate for %s: %w", s.Date.Format("2006-01-02"), err)
		}
	}

	for _, s := range planned {
		d := persistence.MealPlanDecision{
			PlanID:         plan.ID,
			SlotDate:       s.Date,
			MealieRecipeID: s.Winner.Candidate.MealieRecipeID,
		}
		if err := store.SetDecision(ctx, d); err != nil {
			return fmt.Errorf("persist plan: set decision for %s: %w", s.Date.Format("2006-01-02"), err)
		}
	}

	for _, r := range reqs {
		rr := persistence.ShoppingRequirement{
			PlanID:          plan.ID,
			IngredientID:    r.IngredientID,
			Quantity:        r.Quantity,
			Unit:            r.Unit,
			AcceptableForms: r.AcceptableForms,
			PreferredForm:   strPtr(r.PreferredForm),
		}
		if err := store.InsertShoppingRequirement(ctx, rr); err != nil {
			return fmt.Errorf("persist plan: shopping requirement %s: %w", r.IngredientID, err)
		}
	}

	return nil
}

// breakdownToMap renders a scoring.Breakdown (named struct) as the JSONB map the
// schema's meal_plan_candidate.breakdown column expects.
func breakdownToMap(b scoring.Breakdown) map[string]float64 {
	return map[string]float64{
		"preference":  b.Preference,
		"effort":      b.Effort,
		"repetition":  b.Repetition,
		"schoolDedup": b.SchoolDedup,
		"campaign":    b.Campaign,
		"familiarity": b.Familiarity,
	}
}

// strPtr returns &s for non-empty strings, nil otherwise — matches the
// nullable preferred_form column.
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
