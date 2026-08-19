package httpapi

import (
	"context"
	"net/http"
	"time"
)

// RecipeRefResponse mirrors api/openapi.yaml components/schemas/RecipeRef.
type RecipeRefResponse struct {
	MealieRecipeID string    `json:"mealie_recipe_id"`
	Title          string    `json:"title"`
	Tags           []string  `json:"tags"`
	Effort         int       `json:"effort"` // 1..3
	LastSyncedAt   time.Time `json:"last_synced_at"`
}

// RecipesService is the read surface the /recipes handler needs.
type RecipesService interface {
	ListRecipes(ctx context.Context) ([]RecipeRefResponse, error)
}

type recipesHandler struct {
	svc RecipesService
}

func (h *recipesHandler) listRecipes(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListRecipes(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list recipes: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}
