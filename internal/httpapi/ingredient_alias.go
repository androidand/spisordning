package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/androidand/spisordning/internal/dto"
)

// ingredientAliasHandler handles /ingredient-aliases routes (household
// nickname → canonical ingredient).
type ingredientAliasHandler struct {
	svc dto.IngredientAliasService
}

func (h *ingredientAliasHandler) listAliases(w http.ResponseWriter, r *http.Request) {
	householdID := r.URL.Query().Get("householdId")
	out, err := h.svc.List(r.Context(), householdID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list ingredient aliases: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *ingredientAliasHandler) createAlias(w http.ResponseWriter, r *http.Request) {
	var in dto.IngredientAliasNew
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	out, err := h.svc.Create(r.Context(), in)
	if err != nil {
		if errors.Is(err, dto.ErrInvalidAlias) {
			writeJSON(w, http.StatusBadRequest, errorBody{Message: err.Error()})
			return
		}
		writeError(w, http.StatusInternalServerError, "create ingredient alias: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *ingredientAliasHandler) deleteAlias(w http.ResponseWriter, r *http.Request) {
	householdID := r.URL.Query().Get("householdId")
	alias := r.PathValue("alias")
	if err := h.svc.Delete(r.Context(), householdID, alias); err != nil {
		writeError(w, http.StatusInternalServerError, "delete ingredient alias: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *ingredientAliasHandler) resolveAlias(w http.ResponseWriter, r *http.Request) {
	householdID := r.URL.Query().Get("householdId")
	alias := r.PathValue("alias")
	id, err := h.svc.Resolve(r.Context(), householdID, alias)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "resolve ingredient alias: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ingredient_id": id})
}
