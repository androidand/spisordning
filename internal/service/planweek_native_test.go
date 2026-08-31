package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/androidand/spisordning/internal/persistence"
	"github.com/androidand/spisordning/internal/service"
)

// planStore implements service.Store for the native planning integration test.
type planStore struct {
	refs          []persistence.RecipeRef
	recipeIngs    map[domain.RecipeRefID][]persistence.RecipeIngredient
	families      []persistence.RecipeFamily
	variants      []persistence.RecipeVariant
	revisions     []persistence.RecipeRevision
	sourceRefs    []persistence.RecipeSourceRef
	mealPlan      persistence.MealPlan
	mealPlanSet   bool
	candidates    []persistence.MealPlanCandidate
	decisions     []persistence.MealPlanDecision
	shopReqs      []persistence.ShoppingRequirement
}

func (s *planStore) ListRecipeRefs(_ context.Context) ([]persistence.RecipeRef, error) {
	return s.refs, nil
}

func (s *planStore) GetRecipeRef(_ context.Context, id domain.RecipeRefID) (persistence.RecipeRef, error) {
	for _, r := range s.refs {
		if r.ID == id {
			return r, nil
		}
	}
	return persistence.RecipeRef{}, persistence.ErrNoRows
}

func (s *planStore) GetRecipeRefByMealieID(_ context.Context, id string) (persistence.RecipeRef, error) {
	for _, r := range s.refs {
		if r.MealieRecipeID == id {
			return r, nil
		}
	}
	return persistence.RecipeRef{}, persistence.ErrNoRows
}

func (s *planStore) UpsertRecipeRef(_ context.Context, _ persistence.RecipeRef) error { return nil }

func (s *planStore) ListRecipeIngredients(_ context.Context, id domain.RecipeRefID) ([]persistence.RecipeIngredient, error) {
	return s.recipeIngs[id], nil
}

func (s *planStore) GetRecipeFamily(_ context.Context, id domain.RecipeFamilyID) (persistence.RecipeFamily, error) {
	for _, f := range s.families {
		if f.ID == id {
			return f, nil
		}
	}
	return persistence.RecipeFamily{}, persistence.ErrNoRows
}

func (s *planStore) GetRecipeFamilyBySlug(_ context.Context, slug string) (persistence.RecipeFamily, error) {
	for _, f := range s.families {
		if f.Slug == slug {
			return f, nil
		}
	}
	return persistence.RecipeFamily{}, persistence.ErrNoRows
}

func (s *planStore) CreateRecipeFamily(_ context.Context, f persistence.RecipeFamily) error {
	s.families = append(s.families, f)
	return nil
}

func (s *planStore) ListRecipeFamilies(_ context.Context) ([]persistence.RecipeFamily, error) {
	return s.families, nil
}

func (s *planStore) SetRecipeFamilyDefaultVariant(_ context.Context, _ domain.RecipeFamilyID, _ domain.RecipeVariantID) error {
	return nil
}

func (s *planStore) CreateRecipeVariant(_ context.Context, v persistence.RecipeVariant) error {
	s.variants = append(s.variants, v)
	return nil
}

func (s *planStore) GetRecipeVariant(_ context.Context, _ domain.RecipeVariantID) (persistence.RecipeVariant, error) {
	return persistence.RecipeVariant{}, persistence.ErrNoRows
}

func (s *planStore) ListRecipeVariants(_ context.Context, _ domain.RecipeFamilyID) ([]persistence.RecipeVariant, error) {
	return nil, nil
}

func (s *planStore) CreateRecipeRevision(_ context.Context, r persistence.RecipeRevision) (domain.RecipeRevisionID, error) {
	s.revisions = append(s.revisions, r)
	return r.ID, nil
}

func (s *planStore) GetRecipeRevision(_ context.Context, _ domain.RecipeRevisionID) (persistence.RecipeRevision, error) {
	return persistence.RecipeRevision{}, persistence.ErrNoRows
}

func (s *planStore) ListRecipeRevisions(_ context.Context, _ domain.RecipeVariantID) ([]persistence.RecipeRevision, error) {
	return nil, nil
}

func (s *planStore) AddRecipeRevisionParent(_ context.Context, _, _ domain.RecipeRevisionID) error {
	return nil
}

func (s *planStore) ListRecipeRevisionParents(_ context.Context, _ domain.RecipeRevisionID) ([]domain.RecipeRevisionID, error) {
	return nil, nil
}

func (s *planStore) GetRecipeSourceRefBySource(_ context.Context, source, recipeID string) (persistence.RecipeSourceRef, error) {
	for _, r := range s.sourceRefs {
		if r.Source == source && r.SourceRecipeID == recipeID {
			return r, nil
		}
	}
	return persistence.RecipeSourceRef{}, persistence.ErrNoRows
}

func (s *planStore) GetRecipeSourceRefByFamily(_ context.Context, id domain.RecipeFamilyID) (persistence.RecipeSourceRef, error) {
	for _, r := range s.sourceRefs {
		if r.RecipeFamilyID == id {
			return r, nil
		}
	}
	return persistence.RecipeSourceRef{}, persistence.ErrNoRows
}

func (s *planStore) UpsertRecipeSourceRef(_ context.Context, r persistence.RecipeSourceRef) error {
	s.sourceRefs = append(s.sourceRefs, r)
	return nil
}

func (s *planStore) ListUnmappedMealieRecipes(_ context.Context) ([]string, error) {
	return nil, nil
}

func (s *planStore) GetOrCreateMealPlan(_ context.Context, weekStart time.Time) (persistence.MealPlan, error) {
	if s.mealPlanSet {
		return s.mealPlan, nil
	}
	s.mealPlan = persistence.MealPlan{
		ID: domain.NewMealPlanID(), WeekStart: weekStart, Status: "draft", CreatedAt: time.Now(),
	}
	s.mealPlanSet = true
	return s.mealPlan, nil
}

func (s *planStore) GetMealPlan(_ context.Context, id domain.MealPlanID) (persistence.MealPlan, error) {
	if s.mealPlan.ID == id {
		return s.mealPlan, nil
	}
	return persistence.MealPlan{}, persistence.ErrNoRows
}

func (s *planStore) ListMealPlans(_ context.Context) ([]persistence.MealPlan, error) {
	if s.mealPlanSet {
		return []persistence.MealPlan{s.mealPlan}, nil
	}
	return nil, nil
}

func (s *planStore) SetMealPlanStatus(_ context.Context, id domain.MealPlanID, status string) error {
	if s.mealPlan.ID == id {
		s.mealPlan.Status = status
	}
	return nil
}

func (s *planStore) InsertCandidate(_ context.Context, c persistence.MealPlanCandidate) error {
	s.candidates = append(s.candidates, c)
	return nil
}

func (s *planStore) ListCandidates(_ context.Context, _ domain.MealPlanID) ([]persistence.MealPlanCandidate, error) {
	return s.candidates, nil
}

func (s *planStore) SetDecision(_ context.Context, d persistence.MealPlanDecision) error {
	s.decisions = append(s.decisions, d)
	return nil
}

func (s *planStore) ListDecisions(_ context.Context, _ domain.MealPlanID) ([]persistence.MealPlanDecision, error) {
	return s.decisions, nil
}

func (s *planStore) InsertShoppingRequirement(_ context.Context, r persistence.ShoppingRequirement) error {
	s.shopReqs = append(s.shopReqs, r)
	return nil
}

func (s *planStore) ListShoppingRequirements(_ context.Context, _ domain.MealPlanID) ([]persistence.ShoppingRequirement, error) {
	return s.shopReqs, nil
}

// Stub methods for the rest of the Store interface.
func (s *planStore) CreatePerson(_ context.Context, _ persistence.Person) error { return nil }
func (s *planStore) UpdatePerson(_ context.Context, _ persistence.Person) error { return nil }
func (s *planStore) GetPerson(_ context.Context, _ string) (persistence.Person, error) {
	return persistence.Person{}, persistence.ErrNoRows
}
func (s *planStore) ListPeople(_ context.Context) ([]persistence.Person, error) { return nil, nil }
func (s *planStore) UpsertPreference(_ context.Context, _ persistence.PersonPreference) error {
	return nil
}
func (s *planStore) ListPreferences(_ context.Context, _ domain.PersonID) ([]persistence.PersonPreference, error) {
	return nil, nil
}
func (s *planStore) RecordObservation(_ context.Context, _ persistence.PreferenceObservation) error {
	return nil
}
func (s *planStore) CreateMealEvent(_ context.Context, _ domain.RecipeRefID, _ time.Time, _ *domain.MealPlanID, _ *time.Time) (domain.MealEventID, error) {
	return domain.MealEventID{}, nil
}
func (s *planStore) AddMealReaction(_ context.Context, _ persistence.MealReaction) error { return nil }
func (s *planStore) ListMealReactions(_ context.Context, _ domain.MealEventID) ([]persistence.MealReaction, error) {
	return nil, nil
}
func (s *planStore) UpsertIngredientMapping(_ context.Context, _ persistence.IngredientMapping) error {
	return nil
}
func (s *planStore) UpsertIngredient(_ context.Context, _ persistence.Ingredient) error { return nil }
func (s *planStore) AddRecipeIngredient(_ context.Context, _ persistence.RecipeIngredient) error {
	return nil
}
func (s *planStore) BeginTx(_ context.Context) (persistence.Tx, error) { return nil, nil }
func (s *planStore) CreateInventoryLocation(_ context.Context, _ persistence.InventoryLocation) error {
	return nil
}
func (s *planStore) GetInventoryLocation(_ context.Context, _ domain.InventoryLocationID) (persistence.InventoryLocation, error) {
	return persistence.InventoryLocation{}, persistence.ErrNoRows
}
func (s *planStore) ListLotsUnderLocation(_ context.Context, _ domain.InventoryLocationID) ([]persistence.InventoryLot, error) {
	return nil, nil
}
func (s *planStore) ListInventoryLocations(_ context.Context, _ string) ([]persistence.InventoryLocation, error) {
	return nil, nil
}
func (s *planStore) RecordPurchase(_ context.Context, _ domain.IngredientID, _ *domain.ProductID, _ domain.InventoryLocationID, _ float64, _ string, _ *time.Time, _ string) (domain.InventoryLotID, error) {
	return domain.InventoryLotID{}, nil
}
func (s *planStore) RecordConsume(_ context.Context, _ domain.InventoryLotID, _ float64, _ bool, _ string) error {
	return nil
}
func (s *planStore) RecordDiscard(_ context.Context, _ domain.InventoryLotID, _ float64, _ bool, _, _ string) error {
	return nil
}
func (s *planStore) RecordAdjust(_ context.Context, _ domain.InventoryLotID, _ float64, _ bool, _, _ string) error {
	return nil
}
func (s *planStore) RecordMarkEmpty(_ context.Context, _ domain.InventoryLotID) error { return nil }
func (s *planStore) RecordOpen(_ context.Context, _ domain.InventoryLotID, _ string) error {
	return nil
}
func (s *planStore) RecordTransfer(_ context.Context, _ domain.InventoryLotID, _ domain.InventoryLocationID, _ float64, _ string) (domain.InventoryLotID, error) {
	return domain.InventoryLotID{}, nil
}
func (s *planStore) GetInventoryLot(_ context.Context, _ domain.InventoryLotID) (persistence.InventoryLot, error) {
	return persistence.InventoryLot{}, persistence.ErrNoRows
}
func (s *planStore) ListMealEvents(_ context.Context, _ domain.RecipeRefID, _ string) ([]persistence.MealEvent, error) {
	return nil, nil
}
func (s *planStore) GetMealEvent(_ context.Context, _ domain.MealEventID) (persistence.MealEvent, error) {
	return persistence.MealEvent{}, persistence.ErrNoRows
}
func (s *planStore) GetIngredientMapping(_ context.Context, _ string) (persistence.IngredientMapping, error) {
	return persistence.IngredientMapping{}, persistence.ErrNoRows
}
func (s *planStore) UpsertIngredientAlias(_ context.Context, _ persistence.IngredientAlias) error {
	return nil
}
func (s *planStore) GetIngredientAlias(_ context.Context, _, _ string) (persistence.IngredientAlias, error) {
	return persistence.IngredientAlias{}, persistence.ErrNoRows
}
func (s *planStore) ListIngredientAliases(_ context.Context, _ string) ([]persistence.IngredientAlias, error) {
	return nil, nil
}
func (s *planStore) DeleteIngredientAlias(_ context.Context, _, _ string) error { return nil }
func (s *planStore) ResolveIngredientAlias(_ context.Context, _, _ string) (string, error) {
	return "", persistence.ErrNoRows
}
func (s *planStore) ListAllStores(_ context.Context) ([]domain.Store, error) { return nil, nil }
func (s *planStore) ListStoreProductOffers(_ context.Context, _ domain.StoreID) ([]domain.StoreProductOffer, error) {
	return nil, nil
}
func (s *planStore) UpsertFavorite(_ context.Context, _, _ string, _ domain.RecipeRefID) error {
	return nil
}
func (s *planStore) DeleteFavorite(_ context.Context, _, _ string, _ domain.RecipeRefID) error {
	return nil
}
func (s *planStore) ListFavoritesForRecipe(_ context.Context, _ domain.RecipeRefID) ([]persistence.Favorite, error) {
	return nil, nil
}
func (s *planStore) GetRecipeRating(_ context.Context, _ domain.RecipeRefID) (persistence.RecipeRating, error) {
	return persistence.RecipeRating{}, persistence.ErrNoRows
}
func (s *planStore) ListRetailers(_ context.Context) ([]domain.Retailer, error) { return nil, nil }
func (s *planStore) ListStores(_ context.Context, _ domain.RetailerID) ([]domain.Store, error) {
	return nil, nil
}
func (s *planStore) ListRetailerProducts(_ context.Context, _ domain.RetailerID) ([]domain.RetailerProduct, error) {
	return nil, nil
}
func (s *planStore) ListCurrentPrices(_ context.Context) ([]domain.CurrentStoreProductPrice, error) {
	return nil, nil
}
func (s *planStore) PriceObservationsForProduct(_ context.Context, _ domain.RetailerProductID) ([]domain.PriceObservation, error) {
	return nil, nil
}
func (s *planStore) PriceObservationsForStore(_ context.Context, _ domain.StoreID) ([]domain.PriceObservation, error) {
	return nil, nil
}
func (s *planStore) ListExpiringLots(_ context.Context, _ time.Duration) ([]persistence.InventoryLot, error) {
	return nil, nil
}
func (s *planStore) ListPantryIngredientIDs(_ context.Context) ([]domain.IngredientID, error) {
	return nil, nil
}
func (s *planStore) ListAllRecipeIngredients(_ context.Context) ([]persistence.RecipeIngredient, error) {
	return nil, nil
}
func (s *planStore) GetExternalRecipeSource(_ context.Context, _ string) (persistence.ExternalRecipeSource, error) {
	return persistence.ExternalRecipeSource{}, persistence.ErrNoRows
}
func (s *planStore) UpsertExternalRecipeSource(_ context.Context, _ persistence.ExternalRecipeSource) error {
	return nil
}
func (s *planStore) SaveImportCandidate(_ context.Context, _ persistence.ImportCandidate) (domain.RecipeImportCandidateID, error) {
	return domain.RecipeImportCandidateID{}, nil
}
func (s *planStore) SaveCandidateIngredients(_ context.Context, _ domain.RecipeImportCandidateID, _ []persistence.ImportCandidateIngredient) error {
	return nil
}
func (s *planStore) GetImportCandidate(_ context.Context, _ domain.RecipeImportCandidateID) (persistence.ImportCandidate, error) {
	return persistence.ImportCandidate{}, persistence.ErrNoRows
}
func (s *planStore) ListImportCandidates(_ context.Context, _ *string) ([]persistence.ImportCandidate, error) {
	return nil, nil
}
func (s *planStore) ListCandidateIngredients(_ context.Context, _ domain.RecipeImportCandidateID) ([]persistence.ImportCandidateIngredient, error) {
	return nil, nil
}
func (s *planStore) SetCandidateStatus(_ context.Context, _ domain.RecipeImportCandidateID, _ string) error {
	return nil
}
func (s *planStore) SetCandidatePromoted(_ context.Context, _ domain.RecipeImportCandidateID, _ domain.RecipeVariantID) error {
	return nil
}
func (s *planStore) ListEffortProfiles(_ context.Context) ([]persistence.EffortProfile, error) {
	return nil, nil
}
func (s *planStore) CreateShoppingListWithItems(_ context.Context, _ persistence.ShoppingList, _ []persistence.ShoppingListItem) (domain.ShoppingListID, []persistence.ShoppingListItem, error) {
	return domain.ShoppingListID{}, nil, nil
}
func (s *planStore) CreateOrUpdateRetailerListBinding(_ context.Context, _ persistence.RetailerListBinding) error {
	return nil
}

func TestPlanWeek_NativeMode_NoMealie(t *testing.T) {
	famA := domain.NewRecipeFamilyID()
	famB := domain.NewRecipeFamilyID()
	ref1 := persistence.RecipeRef{
		ID: domain.NewRecipeRefID(), MealieRecipeID: "recipe-a", Title: "Recipe A",
		Effort: 2,
	}
	ref2 := persistence.RecipeRef{
		ID: domain.NewRecipeRefID(), MealieRecipeID: "recipe-b", Title: "Recipe B",
		Effort: 1,
	}
	store := &planStore{
		refs: []persistence.RecipeRef{ref1, ref2},
		families: []persistence.RecipeFamily{
			{ID: famA, Slug: "recipe-a", Name: "Recipe A"},
			{ID: famB, Slug: "recipe-b", Name: "Recipe B"},
		},
		sourceRefs: []persistence.RecipeSourceRef{
			{RecipeFamilyID: famA, Source: "mealie", SourceRecipeID: "recipe-a"},
			{RecipeFamilyID: famB, Source: "mealie", SourceRecipeID: "recipe-b"},
		},
		recipeIngs: map[domain.RecipeRefID][]persistence.RecipeIngredient{
			ref1.ID: {
				{IngredientID: domain.IngredientIDForName("tomato"), Quantity: 2, Unit: "pcs"},
				{IngredientID: domain.IngredientIDForName("onion"), Quantity: 1, Unit: "pcs"},
			},
			ref2.ID: {
				{IngredientID: domain.IngredientIDForName("rice"), Quantity: 200, Unit: "g"},
			},
		},
	}

	// Construct Planning with native mode (no Mealie client).
	p := service.NewPlanning(store, nil)
	// Override the resolver to use native mode.
	// NewPlanning uses RecipeSourceModeFromEnv() which defaults to mealie,
	// so we need to set RECIPE_SOURCE=native for this test.
	t.Setenv("RECIPE_SOURCE", "native")
	p = service.NewPlanning(store, nil)

	weekStart := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC) // Monday
	res, err := p.PlanWeek(context.Background(), service.PlanWeekInput{
		WeekStart: weekStart,
		Days:      7,
		People:    []domain.Person{{ID: domain.NewPersonID(), Name: "Test", Weight: 1}},
		EnergyFor: func(time.Time) domain.Effort { return domain.EffortMedium },
	})
	if err != nil {
		t.Fatalf("PlanWeek: %v", err)
	}
	if len(res.Planned) == 0 {
		t.Fatal("expected at least one planned slot")
	}
	if !res.Persisted {
		t.Fatalf("expected plan to be persisted, got PersistError: %v", res.PersistError)
	}
	if len(store.candidates) == 0 {
		t.Fatal("expected candidates to be persisted")
	}
}

func TestPlanWeek_NativeMode_NoRecipes(t *testing.T) {
	store := &planStore{}
	t.Setenv("RECIPE_SOURCE", "native")
	p := service.NewPlanning(store, nil)

	_, err := p.PlanWeek(context.Background(), service.PlanWeekInput{
		WeekStart: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC),
		Days:      7,
	})
	if err == nil {
		t.Fatal("expected error when no recipes in database")
	}
}
