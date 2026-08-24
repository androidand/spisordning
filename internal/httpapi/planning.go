package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/androidand/spisordning/internal/dto"
)

// plansHandler handles /plans routes.
type plansHandler struct {
	svc dto.PlanningService
}

func (h *plansHandler) listPlans(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListPlans(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list plans: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *plansHandler) createPlan(w http.ResponseWriter, r *http.Request) {
	var in dto.MealPlanNew
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	if in.WeekStart == "" {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "field 'week_start' is required"})
		return
	}
	if _, err := parseDate(in.WeekStart); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "field 'week_start' must be a date (YYYY-MM-DD)"})
		return
	}
	out, err := h.svc.CreatePlan(r.Context(), in.WeekStart)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create plan: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *plansHandler) getPlan(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid plan id: " + idStr})
		return
	}
	out, err := h.svc.GetPlan(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get plan: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *plansHandler) updatePlan(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid plan id: " + idStr})
		return
	}
	var in dto.MealPlanUpdate
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	out, err := h.svc.UpdatePlan(r.Context(), id, in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "update plan: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *plansHandler) setDecisions(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid plan id: " + idStr})
		return
	}
	var in []dto.MealPlanDecision
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	out, err := h.svc.SetDecisions(r.Context(), id, in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "set decisions: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *plansHandler) listShoppingRequirements(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid plan id: " + idStr})
		return
	}
	out, err := h.svc.ListShoppingRequirements(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list shopping requirements: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}
