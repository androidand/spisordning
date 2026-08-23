// Package service — ingredients / nutrition service.
package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/androidand/spisordning/internal/httpapi"
	"github.com/androidand/spisordning/internal/ingredients"
	"github.com/androidand/spisordning/internal/persistence"
)

// IngredientsService is the surface the /ingredients handlers need.
type IngredientsService interface {
	SearchFood(ctx context.Context, query string, limit int) ([]ingredients.Food, error)
	LookupNutrition(ctx context.Context, nummer int) ([]ingredients.Nutrient, error)
	SearchDabas(ctx context.Context, query string) ([]ingredients.DabasProduct, error)
	SearchMatpriskollen(ctx context.Context, query string) ([]ingredients.MPKProduct, error)
	ResolveMapping(ctx context.Context, mealieFoodID string, in httpapi.IngredientMappingResolve) (httpapi.IngredientMapping, error)
	GetMapping(ctx context.Context, ingredientID string) (httpapi.IngredientMapping, error)
}

// Ingredients implements IngredientsService.
type Ingredients struct {
	db           Store
	slv          *ingredients.Client
	dabas        *ingredients.DabasClient
	matpriskollen *ingredients.MPKClient
}

// NewIngredients returns an Ingredients service. Pass nil for any client that
// is not configured (e.g. no SLV_BASE_URL → nil).
func NewIngredients(db Store, slv *ingredients.Client, dabas *ingredients.DabasClient, mpk *ingredients.MPKClient) *Ingredients {
	return &Ingredients{db: db, slv: slv, dabas: dabas, matpriskollen: mpk}
}

func (s *Ingredients) SearchFood(ctx context.Context, query string, limit int) ([]ingredients.Food, error) {
	if s.slv == nil {
		return nil, fmt.Errorf("service: ingredients: SLV client not configured")
	}
	page, err := s.slv.SearchFood(ctx, ingredients.SprakSwedish, limit)
	if err != nil {
		return nil, fmt.Errorf("service: search food: %w", err)
	}
	// Client-side filter: SLV API does not support name-based filtering.
	query = strings.ToLower(query)
	var out []ingredients.Food
	for _, f := range page.Foods {
		if strings.Contains(strings.ToLower(f.Namn), query) || strings.Contains(strings.ToLower(f.VetenskapligtNamn), query) {
			out = append(out, f)
		}
	}
	return out, nil
}

func (s *Ingredients) LookupNutrition(ctx context.Context, nummer int) ([]ingredients.Nutrient, error) {
	if s.slv == nil {
		return nil, fmt.Errorf("service: nutrition: SLV client not configured")
	}
	nutr, err := s.slv.LookupNutrition(ctx, nummer, ingredients.SprakSwedish)
	if err != nil {
		return nil, fmt.Errorf("service: lookup nutrition: %w", err)
	}
	return nutr, nil
}

func (s *Ingredients) SearchDabas(ctx context.Context, query string) ([]ingredients.DabasProduct, error) {
	if s.dabas == nil {
		return nil, fmt.Errorf("service: search dabas: Dabas client not configured")
	}
	result, err := s.dabas.Search(ctx, query, 0)
	if err != nil {
		return nil, fmt.Errorf("service: search dabas: %w", err)
	}
	return result.Results, nil
}

func (s *Ingredients) SearchMatpriskollen(ctx context.Context, query string) ([]ingredients.MPKProduct, error) {
	if s.matpriskollen == nil {
		return nil, fmt.Errorf("service: search matpriskollen: MPK client not configured")
	}
	products, err := s.matpriskollen.Search(ctx, query, 50)
	if err != nil {
		return nil, fmt.Errorf("service: search matpriskollen: %w", err)
	}
	return products, nil
}

func (s *Ingredients) ResolveMapping(ctx context.Context, mealieFoodID string, in httpapi.IngredientMappingResolve) (httpapi.IngredientMapping, error) {
	m := persistence.IngredientMapping{
		MealieFoodID: mealieFoodID,
		IngredientID: in.IngredientID,
		DefaultForm:  ptrToString(in.PreferredForm),
		NeedsReview:  false,
	}
	if err := s.db.UpsertIngredientMapping(ctx, m); err != nil {
		return httpapi.IngredientMapping{}, fmt.Errorf("service: resolve mapping: %w", err)
	}
	return httpapi.IngredientMapping{
		MealieFoodID: m.MealieFoodID, IngredientID: m.IngredientID,
		NeedsReview: m.NeedsReview, UpdatedAt: m.UpdatedAt,
	}, nil
}

func (s *Ingredients) GetMapping(ctx context.Context, ingredientID string) (httpapi.IngredientMapping, error) {
	// ingredientID here is the lowercase canonical id used in the path.
	// We need to look up by mealie_food_id; the path param is actually the
	// ingredient id per openapi.yaml. This is a gap — the current persistence
	// only supports lookup by mealie_food_id. For now return not-found.
	return httpapi.IngredientMapping{}, fmt.Errorf("service: get mapping: not yet implemented (needs lookup by ingredient_id)")
}

func ptrToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
