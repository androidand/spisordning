package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/androidand/spisordning/internal/persistence"
	"github.com/oapi-codegen/runtime/types"
)

func TestIntegration_PlanLifecycle(t *testing.T) {
	s := skipWithoutDB(t)
	adapter := &testAdapter{db: s}
	mux := newMux(t, Dependencies{Plans: adapter})

	// 1. Create a plan.
	body, _ := json.Marshal(map[string]string{"week_start": "2026-08-25"})
	rec := doPost(t, mux, "/plans", string(body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create plan: status = %d, body: %s", rec.Code, rec.Body)
	}
	var plan PlanResponse
	mustJSON(t, rec.Body.Bytes(), &plan)
	if plan.Status != "draft" {
		t.Fatalf("status = %q, want draft", plan.Status)
	}
	if plan.ID == "" {
		t.Fatal("plan ID is empty")
	}

	// 2. List plans — should include the created plan.
	rec = doGet(t, mux, "/plans")
	if rec.Code != http.StatusOK {
		t.Fatalf("list plans: status = %d", rec.Code)
	}
	var plans []PlanResponse
	mustJSON(t, rec.Body.Bytes(), &plans)
	found := false
	for _, p := range plans {
		if p.ID == plan.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("created plan not found in list")
	}

	// 3. Get plan — should return plan with empty candidates/decisions.
	rec = doGet(t, mux, "/plans/"+plan.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("get plan: status = %d, body: %s", rec.Code, rec.Body)
	}
	var view PlanView
	mustJSON(t, rec.Body.Bytes(), &view)
	if view.Plan.ID != plan.ID {
		t.Fatalf("plan ID mismatch: %s vs %s", view.Plan.ID, plan.ID)
	}

	// 4. Update plan status to approved.
	body, _ = json.Marshal(map[string]string{"status": "approved"})
	rec = doPatch(t, mux, "/plans/"+plan.ID, string(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("update plan: status = %d, body: %s", rec.Code, rec.Body)
	}
	var updated PlanResponse
	mustJSON(t, rec.Body.Bytes(), &updated)
	if updated.Status != "approved" {
		t.Fatalf("status = %q, want approved", updated.Status)
	}

	// 5. Get plan after approval.
	rec = doGet(t, mux, "/plans/"+plan.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("get plan after approve: status = %d", rec.Code)
	}
	mustJSON(t, rec.Body.Bytes(), &view)
	if view.Plan.Status != "approved" {
		t.Fatalf("status = %q, want approved", view.Plan.Status)
	}

	// 6. Get non-existent plan → 404.
	rec = doGet(t, mux, "/plans/99999")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get missing plan: status = %d, want 404", rec.Code)
	}
}

func TestIntegration_PlanDecisions(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	// meal_plan_decision has an FK to recipe_ref, so seed the referenced recipes.
	for _, id := range []string{"r-1", "r-2"} {
		if err := s.UpsertRecipeRef(ctx, persistence.RecipeRef{MealieRecipeID: id, Title: id, Effort: 2}); err != nil {
			t.Fatalf("UpsertRecipeRef(%s): %v", id, err)
		}
	}
	adapter := &testAdapter{db: s}
	mux := newMux(t, Dependencies{Plans: adapter})

	// Create a plan.
	body, _ := json.Marshal(map[string]string{"week_start": "2026-09-01"})
	rec := doPost(t, mux, "/plans", string(body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create plan: status = %d", rec.Code)
	}
	var plan PlanResponse
	mustJSON(t, rec.Body.Bytes(), &plan)

	// Set decisions.
	decisions := []PlanDecisionInput{
		{SlotDate: types.Date{Time: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)}, MealieRecipeID: "r-1"},
		{SlotDate: types.Date{Time: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)}, MealieRecipeID: "r-2"},
	}
	body, _ = json.Marshal(decisions)
	rec = doPost(t, mux, "/plans/"+plan.ID+"/decisions", string(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("set decisions: status = %d, body: %s", rec.Code, rec.Body)
	}
	var gotDecisions *[]PlanDecisionResponse
	mustJSON(t, rec.Body.Bytes(), &gotDecisions)
	if gotDecisions == nil || len(*gotDecisions) != 2 {
		t.Fatalf("expected 2 decisions, got %d", len(*gotDecisions))
	}

	// Verify via get plan.
	rec = doGet(t, mux, "/plans/"+plan.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("get plan: status = %d", rec.Code)
	}
	var view PlanView
	mustJSON(t, rec.Body.Bytes(), &view)
	if view.Decisions == nil || len(*view.Decisions) != 2 {
		t.Fatalf("expected 2 decisions in view, got %d", len(*view.Decisions))
	}
}

func TestIntegration_EffortProfile(t *testing.T) {
	s := skipWithoutDB(t)
	adapter := &testAdapter{db: s}
	mux := newMux(t, Dependencies{Plans: adapter, EffortProfiles: adapter})

	// Upsert effort profile.
	body, _ := json.Marshal(map[string]int{"weekday": 1, "kitchen_energy": 2})
	rec := doPost(t, mux, "/effort-profiles", string(body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("upsert effort profile: status = %d, body: %s", rec.Code, rec.Body)
	}

	// List effort profiles.
	rec = doGet(t, mux, "/effort-profiles")
	if rec.Code != http.StatusOK {
		t.Fatalf("list effort profiles: status = %d", rec.Code)
	}
	var profiles []EffortProfileResponse
	mustJSON(t, rec.Body.Bytes(), &profiles)
	// Check the weekday-1 profile is present (not a global count, which is
	// unsafe when other packages' tests write to the same DB in parallel).
	profileFound := false
	for _, p := range profiles {
		if p.Weekday == 1 && p.KitchenEnergy == 2 {
			profileFound = true
			break
		}
	}
	if !profileFound {
		t.Fatalf("profile for weekday 1 not found in list of %d profiles", len(profiles))
	}
}

func TestIntegration_PlanningConstraint(t *testing.T) {
	s := skipWithoutDB(t)
	truncateTables(t, s, "planning_constraint")
	adapter := &testAdapter{db: s}
	mux := newMux(t, Dependencies{Plans: adapter, PlanningConstraints: adapter})

	// Create constraint.
	body, _ := json.Marshal(map[string]interface{}{"kind": "avoid_tag", "value": "seafood", "active": true})
	rec := doPost(t, mux, "/constraints", string(body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create constraint: status = %d, body: %s", rec.Code, rec.Body)
	}
	var constraint PlanningConstraintResponse
	mustJSON(t, rec.Body.Bytes(), &constraint)
	if constraint.Kind != "avoid_tag" || constraint.Value != "seafood" || !constraint.Active {
		t.Fatalf("unexpected constraint: %+v", constraint)
	}

	// List constraints.
	rec = doGet(t, mux, "/constraints")
	if rec.Code != http.StatusOK {
		t.Fatalf("list constraints: status = %d", rec.Code)
	}
	var constraints []PlanningConstraintResponse
	mustJSON(t, rec.Body.Bytes(), &constraints)
	// Check the created constraint is present (not a global count, which is
	// unsafe when other packages' tests write to the same DB in parallel).
	constraintFound := false
	for _, c := range constraints {
		if c.ID == constraint.ID {
			constraintFound = true
			break
		}
	}
	if !constraintFound {
		t.Fatalf("created constraint %s not found in list of %d constraints", constraint.ID, len(constraints))
	}
}

func TestIntegration_ShoppingRequirements(t *testing.T) {
	s := skipWithoutDB(t)
	adapter := &testAdapter{db: s}
	mux := newMux(t, Dependencies{Plans: adapter})

	// Create a plan.
	body, _ := json.Marshal(map[string]string{"week_start": "2026-10-06"})
	rec := doPost(t, mux, "/plans", string(body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create plan: status = %d", rec.Code)
	}
	var plan PlanResponse
	mustJSON(t, rec.Body.Bytes(), &plan)

	// List shopping requirements — should be empty.
	rec = doGet(t, mux, "/plans/"+plan.ID+"/shopping-requirements")
	if rec.Code != http.StatusOK {
		t.Fatalf("list shopping reqs: status = %d", rec.Code)
	}
	var reqs []ShoppingRequirementResponse
	mustJSON(t, rec.Body.Bytes(), &reqs)
	if len(reqs) != 0 {
		t.Fatalf("expected 0 requirements, got %d", len(reqs))
	}
}
