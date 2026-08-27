package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/androidand/spisordning/internal/llm"
	"github.com/androidand/spisordning/internal/mealie"
	"github.com/androidand/spisordning/internal/persistence"
	"github.com/androidand/spisordning/internal/planning"
	"github.com/androidand/spisordning/internal/scoring"
)

// PlanWeekInput carries the parameters for a weekly plan run. It is shared by
// the `food-brain plan` CLI and the MCP plan_week tool so both drive the same
// orchestration. The function fields (EnergyFor, SchoolTagsFor, Reorder) are
// injected so the service stays free of any dependency on the ambient, school,
// or LLM wiring the composition root owns.
type PlanWeekInput struct {
	WeekStart time.Time
	Days      int

	People      []domain.Person
	Preferences []domain.Preference

	EnergyFor     func(date time.Time) domain.Effort
	SchoolTagsFor func(date time.Time) []string
	Reorder       func(ctx context.Context, ranked []scoring.ScoredCandidate) []scoring.ScoredCandidate

	// Olla is optional; when non-nil each planned meal gets a one-line
	// explanation.
	Olla llm.Provider
}

// PlanWeekResult carries the outcome of a weekly plan run.
type PlanWeekResult struct {
	WeekStart time.Time
	Planned   []planning.PlannedSlot

	// Explanations maps mealie_recipe_id -> one-line explanation (empty when no
	// LLM provider was supplied).
	Explanations map[string]string

	// Meals are the chosen recipes in slot order, used for search-term building.
	Meals []planning.ChosenMeal

	// Reqs are the canonical shopping requirements with household staples
	// dropped (the "what to buy" list).
	Reqs []domain.ShoppingRequirement

	// IngredientLines maps mealie_recipe_id -> the recipe's canonical ingredient
	// lines, used for ambient projection and EAN resolution.
	IngredientLines map[string]planning.ChosenMeal

	// Persisted reports whether the plan was written to the database.
	Persisted bool
	// PersistError is set when persistence failed. It is non-fatal: a
	// reachable-but-unwritable database must not fail an otherwise good plan.
	PersistError error
}

// PlanWeek orchestrates mealie (candidates), scoring, availability, and
// persistence for a meal plan. It is the shared core of the `food-brain plan`
// CLI and the MCP plan_week tool. When no database is configured (db is nil)
// the plan stays in-memory.
func (s *Planning) PlanWeek(ctx context.Context, in PlanWeekInput) (PlanWeekResult, error) {
	if s.mealie == nil {
		return PlanWeekResult{}, fmt.Errorf("service: plan week: no Mealie client configured")
	}
	if in.WeekStart.IsZero() {
		return PlanWeekResult{}, fmt.Errorf("service: plan week: week start is required")
	}
	if in.Days <= 0 {
		in.Days = 7
	}

	refs, err := s.mealie.SyncRecipes(ctx)
	if err != nil {
		return PlanWeekResult{}, fmt.Errorf("service: plan week: mealie sync: %w", err)
	}
	if len(refs) == 0 {
		return PlanWeekResult{}, fmt.Errorf("service: plan week: no recipes in Mealie — add some first")
	}
	candidates, lines := candidatesFromRefs(refs)

	planned := planning.PlanWeek(ctx, planning.WeekConfig{
		Candidates:    candidates,
		People:        in.People,
		Preferences:   in.Preferences,
		EnergyFor:     in.EnergyFor,
		SchoolTagsFor: in.SchoolTagsFor,
		Reorder:       in.Reorder,
	}, in.WeekStart, in.Days)

	explanations := map[string]string{}
	if in.Olla != nil {
		for _, slot := range planned {
			if expl, err := llm.Explain(in.Olla, ctx, slot.Winner); err == nil && expl != "" {
				explanations[slot.Winner.Candidate.MealieRecipeID] = strings.TrimSpace(expl)
			}
		}
	}

	var meals []planning.ChosenMeal
	for _, slot := range planned {
		meals = append(meals, lines[slot.Winner.Candidate.MealieRecipeID])
	}
	allReqs := planning.BuildRequirements(meals)
	// Drop pantry staples (salt, pepper, oil, …) — assumed on hand, never bought.
	reqs, _ := planning.PartitionStaples(allReqs)

	result := PlanWeekResult{
		WeekStart:       in.WeekStart,
		Planned:         planned,
		Explanations:    explanations,
		Meals:           meals,
		Reqs:            reqs,
		IngredientLines: lines,
	}

	if s.db != nil {
		if err := persistWeek(ctx, s.db, in.WeekStart, planned, reqs); err != nil {
			result.PersistError = fmt.Errorf("service: plan week: persist: %w", err)
		} else {
			result.Persisted = true
		}
	}

	return result, nil
}

// candidatesFromRefs converts Mealie references into scorer candidates and a
// lookup of each recipe's canonical ingredient lines for requirements.
func candidatesFromRefs(refs []mealie.RecipeRef) ([]domain.Candidate, map[string]planning.ChosenMeal) {
	var candidates []domain.Candidate
	lines := map[string]planning.ChosenMeal{}

	for _, ref := range refs {
		c := domain.Candidate{
			MealieRecipeID: ref.MealieRecipeID,
			Title:          ref.Title,
			Tags:           ref.Tags,
			Effort:         ref.Effort,
		}
		meal := planning.ChosenMeal{MealieRecipeID: ref.MealieRecipeID}
		for _, ing := range ref.Ingredients {
			if ing.FoodName == "" {
				continue // unmapped free-text line; ingredient_mapping review picks these up
			}
			// Canonical id: lowercase food name until the mapping table refines it.
			id := domain.CanonicalIngredientID(ing.FoodName)
			c.Ingredients = append(c.Ingredients, id)
			qty := ing.Quantity
			if qty <= 0 {
				qty = 1
			}
			unit := ing.Unit
			if unit == "" {
				unit = "st"
			}
			meal.Ingredients = append(meal.Ingredients, domain.Ingredient{
				IngredientID: id, Quantity: qty, Unit: unit,
			})
		}
		candidates = append(candidates, c)
		lines[ref.MealieRecipeID] = meal
	}
	return candidates, lines
}

// persistWeek writes a planned week to the store: one meal_plan row
// (get-or-create by week), one candidate + decision per planned slot, and one
// shopping requirement per canonical requirement.
func persistWeek(ctx context.Context, db Store, weekStart time.Time,
	planned []planning.PlannedSlot, reqs []domain.ShoppingRequirement) error {

	plan, err := db.GetOrCreateMealPlan(ctx, weekStart)
	if err != nil {
		return fmt.Errorf("persist plan: meal_plan: %w", err)
	}

	for _, slot := range planned {
		c := persistence.MealPlanCandidate{
			PlanID:         plan.ID,
			SlotDate:       slot.Date,
			MealieRecipeID: slot.Winner.Candidate.MealieRecipeID,
			Score:          slot.Winner.Score,
			Breakdown:      breakdownToMap(slot.Winner.Breakdown),
			Feasible:       slot.Winner.Feasible,
			Rank:           0, // the persisted winner is the top-scored candidate
		}
		if err := db.InsertCandidate(ctx, c); err != nil {
			return fmt.Errorf("persist plan: insert candidate for %s: %w", slot.Date.Format("2006-01-02"), err)
		}
	}

	for _, slot := range planned {
		d := persistence.MealPlanDecision{
			PlanID:         plan.ID,
			SlotDate:       slot.Date,
			MealieRecipeID: slot.Winner.Candidate.MealieRecipeID,
		}
		if err := db.SetDecision(ctx, d); err != nil {
			return fmt.Errorf("persist plan: set decision for %s: %w", slot.Date.Format("2006-01-02"), err)
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
		if err := db.InsertShoppingRequirement(ctx, rr); err != nil {
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
