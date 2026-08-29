package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/androidand/spisordning/internal/dto"
)

// favoritesHandler handles /recipes/{id}/favorites and /recipes/{id}/rating
// routes.
type favoritesHandler struct {
	svc dto.FavoritesService
}

func (h *favoritesHandler) listFavorites(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	out, err := h.svc.ListFavoritesForRecipe(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list favorites: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *favoritesHandler) getRating(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	out, err := h.svc.GetRecipeRating(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get recipe rating: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *favoritesHandler) setFavorite(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in dto.SetFavoriteInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	if in.PersonID == "" && in.HouseholdID == "" {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "either person_id or household_id is required"})
		return
	}
	if in.PersonID != "" && in.HouseholdID != "" {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "exactly one of person_id or household_id is required"})
		return
	}
	if err := h.svc.SetFavorite(r.Context(), id, in); err != nil {
		writeError(w, http.StatusInternalServerError, "set favorite: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *favoritesHandler) unsetFavorite(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in dto.SetFavoriteInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	if in.PersonID == "" && in.HouseholdID == "" {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "either person_id or household_id is required"})
		return
	}
	if err := h.svc.UnsetFavorite(r.Context(), id, in); err != nil {
		writeError(w, http.StatusInternalServerError, "unset favorite: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
