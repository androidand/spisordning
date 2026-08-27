package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/androidand/spisordning/internal/dto"
)

// fakeNutritionSvc is an in-memory IngredientsService for nutrition-by-id tests.
type fakeNutritionSvc struct {
	nutrition map[string][]dto.IngredientNutrient
	err       error
}

func (f *fakeNutritionSvc) SearchFood(ctx context.Context, query string, limit int) ([]dto.Ingredient, error) {
	return nil, nil
}

func (f *fakeNutritionSvc) LookupNutrition(ctx context.Context, nummer int) ([]dto.IngredientNutrient, error) {
	return nil, nil
}

func (f *fakeNutritionSvc) NutritionByID(ctx context.Context, id string) ([]dto.IngredientNutrient, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.nutrition[id], nil
}

func (f *fakeNutritionSvc) SearchDabas(ctx context.Context, query string) ([]dto.IngredientProduct, error) {
	return nil, nil
}

func (f *fakeNutritionSvc) SearchMatpriskollen(ctx context.Context, query string) ([]dto.IngredientProduct, error) {
	return nil, nil
}

func (f *fakeNutritionSvc) ResolveMapping(ctx context.Context, mealieFoodID string, in dto.IngredientMappingResolve) (dto.IngredientMapping, error) {
	return dto.IngredientMapping{}, nil
}

func TestGetIngredientNutrition_HappyPath(t *testing.T) {
	svc := &fakeNutritionSvc{nutrition: map[string][]dto.IngredientNutrient{
		"slv-12345": {{Name: "Energi", Value: 100, Unit: "kJ"}},
	}}
	mux := newMux(t, Dependencies{Ingredients: svc})

	rec := doGet(t, mux, "/ingredients/by-id/slv-12345/nutrition")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got []dto.IngredientNutrient
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].Name != "Energi" || got[0].Value != 100 {
		t.Fatalf("unexpected nutrients: %+v", got)
	}
}

func TestGetIngredientNutrition_Error(t *testing.T) {
	svc := &fakeNutritionSvc{err: errSentinel("boom")}
	mux := newMux(t, Dependencies{Ingredients: svc})

	rec := doGet(t, mux, "/ingredients/by-id/slv-12345/nutrition")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
