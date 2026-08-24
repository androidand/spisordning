// Package service — ingredients / nutrition service.
package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/androidand/spisordning/internal/dto"
	"github.com/androidand/spisordning/internal/ingredients"
	"github.com/androidand/spisordning/internal/persistence"
)

// Ingredients implements dto.IngredientsService.
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

func (s *Ingredients) SearchFood(ctx context.Context, query string, limit int) ([]dto.Ingredient, error) {
	if s.slv == nil {
		return nil, fmt.Errorf("service: ingredients: SLV client not configured")
	}
	page, err := s.slv.SearchFood(ctx, ingredients.SprakSwedish, limit)
	if err != nil {
		return nil, fmt.Errorf("service: search food: %w", err)
	}
	// Client-side filter: SLV API does not support name-based filtering.
	query = strings.ToLower(query)
	var out []dto.Ingredient
	for _, f := range page.Foods {
		if strings.Contains(strings.ToLower(f.Namn), query) || strings.Contains(strings.ToLower(f.VetenskapligtNamn), query) {
			out = append(out, dto.Ingredient{
				ID:        fmt.Sprintf("slv-%d", f.Nummer),
				Display:   f.Namn,
				Source:    "slv",
				SlvNummer: f.Nummer,
			})
		}
	}
	return out, nil
}

func (s *Ingredients) LookupNutrition(ctx context.Context, nummer int) ([]dto.IngredientNutrient, error) {
	if s.slv == nil {
		return nil, fmt.Errorf("service: nutrition: SLV client not configured")
	}
	nutr, err := s.slv.LookupNutrition(ctx, nummer, ingredients.SprakSwedish)
	if err != nil {
		return nil, fmt.Errorf("service: lookup nutrition: %w", err)
	}
	out := make([]dto.IngredientNutrient, 0, len(nutr))
	for _, n := range nutr {
		out = append(out, dto.IngredientNutrient{
			Name:  n.Namn,
			Value: n.Värde,
			Unit:  n.Enhet,
		})
	}
	return out, nil
}

func (s *Ingredients) SearchDabas(ctx context.Context, query string) ([]dto.IngredientProduct, error) {
	if s.dabas == nil {
		return nil, fmt.Errorf("service: search dabas: Dabas client not configured")
	}
	result, err := s.dabas.Search(ctx, query, 0)
	if err != nil {
		return nil, fmt.Errorf("service: search dabas: %w", err)
	}
	out := make([]dto.IngredientProduct, 0, len(result.Results))
	for _, p := range result.Results {
		out = append(out, dto.IngredientProduct{
			Key:       p.ArticleID,
			GTIN:      p.GTIN,
			Name:      p.ArticleName,
			Brand:     p.Brand,
			Amount:    p.Package,
			ImageURL:  p.ImageMedium,
		})
	}
	return out, nil
}

func (s *Ingredients) SearchMatpriskollen(ctx context.Context, query string) ([]dto.IngredientProduct, error) {
	if s.matpriskollen == nil {
		return nil, fmt.Errorf("service: search matpriskollen: MPK client not configured")
	}
	products, err := s.matpriskollen.Search(ctx, query, 50)
	if err != nil {
		return nil, fmt.Errorf("service: search matpriskollen: %w", err)
	}
	out := make([]dto.IngredientProduct, 0, len(products))
	for _, p := range products {
		out = append(out, dto.IngredientProduct{
			Key:         p.Key,
			GTIN:        p.GTIN,
			Name:        p.Name,
			Brand:       p.Brand,
			Description: p.Description,
			Amount:      p.Amount,
			ImageURL:    p.ThumbnailURL,
		})
	}
	return out, nil
}

func (s *Ingredients) ResolveMapping(ctx context.Context, mealieFoodID string, in dto.IngredientMappingResolve) (dto.IngredientMapping, error) {
	m := persistence.IngredientMapping{
		MealieFoodID: mealieFoodID,
		IngredientID: in.IngredientID,
		DefaultForm:  ptrToString(in.PreferredForm),
		NeedsReview:  false,
	}
	if err := s.db.UpsertIngredientMapping(ctx, m); err != nil {
		return dto.IngredientMapping{}, fmt.Errorf("service: resolve mapping: %w", err)
	}
	return dto.IngredientMapping{
		MealieFoodID: m.MealieFoodID, IngredientID: m.IngredientID,
		NeedsReview: m.NeedsReview, UpdatedAt: m.UpdatedAt,
	}, nil
}

func (s *Ingredients) GetMapping(ctx context.Context, mealieFoodID string) (dto.IngredientMapping, error) {
	m, err := s.db.GetIngredientMapping(ctx, mealieFoodID)
	if err != nil {
		return dto.IngredientMapping{}, fmt.Errorf("service: get mapping: %w", err)
	}
	return dto.IngredientMapping{
		MealieFoodID: m.MealieFoodID, IngredientID: m.IngredientID,
		NeedsReview: m.NeedsReview, UpdatedAt: m.UpdatedAt,
	}, nil
}

func ptrToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
