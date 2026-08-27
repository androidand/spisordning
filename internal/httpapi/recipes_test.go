package httpapi

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/androidand/spisordning/internal/dto"
)

// fakeRecipesSvc is an in-memory RecipesService for handler unit tests.
type fakeRecipesSvc struct {
	recipes []dto.RecipeRefResponse
	err     error
}

func (f *fakeRecipesSvc) ListRecipes(ctx context.Context) ([]dto.RecipeRefResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.recipes, nil
}

func (f *fakeRecipesSvc) GetRecipe(ctx context.Context, id string) (dto.RecipeRefResponse, error) {
	if f.err != nil {
		return dto.RecipeRefResponse{}, f.err
	}
	for _, r := range f.recipes {
		if r.MealieRecipeID == id {
			return r, nil
		}
	}
	return dto.RecipeRefResponse{}, dto.ErrNotFound
}

func TestGetRecipe_HappyPath(t *testing.T) {
	svc := &fakeRecipesSvc{recipes: []dto.RecipeRefResponse{
		{MealieRecipeID: "r1", Title: "Pasta", Tags: []string{"italian"}, Effort: 1, LastSyncedAt: time.Now()},
	}}
	mux := newMux(t, Dependencies{Recipes: svc})

	rec := doGet(t, mux, "/recipes/r1")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got dto.RecipeRefResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.Title != "Pasta" || got.Effort != 1 {
		t.Fatalf("unexpected recipe: %+v", got)
	}
}

func TestGetRecipe_NotFound(t *testing.T) {
	mux := newMux(t, Dependencies{Recipes: &fakeRecipesSvc{}})

	rec := doGet(t, mux, "/recipes/missing")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	var body errorBody
	mustJSON(t, rec.Body.Bytes(), &body)
	if body.Message == "" {
		t.Fatal("expected error message in 404 body")
	}
}

func TestGetRecipe_Error(t *testing.T) {
	svc := &fakeRecipesSvc{err: errSentinel("boom")}
	mux := newMux(t, Dependencies{Recipes: svc})

	rec := doGet(t, mux, "/recipes/r1")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
