package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/androidand/spisordning/internal/dto"
)

type mealsHandler struct {
	svc dto.MealsService
}

func (h *mealsHandler) createMealEvent(w http.ResponseWriter, r *http.Request) {
	var in dto.MealEventNew
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	if in.MealieRecipeID == "" {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "field 'mealie_recipe_id' is required"})
		return
	}
	if in.ServedOn == "" {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "field 'served_on' is required"})
		return
	}
	if _, err := parseDate(in.ServedOn); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "field 'served_on' must be a date (YYYY-MM-DD)"})
		return
	}
	for _, rx := range in.Reactions {
		if rx.Sentiment < -2 || rx.Sentiment > 2 {
			writeJSON(w, http.StatusBadRequest, errorBody{Message: "reaction sentiment must be in [-2,2]"})
			return
		}
	}
	out, err := h.svc.CreateMealEvent(r.Context(), in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create meal event: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *mealsHandler) listMeals(w http.ResponseWriter, r *http.Request) {
	mealieRecipeID := r.URL.Query().Get("mealieRecipeId")
	servedOn := r.URL.Query().Get("servedOn")
	out, err := h.svc.ListMeals(r.Context(), mealieRecipeID, servedOn)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list meals: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *mealsHandler) getMeal(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	out, err := h.svc.GetMeal(r.Context(), idStr)
	if errors.Is(err, ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorBody{Message: "meal " + idStr + " not found"})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get meal: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// parseDate validates that s is a date-only (YYYY-MM-DD) value per api/openapi.yaml
// served_on format. The adapter re-parses defensively, but input validation belongs
// here so bad shapes never reach the service layer.
func parseDate(s string) (time.Time, error) { return time.Parse("2006-01-02", s) }
