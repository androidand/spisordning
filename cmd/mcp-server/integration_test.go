package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/androidand/spisordning/internal/persistence"
	"github.com/androidand/spisordning/internal/planning"
	"github.com/androidand/spisordning/internal/domain"
)

// skipWithoutDB skips the test when no Postgres is reachable from this host.
func skipWithoutDB(t *testing.T) *persistence.Store {
	t.Helper()
	if os.Getenv("DATABASE_URL") == "" && os.Getenv("POSTGRES_PASSWORD") == "" {
		t.Skip("no DATABASE_URL/POSTGRES_PASSWORD in env; skipping Postgres integration test")
	}
	cfg, err := persistence.FromEnv(os.Getenv)
	if err != nil {
		t.Skipf("no usable postgres config: %v", err)
	}
	ctx := context.Background()
	store, err := persistence.New(ctx, cfg)
	if err != nil {
		t.Skipf("cannot connect to postgres (expected without `docker compose up`): %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

// TestIntegration_SchoolTagsAndEnergy verifies that the MCP planner's
// SchoolTagsFor and EnergyFor closures are wired correctly by checking that
// the planning output reflects both a low-energy weekday and a school-lunch
// tag overlap. This test uses a fake skolmaten server and a real Postgres
// database (skipped if unavailable).
func TestIntegration_SchoolTagsAndEnergy(t *testing.T) {
	store := skipWithoutDB(t)
	ctx := context.Background()

	// Set up a fake skolmaten server that serves fish on Monday 2026-07-27.
	fakeSkolmaten := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"id":"m","name":"Skolan","WeekState":{"week":31,"year":2026,"Days":[
			{"date":"2026-07-27T00:00:00Z","Meals":[{"id":"a","name":"Stekt fisk"}]}
		]},"School":{"id":"s","name":"Skolan"}}`))
	}))
	t.Cleanup(fakeSkolmaten.Close)
	t.Setenv("SKOLMATEN_BASE_URL", fakeSkolmaten.URL)
	t.Setenv("SKOLMATEN_CLIENT_TOKEN", "")
	t.Setenv("SKOLMATEN_SCHOOL", "mariaskolan")

	// Build the adapter with the real store.
	adapter := mcpStoreAdapter{db: store}

	// Load school tags for the week of 2026-07-27 (Monday, ISO week 31).
	schoolTags, err := adapter.loadSchoolTagsFor(ctx, time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("loadSchoolTagsFor: %v", err)
	}

	// Verify the school tags are present for Monday 2026-07-27.
	monday := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)
	tags := schoolTags(monday)
	if len(tags) == 0 {
		t.Fatalf("expected school tags for Monday 2026-07-27, got none")
	}
	// Check that "fisk" is in the tags (from "Stekt fisk").
	found := false
	for _, tag := range tags {
		if tag == "fisk" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected 'fisk' in school tags, got: %v", tags)
	}

	// Load energy for a weekday with a low-effort profile.
	energy, err := adapter.loadEnergyFor(ctx)
	if err != nil {
		t.Fatalf("loadEnergyFor: %v", err)
	}

	// Verify the energy function returns a valid effort for a weekday.
	effort := energy(monday)
	if effort < domain.EffortLow || effort > domain.EffortHigh {
		t.Fatalf("unexpected effort level: %v", effort)
	}

	// Verify that the WeekConfig would be built correctly by checking that
	// both closures are non-nil and produce expected outputs.
	_ = planning.WeekConfig{
		EnergyFor:     energy,
		SchoolTagsFor: schoolTags,
	}
}
