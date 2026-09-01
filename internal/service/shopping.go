// Package service — meal planning and household services.
package service

import (
	"context"
	"fmt"

	"github.com/androidand/spisordning/internal/coverage"
	"github.com/androidand/spisordning/internal/domain"
)

// CoverageService checks whether a filled shopping list satisfies the
// ingredient needs of a meal plan. It is thin: the aggregation lives in the
// pure coverage package; this type only translates between persistence rows and
// that package.
type CoverageService struct {
	db Store
}

// NewCoverageService returns a CoverageService backed by db.
func NewCoverageService(db Store) *CoverageService {
	return &CoverageService{db: db}
}

// CheckCoverage compares the items of listID against the persisted
// shopping_requirement rows of planID and returns a per-ingredient
// covered/short/missing report. Requirements are grouped by
// (ingredient_id, unit) and matched to the list's supply the same way, so no
// unit conversion is attempted. List items without an ingredient_id (checklist
// labels) are reported as not-plan-derived and neither satisfy nor reduce any
// requirement.
func (s *CoverageService) CheckCoverage(ctx context.Context, listID domain.ShoppingListID, planID domain.MealPlanID) (coverage.Report, error) {
	reqs, err := s.db.ListShoppingRequirements(ctx, planID)
	if err != nil {
		return coverage.Report{}, fmt.Errorf("service: check coverage: list requirements: %w", err)
	}
	items, err := s.db.ListShoppingListItems(ctx, listID)
	if err != nil {
		return coverage.Report{}, fmt.Errorf("service: check coverage: list items: %w", err)
	}

	requirements := make([]coverage.Requirement, 0, len(reqs))
	for _, r := range reqs {
		requirements = append(requirements, coverage.Requirement{
			Key:      coverage.Key{IngredientID: r.IngredientID.String(), Unit: r.Unit},
			Name:     r.IngredientName,
			Quantity: r.Quantity,
		})
	}

	supplies := make([]coverage.Supply, 0, len(items))
	notPlanDerived := 0
	for _, it := range items {
		if it.IngredientID == nil {
			// Only a label (e.g. a checklist line) — cannot be traced to a plan meal.
			notPlanDerived++
			continue
		}
		supplies = append(supplies, coverage.Supply{
			Key:      coverage.Key{IngredientID: it.IngredientID.String(), Unit: it.Unit},
			Quantity: it.Quantity,
		})
	}

	supplied := coverage.Aggregate(supplies)
	rep := coverage.Check(requirements, supplied)
	rep.NotPlanDerived = notPlanDerived
	return rep, nil
}
