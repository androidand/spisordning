package httpapi

import (
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
