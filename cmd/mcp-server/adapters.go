// Package main is the composition root for the MCP server: it owns the only edge
// that may import both the persistence layer and the mcptools layer, bridging the
// two with a small adapter that keeps internal/mcptools free of persistence
// (enforced by internal/architecturetest). The adapter loads the household and
// recipe candidates from persistence and delegates planning to the application
// layer (internal/planning) — it never constructs or executes SQL itself.
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/androidand/spisordning/internal/mcptools"
	"github.com/androidand/spisordning/internal/persistence"
	"github.com/androidand/spisordning/internal/planning"
)

// mcpStoreAdapter adapts *persistence.Store to the mcptools service interfaces.
// It is the sole place that knows both the persistence row types and the
// mcptools DTOs; mcptools sees only the interfaces it defines itself.
type mcpStoreAdapter struct {
	db *persistence.Store
}

// PlanDinners loads the household and recipe candidates, then delegates to the
// application-layer planner.
func (a mcpStoreAdapter) PlanDinners(ctx context.Context, date time.Time, days int) ([]mcptools.PlannedDinner, error) {
	candidates, err := a.loadCandidates(ctx)
	if err != nil {
		return nil, fmt.Errorf("plan dinners: load candidates: %w", err)
	}
	people, prefs, err := a.loadHousehold(ctx)
	if err != nil {
		return nil, fmt.Errorf("plan dinners: load household: %w", err)
	}

	slots := planning.PlanWeek(ctx, planning.WeekConfig{
		Candidates:  candidates,
		People:      people,
		Preferences: prefs,
	}, date, days)

	out := make([]mcptools.PlannedDinner, 0, len(slots))
	for _, slot := range slots {
		out = append(out, mcptools.PlannedDinner{
			Date:   slot.Date.Format("2006-01-02"),
			Recipe: slot.Winner.Candidate.MealieRecipeID,
			Title:  slot.Winner.Candidate.Title,
			Score:  slot.Winner.Score,
		})
	}
	return out, nil
}

// RecordReaction creates a meal event for the served recipe/date and records the
// household member's reaction.
func (a mcpStoreAdapter) RecordReaction(ctx context.Context, in mcptools.RecordReactionInput) (mcptools.RecordReactionResult, error) {
	servedOn, err := time.Parse("2006-01-02", in.ServedOn)
	if err != nil {
		return mcptools.RecordReactionResult{}, fmt.Errorf("record reaction: invalid served_on %q: %w", in.ServedOn, err)
	}
	eventID, err := a.db.CreateMealEvent(ctx, in.Recipe, servedOn)
	if err != nil {
		return mcptools.RecordReactionResult{}, fmt.Errorf("record reaction: create meal event: %w", err)
	}
	if err := a.db.AddMealReaction(ctx, persistence.MealReaction{
		MealEventID: eventID,
		PersonID:    in.PersonID,
		Sentiment:   in.Sentiment,
	}); err != nil {
		return mcptools.RecordReactionResult{}, fmt.Errorf("record reaction: add reaction: %w", err)
	}
	return mcptools.RecordReactionResult{
		MealEventID: eventID,
		Recipe:      in.Recipe,
		ServedOn:    in.ServedOn,
		PersonID:    in.PersonID,
		Sentiment:   in.Sentiment,
	}, nil
}

// ShoppingRequirements loads each recipe's canonical ingredient lines and
// aggregates them into shopping requirements via the application layer.
func (a mcpStoreAdapter) ShoppingRequirements(ctx context.Context, recipeIDs []string) ([]mcptools.ShoppingRequirement, error) {
	meals := make([]planning.ChosenMeal, 0, len(recipeIDs))
	for _, id := range recipeIDs {
		lines, err := a.db.ListRecipeIngredients(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("shopping requirements: list ingredients for %q: %w", id, err)
		}
		ings := make([]domain.Ingredient, 0, len(lines))
		for _, l := range lines {
			ings = append(ings, domain.Ingredient{
				IngredientID: l.IngredientID,
				Quantity:     l.Quantity,
				Unit:         l.Unit,
			})
		}
		meals = append(meals, planning.ChosenMeal{MealieRecipeID: id, Ingredients: ings})
	}

	reqs := planning.BuildRequirements(meals)
	out := make([]mcptools.ShoppingRequirement, 0, len(reqs))
	for _, r := range reqs {
		out = append(out, mcptools.ShoppingRequirement{
			Ingredient:      r.IngredientID,
			Quantity:        r.Quantity,
			Unit:            r.Unit,
			AcceptableForms: r.AcceptableForms,
			PreferredForm:   r.PreferredForm,
		})
	}
	return out, nil
}

// loadCandidates builds the planner's candidate list from the cached recipe
// references and their canonical ingredient lines.
func (a mcpStoreAdapter) loadCandidates(ctx context.Context) ([]domain.Candidate, error) {
	refs, err := a.db.ListRecipeRefs(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Candidate, 0, len(refs))
	for _, ref := range refs {
		lines, err := a.db.ListRecipeIngredients(ctx, ref.MealieRecipeID)
		if err != nil {
			return nil, fmt.Errorf("ingredients for %q: %w", ref.MealieRecipeID, err)
		}
		ids := make([]string, 0, len(lines))
		for _, l := range lines {
			ids = append(ids, l.IngredientID)
		}
		out = append(out, domain.Candidate{
			MealieRecipeID: ref.MealieRecipeID,
			Title:          ref.Title,
			Tags:           ref.Tags,
			Ingredients:    ids,
			Effort:         domain.Effort(ref.Effort),
		})
	}
	return out, nil
}

// loadHousehold builds the planner's people and their preferences.
func (a mcpStoreAdapter) loadHousehold(ctx context.Context) ([]domain.Person, []domain.Preference, error) {
	people, err := a.db.ListPeople(ctx)
	if err != nil {
		return nil, nil, err
	}
	domainPeople := make([]domain.Person, 0, len(people))
	for _, p := range people {
		domainPeople = append(domainPeople, domain.Person{ID: p.ID, Name: p.Name, Weight: p.Weight})
	}

	var prefs []domain.Preference
	for _, p := range people {
		rows, err := a.db.ListPreferences(ctx, p.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("preferences for %q: %w", p.ID, err)
		}
		for _, pr := range rows {
			prefs = append(prefs, domain.Preference{
				PersonID:   pr.PersonID,
				Tag:        pr.Tag,
				Sentiment:  domain.Sentiment(pr.Sentiment),
				Confidence: pr.Confidence,
			})
		}
	}
	return domainPeople, prefs, nil
}
