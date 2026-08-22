package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
)

// EffortProfileService is the application surface for effort_profile.
type EffortProfileService interface {
	ListEffortProfiles(ctx context.Context) ([]EffortProfileResponse, error)
	UpsertEffortProfile(ctx context.Context, in EffortProfileInput) error
}

// EffortProfileResponse is the JSON view (openapi: components/schemas/EffortProfile).
type EffortProfileResponse struct {
	Weekday       int     `json:"weekday"`
	KitchenEnergy int     `json:"kitchen_energy"`
}

// EffortProfileInput is the request body for POST /effort-profiles
// (openapi: components/schemas/EffortProfile).
type EffortProfileInput struct {
	Weekday       int `json:"weekday"`
	KitchenEnergy int `json:"kitchen_energy"`
}

// PlanningConstraintService is the application surface for planning_constraint.
type PlanningConstraintService interface {
	ListPlanningConstraints(ctx context.Context) ([]PlanningConstraintResponse, error)
	CreatePlanningConstraint(ctx context.Context, in PlanningConstraintInput) (PlanningConstraintResponse, error)
}

// PlanningConstraintResponse is the JSON view (openapi: components/schemas/PlanningConstraint).
type PlanningConstraintResponse struct {
	ID     int  `json:"id"`
	Kind   string `json:"kind"`
	Value  string `json:"value"`
	Active bool `json:"active"`
}

// PlanningConstraintInput is the request body for POST /constraints
// (openapi: components/schemas/PlanningConstraintNew).
type PlanningConstraintInput struct {
	Kind   string `json:"kind"`
	Value  string `json:"value"`
	Active bool   `json:"active"`
}

// ── effort profile handlers ─────────────────────────────────────────────────

type effortProfileListHandler struct {
	svc EffortProfileService
}

func (h *effortProfileListHandler) listEffortProfiles(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListEffortProfiles(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list effort profiles: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

type effortProfileUpsertHandler struct {
	svc EffortProfileService
}

func (h *effortProfileUpsertHandler) upsertEffortProfile(w http.ResponseWriter, r *http.Request) {
	var in EffortProfileInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	if in.KitchenEnergy < 1 || in.KitchenEnergy > 3 {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "kitchen_energy must be 1..3"})
		return
	}
	if err := h.svc.UpsertEffortProfile(r.Context(), in); err != nil {
		writeError(w, http.StatusInternalServerError, "upsert effort profile: "+err.Error())
		return
	}
	out := EffortProfileResponse{Weekday: in.Weekday, KitchenEnergy: in.KitchenEnergy}
	writeJSON(w, http.StatusCreated, out)
}

// ── planning constraint handlers ─────────────────────────────────────────────

type planningConstraintListHandler struct {
	svc PlanningConstraintService
}

func (h *planningConstraintListHandler) listConstraints(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListPlanningConstraints(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list constraints: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

type planningConstraintCreateHandler struct {
	svc PlanningConstraintService
}

func (h *planningConstraintCreateHandler) createConstraint(w http.ResponseWriter, r *http.Request) {
	var in PlanningConstraintInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	if in.Kind == "" {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "kind is required"})
		return
	}
	out, err := h.svc.CreatePlanningConstraint(r.Context(), in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create constraint: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, out)
}


