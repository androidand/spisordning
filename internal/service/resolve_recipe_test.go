package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/androidand/spisordning/internal/persistence"
	"github.com/androidand/spisordning/internal/service"
)

func TestRecipeSourceModeFromEnv_Default(t *testing.T) {
	t.Setenv("RECIPE_SOURCE", "")
	if got := service.RecipeSourceModeFromEnv(); got != service.SourceNative {
		t.Fatalf("expected default=native, got %q", got)
	}
}

func TestRecipeSourceModeFromEnv_Mealie(t *testing.T) {
	t.Setenv("RECIPE_SOURCE", "mealie")
	if got := service.RecipeSourceModeFromEnv(); got != service.SourceMealie {
		t.Fatalf("expected mealie, got %q", got)
	}
}

func TestRecipeSourceModeFromEnv_Dual(t *testing.T) {
	t.Setenv("RECIPE_SOURCE", "dual")
	if got := service.RecipeSourceModeFromEnv(); got != service.SourceDual {
		t.Fatalf("expected dual, got %q", got)
	}
}

// testStore implements service.Store with just the methods the resolver needs.
type testStore struct {
	family persistence.RecipeFamily
	srcRef persistence.RecipeSourceRef
	hasRef bool
	refs   []persistence.RecipeRef
}

func (s *testStore) GetRecipeFamily(_ context.Context, id domain.RecipeFamilyID) (persistence.RecipeFamily, error) {
	if s.family.ID == id {
		return s.family, nil
	}
	return persistence.RecipeFamily{}, persistence.ErrNoRows
}

func (s *testStore) GetRecipeSourceRefBySource(_ context.Context, _, _ string) (persistence.RecipeSourceRef, error) {
	if s.hasRef {
		return s.srcRef, nil
	}
	return persistence.RecipeSourceRef{}, persistence.ErrNoRows
}

func (s *testStore) GetRecipeSourceRefByFamily(_ context.Context, _ domain.RecipeFamilyID) (persistence.RecipeSourceRef, error) {
	if s.hasRef {
		return s.srcRef, nil
	}
	return persistence.RecipeSourceRef{}, persistence.ErrNoRows
}

func (s *testStore) GetRecipeRefByMealieID(_ context.Context, id string) (persistence.RecipeRef, error) {
	for _, r := range s.refs {
		if r.MealieRecipeID == id {
			return r, nil
		}
	}
	return persistence.RecipeRef{}, persistence.ErrNoRows
}

func (s *testStore) ListRecipeRefs(_ context.Context) ([]persistence.RecipeRef, error) {
	return s.refs, nil
}

func (s *testStore) GetRecipeRef(_ context.Context, id domain.RecipeRefID) (persistence.RecipeRef, error) {
	for _, r := range s.refs {
		if r.ID == id {
			return r, nil
		}
	}
	return persistence.RecipeRef{}, persistence.ErrNoRows
}

func (s *testStore) UpsertRecipeRef(_ context.Context, r persistence.RecipeRef) error { return nil }
func (s *testStore) CreatePerson(_ context.Context, p persistence.Person) error      { return nil }
func (s *testStore) UpdatePerson(_ context.Context, p persistence.Person) error      { return nil }
func (s *testStore) GetPerson(_ context.Context, id string) (persistence.Person, error) {
	return persistence.Person{}, persistence.ErrNoRows
}
func (s *testStore) ListPeople(_ context.Context) ([]persistence.Person, error) { return nil, nil }
func (s *testStore) UpsertPreference(_ context.Context, p persistence.PersonPreference) error {
	return nil
}
func (s *testStore) ListPreferences(_ context.Context, _ domain.PersonID) ([]persistence.PersonPreference, error) {
	return nil, nil
}
func (s *testStore) RecordObservation(_ context.Context, o persistence.PreferenceObservation) error {
	return nil
}
func (s *testStore) CreateMealEvent(_ context.Context, _ domain.RecipeRefID, _ time.Time, _ *domain.MealPlanID, _ *time.Time) (domain.MealEventID, error) {
	return domain.MealEventID{}, nil
}
func (s *testStore) AddMealReaction(_ context.Context, r persistence.MealReaction) error { return nil }
func (s *testStore) ListMealReactions(_ context.Context, _ domain.MealEventID) ([]persistence.MealReaction, error) {
	return nil, nil
}
func (s *testStore) GetMealPlan(_ context.Context, _ domain.MealPlanID) (persistence.MealPlan, error) {
	return persistence.MealPlan{}, persistence.ErrNoRows
}
func (s *testStore) GetOrCreateMealPlan(_ context.Context, _ time.Time) (persistence.MealPlan, error) {
	return persistence.MealPlan{}, nil
}
func (s *testStore) SetMealPlanStatus(_ context.Context, _ domain.MealPlanID, _ string) error { return nil }
func (s *testStore) InsertCandidate(_ context.Context, c persistence.MealPlanCandidate) error {
	return nil
}
func (s *testStore) ListCandidates(_ context.Context, _ domain.MealPlanID) ([]persistence.MealPlanCandidate, error) {
	return nil, nil
}
func (s *testStore) SetDecision(_ context.Context, d persistence.MealPlanDecision) error { return nil }
func (s *testStore) ListDecisions(_ context.Context, _ domain.MealPlanID) ([]persistence.MealPlanDecision, error) {
	return nil, nil
}
func (s *testStore) InsertShoppingRequirement(_ context.Context, r persistence.ShoppingRequirement) error {
	return nil
}
func (s *testStore) ListShoppingRequirements(_ context.Context, _ domain.MealPlanID) ([]persistence.ShoppingRequirement, error) {
	return nil, nil
}
func (s *testStore) UpsertIngredientMapping(_ context.Context, m persistence.IngredientMapping) error {
	return nil
}
func (s *testStore) UpsertIngredient(_ context.Context, i persistence.Ingredient) error { return nil }
func (s *testStore) AddRecipeIngredient(_ context.Context, ri persistence.RecipeIngredient) error {
	return nil
}
func (s *testStore) BeginTx(_ context.Context) (persistence.Tx, error) { return nil, nil }
func (s *testStore) CreateInventoryLocation(_ context.Context, l persistence.InventoryLocation) error {
	return nil
}
func (s *testStore) GetInventoryLocation(_ context.Context, _ domain.InventoryLocationID) (persistence.InventoryLocation, error) {
	return persistence.InventoryLocation{}, persistence.ErrNoRows
}
func (s *testStore) ListLotsUnderLocation(_ context.Context, _ domain.InventoryLocationID) ([]persistence.InventoryLot, error) {
	return nil, nil
}
func (s *testStore) ListInventoryLocations(_ context.Context, _ string) ([]persistence.InventoryLocation, error) {
	return nil, nil
}
func (s *testStore) RecordPurchase(_ context.Context, _ domain.IngredientID, _ *domain.ProductID, _ domain.InventoryLocationID, _ float64, _ string, _ *time.Time, _ string) (domain.InventoryLotID, error) {
	return domain.InventoryLotID{}, nil
}
func (s *testStore) RecordConsume(_ context.Context, _ domain.InventoryLotID, _ float64, _ bool, _ string) error {
	return nil
}
func (s *testStore) RecordDiscard(_ context.Context, _ domain.InventoryLotID, _ float64, _ bool, _, _ string) error {
	return nil
}
func (s *testStore) RecordAdjust(_ context.Context, _ domain.InventoryLotID, _ float64, _ bool, _, _ string) error {
	return nil
}
func (s *testStore) RecordMarkEmpty(_ context.Context, _ domain.InventoryLotID) error { return nil }
func (s *testStore) RecordOpen(_ context.Context, _ domain.InventoryLotID, _ string) error {
	return nil
}
func (s *testStore) RecordTransfer(_ context.Context, _ domain.InventoryLotID, _ domain.InventoryLocationID, _ float64, _ string) (domain.InventoryLotID, error) {
	return domain.InventoryLotID{}, nil
}
func (s *testStore) GetInventoryLot(_ context.Context, _ domain.InventoryLotID) (persistence.InventoryLot, error) {
	return persistence.InventoryLot{}, persistence.ErrNoRows
}
func (s *testStore) ListMealEvents(_ context.Context, _ domain.RecipeRefID, _ string) ([]persistence.MealEvent, error) {
	return nil, nil
}
func (s *testStore) GetMealEvent(_ context.Context, _ domain.MealEventID) (persistence.MealEvent, error) {
	return persistence.MealEvent{}, persistence.ErrNoRows
}
func (s *testStore) ListMealPlans(_ context.Context) ([]persistence.MealPlan, error) { return nil, nil }
func (s *testStore) GetIngredientMapping(_ context.Context, _ string) (persistence.IngredientMapping, error) {
	return persistence.IngredientMapping{}, persistence.ErrNoRows
}
func (s *testStore) UpsertIngredientAlias(_ context.Context, a persistence.IngredientAlias) error {
	return nil
}
func (s *testStore) GetIngredientAlias(_ context.Context, _, _ string) (persistence.IngredientAlias, error) {
	return persistence.IngredientAlias{}, persistence.ErrNoRows
}
func (s *testStore) ListIngredientAliases(_ context.Context, _ string) ([]persistence.IngredientAlias, error) {
	return nil, nil
}
func (s *testStore) DeleteIngredientAlias(_ context.Context, _, _ string) error { return nil }
func (s *testStore) ResolveIngredientAlias(_ context.Context, _, _ string) (string, error) {
	return "", persistence.ErrNoRows
}
func (s *testStore) ListAllStores(_ context.Context) ([]domain.Store, error) { return nil, nil }
func (s *testStore) ListStoreProductOffers(_ context.Context, _ domain.StoreID) ([]domain.StoreProductOffer, error) {
	return nil, nil
}
func (s *testStore) CreateRecipeFamily(_ context.Context, f persistence.RecipeFamily) error {
	s.family = f
	return nil
}
func (s *testStore) GetRecipeFamilyBySlug(_ context.Context, _ string) (persistence.RecipeFamily, error) {
	return persistence.RecipeFamily{}, persistence.ErrNoRows
}
func (s *testStore) ListRecipeFamilies(_ context.Context) ([]persistence.RecipeFamily, error) {
	return nil, nil
}
func (s *testStore) SetRecipeFamilyDefaultVariant(_ context.Context, _ domain.RecipeFamilyID, _ domain.RecipeVariantID) error {
	return nil
}
func (s *testStore) CreateRecipeVariant(_ context.Context, v persistence.RecipeVariant) error { return nil }
func (s *testStore) GetRecipeVariant(_ context.Context, _ domain.RecipeVariantID) (persistence.RecipeVariant, error) {
	return persistence.RecipeVariant{}, persistence.ErrNoRows
}
func (s *testStore) ListRecipeVariants(_ context.Context, _ domain.RecipeFamilyID) ([]persistence.RecipeVariant, error) {
	return nil, nil
}
func (s *testStore) CreateRecipeRevision(_ context.Context, r persistence.RecipeRevision) (domain.RecipeRevisionID, error) {
	return domain.RecipeRevisionID{}, nil
}
func (s *testStore) GetRecipeRevision(_ context.Context, _ domain.RecipeRevisionID) (persistence.RecipeRevision, error) {
	return persistence.RecipeRevision{}, persistence.ErrNoRows
}
func (s *testStore) ListRecipeRevisions(_ context.Context, _ domain.RecipeVariantID) ([]persistence.RecipeRevision, error) {
	return nil, nil
}
func (s *testStore) AddRecipeRevisionParent(_ context.Context, _, _ domain.RecipeRevisionID) error {
	return nil
}
func (s *testStore) ListRecipeRevisionParents(_ context.Context, _ domain.RecipeRevisionID) ([]domain.RecipeRevisionID, error) {
	return nil, nil
}
func (s *testStore) UpsertFavorite(_ context.Context, _, _ string, _ domain.RecipeRefID) error {
	return nil
}
func (s *testStore) DeleteFavorite(_ context.Context, _, _ string, _ domain.RecipeRefID) error {
	return nil
}
func (s *testStore) ListFavoritesForRecipe(_ context.Context, _ domain.RecipeRefID) ([]persistence.Favorite, error) {
	return nil, nil
}
func (s *testStore) GetRecipeRating(_ context.Context, _ domain.RecipeRefID) (persistence.RecipeRating, error) {
	return persistence.RecipeRating{}, persistence.ErrNoRows
}
func (s *testStore) ListRetailers(_ context.Context) ([]domain.Retailer, error) { return nil, nil }
func (s *testStore) ListStores(_ context.Context, _ domain.RetailerID) ([]domain.Store, error) {
	return nil, nil
}
func (s *testStore) ListRetailerProducts(_ context.Context, _ domain.RetailerID) ([]domain.RetailerProduct, error) {
	return nil, nil
}
func (s *testStore) ListCurrentPrices(_ context.Context) ([]domain.CurrentStoreProductPrice, error) {
	return nil, nil
}
func (s *testStore) PriceObservationsForProduct(_ context.Context, _ domain.RetailerProductID) ([]domain.PriceObservation, error) {
	return nil, nil
}
func (s *testStore) PriceObservationsForStore(_ context.Context, _ domain.StoreID) ([]domain.PriceObservation, error) {
	return nil, nil
}
func (s *testStore) ListExpiringLots(_ context.Context, _ time.Duration) ([]persistence.InventoryLot, error) {
	return nil, nil
}
func (s *testStore) ListPantryIngredientIDs(_ context.Context) ([]domain.IngredientID, error) {
	return nil, nil
}
func (s *testStore) ListAllRecipeIngredients(_ context.Context) ([]persistence.RecipeIngredient, error) {
	return nil, nil
}
func (s *testStore) GetExternalRecipeSource(_ context.Context, _ string) (persistence.ExternalRecipeSource, error) {
	return persistence.ExternalRecipeSource{}, persistence.ErrNoRows
}
func (s *testStore) UpsertExternalRecipeSource(_ context.Context, src persistence.ExternalRecipeSource) error {
	return nil
}
func (s *testStore) SaveImportCandidate(_ context.Context, c persistence.ImportCandidate) (domain.RecipeImportCandidateID, error) {
	return domain.RecipeImportCandidateID{}, nil
}
func (s *testStore) SaveCandidateIngredients(_ context.Context, _ domain.RecipeImportCandidateID, _ []persistence.ImportCandidateIngredient) error {
	return nil
}
func (s *testStore) GetImportCandidate(_ context.Context, _ domain.RecipeImportCandidateID) (persistence.ImportCandidate, error) {
	return persistence.ImportCandidate{}, persistence.ErrNoRows
}
func (s *testStore) ListImportCandidates(_ context.Context, _ *string) ([]persistence.ImportCandidate, error) {
	return nil, nil
}
func (s *testStore) ListCandidateIngredients(_ context.Context, _ domain.RecipeImportCandidateID) ([]persistence.ImportCandidateIngredient, error) {
	return nil, nil
}
func (s *testStore) SetCandidateStatus(_ context.Context, _ domain.RecipeImportCandidateID, _ string) error {
	return nil
}
func (s *testStore) SetCandidatePromoted(_ context.Context, _ domain.RecipeImportCandidateID, _ domain.RecipeVariantID) error {
	return nil
}
func (s *testStore) UpsertRecipeSourceRef(_ context.Context, r persistence.RecipeSourceRef) error {
	s.srcRef = r
	s.hasRef = true
	return nil
}
func (s *testStore) ListUnmappedMealieRecipes(_ context.Context) ([]string, error) {
	return nil, nil
}

func TestResolveRecipe_MealieMode(t *testing.T) {
	db := &testStore{}
	r := service.NewResolveRecipeResolver(db, service.SourceMealie)

	res, err := r.ResolveRecipe(context.Background(), "test-recipe")
	if err != nil {
		t.Fatalf("ResolveRecipe: %v", err)
	}
	if res.Source != "mealie" {
		t.Fatalf("expected source=mealie, got %q", res.Source)
	}
	if res.FamilyID != nil {
		t.Fatal("expected nil FamilyID in mealie mode")
	}
}

func TestResolveRecipe_DualMode_Mapped(t *testing.T) {
	famID := domain.NewRecipeFamilyID()
	fam := persistence.RecipeFamily{ID: famID, Slug: "test", Name: "Test"}
	db := &testStore{
		family: fam,
		srcRef: persistence.RecipeSourceRef{RecipeFamilyID: famID, Source: "mealie", SourceRecipeID: "test-recipe"},
		hasRef: true,
	}
	r := service.NewResolveRecipeResolver(db, service.SourceDual)

	res, err := r.ResolveRecipe(context.Background(), "test-recipe")
	if err != nil {
		t.Fatalf("ResolveRecipe: %v", err)
	}
	if res.Source != "native" {
		t.Fatalf("expected source=native, got %q", res.Source)
	}
	if res.FamilyID == nil || res.FamilyID.ID != famID {
		t.Fatal("expected FamilyID to be set")
	}
}

func TestResolveRecipe_DualMode_Unmapped(t *testing.T) {
	db := &testStore{}
	r := service.NewResolveRecipeResolver(db, service.SourceDual)

	res, err := r.ResolveRecipe(context.Background(), "unknown-recipe")
	if err != nil {
		t.Fatalf("ResolveRecipe: %v", err)
	}
	if res.Source != "mealie" {
		t.Fatalf("expected source=mealie (dual fallback), got %q", res.Source)
	}
}

func TestResolveRecipe_NativeMode_Mapped(t *testing.T) {
	famID := domain.NewRecipeFamilyID()
	fam := persistence.RecipeFamily{ID: famID, Slug: "test", Name: "Test"}
	db := &testStore{
		family: fam,
		srcRef: persistence.RecipeSourceRef{RecipeFamilyID: famID, Source: "mealie", SourceRecipeID: "test-recipe"},
		hasRef: true,
	}
	r := service.NewResolveRecipeResolver(db, service.SourceNative)

	res, err := r.ResolveRecipe(context.Background(), "test-recipe")
	if err != nil {
		t.Fatalf("ResolveRecipe: %v", err)
	}
	if res.Source != "native" {
		t.Fatalf("expected source=native, got %q", res.Source)
	}
}

func TestResolveRecipe_NativeMode_Unmapped(t *testing.T) {
	db := &testStore{}
	r := service.NewResolveRecipeResolver(db, service.SourceNative)

	res, err := r.ResolveRecipe(context.Background(), "unknown-recipe")
	if err != nil {
		t.Fatalf("ResolveRecipe: %v", err)
	}
	if res.Source != "unmapped" {
		t.Fatalf("expected source=unmapped, got %q", res.Source)
	}
}

func TestResolveRecipeRef_MealieMode(t *testing.T) {
	db := &testStore{refs: []persistence.RecipeRef{{MealieRecipeID: "test-recipe", Title: "Test"}}}
	r := service.NewResolveRecipeResolver(db, service.SourceMealie)

	ref, err := r.ResolveRecipeRef(context.Background(), "test-recipe")
	if err != nil {
		t.Fatalf("ResolveRecipeRef: %v", err)
	}
	if ref.MealieRecipeID != "test-recipe" {
		t.Fatalf("unexpected ref: %+v", ref)
	}
}

func TestResolveRecipeRef_NativeMode_Unmapped(t *testing.T) {
	db := &testStore{}
	r := service.NewResolveRecipeResolver(db, service.SourceNative)

	_, err := r.ResolveRecipeRef(context.Background(), "unknown-recipe")
	if err == nil {
		t.Fatal("expected error for unmapped recipe in native mode")
	}
}

func TestResolveRecipeRef_DualMode_Fallback(t *testing.T) {
	db := &testStore{refs: []persistence.RecipeRef{{MealieRecipeID: "test-recipe", Title: "Test"}}}
	r := service.NewResolveRecipeResolver(db, service.SourceDual)

	ref, err := r.ResolveRecipeRef(context.Background(), "test-recipe")
	if err != nil {
		t.Fatalf("ResolveRecipeRef: %v", err)
	}
	if ref.MealieRecipeID != "test-recipe" {
		t.Fatalf("unexpected ref: %+v", ref)
	}
}

func TestResolveRecipeByFamilyID(t *testing.T) {
	famID := domain.NewRecipeFamilyID()
	fam := persistence.RecipeFamily{ID: famID, Slug: "test", Name: "Test"}
	db := &testStore{
		family: fam,
		srcRef: persistence.RecipeSourceRef{RecipeFamilyID: famID, Source: "mealie", SourceRecipeID: "mealie-slug"},
		hasRef: true,
	}
	r := service.NewResolveRecipeResolver(db, service.SourceNative)

	res, err := r.ResolveRecipeByFamilyID(context.Background(), famID)
	if err != nil {
		t.Fatalf("ResolveRecipeByFamilyID: %v", err)
	}
	if res.Source != "native" {
		t.Fatalf("expected source=native, got %q", res.Source)
	}
	if res.MealieRecipeID != "mealie-slug" {
		t.Fatalf("expected mealie-slug, got %q", res.MealieRecipeID)
	}
}
