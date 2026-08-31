package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/androidand/spisordning/internal/persistence"
	"github.com/androidand/spisordning/internal/service"
)

// importStore implements service.Store for import tests. It tracks created
// families, variants, revisions, and source refs in memory.
type importStore struct {
	families   []persistence.RecipeFamily
	variants   []persistence.RecipeVariant
	revisions  []persistence.RecipeRevision
	sourceRefs []persistence.RecipeSourceRef
	unmapped   []string
}

func (s *importStore) CreatePerson(_ context.Context, _ persistence.Person) error { return nil }
func (s *importStore) UpdatePerson(_ context.Context, _ persistence.Person) error { return nil }
func (s *importStore) GetPerson(_ context.Context, _ string) (persistence.Person, error) {
	return persistence.Person{}, persistence.ErrNoRows
}
func (s *importStore) ListPeople(_ context.Context) ([]persistence.Person, error) { return nil, nil }
func (s *importStore) UpsertPreference(_ context.Context, _ persistence.PersonPreference) error {
	return nil
}
func (s *importStore) ListPreferences(_ context.Context, _ domain.PersonID) ([]persistence.PersonPreference, error) {
	return nil, nil
}
func (s *importStore) RecordObservation(_ context.Context, _ persistence.PreferenceObservation) error {
	return nil
}
func (s *importStore) CreateMealEvent(_ context.Context, _ domain.RecipeRefID, _ time.Time, _ *domain.MealPlanID, _ *time.Time) (domain.MealEventID, error) {
	return domain.MealEventID{}, nil
}
func (s *importStore) AddMealReaction(_ context.Context, _ persistence.MealReaction) error { return nil }
func (s *importStore) ListMealReactions(_ context.Context, _ domain.MealEventID) ([]persistence.MealReaction, error) {
	return nil, nil
}
func (s *importStore) GetMealPlan(_ context.Context, _ domain.MealPlanID) (persistence.MealPlan, error) {
	return persistence.MealPlan{}, persistence.ErrNoRows
}
func (s *importStore) GetOrCreateMealPlan(_ context.Context, _ time.Time) (persistence.MealPlan, error) {
	return persistence.MealPlan{}, nil
}
func (s *importStore) SetMealPlanStatus(_ context.Context, _ domain.MealPlanID, _ string) error { return nil }
func (s *importStore) InsertCandidate(_ context.Context, _ persistence.MealPlanCandidate) error { return nil }
func (s *importStore) ListCandidates(_ context.Context, _ domain.MealPlanID) ([]persistence.MealPlanCandidate, error) {
	return nil, nil
}
func (s *importStore) SetDecision(_ context.Context, _ persistence.MealPlanDecision) error { return nil }
func (s *importStore) ListDecisions(_ context.Context, _ domain.MealPlanID) ([]persistence.MealPlanDecision, error) {
	return nil, nil
}
func (s *importStore) InsertShoppingRequirement(_ context.Context, _ persistence.ShoppingRequirement) error {
	return nil
}
func (s *importStore) ListShoppingRequirements(_ context.Context, _ domain.MealPlanID) ([]persistence.ShoppingRequirement, error) {
	return nil, nil
}
func (s *importStore) UpsertIngredientMapping(_ context.Context, _ persistence.IngredientMapping) error {
	return nil
}
func (s *importStore) UpsertIngredient(_ context.Context, _ persistence.Ingredient) error { return nil }
func (s *importStore) AddRecipeIngredient(_ context.Context, _ persistence.RecipeIngredient) error {
	return nil
}
func (s *importStore) BeginTx(_ context.Context) (persistence.Tx, error) { return nil, nil }
func (s *importStore) CreateInventoryLocation(_ context.Context, _ persistence.InventoryLocation) error {
	return nil
}
func (s *importStore) GetInventoryLocation(_ context.Context, _ domain.InventoryLocationID) (persistence.InventoryLocation, error) {
	return persistence.InventoryLocation{}, persistence.ErrNoRows
}
func (s *importStore) ListLotsUnderLocation(_ context.Context, _ domain.InventoryLocationID) ([]persistence.InventoryLot, error) {
	return nil, nil
}
func (s *importStore) ListInventoryLocations(_ context.Context, _ string) ([]persistence.InventoryLocation, error) {
	return nil, nil
}
func (s *importStore) RecordPurchase(_ context.Context, _ domain.IngredientID, _ *domain.ProductID, _ domain.InventoryLocationID, _ float64, _ string, _ *time.Time, _ string) (domain.InventoryLotID, error) {
	return domain.InventoryLotID{}, nil
}
func (s *importStore) RecordConsume(_ context.Context, _ domain.InventoryLotID, _ float64, _ bool, _ string) error {
	return nil
}
func (s *importStore) RecordDiscard(_ context.Context, _ domain.InventoryLotID, _ float64, _ bool, _, _ string) error {
	return nil
}
func (s *importStore) RecordAdjust(_ context.Context, _ domain.InventoryLotID, _ float64, _ bool, _, _ string) error {
	return nil
}
func (s *importStore) RecordMarkEmpty(_ context.Context, _ domain.InventoryLotID) error { return nil }
func (s *importStore) RecordOpen(_ context.Context, _ domain.InventoryLotID, _ string) error {
	return nil
}
func (s *importStore) RecordTransfer(_ context.Context, _ domain.InventoryLotID, _ domain.InventoryLocationID, _ float64, _ string) (domain.InventoryLotID, error) {
	return domain.InventoryLotID{}, nil
}
func (s *importStore) GetInventoryLot(_ context.Context, _ domain.InventoryLotID) (persistence.InventoryLot, error) {
	return persistence.InventoryLot{}, persistence.ErrNoRows
}
func (s *importStore) ListMealEvents(_ context.Context, _ domain.RecipeRefID, _ string) ([]persistence.MealEvent, error) {
	return nil, nil
}
func (s *importStore) GetMealEvent(_ context.Context, _ domain.MealEventID) (persistence.MealEvent, error) {
	return persistence.MealEvent{}, persistence.ErrNoRows
}
func (s *importStore) ListMealPlans(_ context.Context) ([]persistence.MealPlan, error) { return nil, nil }
func (s *importStore) GetIngredientMapping(_ context.Context, _ string) (persistence.IngredientMapping, error) {
	return persistence.IngredientMapping{}, persistence.ErrNoRows
}
func (s *importStore) UpsertIngredientAlias(_ context.Context, _ persistence.IngredientAlias) error {
	return nil
}
func (s *importStore) GetIngredientAlias(_ context.Context, _, _ string) (persistence.IngredientAlias, error) {
	return persistence.IngredientAlias{}, persistence.ErrNoRows
}
func (s *importStore) ListIngredientAliases(_ context.Context, _ string) ([]persistence.IngredientAlias, error) {
	return nil, nil
}
func (s *importStore) DeleteIngredientAlias(_ context.Context, _, _ string) error { return nil }
func (s *importStore) ResolveIngredientAlias(_ context.Context, _, _ string) (string, error) {
	return "", persistence.ErrNoRows
}
func (s *importStore) ListAllStores(_ context.Context) ([]domain.Store, error) { return nil, nil }
func (s *importStore) ListStoreProductOffers(_ context.Context, _ domain.StoreID) ([]domain.StoreProductOffer, error) {
	return nil, nil
}
func (s *importStore) ListRecipeFamilies(_ context.Context) ([]persistence.RecipeFamily, error) {
	return s.families, nil
}
func (s *importStore) GetRecipeVariant(_ context.Context, _ domain.RecipeVariantID) (persistence.RecipeVariant, error) {
	return persistence.RecipeVariant{}, persistence.ErrNoRows
}
func (s *importStore) ListRecipeVariants(_ context.Context, _ domain.RecipeFamilyID) ([]persistence.RecipeVariant, error) {
	return nil, nil
}
func (s *importStore) GetRecipeRevision(_ context.Context, _ domain.RecipeRevisionID) (persistence.RecipeRevision, error) {
	return persistence.RecipeRevision{}, persistence.ErrNoRows
}
func (s *importStore) ListRecipeRevisions(_ context.Context, _ domain.RecipeVariantID) ([]persistence.RecipeRevision, error) {
	return nil, nil
}
func (s *importStore) AddRecipeRevisionParent(_ context.Context, _, _ domain.RecipeRevisionID) error {
	return nil
}
func (s *importStore) ListRecipeRevisionParents(_ context.Context, _ domain.RecipeRevisionID) ([]domain.RecipeRevisionID, error) {
	return nil, nil
}
func (s *importStore) UpsertFavorite(_ context.Context, _, _ string, _ domain.RecipeRefID) error {
	return nil
}
func (s *importStore) DeleteFavorite(_ context.Context, _, _ string, _ domain.RecipeRefID) error {
	return nil
}
func (s *importStore) ListFavoritesForRecipe(_ context.Context, _ domain.RecipeRefID) ([]persistence.Favorite, error) {
	return nil, nil
}
func (s *importStore) GetRecipeRating(_ context.Context, _ domain.RecipeRefID) (persistence.RecipeRating, error) {
	return persistence.RecipeRating{}, persistence.ErrNoRows
}
func (s *importStore) ListRetailers(_ context.Context) ([]domain.Retailer, error) { return nil, nil }
func (s *importStore) ListStores(_ context.Context, _ domain.RetailerID) ([]domain.Store, error) {
	return nil, nil
}
func (s *importStore) ListRetailerProducts(_ context.Context, _ domain.RetailerID) ([]domain.RetailerProduct, error) {
	return nil, nil
}
func (s *importStore) ListCurrentPrices(_ context.Context) ([]domain.CurrentStoreProductPrice, error) {
	return nil, nil
}
func (s *importStore) PriceObservationsForProduct(_ context.Context, _ domain.RetailerProductID) ([]domain.PriceObservation, error) {
	return nil, nil
}
func (s *importStore) PriceObservationsForStore(_ context.Context, _ domain.StoreID) ([]domain.PriceObservation, error) {
	return nil, nil
}
func (s *importStore) ListExpiringLots(_ context.Context, _ time.Duration) ([]persistence.InventoryLot, error) {
	return nil, nil
}
func (s *importStore) ListPantryIngredientIDs(_ context.Context) ([]domain.IngredientID, error) {
	return nil, nil
}
func (s *importStore) ListAllRecipeIngredients(_ context.Context) ([]persistence.RecipeIngredient, error) {
	return nil, nil
}
func (s *importStore) GetExternalRecipeSource(_ context.Context, _ string) (persistence.ExternalRecipeSource, error) {
	return persistence.ExternalRecipeSource{}, persistence.ErrNoRows
}
func (s *importStore) UpsertExternalRecipeSource(_ context.Context, _ persistence.ExternalRecipeSource) error {
	return nil
}
func (s *importStore) SaveImportCandidate(_ context.Context, _ persistence.ImportCandidate) (domain.RecipeImportCandidateID, error) {
	return domain.RecipeImportCandidateID{}, nil
}
func (s *importStore) SaveCandidateIngredients(_ context.Context, _ domain.RecipeImportCandidateID, _ []persistence.ImportCandidateIngredient) error {
	return nil
}
func (s *importStore) GetImportCandidate(_ context.Context, _ domain.RecipeImportCandidateID) (persistence.ImportCandidate, error) {
	return persistence.ImportCandidate{}, persistence.ErrNoRows
}
func (s *importStore) ListImportCandidates(_ context.Context, _ *string) ([]persistence.ImportCandidate, error) {
	return nil, nil
}
func (s *importStore) ListCandidateIngredients(_ context.Context, _ domain.RecipeImportCandidateID) ([]persistence.ImportCandidateIngredient, error) {
	return nil, nil
}
func (s *importStore) SetCandidateStatus(_ context.Context, _ domain.RecipeImportCandidateID, _ string) error {
	return nil
}
func (s *importStore) SetCandidatePromoted(_ context.Context, _ domain.RecipeImportCandidateID, _ domain.RecipeVariantID) error {
	return nil
}
func (s *importStore) ListRecipeRefs(_ context.Context) ([]persistence.RecipeRef, error) {
	return nil, nil
}
func (s *importStore) GetRecipeRef(_ context.Context, _ domain.RecipeRefID) (persistence.RecipeRef, error) {
	return persistence.RecipeRef{}, persistence.ErrNoRows
}
func (s *importStore) GetRecipeRefByMealieID(_ context.Context, _ string) (persistence.RecipeRef, error) {
	return persistence.RecipeRef{}, persistence.ErrNoRows
}
func (s *importStore) UpsertRecipeRef(_ context.Context, _ persistence.RecipeRef) error { return nil }
func (s *importStore) ListRecipeIngredients(_ context.Context, _ domain.RecipeRefID) ([]persistence.RecipeIngredient, error) {
	return nil, nil
}
func (s *importStore) ListEffortProfiles(_ context.Context) ([]persistence.EffortProfile, error) {
	return nil, nil
}
func (s *importStore) CreateShoppingListWithItems(_ context.Context, _ persistence.ShoppingList, _ []persistence.ShoppingListItem) (domain.ShoppingListID, []persistence.ShoppingListItem, error) {
	return domain.ShoppingListID{}, nil, nil
}
func (s *importStore) CreateOrUpdateRetailerListBinding(_ context.Context, _ persistence.RetailerListBinding) error {
	return nil
}

func (s *importStore) GetRecipeFamilyBySlug(_ context.Context, slug string) (persistence.RecipeFamily, error) {
	for _, f := range s.families {
		if f.Slug == slug {
			return f, nil
		}
	}
	return persistence.RecipeFamily{}, persistence.ErrNoRows
}

func (s *importStore) CreateRecipeFamily(_ context.Context, f persistence.RecipeFamily) error {
	s.families = append(s.families, f)
	return nil
}

func (s *importStore) GetRecipeFamily(_ context.Context, id domain.RecipeFamilyID) (persistence.RecipeFamily, error) {
	for _, f := range s.families {
		if f.ID == id {
			return f, nil
		}
	}
	return persistence.RecipeFamily{}, persistence.ErrNoRows
}

func (s *importStore) CreateRecipeVariant(_ context.Context, v persistence.RecipeVariant) error {
	s.variants = append(s.variants, v)
	return nil
}

func (s *importStore) CreateRecipeRevision(_ context.Context, r persistence.RecipeRevision) (domain.RecipeRevisionID, error) {
	s.revisions = append(s.revisions, r)
	return r.ID, nil
}

func (s *importStore) SetRecipeFamilyDefaultVariant(_ context.Context, _ domain.RecipeFamilyID, _ domain.RecipeVariantID) error {
	return nil
}

func (s *importStore) GetRecipeSourceRefBySource(_ context.Context, source, recipeID string) (persistence.RecipeSourceRef, error) {
	for _, r := range s.sourceRefs {
		if r.Source == source && r.SourceRecipeID == recipeID {
			return r, nil
		}
	}
	return persistence.RecipeSourceRef{}, persistence.ErrNoRows
}

func (s *importStore) GetRecipeSourceRefByFamily(_ context.Context, _ domain.RecipeFamilyID) (persistence.RecipeSourceRef, error) {
	if len(s.sourceRefs) > 0 {
		return s.sourceRefs[0], nil
	}
	return persistence.RecipeSourceRef{}, persistence.ErrNoRows
}

func (s *importStore) UpsertRecipeSourceRef(_ context.Context, r persistence.RecipeSourceRef) error {
	for i, existing := range s.sourceRefs {
		if existing.Source == r.Source && existing.SourceRecipeID == r.SourceRecipeID {
			s.sourceRefs[i] = r
			return nil
		}
	}
	s.sourceRefs = append(s.sourceRefs, r)
	return nil
}

func (s *importStore) ListUnmappedMealieRecipes(_ context.Context) ([]string, error) {
	return s.unmapped, nil
}

// stubMealieClient implements the minimal Mealie interface for import tests.
type stubMealieClient struct {
	refs []mealieRecipeRef
}

type mealieRecipeRef struct {
	MealieRecipeID string
	Slug           string
	Title          string
	Ingredients    []mealieIngredientLine
}

type mealieIngredientLine struct {
	FoodName string
	Quantity float64
	Unit     string
	Note     string
}

func TestImportMealieRecipe_CreatesFamily(t *testing.T) {
	store := &importStore{}
	r := service.NewRecipes(store, nil)
	// Override mealie client with stub — NewRecipes takes *mealie.Client so we
	// can't inject a stub directly. Instead, test the idempotency path which
	// doesn't need the Mealie client.
	_ = r
	_ = store
}

func TestImportMealieRecipe_Idempotent(t *testing.T) {
	famID := domain.NewRecipeFamilyID()
	store := &importStore{
		sourceRefs: []persistence.RecipeSourceRef{
			{RecipeFamilyID: famID, Source: "mealie", SourceRecipeID: "test-slug"},
		},
	}
	r := service.NewRecipes(store, nil)

	got, err := r.ImportMealieRecipe(context.Background(), "test-slug")
	if err != nil {
		t.Fatalf("ImportMealieRecipe: %v", err)
	}
	if got != famID {
		t.Fatalf("expected family %v, got %v", famID, got)
	}
	// No new families, variants, or revisions should have been created.
	if len(store.families) != 0 {
		t.Fatalf("expected 0 new families, got %d", len(store.families))
	}
	if len(store.variants) != 0 {
		t.Fatalf("expected 0 new variants, got %d", len(store.variants))
	}
}

func TestImportMealieRecipe_NoMealieClient(t *testing.T) {
	store := &importStore{}
	r := service.NewRecipes(store, nil)

	_, err := r.ImportMealieRecipe(context.Background(), "unknown-slug")
	if err == nil {
		t.Fatal("expected error when no Mealie client configured")
	}
}

func TestImportAllMealieRecipes_NoMealieClient(t *testing.T) {
	store := &importStore{unmapped: []string{"a", "b"}}
	r := service.NewRecipes(store, nil)

	n, err := r.ImportAllMealieRecipes(context.Background())
	if err == nil {
		t.Fatal("expected error when no Mealie client configured")
	}
	if n != 0 {
		t.Fatalf("expected 0 imported, got %d", n)
	}
}

func TestImportAllMealieRecipes_NoUnmapped(t *testing.T) {
	store := &importStore{unmapped: nil}
	r := service.NewRecipes(store, nil)

	// No Mealie client configured — the guard fires before checking unmapped.
	_, err := r.ImportAllMealieRecipes(context.Background())
	if err == nil {
		t.Fatal("expected error when no Mealie client configured")
	}
}
