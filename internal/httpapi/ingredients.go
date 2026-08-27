package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/androidand/spisordning/internal/dto"
)

// ingredientsHandler handles /ingredients routes.
type ingredientsHandler struct {
	svc dto.IngredientsService
}

func (h *ingredientsHandler) searchFood(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	out, err := h.svc.SearchFood(r.Context(), query, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search food: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *ingredientsHandler) lookupNutrition(w http.ResponseWriter, r *http.Request) {
	nummerStr := r.PathValue("nummer")
	nummer, err := strconv.Atoi(nummerStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid slv nummer: " + nummerStr})
		return
	}
	out, err := h.svc.LookupNutrition(r.Context(), nummer)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup nutrition: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *ingredientsHandler) nutritionByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	out, err := h.svc.NutritionByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "nutrition by id: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *ingredientsHandler) searchDabas(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	out, err := h.svc.SearchDabas(r.Context(), query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search dabas: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *ingredientsHandler) searchMatpriskollen(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	out, err := h.svc.SearchMatpriskollen(r.Context(), query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search matpriskollen: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *ingredientsHandler) resolveMapping(w http.ResponseWriter, r *http.Request) {
	mealieFoodID := r.PathValue("mealieFoodId")
	var in dto.IngredientMappingResolve
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	if in.IngredientID == "" {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "field 'ingredient_id' is required"})
		return
	}
	out, err := h.svc.ResolveMapping(r.Context(), mealieFoodID, in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "resolve mapping: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}
