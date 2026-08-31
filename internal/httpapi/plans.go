package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/androidand/spisordning/internal/dto"
	"github.com/oapi-codegen/runtime/types"
)

// PlanRunInput is the request body for POST /plans/run (api/openapi.yaml PlanRunInput).
type PlanRunInput struct {
	Week           string `json:"week,omitempty"`
	Days           int    `json:"days,omitempty"`
	CreateWishlist bool   `json:"create_wishlist,omitempty"`
}

// PlanRunResult is the response body for POST /plans/run (api/openapi.yaml PlanRunResult).
type PlanRunResult struct {
	Status   string  `json:"status"`
	Message  string  `json:"message"`
	WeekStart *string `json:"week_start,omitempty"`
}

// PlanProgress is one SSE progress event for a running plan (api/openapi.yaml
// PlanProgress). The payload shape is intentionally minimal and not finalized
// until the frontend's SSE consumer needs it (task 3.4).
type PlanProgress struct {
	Phase   string    `json:"phase"`
	Message string    `json:"message"`
	At      time.Time `json:"at"`
}

// PlanService is the application surface the /plans handlers need.
// It is defined here so httpapi stays dependency-free of persistence.
// The cmd composition root supplies an implementation backed by persistence.Store.
type PlanService interface {
	ListPlans(ctx context.Context) ([]PlanResponse, error)
	CreatePlan(ctx context.Context, weekStart time.Time) (PlanResponse, error)
	GetPlan(ctx context.Context, planID string) (PlanView, error)
	UpdatePlan(ctx context.Context, planID string, status string) (PlanResponse, error)
	SetDecisions(ctx context.Context, planID string, decisions []PlanDecisionInput) error
	ListCandidates(ctx context.Context, planID string) ([]PlanCandidateResponse, error)
	InsertCandidates(ctx context.Context, candidates []PlanCandidateInput) error
	ListShoppingRequirements(ctx context.Context, planID string) ([]ShoppingRequirementResponse, error)
	RunPlan(ctx context.Context, in PlanRunInput) (PlanRunResult, error)
	// RunPlanWithProgress runs the plan and reports progress via the callback
	// as each phase completes. The SSE endpoint (POST /plans/run/stream) uses
	// this to stream progress events.
	RunPlanWithProgress(ctx context.Context, in PlanRunInput, progress func(PlanProgress)) (PlanRunResult, error)
}

// PlanResponse is the JSON view of a meal plan (openapi: components/schemas/MealPlan).
type PlanResponse struct {
	ID        string            `json:"id"`
	WeekStart types.Date        `json:"week_start"`
	Status    string            `json:"status"`
	CreatedAt time.Time         `json:"created_at"`
}

// PlanView is the JSON view of a plan with its candidates and decisions
// (openapi: components/schemas/MealPlanView).
type PlanView struct {
	Plan      PlanResponse            `json:"plan"`
	Candidates []PlanCandidateResponse `json:"candidates"`
	Decisions  *[]PlanDecisionResponse `json:"decisions,omitempty"`
}

// PlanCandidateResponse is the JSON view of a candidate (openapi: components/schemas/MealPlanCandidate).
type PlanCandidateResponse struct {
	ID        string            `json:"id"`
	SlotDate  types.Date        `json:"slot_date"`
	SlotKind  string            `json:"slot_kind"`
	Rank      int               `json:"rank"`
	Score     float64           `json:"score"`
	Breakdown map[string]float64 `json:"breakdown"`
	Feasible  bool              `json:"feasible"`
	Recipe    dto.RecipeRefResponse `json:"recipe"`
}

// PlanDecisionInput is the request body for POST /plans/:id/decisions
// (openapi: components/schemas/MealPlanDecision).
type PlanDecisionInput struct {
	SlotDate       types.Date `json:"slot_date"`
	SlotKind       string     `json:"slot_kind,omitempty"`
	MealieRecipeID string     `json:"mealie_recipe_id"`
}

// PlanDecisionResponse is the JSON view of a decision (openapi: components/schemas/MealPlanDecision).
type PlanDecisionResponse struct {
	PlanID         string     `json:"plan_id"`
	SlotDate       types.Date `json:"slot_date"`
	SlotKind       string     `json:"slot_kind,omitempty"`
	MealieRecipeID string     `json:"mealie_recipe_id"`
	DecidedAt      *time.Time `json:"decided_at,omitempty"`
}

// PlanCandidateInput is the input for inserting a ranked candidate.
type PlanCandidateInput struct {
	PlanID         string
	SlotDate       time.Time
	SlotKind       string
	MealieRecipeID string
	Score          float64
	Breakdown      map[string]float64
	Feasible       bool
	Rank           int
}

// ShoppingRequirementResponse is the JSON view of a shopping requirement
// (openapi: components/schemas/ShoppingRequirement).
type ShoppingRequirementResponse struct {
	ID              string  `json:"id"`
	IngredientID    string  `json:"ingredient_id"`
	IngredientName  string  `json:"ingredient_name,omitempty"`
	Quantity        float64 `json:"quantity"`
	Unit            string  `json:"unit"`
	AcceptableForms []string `json:"acceptable_forms"`
	PreferredForm   *string `json:"preferred_form,omitempty"`
}

// PlanUpdateInput is the request body for PATCH /plans/:id
// (openapi: components/schemas/MealPlanUpdate).
type PlanUpdateInput struct {
	Status *string `json:"status,omitempty"`
}

// ── plan handlers ────────────────────────────────────────────────────────────

type planListHandler struct {
	svc PlanService
}

func (h *planListHandler) listPlans(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListPlans(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list plans: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

type planCreateHandler struct {
	svc PlanService
}

func (h *planCreateHandler) createPlan(w http.ResponseWriter, r *http.Request) {
	var in struct {
		WeekStart *string `json:"week_start"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	var weekStart time.Time
	if in.WeekStart != nil && *in.WeekStart != "" {
		var err error
		weekStart, err = time.Parse("2006-01-02", *in.WeekStart)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid week_start (want YYYY-MM-DD)"})
			return
		}
	} else {
		// Default: next Monday.
		weekStart = nextMonday(time.Now())
	}
	out, err := h.svc.CreatePlan(r.Context(), weekStart)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create plan: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

type planGetHandler struct {
	svc PlanService
}

func (h *planGetHandler) getPlan(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("planId")
	out, err := h.svc.GetPlan(r.Context(), idStr)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errorBody{Message: "plan " + idStr + " not found"})
			return
		}
		writeError(w, http.StatusInternalServerError, "get plan: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

type planUpdateHandler struct {
	svc PlanService
}

func (h *planUpdateHandler) updatePlan(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("planId")
	var in PlanUpdateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	if in.Status == nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "status is required"})
		return
	}
	// Validate status against known values.
	status := *in.Status
	switch status {
	case "draft", "approved", "archived":
		// valid
	default:
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "status must be draft, approved, or archived"})
		return
	}
	out, err := h.svc.UpdatePlan(r.Context(), idStr, status)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errorBody{Message: "plan " + idStr + " not found"})
			return
		}
		writeError(w, http.StatusInternalServerError, "update plan: " + err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

type planDecisionsHandler struct {
	svc PlanService
}

func (h *planDecisionsHandler) setDecisions(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("planId")
	var in []PlanDecisionInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	if err := h.svc.SetDecisions(r.Context(), idStr, in); err != nil {
		writeError(w, http.StatusInternalServerError, "set decisions: "+err.Error())
		return
	}
	// Return the persisted decisions.
	out, err := h.svc.GetPlan(r.Context(), idStr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get plan after set decisions: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out.Decisions)
}

type planCandidatesHandler struct {
	svc PlanService
}

func (h *planCandidatesHandler) listCandidates(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("planId")
	out, err := h.svc.ListCandidates(r.Context(), idStr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list candidates: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

type planShoppingRequirementsHandler struct {
	svc PlanService
}

func (h *planShoppingRequirementsHandler) listShoppingRequirements(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("planId")
	out, err := h.svc.ListShoppingRequirements(r.Context(), idStr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list shopping requirements: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// planRunHandler handles POST /plans/run.
type planRunHandler struct {
	svc PlanService
}

func (h *planRunHandler) runPlan(w http.ResponseWriter, r *http.Request) {
	var in PlanRunInput
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			// Empty body is fine — use defaults.
			in = PlanRunInput{}
		}
	}
	out, err := h.svc.RunPlan(r.Context(), in)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, PlanRunResult{
			Status:  "failed",
			Message: "plan run: " + err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusAccepted, out)
}

// nextMonday returns the Monday of the week after the one containing t.
// This matches the CLI's nextISOWeek behavior (plans the week after current).
func nextMonday(t time.Time) time.Time {
	// Add 7 days to ensure we're in the next week, then find that week's Monday.
	next := t.AddDate(0, 0, 7)
	daysSinceMonday := (int(next.Weekday()) - 1 + 7) % 7
	return next.AddDate(0, 0, -daysSinceMonday)
}
