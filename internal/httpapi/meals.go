package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// MealReactionResponse mirrors api/openapi.yaml components/schemas/MealReaction.
type MealReactionResponse struct {
	PersonID  string `json:"person_id"`
	Sentiment int    `json:"sentiment"` // -2..2
}

// MealEventResponse mirrors api/openapi.yaml components/schemas/MealEvent.
type MealEventResponse struct {
	ID             int64                  `json:"id"`
	MealieRecipeID string                 `json:"mealie_recipe_id"`
	ServedOn       string                 `json:"served_on"`  // date (date-only)
	CreatedAt      time.Time              `json:"created_at"` // rendered as RFC3339
	Reactions      []MealReactionResponse `json:"reactions"`
}

// MealEventNew is the POST /meals request body (api/openapi.yaml MealEventNew).
type MealEventNew struct {
	MealieRecipeID string              `json:"mealie_recipe_id"`
	ServedOn       string              `json:"served_on"` // date
	Reactions      []MealReactionInput `json:"reactions"`
}

// MealReactionInput is the request-side view of a reaction (person_id + sentiment).
type MealReactionInput struct {
	PersonID  string `json:"person_id"`
	Sentiment int    `json:"sentiment"` // -2..2
}

// MealsService is the write surface the /meals handler needs.
type MealsService interface {
	CreateMealEvent(ctx context.Context, in MealEventNew) (MealEventResponse, error)
}

type mealsHandler struct {
	svc MealsService
}

func (h *mealsHandler) createMealEvent(w http.ResponseWriter, r *http.Request) {
	var in MealEventNew
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

// parseDate validates that s is a date-only (YYYY-MM-DD) value per api/openapi.yaml
// served_on format. The adapter re-parses defensively, but input validation belongs
// here so bad shapes never reach the service layer.
func parseDate(s string) (time.Time, error) { return time.Parse("2006-01-02", s) }
