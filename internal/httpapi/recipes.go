package httpapi

import (
	"errors"
	"net/http"

	"github.com/androidand/spisordning/internal/dto"
)

// recipesHandler handles /recipes routes.
type recipesHandler struct {
	svc dto.RecipesService
}

func (h *recipesHandler) listRecipes(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListRecipes(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list recipes: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *recipesHandler) getRecipe(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	out, err := h.svc.GetRecipe(r.Context(), id)
	if errors.Is(err, dto.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorBody{Message: "recipe " + id + " not found"})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get recipe: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}
