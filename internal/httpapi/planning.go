package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// MealPlan mirrors api/openapi.yaml components/schemas/MealPlan.
type MealPlan struct {
	ID        int64     `json:"id"`
	WeekStart string    `json:"week_start"` // date (YYYY-MM-DD)
	Status    string    `json:"status"`     // draft | approved | archived
	CreatedAt time.Time `json:"created_at"`
}

// MealPlanNew is the POST /plans request body (api/openapi.yaml MealPlanNew).
type MealPlanNew struct {
	WeekStart string `json:"week_start"`
}

// MealPlanUpdate is the PATCH /plans/{id} request body.
type MealPlanUpdate struct {
	Status string `json:"status"`
}

// MealPlanCandidate mirrors api/openapi.yaml components/schemas/MealPlanCandidate.
type MealPlanCandidate struct {
	ID        int64              `json:"id"`
	Recipe    RecipeRefResponse  `json:"recipe"`
	SlotDate  string             `json:"slot_date"` // date (YYYY-MM-DD)
	Score     float64            `json:"score"`
	Breakdown map[string]float64 `json:"breakdown"`
	Feasible  bool               `json:"feasible"`
	Rank      int                `json:"rank"`
}

// MealPlanDecision mirrors api/openapi.yaml components/schemas/MealPlanDecision.
type MealPlanDecision struct {
	PlanID         int64     `json:"plan_id"`
	SlotDate       string    `json:"slot_date"`
	MealieRecipeID string    `json:"mealie_recipe_id"`
	DecidedAt      time.Time `json:"decided_at,omitempty"`
}

// MealPlanView mirrors api/openapi.yaml components/schemas/MealPlanView.
type MealPlanView struct {
	Plan       MealPlan          `json:"plan"`
	Candidates []MealPlanCandidate `json:"candidates"`
	Decisions  []MealPlanDecision  `json:"decisions"`
}

// ShoppingRequirement mirrors api/openapi.yaml components/schemas/ShoppingRequirement.
type ShoppingRequirement struct {
	ID              int64     `json:"id"`
	IngredientID    string    `json:"ingredient_id"`
	Quantity        float64   `json:"quantity"`
	Unit            string    `json:"unit"`
	AcceptableForms []string  `json:"acceptable_forms"`
	PreferredForm   *string   `json:"preferred_form,omitempty"`
}

// PlanningService is the surface the /plans handlers need.
type PlanningService interface {
	ListPlans(ctx context.Context) ([]MealPlan, error)
	CreatePlan(ctx context.Context, weekStart string) (MealPlan, error)
	GetPlan(ctx context.Context, id int64) (MealPlanView, error)
	UpdatePlan(ctx context.Context, id int64, in MealPlanUpdate) (MealPlan, error)
	SetDecisions(ctx context.Context, planID int64, in []MealPlanDecision) ([]MealPlanDecision, error)
	ListShoppingRequirements(ctx context.Context, planID int64) ([]ShoppingRequirement, error)
}

// plansHandler handles /plans routes.
type plansHandler struct {
	svc PlanningService
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
	var in MealPlanNew
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
	var in MealPlanUpdate
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
	var in []MealPlanDecision
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
