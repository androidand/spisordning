package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/oapi-codegen/runtime/types"
)

// ---- Plan handler unit tests ----

func TestListPlans_HappyPath(t *testing.T) {
	svc := &fakePlansSvc{
		plans: []PlanResponse{{ID: "plan-1", WeekStart: types.Date{Time: time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)}, Status: "draft"}},
	}
	mux := newMux(t, Dependencies{Plans: svc})
	rec := doGet(t, mux, "/plans")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got []PlanResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].ID != "plan-1" {
		t.Fatalf("unexpected plans: %+v", got)
	}
}

func TestListPlans_ServiceError(t *testing.T) {
	svc := &fakePlansSvc{planErr: errors.New("db error")}
	mux := newMux(t, Dependencies{Plans: svc})
	rec := doGet(t, mux, "/plans")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestCreatePlan_HappyPath(t *testing.T) {
	svc := &fakePlansSvc{}
	mux := newMux(t, Dependencies{Plans: svc})
	body, _ := json.Marshal(map[string]string{"week_start": "2026-09-01"})
	rec := doPost(t, mux, "/plans", string(body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body)
	}
	var got PlanResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.ID != "plan-1" || got.Status != "draft" {
		t.Fatalf("unexpected plan: %+v", got)
	}
}

func TestCreatePlan_InvalidWeekStart(t *testing.T) {
	svc := &fakePlansSvc{}
	mux := newMux(t, Dependencies{Plans: svc})
	body := `{"week_start": "not-a-date"}`
	rec := doPost(t, mux, "/plans", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestGetPlan_HappyPath(t *testing.T) {
	svc := &fakePlansSvc{
		planView: PlanView{
			Plan: PlanResponse{ID: "plan-1", WeekStart: types.Date{Time: time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)}, Status: "draft"},
			Candidates: []PlanCandidateResponse{
				{ID: "cand-1", SlotDate: types.Date{Time: time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)}, Rank: 1, Score: 0.9, Feasible: true},
			},
		},
	}
	mux := newMux(t, Dependencies{Plans: svc})
	rec := doGet(t, mux, "/plans/1")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got PlanView
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.Plan.ID != "plan-1" || len(got.Candidates) != 1 {
		t.Fatalf("unexpected view: %+v", got)
	}
}

func TestGetPlan_NotFound(t *testing.T) {
	svc := &fakePlansSvc{planErr: ErrNotFound}
	mux := newMux(t, Dependencies{Plans: svc})
	rec := doGet(t, mux, "/plans/999")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestUpdatePlan_HappyPath(t *testing.T) {
	svc := &fakePlansSvc{}
	mux := newMux(t, Dependencies{Plans: svc})
	body, _ := json.Marshal(map[string]string{"status": "approved"})
	rec := doPatch(t, mux, "/plans/1", string(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got PlanResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.Status != "approved" {
		t.Fatalf("status = %q, want approved", got.Status)
	}
}

func TestUpdatePlan_InvalidStatus(t *testing.T) {
	svc := &fakePlansSvc{}
	mux := newMux(t, Dependencies{Plans: svc})
	body, _ := json.Marshal(map[string]string{"status": "invalid"})
	rec := doPatch(t, mux, "/plans/1", string(body))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestUpdatePlan_NotFound(t *testing.T) {
	svc := &fakePlansSvc{planErr: ErrNotFound}
	mux := newMux(t, Dependencies{Plans: svc})
	body, _ := json.Marshal(map[string]string{"status": "approved"})
	rec := doPatch(t, mux, "/plans/999", string(body))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestSetDecisions_HappyPath(t *testing.T) {
	svc := &fakePlansSvc{
		planView: PlanView{
			Plan:    PlanResponse{ID: "plan-1", Status: "draft"},
			Decisions: &[]PlanDecisionResponse{{PlanID: "plan-1", SlotDate: types.Date{Time: time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)}, MealieRecipeID: "r-1"}},
		},
	}
	mux := newMux(t, Dependencies{Plans: svc})
	body, _ := json.Marshal([]PlanDecisionInput{{SlotDate: types.Date{Time: time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)}, MealieRecipeID: "r-1"}})
	rec := doPost(t, mux, "/plans/1/decisions", string(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
}

func TestListCandidates_HappyPath(t *testing.T) {
	svc := &fakePlansSvc{
		candidates: []PlanCandidateResponse{
			{ID: "cand-1", SlotDate: types.Date{Time: time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)}, Rank: 1, Score: 0.9, Feasible: true},
		},
	}
	mux := newMux(t, Dependencies{Plans: svc})
	rec := doGet(t, mux, "/plans/1/candidates")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got []PlanCandidateResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].Rank != 1 {
		t.Fatalf("unexpected candidates: %+v", got)
	}
}

func TestListShoppingRequirements_HappyPath(t *testing.T) {
	svc := &fakePlansSvc{
		shoppingReqs: []ShoppingRequirementResponse{{ID: "req-1", IngredientID: "pasta", Quantity: 500, Unit: "g"}},
	}
	mux := newMux(t, Dependencies{Plans: svc})
	rec := doGet(t, mux, "/plans/1/shopping-requirements")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got []ShoppingRequirementResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].IngredientID != "pasta" {
		t.Fatalf("unexpected reqs: %+v", got)
	}
}

// ---- Effort profile handler unit tests ----

func TestListEffortProfiles_HappyPath(t *testing.T) {
	svc := &fakeEffortProfileSvc{
		profiles: []EffortProfileResponse{{Weekday: 1, KitchenEnergy: 2}},
	}
	mux := newMux(t, Dependencies{EffortProfiles: svc})
	rec := doGet(t, mux, "/effort-profiles")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got []EffortProfileResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].Weekday != 1 {
		t.Fatalf("unexpected profiles: %+v", got)
	}
}

func TestUpsertEffortProfile_HappyPath(t *testing.T) {
	svc := &fakeEffortProfileSvc{}
	mux := newMux(t, Dependencies{EffortProfiles: svc})
	body, _ := json.Marshal(map[string]int{"weekday": 2, "kitchen_energy": 3})
	rec := doPost(t, mux, "/effort-profiles", string(body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
}

func TestUpsertEffortProfile_InvalidEnergy(t *testing.T) {
	svc := &fakeEffortProfileSvc{}
	mux := newMux(t, Dependencies{EffortProfiles: svc})
	body, _ := json.Marshal(map[string]int{"weekday": 2, "kitchen_energy": 5})
	rec := doPost(t, mux, "/effort-profiles", string(body))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// ---- Planning constraint handler unit tests ----

func TestListPlanningConstraints_HappyPath(t *testing.T) {
	svc := &fakeConstraintSvc{
		constraints: []PlanningConstraintResponse{{ID: "con-1", Kind: "avoid_tag", Value: "seafood", Active: true}},
	}
	mux := newMux(t, Dependencies{PlanningConstraints: svc})
	rec := doGet(t, mux, "/constraints")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got []PlanningConstraintResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].Kind != "avoid_tag" {
		t.Fatalf("unexpected constraints: %+v", got)
	}
}

func TestCreatePlanningConstraint_HappyPath(t *testing.T) {
	svc := &fakeConstraintSvc{}
	mux := newMux(t, Dependencies{PlanningConstraints: svc})
	body, _ := json.Marshal(map[string]interface{}{"kind": "max_effort", "value": "2", "active": true})
	rec := doPost(t, mux, "/constraints", string(body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	var got PlanningConstraintResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.ID != "con-1" || got.Kind != "max_effort" {
		t.Fatalf("unexpected constraint: %+v", got)
	}
}

func TestCreatePlanningConstraint_MissingKind(t *testing.T) {
	svc := &fakeConstraintSvc{}
	mux := newMux(t, Dependencies{PlanningConstraints: svc})
	body, _ := json.Marshal(map[string]interface{}{"value": "seafood", "active": true})
	rec := doPost(t, mux, "/constraints", string(body))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// ---- nextMonday unit tests ----

func TestNextMonday(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected time.Time
	}{
		{
			name:     "Monday returns next Monday",
			input:    time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC),
			expected: time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "Tuesday returns next Monday",
			input:    time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC),
			expected: time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "Sunday returns next Monday",
			input:    time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC), // Sunday
			expected: time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC), // next Monday (same ISO week as the Sunday)
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nextMonday(tt.input)
			if !got.Equal(tt.expected) {
				t.Fatalf("nextMonday(%v) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

// ---- fakes ----

type fakeEffortProfileSvc struct {
	profiles []EffortProfileResponse
	err      error
}

func (f *fakeEffortProfileSvc) ListEffortProfiles(ctx context.Context) ([]EffortProfileResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.profiles, nil
}

func (f *fakeEffortProfileSvc) UpsertEffortProfile(ctx context.Context, in EffortProfileInput) error {
	return f.err
}

type fakeConstraintSvc struct {
	constraints []PlanningConstraintResponse
	err         error
}

func (f *fakeConstraintSvc) ListPlanningConstraints(ctx context.Context) ([]PlanningConstraintResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.constraints, nil
}

func (f *fakeConstraintSvc) CreatePlanningConstraint(ctx context.Context, in PlanningConstraintInput) (PlanningConstraintResponse, error) {
	return PlanningConstraintResponse{ID: "con-1", Kind: in.Kind, Value: in.Value, Active: in.Active}, f.err
}
