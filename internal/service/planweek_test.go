package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/androidand/spisordning/internal/mealie"
	"github.com/androidand/spisordning/internal/persistence"
	"github.com/androidand/spisordning/internal/planning"
	"github.com/androidand/spisordning/internal/scoring"
)

type persistErrSentinel string

func (e persistErrSentinel) Error() string { return string(e) }

// fakePersistStore records every call persistWeek makes against a Store. It
// embeds Store (nil) so only the four plan-persistence methods are implemented;
// persistWeek never calls anything else.
type fakePersistStore struct {
	Store
	planID         domain.MealPlanID
	getOrCreateErr error
	insCandErr     error
	setDecErr      error
	insReqErr      error

	candidates   []persistence.MealPlanCandidate
	decisions    []persistence.MealPlanDecision
	requirements []persistence.ShoppingRequirement
}

func (f *fakePersistStore) GetOrCreateMealPlan(ctx context.Context, weekStart time.Time) (persistence.MealPlan, error) {
	if f.getOrCreateErr != nil {
		return persistence.MealPlan{}, f.getOrCreateErr
	}
	return persistence.MealPlan{ID: f.planID, WeekStart: weekStart, Status: "draft"}, nil
}
func (f *fakePersistStore) GetRecipeRefByMealieID(ctx context.Context, mealieRecipeID string) (persistence.RecipeRef, error) {
	return persistence.RecipeRef{ID: domain.NewRecipeRefID(), MealieRecipeID: mealieRecipeID}, nil
}
func (f *fakePersistStore) InsertCandidate(ctx context.Context, c persistence.MealPlanCandidate) error {
	if f.insCandErr != nil {
		return f.insCandErr
	}
	f.candidates = append(f.candidates, c)
	return nil
}
func (f *fakePersistStore) SetDecision(ctx context.Context, d persistence.MealPlanDecision) error {
	if f.setDecErr != nil {
		return f.setDecErr
	}
	f.decisions = append(f.decisions, d)
	return nil
}
func (f *fakePersistStore) InsertShoppingRequirement(ctx context.Context, r persistence.ShoppingRequirement) error {
	if f.insReqErr != nil {
		return f.insReqErr
	}
	f.requirements = append(f.requirements, r)
	return nil
}

func testSlots() []planning.PlannedSlot {
	monday := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC) // Mon
	return []planning.PlannedSlot{
		{Date: monday, Winner: scoring.ScoredCandidate{
			Candidate: domain.Candidate{MealieRecipeID: "r-1", Title: "Pasta"},
			Score:     0.9,
			Breakdown: scoring.Breakdown{Preference: 0.5, Effort: 0.4},
			Feasible:  true,
		}},
		{Date: monday.AddDate(0, 0, 1), Winner: scoring.ScoredCandidate{
			Candidate: domain.Candidate{MealieRecipeID: "r-2", Title: "Tacos"},
			Score:     0.8,
			Feasible:  true,
		}},
	}
}

func TestPersistWeek_HappyPath(t *testing.T) {
	ctx := context.Background()
	planID := domain.NewMealPlanID()
	store := &fakePersistStore{planID: planID}
	sl := testSlots()
	reqs := []domain.ShoppingRequirement{
		{IngredientID: "pasta", Quantity: 400, Unit: "g"},
		{IngredientID: "ost", Quantity: 200, Unit: "g", PreferredForm: "riven"},
	}

	if err := persistWeek(ctx, store, sl[0].Date, sl, reqs); err != nil {
		t.Fatalf("persistWeek: %v", err)
	}

	if len(store.candidates) != 2 {
		t.Errorf("candidates = %d, want 2", len(store.candidates))
	}
	for _, c := range store.candidates {
		if c.PlanID != planID || c.RecipeRefID == (domain.RecipeRefID{}) || c.Score == 0 {
			t.Errorf("unexpected candidate: %+v", c)
		}
	}
	if len(store.decisions) != 2 {
		t.Errorf("decisions = %d, want 2", len(store.decisions))
	}
	if len(store.requirements) != 2 {
		t.Errorf("requirements = %d, want 2", len(store.requirements))
	}
	// PreferredForm must become a non-nil *string; empty stays nil.
	ostID := domain.IngredientIDForName("ost")
	var ost *persistence.ShoppingRequirement
	for i := range store.requirements {
		if store.requirements[i].IngredientID == ostID {
			ost = &store.requirements[i]
		}
	}
	if ost == nil || ost.PreferredForm == nil || *ost.PreferredForm != "riven" {
		t.Errorf("ost preferred_form = %+v", ost)
	}
}

func TestPersistWeek_ErrorsPropagate(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name  string
		text  string
		field func(*fakePersistStore)
	}{
		{"get_or_create", "lock", func(s *fakePersistStore) { s.getOrCreateErr = persistErrSentinel("lock") }},
		{"insert_candidate", "dup", func(s *fakePersistStore) { s.insCandErr = persistErrSentinel("dup") }},
		{"set_decision", "conflict", func(s *fakePersistStore) { s.setDecErr = persistErrSentinel("conflict") }},
		{"insert_requirement", "bad", func(s *fakePersistStore) { s.insReqErr = persistErrSentinel("bad") }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			store := &fakePersistStore{planID: domain.NewMealPlanID()}
			c.field(store)
			err := persistWeek(ctx, store, time.Now(), testSlots(), []domain.ShoppingRequirement{{IngredientID: "x"}})
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), c.text) {
				t.Fatalf("err = %q, want it to contain %q", err.Error(), c.text)
			}
		})
	}
}

func TestPersistWeek_EmptyPlannedStillCreatesPlan(t *testing.T) {
	ctx := context.Background()
	store := &fakePersistStore{planID: domain.NewMealPlanID()}
	// No slots, no requirements — the meal_plan row should still be created.
	if err := persistWeek(ctx, store, time.Now(), nil, nil); err != nil {
		t.Fatalf("persistWeek: %v", err)
	}
	if len(store.candidates) != 0 || len(store.decisions) != 0 || len(store.requirements) != 0 {
		t.Errorf("expected no child rows, got cand=%d dec=%d req=%d",
			len(store.candidates), len(store.decisions), len(store.requirements))
	}
}

func TestPlanWeek_Orchestrates(t *testing.T) {
	fakeMealie := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/recipes":
			w.Write([]byte(`{"page":1,"perPage":50,"total":2,"items":[
				{"id":"r-pasta","slug":"pasta","name":"Pasta Bolognese"},
				{"id":"r-tacos","slug":"tacos","name":"Tacos"}]}`))
		case "/api/recipes/pasta":
			w.Write([]byte(`{"id":"r-pasta","slug":"pasta","name":"Pasta Bolognese","totalTime":"20 min",
				"tags":[{"name":"pasta"}],
				"recipeIngredient":[{"quantity":400,"unit":{"name":"g"},"food":{"id":"f1","name":"köttfärs"}}]}`))
		case "/api/recipes/tacos":
			w.Write([]byte(`{"id":"r-tacos","slug":"tacos","name":"Tacos","totalTime":"30 min",
				"tags":[{"name":"mexican"}],
				"recipeIngredient":[{"quantity":200,"unit":{"name":"g"},"food":{"id":"f2","name":"ost"}}]}`))
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(fakeMealie.Close)

	store := &fakePersistStore{planID: domain.NewMealPlanID()}
	svc := NewPlanning(store, mealie.New(fakeMealie.URL, "tok"))

	res, err := svc.PlanWeek(context.Background(), PlanWeekInput{
		WeekStart: time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC),
		Days:      7,
		People:    []domain.Person{{ID: domain.NewPersonID(), Name: "Andreas", Weight: 1.0}},
	})
	if err != nil {
		t.Fatalf("PlanWeek: %v", err)
	}
	if len(res.Planned) == 0 {
		t.Fatal("expected planned slots, got none")
	}
	if !res.Persisted || res.PersistError != nil {
		t.Errorf("persisted = %v, err = %v; want persisted with no error", res.Persisted, res.PersistError)
	}
	// One candidate + decision persisted per planned slot.
	if len(store.candidates) != len(res.Planned) || len(store.decisions) != len(res.Planned) {
		t.Errorf("persisted cand=%d dec=%d, want %d of each",
			len(store.candidates), len(store.decisions), len(res.Planned))
	}
	if len(res.Reqs) == 0 {
		t.Error("expected shopping requirements for planned meals")
	}
}

func TestPlanWeek_NoMealieClient(t *testing.T) {
	svc := NewPlanning(nil, nil)
	if _, err := svc.PlanWeek(context.Background(), PlanWeekInput{
		WeekStart: time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC),
	}); err == nil {
		t.Fatal("expected error when no Mealie client is configured")
	}
}

// resolverFakeStore is a minimal Store for testing the resolver path in PlanWeek.
type resolverFakeStore struct {
	recipeRefs []persistence.RecipeRef
	families   map[domain.RecipeFamilyID]persistence.RecipeFamily
	srcRefs    map[string]persistence.RecipeSourceRef // key: "source:recipeID"
	variants   map[domain.RecipeVariantID]persistence.RecipeVariant
	revisions  map[domain.RecipeVariantID][]persistence.RecipeRevision
	planID     domain.MealPlanID
	insCandErr error
	setDecErr  error
	insReqErr  error

	candidates   []persistence.MealPlanCandidate
	decisions    []persistence.MealPlanDecision
	requirements []persistence.ShoppingRequirement
}

func (f *resolverFakeStore) GetOrCreateMealPlan(_ context.Context, weekStart time.Time) (persistence.MealPlan, error) {
	return persistence.MealPlan{ID: f.planID, WeekStart: weekStart, Status: "draft"}, nil
}
func (f *resolverFakeStore) GetRecipeRefByMealieID(_ context.Context, mealieRecipeID string) (persistence.RecipeRef, error) {
	for _, r := range f.recipeRefs {
		if r.MealieRecipeID == mealieRecipeID {
			return r, nil
		}
	}
	return persistence.RecipeRef{}, persistence.ErrNoRows
}
func (f *resolverFakeStore) GetRecipeRef(_ context.Context, id domain.RecipeRefID) (persistence.RecipeRef, error) {
	for _, r := range f.recipeRefs {
		if r.ID == id {
			return r, nil
		}
	}
	return persistence.RecipeRef{}, persistence.ErrNoRows
}
func (f *resolverFakeStore) InsertCandidate(_ context.Context, c persistence.MealPlanCandidate) error {
	if f.insCandErr != nil {
		return f.insCandErr
	}
	f.candidates = append(f.candidates, c)
	return nil
}
func (f *resolverFakeStore) SetDecision(_ context.Context, d persistence.MealPlanDecision) error {
	if f.setDecErr != nil {
		return f.setDecErr
	}
	f.decisions = append(f.decisions, d)
	return nil
}
func (f *resolverFakeStore) InsertShoppingRequirement(_ context.Context, r persistence.ShoppingRequirement) error {
	if f.insReqErr != nil {
		return f.insReqErr
	}
	f.requirements = append(f.requirements, r)
	return nil
}
func (f *resolverFakeStore) ListRecipeRefs(_ context.Context) ([]persistence.RecipeRef, error) {
	return f.recipeRefs, nil
}
func (f *resolverFakeStore) GetRecipeSourceRefBySource(_ context.Context, source, recipeID string) (persistence.RecipeSourceRef, error) {
	k := source + ":" + recipeID
	r, ok := f.srcRefs[k]
	if !ok {
		return persistence.RecipeSourceRef{}, persistence.ErrNoRows
	}
	return r, nil
}
func (f *resolverFakeStore) GetRecipeSourceRefByFamily(_ context.Context, _ domain.RecipeFamilyID) (persistence.RecipeSourceRef, error) {
	return persistence.RecipeSourceRef{}, persistence.ErrNoRows
}
func (f *resolverFakeStore) GetRecipeFamily(_ context.Context, id domain.RecipeFamilyID) (persistence.RecipeFamily, error) {
	fam, ok := f.families[id]
	if !ok {
		return persistence.RecipeFamily{}, persistence.ErrNoRows
	}
	return fam, nil
}
func (f *resolverFakeStore) ListRecipeRevisions(_ context.Context, variantID domain.RecipeVariantID) ([]persistence.RecipeRevision, error) {
	return f.revisions[variantID], nil
}

// unimplemented stubs — PlanWeek under native/dual mode does not call these.
func (f *resolverFakeStore) CreatePerson(_ context.Context, _ persistence.Person) error { return nil }
func (f *resolverFakeStore) UpdatePerson(_ context.Context, _ persistence.Person) error { return nil }
func (f *resolverFakeStore) GetPerson(_ context.Context, _ string) (persistence.Person, error) {
	return persistence.Person{}, persistence.ErrNoRows
}
func (f *resolverFakeStore) ListPeople(_ context.Context) ([]persistence.Person, error) { return nil, nil }
func (f *resolverFakeStore) UpsertPreference(_ context.Context, _ persistence.PersonPreference) error { return nil }
func (f *resolverFakeStore) ListPreferences(_ context.Context, _ domain.PersonID) ([]persistence.PersonPreference, error) {
	return nil, nil
}
func (f *resolverFakeStore) RecordObservation(_ context.Context, _ persistence.PreferenceObservation) error { return nil }
func (f *resolverFakeStore) UpsertRecipeRef(_ context.Context, _ persistence.RecipeRef) error { return nil }
func (f *resolverFakeStore) CreateMealEvent(_ context.Context, _ domain.RecipeRefID, _ time.Time, _ *domain.MealPlanID, _ *time.Time) (domain.MealEventID, error) {
	return domain.MealEventID{}, nil
}
func (f *resolverFakeStore) AddMealReaction(_ context.Context, _ persistence.MealReaction) error { return nil }
func (f *resolverFakeStore) ListMealReactions(_ context.Context, _ domain.MealEventID) ([]persistence.MealReaction, error) {
	return nil, nil
}
func (f *resolverFakeStore) GetMealPlan(_ context.Context, _ domain.MealPlanID) (persistence.MealPlan, error) {
	return persistence.MealPlan{}, persistence.ErrNoRows
}
func (f *resolverFakeStore) SetMealPlanStatus(_ context.Context, _ domain.MealPlanID, _ string) error { return nil }
func (f *resolverFakeStore) ListCandidates(_ context.Context, _ domain.MealPlanID) ([]persistence.MealPlanCandidate, error) {
	return nil, nil
}
func (f *resolverFakeStore) ListDecisions(_ context.Context, _ domain.MealPlanID) ([]persistence.MealPlanDecision, error) {
	return nil, nil
}
func (f *resolverFakeStore) ListShoppingRequirements(_ context.Context, _ domain.MealPlanID) ([]persistence.ShoppingRequirement, error) {
	return nil, nil
}
func (f *resolverFakeStore) UpsertIngredientMapping(_ context.Context, _ persistence.IngredientMapping) error { return nil }
func (f *resolverFakeStore) UpsertIngredient(_ context.Context, _ persistence.Ingredient) error { return nil }
func (f *resolverFakeStore) AddRecipeIngredient(_ context.Context, _ persistence.RecipeIngredient) error { return nil }
func (f *resolverFakeStore) BeginTx(_ context.Context) (persistence.Tx, error) { return nil, nil }
func (f *resolverFakeStore) CreateInventoryLocation(_ context.Context, _ persistence.InventoryLocation) error {
	return nil
}
func (f *resolverFakeStore) GetInventoryLocation(_ context.Context, _ domain.InventoryLocationID) (persistence.InventoryLocation, error) {
	return persistence.InventoryLocation{}, persistence.ErrNoRows
}
func (f *resolverFakeStore) ListLotsUnderLocation(_ context.Context, _ domain.InventoryLocationID) ([]persistence.InventoryLot, error) {
	return nil, nil
}
func (f *resolverFakeStore) ListInventoryLocations(_ context.Context, _ string) ([]persistence.InventoryLocation, error) {
	return nil, nil
}
func (f *resolverFakeStore) RecordPurchase(_ context.Context, _ domain.IngredientID, _ *domain.ProductID, _ domain.InventoryLocationID, _ float64, _ string, _ *time.Time, _ string) (domain.InventoryLotID, error) {
	return domain.InventoryLotID{}, nil
}
func (f *resolverFakeStore) RecordConsume(_ context.Context, _ domain.InventoryLotID, _ float64, _ bool, _ string) error { return nil }
func (f *resolverFakeStore) RecordDiscard(_ context.Context, _ domain.InventoryLotID, _ float64, _ bool, _, _ string) error {
	return nil
}
func (f *resolverFakeStore) RecordAdjust(_ context.Context, _ domain.InventoryLotID, _ float64, _ bool, _, _ string) error {
	return nil
}
func (f *resolverFakeStore) RecordMarkEmpty(_ context.Context, _ domain.InventoryLotID) error { return nil }
func (f *resolverFakeStore) RecordOpen(_ context.Context, _ domain.InventoryLotID, _ string) error { return nil }
func (f *resolverFakeStore) RecordTransfer(_ context.Context, _ domain.InventoryLotID, _ domain.InventoryLocationID, _ float64, _ string) (domain.InventoryLotID, error) {
	return domain.InventoryLotID{}, nil
}
func (f *resolverFakeStore) GetInventoryLot(_ context.Context, _ domain.InventoryLotID) (persistence.InventoryLot, error) {
	return persistence.InventoryLot{}, persistence.ErrNoRows
}
func (f *resolverFakeStore) ListMealEvents(_ context.Context, _ domain.RecipeRefID, _ string) ([]persistence.MealEvent, error) {
	return nil, nil
}
func (f *resolverFakeStore) GetMealEvent(_ context.Context, _ domain.MealEventID) (persistence.MealEvent, error) {
	return persistence.MealEvent{}, persistence.ErrNoRows
}
func (f *resolverFakeStore) ListMealPlans(_ context.Context) ([]persistence.MealPlan, error) { return nil, nil }
func (f *resolverFakeStore) GetIngredientMapping(_ context.Context, _ string) (persistence.IngredientMapping, error) {
	return persistence.IngredientMapping{}, persistence.ErrNoRows
}
func (f *resolverFakeStore) UpsertIngredientAlias(_ context.Context, _ persistence.IngredientAlias) error { return nil }
func (f *resolverFakeStore) GetIngredientAlias(_ context.Context, _, _ string) (persistence.IngredientAlias, error) {
	return persistence.IngredientAlias{}, persistence.ErrNoRows
}
func (f *resolverFakeStore) ListIngredientAliases(_ context.Context, _ string) ([]persistence.IngredientAlias, error) {
	return nil, nil
}
func (f *resolverFakeStore) DeleteIngredientAlias(_ context.Context, _, _ string) error { return nil }
func (f *resolverFakeStore) ResolveIngredientAlias(_ context.Context, _, _ string) (string, error) {
	return "", persistence.ErrNoRows
}
func (f *resolverFakeStore) ListAllStores(_ context.Context) ([]domain.Store, error) { return nil, nil }
func (f *resolverFakeStore) ListStoreProductOffers(_ context.Context, _ domain.StoreID) ([]domain.StoreProductOffer, error) {
	return nil, nil
}
func (f *resolverFakeStore) CreateRecipeFamily(_ context.Context, _ persistence.RecipeFamily) error { return nil }
func (f *resolverFakeStore) GetRecipeFamilyBySlug(_ context.Context, _ string) (persistence.RecipeFamily, error) {
	return persistence.RecipeFamily{}, persistence.ErrNoRows
}
func (f *resolverFakeStore) ListRecipeFamilies(_ context.Context) ([]persistence.RecipeFamily, error) { return nil, nil }
func (f *resolverFakeStore) SetRecipeFamilyDefaultVariant(_ context.Context, _ domain.RecipeFamilyID, _ domain.RecipeVariantID) error {
	return nil
}
func (f *resolverFakeStore) CreateRecipeVariant(_ context.Context, _ persistence.RecipeVariant) error { return nil }
func (f *resolverFakeStore) GetRecipeVariant(_ context.Context, _ domain.RecipeVariantID) (persistence.RecipeVariant, error) {
	return persistence.RecipeVariant{}, persistence.ErrNoRows
}
func (f *resolverFakeStore) ListRecipeVariants(_ context.Context, _ domain.RecipeFamilyID) ([]persistence.RecipeVariant, error) {
	return nil, nil
}
func (f *resolverFakeStore) CreateRecipeRevision(_ context.Context, _ persistence.RecipeRevision) (domain.RecipeRevisionID, error) {
	return domain.NewRecipeRevisionID(), nil
}
func (f *resolverFakeStore) GetRecipeRevision(_ context.Context, _ domain.RecipeRevisionID) (persistence.RecipeRevision, error) {
	return persistence.RecipeRevision{}, persistence.ErrNoRows
}
func (f *resolverFakeStore) AddRecipeRevisionParent(_ context.Context, _, _ domain.RecipeRevisionID) error { return nil }
func (f *resolverFakeStore) ListRecipeRevisionParents(_ context.Context, _ domain.RecipeRevisionID) ([]domain.RecipeRevisionID, error) {
	return nil, nil
}
func (f *resolverFakeStore) UpsertFavorite(_ context.Context, _, _ string, _ domain.RecipeRefID) error { return nil }
func (f *resolverFakeStore) DeleteFavorite(_ context.Context, _, _ string, _ domain.RecipeRefID) error {
	return nil
}
func (f *resolverFakeStore) ListFavoritesForRecipe(_ context.Context, _ domain.RecipeRefID) ([]persistence.Favorite, error) {
	return nil, nil
}
func (f *resolverFakeStore) GetRecipeRating(_ context.Context, _ domain.RecipeRefID) (persistence.RecipeRating, error) {
	return persistence.RecipeRating{}, persistence.ErrNoRows
}
func (f *resolverFakeStore) ListRetailers(_ context.Context) ([]domain.Retailer, error) { return nil, nil }
func (f *resolverFakeStore) ListStores(_ context.Context, _ domain.RetailerID) ([]domain.Store, error) {
	return nil, nil
}
func (f *resolverFakeStore) ListRetailerProducts(_ context.Context, _ domain.RetailerID) ([]domain.RetailerProduct, error) {
	return nil, nil
}
func (f *resolverFakeStore) ListCurrentPrices(_ context.Context) ([]domain.CurrentStoreProductPrice, error) {
	return nil, nil
}
func (f *resolverFakeStore) PriceObservationsForProduct(_ context.Context, _ domain.RetailerProductID) ([]domain.PriceObservation, error) {
	return nil, nil
}
func (f *resolverFakeStore) PriceObservationsForStore(_ context.Context, _ domain.StoreID) ([]domain.PriceObservation, error) {
	return nil, nil
}
func (f *resolverFakeStore) ListExpiringLots(_ context.Context, _ time.Duration) ([]persistence.InventoryLot, error) {
	return nil, nil
}
func (f *resolverFakeStore) ListPantryIngredientIDs(_ context.Context) ([]domain.IngredientID, error) { return nil, nil }
func (f *resolverFakeStore) ListAllRecipeIngredients(_ context.Context) ([]persistence.RecipeIngredient, error) {
	return nil, nil
}
func (f *resolverFakeStore) GetExternalRecipeSource(_ context.Context, _ string) (persistence.ExternalRecipeSource, error) {
	return persistence.ExternalRecipeSource{}, persistence.ErrNoRows
}
func (f *resolverFakeStore) UpsertExternalRecipeSource(_ context.Context, _ persistence.ExternalRecipeSource) error {
	return nil
}
func (f *resolverFakeStore) SaveImportCandidate(_ context.Context, c persistence.ImportCandidate) (domain.RecipeImportCandidateID, error) {
	return c.ID, nil
}
func (f *resolverFakeStore) SaveCandidateIngredients(_ context.Context, _ domain.RecipeImportCandidateID, _ []persistence.ImportCandidateIngredient) error {
	return nil
}
func (f *resolverFakeStore) GetImportCandidate(_ context.Context, _ domain.RecipeImportCandidateID) (persistence.ImportCandidate, error) {
	return persistence.ImportCandidate{}, persistence.ErrNoRows
}
func (f *resolverFakeStore) ListImportCandidates(_ context.Context, _ *string) ([]persistence.ImportCandidate, error) {
	return nil, nil
}
func (f *resolverFakeStore) ListCandidateIngredients(_ context.Context, _ domain.RecipeImportCandidateID) ([]persistence.ImportCandidateIngredient, error) {
	return nil, nil
}
func (f *resolverFakeStore) SetCandidateStatus(_ context.Context, _ domain.RecipeImportCandidateID, _ string) error {
	return nil
}
func (f *resolverFakeStore) SetCandidatePromoted(_ context.Context, _ domain.RecipeImportCandidateID, _ domain.RecipeVariantID) error {
	return nil
}
func (f *resolverFakeStore) UpsertRecipeSourceRef(_ context.Context, _ persistence.RecipeSourceRef) error { return nil }
func (f *resolverFakeStore) ListUnmappedMealieRecipes(_ context.Context) ([]string, error) { return nil, nil }

func TestPlanWeek_DualMode_UsesNativeFirst(t *testing.T) {
	famID := domain.NewRecipeFamilyID()
	variantID := domain.NewRecipeVariantID()
	revID := domain.NewRecipeRevisionID()
	planID := domain.NewMealPlanID()

	store := &resolverFakeStore{
		planID: planID,
		recipeRefs: []persistence.RecipeRef{
			{ID: domain.NewRecipeRefID(), MealieRecipeID: "pasta", Title: "Pasta Bolognese", Effort: 1},
		},
		srcRefs: map[string]persistence.RecipeSourceRef{
			"mealie:pasta": {RecipeFamilyID: famID, Source: "mealie", SourceRecipeID: "pasta"},
		},
		families: map[domain.RecipeFamilyID]persistence.RecipeFamily{
			famID: {ID: famID, Slug: "pasta", Name: "Pasta Bolognese", DefaultVariantID: variantID},
		},
		variants: map[domain.RecipeVariantID]persistence.RecipeVariant{
			variantID: {ID: variantID, FamilyID: famID, Title: "Default"},
		},
		revisions: map[domain.RecipeVariantID][]persistence.RecipeRevision{
			variantID: {{ID: revID, VariantID: variantID, Ingredients: []domain.Ingredient{
				{IngredientID: "pasta", Quantity: 400, Unit: "g"},
			}}},
		},
	}
	t.Setenv("RECIPE_SOURCE", "dual")
	svc := NewPlanning(store, nil)

	res, err := svc.PlanWeek(context.Background(), PlanWeekInput{
		WeekStart: time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC),
		Days:      1,
		People:    []domain.Person{{ID: domain.NewPersonID(), Name: "Test", Weight: 1.0}},
	})
	if err != nil {
		t.Fatalf("PlanWeek: %v", err)
	}
	if len(res.Planned) == 0 {
		t.Fatal("expected planned slots in dual mode")
	}
	if !res.Persisted {
		t.Error("expected plan to be persisted")
	}
	if len(store.candidates) != len(res.Planned) {
		t.Errorf("candidates = %d, want %d", len(store.candidates), len(res.Planned))
	}
}

func TestPlanWeek_NativeMode_UsesNative(t *testing.T) {
	famID := domain.NewRecipeFamilyID()
	variantID := domain.NewRecipeVariantID()
	planID := domain.NewMealPlanID()

	store := &resolverFakeStore{
		planID: planID,
		recipeRefs: []persistence.RecipeRef{
			{ID: domain.NewRecipeRefID(), MealieRecipeID: "tacos", Title: "Tacos", Effort: 2},
		},
		srcRefs: map[string]persistence.RecipeSourceRef{
			"mealie:tacos": {RecipeFamilyID: famID, Source: "mealie", SourceRecipeID: "tacos"},
		},
		families: map[domain.RecipeFamilyID]persistence.RecipeFamily{
			famID: {ID: famID, Slug: "tacos", Name: "Tacos", DefaultVariantID: variantID},
		},
		variants: map[domain.RecipeVariantID]persistence.RecipeVariant{
			variantID: {ID: variantID, FamilyID: famID, Title: "Default"},
		},
		revisions: map[domain.RecipeVariantID][]persistence.RecipeRevision{
			variantID: {{ID: domain.NewRecipeRevisionID(), VariantID: variantID, Ingredients: []domain.Ingredient{
				{IngredientID: "tortilla", Quantity: 4, Unit: "st"},
			}}},
		},
	}
	t.Setenv("RECIPE_SOURCE", "native")
	svc := NewPlanning(store, nil)

	res, err := svc.PlanWeek(context.Background(), PlanWeekInput{
		WeekStart: time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC),
		Days:      1,
		People:    []domain.Person{{ID: domain.NewPersonID(), Name: "Test", Weight: 1.0}},
	})
	if err != nil {
		t.Fatalf("PlanWeek: %v", err)
	}
	if len(res.Planned) == 0 {
		t.Fatal("expected planned slots in native mode")
	}
}

func TestPlanWeek_NativeMode_UnmappedRecipeIsSurface(t *testing.T) {
	planID := domain.NewMealPlanID()

	store := &resolverFakeStore{
		planID: planID,
		recipeRefs: []persistence.RecipeRef{
			{ID: domain.NewRecipeRefID(), MealieRecipeID: "unknown", Title: "Unknown", Effort: 1},
		},
		// No source ref for "unknown" — it is unmapped.
	}
	t.Setenv("RECIPE_SOURCE", "native")
	svc := NewPlanning(store, nil)

	// In native mode, an unmapped recipe should cause ResolveRecipe to return
	// Source="unmapped", and ResolveRecipeRef to error. PlanWeek should fail
	// because the unmapped recipe cannot be persisted.
	_, err := svc.PlanWeek(context.Background(), PlanWeekInput{
		WeekStart: time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC),
		Days:      1,
		People:    []domain.Person{{ID: domain.NewPersonID(), Name: "Test", Weight: 1.0}},
	})
	if err == nil {
		t.Fatal("expected error for unmapped recipe in native mode")
	}
	if !strings.Contains(err.Error(), "unmapped") {
		t.Errorf("expected error to mention 'unmapped', got: %v", err)
	}
}

func TestPlanWeek_MealieMode_FallsBackToMealie(t *testing.T) {
	fakeMealie := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/recipes":
			w.Write([]byte(`{"page":1,"perPage":50,"total":1,"items":[
				{"id":"r-pasta","slug":"pasta","name":"Pasta"}]}`))
		case "/api/recipes/pasta":
			w.Write([]byte(`{"id":"r-pasta","slug":"pasta","name":"Pasta","totalTime":"20 min",
				"tags":[{"name":"pasta"}],
				"recipeIngredient":[{"quantity":400,"unit":{"name":"g"},"food":{"id":"f1","name":"köttfärs"}}]}`))
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(fakeMealie.Close)

	planID := domain.NewMealPlanID()
	store := &resolverFakeStore{planID: planID}
	svc := NewPlanning(store, mealie.New(fakeMealie.URL, "tok"))

	res, err := svc.PlanWeek(context.Background(), PlanWeekInput{
		WeekStart: time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC),
		Days:      1,
		People:    []domain.Person{{ID: domain.NewPersonID(), Name: "Test", Weight: 1.0}},
	})
	if err != nil {
		t.Fatalf("PlanWeek: %v", err)
	}
	if len(res.Planned) == 0 {
		t.Fatal("expected planned slots in mealie mode")
	}
}
