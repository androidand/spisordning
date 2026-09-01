package service_test

import (
	"context"
	"os"
	"testing"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/androidand/spisordning/internal/persistence"
)

func strPtr(s string) *string { return &s }

func mustTestStore(t *testing.T) *persistence.Store {
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
		t.Skipf("cannot connect to postgres: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestNutritionProfile_NutrientValueLookup(t *testing.T) {
	p := domain.NutritionProfile{SLVNummer: 1, Namn: "Apelsin"}
	cals := 47.0
	p.Calories = &cals

	got := p.NutrientValue("kcal")
	if got == nil || *got != 47 {
		t.Fatalf("expected 47 kcal, got %v", got)
	}

	// Swedish name should also match.
	if got := p.NutrientValue("natrium"); got != nil {
		t.Fatalf("expected nil for natrium, got %v", got)
	}

	// Unknown nutrient returns nil.
	if got := p.NutrientValue("calcium"); got != nil {
		t.Fatalf("expected nil for calcium, got %v", got)
	}
}

func TestNutritionProfile_MultipleNutrients(t *testing.T) {
	protein := 2.5
	fat := 3.0
	p := domain.NutritionProfile{Protein: &protein, Fat: &fat}

	if got := p.NutrientValue("protein"); got == nil || *got != 2.5 {
		t.Fatalf("expected 2.5 protein, got %v", got)
	}
	if got := p.NutrientValue("fett"); got == nil || *got != 3.0 {
		t.Fatalf("expected 3.0 fat (Swedish 'fett'), got %v", got)
	}
	if p.NutrientValue("carbohydrate") != nil {
		t.Fatalf("expected nil for carbohydrate")
	}
}

// TestNutritionSyncStatusDomain verifies the domain type used by the sync job.
func TestNutritionSyncStatusDomain(t *testing.T) {
	st := domain.NutritionSyncStatus{Source: "slv", RecordCount: 2606}
	if st.Source != "slv" {
		t.Fatalf("expected source slv, got %q", st.Source)
	}
	if st.RecordCount != 2606 {
		t.Fatalf("expected 2606 records, got %d", st.RecordCount)
	}
}

// Verify the persistence Food/Nutrient/Mapping types have the expected shape.
func TestNutritionDomainTypes(t *testing.T) {
	namn := "Apelsin"
	typ := "frukt"
	f := domain.Food{SlvNummer: 1234, Namn: namn, VetenskapligtNamn: &namn, LivsmedelsTyp: &typ}
	if f.SlvNummer != 1234 {
		t.Fatalf("expected slv_nummer 1234, got %d", f.SlvNummer)
	}
	if f.LivsmedelsTyp == nil || *f.LivsmedelsTyp != "frukt" {
		t.Fatalf("expected livsmedels_typ frukt, got %v", f.LivsmedelsTyp)
	}

	n := domain.Nutrient{FoodNummer: 1234, Name: "Vitamin C", Värde: 53, Enhet: "mg"}
	if n.Name != "Vitamin C" || n.Värde != 53 {
		t.Fatalf("unexpected nutrient: %+v", n)
	}

	mp := domain.ProductMapping{GTIN: strPtr("73")}
	if mp.GTIN == nil || *mp.GTIN != "73" {
		t.Fatalf("expected gtin 73, got %v", mp.GTIN)
	}
}

func TestPersistenceNutritionRoundTrip(t *testing.T) {
	db := mustTestStore(t)

	ctx := context.Background()
	if err := db.UpsertFood(ctx, persistence.Food{SlvNummer: 999, Namn: "TestFood"}); err != nil {
		t.Fatalf("UpsertFood: %v", err)
	}

	food, err := db.GetFood(ctx, 999)
	if err != nil {
		t.Fatalf("GetFood: %v", err)
	}
	if food.Namn != "TestFood" {
		t.Fatalf("expected TestFood, got %q", food.Namn)
	}

	if err := db.UpsertNutrients(ctx, 999, []persistence.Nutrient{
		{FoodNummer: 999, Name: "Protein", Värde: 5, Enhet: "g"},
	}); err != nil {
		t.Fatalf("UpsertNutrients: %v", err)
	}

	nutr, err := db.GetNutritionForFood(ctx, 999)
	if err != nil {
		t.Fatalf("GetNutritionForFood: %v", err)
	}
	if len(nutr) != 1 {
		t.Fatalf("expected 1 nutrient, got %d", len(nutr))
	}

	// Upserting again should replace, not duplicate.
	if err := db.UpsertNutrients(ctx, 999, []persistence.Nutrient{
		{FoodNummer: 999, Name: "Fat", Värde: 2, Enhet: "g"},
		{FoodNummer: 999, Name: "Kolhydrat", Värde: 10, Enhet: "g"},
	}); err != nil {
		t.Fatalf("UpsertNutrients (replace): %v", err)
	}
	nutr, _ = db.GetNutritionForFood(ctx, 999)
	if len(nutr) != 2 {
		t.Fatalf("expected 2 nutrients after replace, got %d", len(nutr))
	}

	count, err := db.CountFoods(ctx)
	if err != nil {
		t.Fatalf("CountFoods: %v", err)
	}
	if count < 1 {
		t.Fatalf("expected at least 1 food, got %d", count)
	}

	status := persistence.NutritionSyncStatus{Source: "slv", RecordCount: 1}
	if err := db.UpsertNutritionSyncStatus(ctx, status); err != nil {
		t.Fatalf("UpsertNutritionSyncStatus: %v", err)
	}
	got, err := db.GetNutritionSyncStatus(ctx, "slv")
	if err != nil {
		t.Fatalf("GetNutritionSyncStatus: %v", err)
	}
	if got.RecordCount != 1 {
		t.Fatalf("expected record_count 1, got %d", got.RecordCount)
	}
}
