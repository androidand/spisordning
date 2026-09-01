// Package service implements the application service layer for spisordning.
// Services depend on persistence.Store for data access and on other services
// for cross-cutting concerns (e.g., Meals uses Preferences for reaction
// learning). They depend on the shared contract in internal/dto and never
// import httpapi (the HTTP transport layer) or cmd (the composition root).
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/androidand/spisordning/internal/dto"
	"github.com/androidand/spisordning/internal/mealie"
	"github.com/androidand/spisordning/internal/persistence"
)

// Store is the subset of persistence.Store that services need. It is defined
// here so services depend on an interface rather than the concrete Store type,
// making unit testing possible with a fake implementation.
type Store interface {
	CreatePerson(ctx context.Context, p persistence.Person) error
	UpdatePerson(ctx context.Context, p persistence.Person) error
	GetPerson(ctx context.Context, id string) (persistence.Person, error)
	ListPeople(ctx context.Context) ([]persistence.Person, error)
	UpsertPreference(ctx context.Context, p persistence.PersonPreference) error
	ListPreferences(ctx context.Context, personID domain.PersonID) ([]persistence.PersonPreference, error)
	RecordObservation(ctx context.Context, o persistence.PreferenceObservation) error
	ListRecipeRefs(ctx context.Context) ([]persistence.RecipeRef, error)
	GetRecipeRef(ctx context.Context, id domain.RecipeRefID) (persistence.RecipeRef, error)
	GetRecipeRefByMealieID(ctx context.Context, mealieRecipeID string) (persistence.RecipeRef, error)
	UpsertRecipeRef(ctx context.Context, r persistence.RecipeRef) error
	CreateMealEvent(ctx context.Context, recipeRefID domain.RecipeRefID, servedOn time.Time, planID *domain.MealPlanID, planSlotDate *time.Time) (domain.MealEventID, error)
	AddMealReaction(ctx context.Context, r persistence.MealReaction) error
	ListMealReactions(ctx context.Context, eventID domain.MealEventID) ([]persistence.MealReaction, error)
	GetMealPlan(ctx context.Context, id domain.MealPlanID) (persistence.MealPlan, error)
	GetOrCreateMealPlan(ctx context.Context, weekStart time.Time) (persistence.MealPlan, error)
	SetMealPlanStatus(ctx context.Context, id domain.MealPlanID, status string) error
	InsertCandidate(ctx context.Context, c persistence.MealPlanCandidate) error
	ListCandidates(ctx context.Context, planID domain.MealPlanID) ([]persistence.MealPlanCandidate, error)
	SetDecision(ctx context.Context, d persistence.MealPlanDecision) error
	ListDecisions(ctx context.Context, planID domain.MealPlanID) ([]persistence.MealPlanDecision, error)
	InsertShoppingRequirement(ctx context.Context, r persistence.ShoppingRequirement) error
	ListShoppingRequirements(ctx context.Context, planID domain.MealPlanID) ([]persistence.ShoppingRequirement, error)
	ListShoppingListItems(ctx context.Context, listID domain.ShoppingListID) ([]persistence.ShoppingListItem, error)
	UpsertIngredientMapping(ctx context.Context, m persistence.IngredientMapping) error
	UpsertIngredient(ctx context.Context, i persistence.Ingredient) error
	AddRecipeIngredient(ctx context.Context, ri persistence.RecipeIngredient) error
	BeginTx(ctx context.Context) (persistence.Tx, error)
	CreateInventoryLocation(ctx context.Context, l persistence.InventoryLocation) error
	GetInventoryLocation(ctx context.Context, id domain.InventoryLocationID) (persistence.InventoryLocation, error)
	ListLotsUnderLocation(ctx context.Context, id domain.InventoryLocationID) ([]persistence.InventoryLot, error)
	ListInventoryLocations(ctx context.Context, householdID string) ([]persistence.InventoryLocation, error)
	RecordPurchase(ctx context.Context, ingredientID domain.IngredientID, productID *domain.ProductID, locationID domain.InventoryLocationID, quantity float64, unit string, bestBefore *time.Time, source string) (domain.InventoryLotID, error)
	RecordConsume(ctx context.Context, lotID domain.InventoryLotID, quantity float64, estimated bool, source string) error
	RecordDiscard(ctx context.Context, lotID domain.InventoryLotID, quantity float64, estimated bool, reason, source string) error
	RecordAdjust(ctx context.Context, lotID domain.InventoryLotID, newQuantity float64, estimated bool, reason, source string) error
	RecordMarkEmpty(ctx context.Context, lotID domain.InventoryLotID) error
	RecordOpen(ctx context.Context, lotID domain.InventoryLotID, source string) error
	RecordTransfer(ctx context.Context, lotID domain.InventoryLotID, toLocationID domain.InventoryLocationID, quantity float64, source string) (domain.InventoryLotID, error)
	GetInventoryLot(ctx context.Context, id domain.InventoryLotID) (persistence.InventoryLot, error)
	ListMealEvents(ctx context.Context, recipeRefID domain.RecipeRefID, servedOn string) ([]persistence.MealEvent, error)
	GetMealEvent(ctx context.Context, id domain.MealEventID) (persistence.MealEvent, error)
	ListMealPlans(ctx context.Context) ([]persistence.MealPlan, error)
	GetIngredientMapping(ctx context.Context, mealieFoodID string) (persistence.IngredientMapping, error)
	UpsertIngredientAlias(ctx context.Context, a persistence.IngredientAlias) error
	GetIngredientAlias(ctx context.Context, householdID, alias string) (persistence.IngredientAlias, error)
	ListIngredientAliases(ctx context.Context, householdID string) ([]persistence.IngredientAlias, error)
	DeleteIngredientAlias(ctx context.Context, householdID, alias string) error
	ResolveIngredientAlias(ctx context.Context, householdID, alias string) (string, error)
	ListAllStores(ctx context.Context) ([]domain.Store, error)
	ListStoreProductOffers(ctx context.Context, storeID domain.StoreID) ([]domain.StoreProductOffer, error)
	GetRecipeSourceRefByFamily(ctx context.Context, familyID domain.RecipeFamilyID) (persistence.RecipeSourceRef, error)
	GetRecipeSourceRefBySource(ctx context.Context, source, sourceRecipeID string) (persistence.RecipeSourceRef, error)
	UpsertRecipeSourceRef(ctx context.Context, r persistence.RecipeSourceRef) error
	ListUnmappedMealieRecipes(ctx context.Context) ([]string, error)
	CreateRecipeFamily(ctx context.Context, f persistence.RecipeFamily) error
	GetRecipeFamily(ctx context.Context, id domain.RecipeFamilyID) (persistence.RecipeFamily, error)
	GetRecipeFamilyBySlug(ctx context.Context, slug string) (persistence.RecipeFamily, error)
	ListRecipeFamilies(ctx context.Context) ([]persistence.RecipeFamily, error)
	SetRecipeFamilyDefaultVariant(ctx context.Context, familyID domain.RecipeFamilyID, variantID domain.RecipeVariantID) error
	CreateRecipeVariant(ctx context.Context, v persistence.RecipeVariant) error
	GetRecipeVariant(ctx context.Context, id domain.RecipeVariantID) (persistence.RecipeVariant, error)
	ListRecipeVariants(ctx context.Context, familyID domain.RecipeFamilyID) ([]persistence.RecipeVariant, error)
	CreateRecipeRevision(ctx context.Context, r persistence.RecipeRevision) (domain.RecipeRevisionID, error)
	GetRecipeRevision(ctx context.Context, id domain.RecipeRevisionID) (persistence.RecipeRevision, error)
	ListRecipeRevisions(ctx context.Context, variantID domain.RecipeVariantID) ([]persistence.RecipeRevision, error)
	AddRecipeRevisionParent(ctx context.Context, child, parent domain.RecipeRevisionID) error
	ListRecipeRevisionParents(ctx context.Context, revisionID domain.RecipeRevisionID) ([]domain.RecipeRevisionID, error)
	UpsertFavorite(ctx context.Context, scopeType, scopeID string, recipeRefID domain.RecipeRefID) error
	DeleteFavorite(ctx context.Context, scopeType, scopeID string, recipeRefID domain.RecipeRefID) error
	ListFavoritesForRecipe(ctx context.Context, recipeRefID domain.RecipeRefID) ([]persistence.Favorite, error)
	GetRecipeRating(ctx context.Context, recipeRefID domain.RecipeRefID) (persistence.RecipeRating, error)
	ListRetailers(ctx context.Context) ([]domain.Retailer, error)
	ListStores(ctx context.Context, retailerID domain.RetailerID) ([]domain.Store, error)
	ListRetailerProducts(ctx context.Context, retailerID domain.RetailerID) ([]domain.RetailerProduct, error)
	ListCurrentPrices(ctx context.Context) ([]domain.CurrentStoreProductPrice, error)
	PriceObservationsForProduct(ctx context.Context, retailerProductID domain.RetailerProductID) ([]domain.PriceObservation, error)
	PriceObservationsForStore(ctx context.Context, storeID domain.StoreID) ([]domain.PriceObservation, error)
	ListExpiringLots(ctx context.Context, within time.Duration) ([]persistence.InventoryLot, error)
	ListPantryIngredientIDs(ctx context.Context) ([]domain.IngredientID, error)
	ListAllRecipeIngredients(ctx context.Context) ([]persistence.RecipeIngredient, error)
	// Discovery operations (activate-recipe-discovery).
	GetExternalRecipeSource(ctx context.Context, id string) (persistence.ExternalRecipeSource, error)
	UpsertExternalRecipeSource(ctx context.Context, src persistence.ExternalRecipeSource) error
	SaveImportCandidate(ctx context.Context, c persistence.ImportCandidate) (domain.RecipeImportCandidateID, error)
	SaveCandidateIngredients(ctx context.Context, candidateID domain.RecipeImportCandidateID, lines []persistence.ImportCandidateIngredient) error
	GetImportCandidate(ctx context.Context, id domain.RecipeImportCandidateID) (persistence.ImportCandidate, error)
	ListImportCandidates(ctx context.Context, status *string) ([]persistence.ImportCandidate, error)
	ListCandidateIngredients(ctx context.Context, candidateID domain.RecipeImportCandidateID) ([]persistence.ImportCandidateIngredient, error)
	SetCandidateStatus(ctx context.Context, id domain.RecipeImportCandidateID, status string) error
 	SetCandidatePromoted(ctx context.Context, id domain.RecipeImportCandidateID, variantID domain.RecipeVariantID) error
 	// Nutrition operations (research-nutrition-data-sources).
 	UpsertFood(ctx context.Context, f persistence.Food) error
 	UpsertNutrients(ctx context.Context, foodNummer int, nutrients []persistence.Nutrient) error
 	GetFood(ctx context.Context, nummer int) (persistence.Food, error)
 	ListFoods(ctx context.Context) ([]persistence.Food, error)
 	CountFoods(ctx context.Context) (int, error)
 	UpsertProductMapping(ctx context.Context, m persistence.ProductMapping) error
 	GetProductMappingByGTIN(ctx context.Context, gtin string) (persistence.ProductMapping, error)
 	GetProductMappingByDabasARIdent(ctx context.Context, arident string) (persistence.ProductMapping, error)
 	GetNutritionForFood(ctx context.Context, foodNummer int) ([]persistence.Nutrient, error)
 	UpsertNutritionSyncStatus(ctx context.Context, s persistence.NutritionSyncStatus) error
 	GetNutritionSyncStatus(ctx context.Context, source string) (persistence.NutritionSyncStatus, error)
}

// txConn is the minimal transaction surface the Meals service needs.
// It is separate from pgx.Tx so tests can inject a fake without importing pgx.
type txConn interface {
	Exec(ctx context.Context, query string, args ...interface{}) (interface{}, error)
	QueryRow(ctx context.Context, query string, args ...interface{}) persistence.Row
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// People implements the PersonService interface defined in dto.
type People struct{ db Store }

// NewPeople returns a People service backed by db.
func NewPeople(db Store) *People { return &People{db: db} }

func (s *People) ListPeople(ctx context.Context) ([]dto.PersonResponse, error) {
	people, err := s.db.ListPeople(ctx)
	if err != nil {
		return nil, fmt.Errorf("service: list people: %w", err)
	}
	out := make([]dto.PersonResponse, 0, len(people))
	for _, p := range people {
		out = append(out, dto.PersonResponse{
			ID: p.ID, Name: p.Name, Weight: p.Weight, CreatedAt: p.CreatedAt,
		})
	}
	return out, nil
}

func (s *People) GetPerson(ctx context.Context, id string) (dto.PersonResponse, error) {
	p, err := s.db.GetPerson(ctx, id)
	if err != nil {
		return dto.PersonResponse{}, fmt.Errorf("service: get person: %w", err)
	}
	return dto.PersonResponse{
		ID: p.ID, Name: p.Name, Weight: p.Weight, CreatedAt: p.CreatedAt,
	}, nil
}

func (s *People) CreatePerson(ctx context.Context, in dto.PersonInput) (dto.PersonResponse, error) {
	weight := in.Weight
	if weight <= 0 {
		weight = 1.0
	}
	p := persistence.Person{
		ID:        domain.NewPersonID().String(),
		Name:      in.Name,
		Weight:    weight,
		CreatedAt: time.Now(),
	}
	if err := s.db.CreatePerson(ctx, p); err != nil {
		return dto.PersonResponse{}, fmt.Errorf("service: create person: %w", err)
	}
	return dto.PersonResponse{
		ID: p.ID, Name: p.Name, Weight: p.Weight, CreatedAt: p.CreatedAt,
	}, nil
}

func (s *People) UpdatePerson(ctx context.Context, id string, in dto.PersonUpdate) (dto.PersonResponse, error) {
	if in.Name == "" {
		return dto.PersonResponse{}, fmt.Errorf("service: update person: name is required")
	}
	if err := s.db.UpdatePerson(ctx, persistence.Person{ID: id, Name: in.Name, Weight: in.Weight}); err != nil {
		if errors.Is(err, persistence.ErrNoRows) {
			return dto.PersonResponse{}, fmt.Errorf("%w: person %s not found", dto.ErrNotFound, id)
		}
		return dto.PersonResponse{}, fmt.Errorf("service: update person: %w", err)
	}
	return s.GetPerson(ctx, id)
}

// Preferences implements the PreferencesService interface defined in dto.
type Preferences struct{ db Store }

// NewPreferences returns a Preferences service backed by db.
func NewPreferences(db Store) *Preferences { return &Preferences{db: db} }

func (s *Preferences) ListPreferences(ctx context.Context, personID string) ([]dto.PersonPreferenceResponse, error) {
	var pid domain.PersonID
	if personID != "" {
		var err error
		pid, err = domain.ParsePersonID(personID)
		if err != nil {
			return nil, fmt.Errorf("service: list preferences: %w", err)
		}
	}
	prefs, err := s.db.ListPreferences(ctx, pid)
	if err != nil {
		return nil, fmt.Errorf("service: list preferences: %w", err)
	}
	out := make([]dto.PersonPreferenceResponse, 0, len(prefs))
	for _, p := range prefs {
		out = append(out, dto.PersonPreferenceResponse{
			PersonID: p.PersonID.String(), Tag: p.Tag, Sentiment: int(p.Sentiment),
			Confidence: p.Confidence, UpdatedAt: p.UpdatedAt,
		})
	}
	return out, nil
}

// SetPreference validates and upserts a (person, tag) preference.
func (s *Preferences) SetPreference(ctx context.Context, in dto.SetPreferenceInput) (dto.PersonPreferenceResponse, error) {
	if in.PersonID == "" || in.Tag == "" {
		return dto.PersonPreferenceResponse{}, fmt.Errorf("%w: person_id and tag are required", dto.ErrInvalidPreference)
	}
	if in.Sentiment < -2 || in.Sentiment > 2 {
		return dto.PersonPreferenceResponse{}, fmt.Errorf("%w: sentiment must be in [-2, 2]", dto.ErrInvalidPreference)
	}
	if in.Confidence < 0 || in.Confidence > 1 {
		return dto.PersonPreferenceResponse{}, fmt.Errorf("%w: confidence must be in [0, 1]", dto.ErrInvalidPreference)
	}
	pid, err := domain.ParsePersonID(in.PersonID)
	if err != nil {
		return dto.PersonPreferenceResponse{}, fmt.Errorf("service: set preference: %w", err)
	}
	if err := s.db.UpsertPreference(ctx, persistence.PersonPreference{
		PersonID: pid, Tag: in.Tag, Sentiment: in.Sentiment, Confidence: in.Confidence,
	}); err != nil {
		return dto.PersonPreferenceResponse{}, fmt.Errorf("service: set preference: %w", err)
	}
	// Re-read to return the authoritative row (with the server-set updated_at).
	prefs, err := s.db.ListPreferences(ctx, pid)
	if err != nil {
		return dto.PersonPreferenceResponse{}, fmt.Errorf("service: set preference: %w", err)
	}
	for _, p := range prefs {
		if p.Tag == in.Tag {
			return dto.PersonPreferenceResponse{
				PersonID: p.PersonID.String(), Tag: p.Tag, Sentiment: int(p.Sentiment),
				Confidence: p.Confidence, UpdatedAt: p.UpdatedAt,
			}, nil
		}
	}
	return dto.PersonPreferenceResponse{}, fmt.Errorf("service: set preference: row not found after upsert")
}

// Recipes implements the RecipesService interface defined in dto.
type Recipes struct {
	db     Store
	mealie *mealie.Client
}

// NewRecipes returns a Recipes service backed by db. mc may be nil when no
// Mealie instance is configured; SyncFromMealie then reports an error.
func NewRecipes(db Store, mc *mealie.Client) *Recipes {
	return &Recipes{db: db, mealie: mc}
}

func (s *Recipes) ListRecipes(ctx context.Context) ([]dto.RecipeRefResponse, error) {
	refs, err := s.db.ListRecipeRefs(ctx)
	if err != nil {
		return nil, fmt.Errorf("service: list recipes: %w", err)
	}
	out := make([]dto.RecipeRefResponse, 0, len(refs))
	for _, r := range refs {
		out = append(out, dto.RecipeRefResponse{
			MealieRecipeID: r.MealieRecipeID, Title: r.Title, Tags: r.Tags,
			Effort: r.Effort, LastSyncedAt: r.LastSyncedAt,
		})
	}
	return out, nil
}

func (s *Recipes) GetRecipe(ctx context.Context, mealieRecipeID string) (dto.RecipeRefResponse, error) {
	r, err := s.db.GetRecipeRefByMealieID(ctx, mealieRecipeID)
	if err != nil {
		if errors.Is(err, persistence.ErrNoRows) {
			return dto.RecipeRefResponse{}, fmt.Errorf("service: get recipe: %w", dto.ErrNotFound)
		}
		return dto.RecipeRefResponse{}, fmt.Errorf("service: get recipe: %w", err)
	}
	return dto.RecipeRefResponse{
		MealieRecipeID: r.MealieRecipeID, Title: r.Title, Tags: r.Tags,
		Effort: r.Effort, LastSyncedAt: r.LastSyncedAt,
	}, nil
}

// SyncFromMealie fetches every recipe from Mealie and upserts a reference for
// each. It returns the number of recipes synced.
func (s *Recipes) SyncFromMealie(ctx context.Context) (int, error) {
	if s.mealie == nil {
		return 0, fmt.Errorf("service: sync recipes: no Mealie client configured")
	}
	refs, err := s.mealie.SyncRecipes(ctx)
	if err != nil {
		return 0, fmt.Errorf("service: sync recipes: %w", err)
	}
	for _, ref := range refs {
		rr := persistence.RecipeRef{
			MealieRecipeID: ref.MealieRecipeID,
			Title:          ref.Title,
			Tags:           ref.Tags,
			Effort:         int(ref.Effort),
			RawSnapshot:    string(ref.Raw),
		}
		if err := s.db.UpsertRecipeRef(ctx, rr); err != nil {
			return 0, fmt.Errorf("service: sync recipes: upsert %s: %w", ref.MealieRecipeID, err)
		}
		if err := s.syncIngredients(ctx, ref); err != nil {
			return 0, fmt.Errorf("service: sync recipes: ingredients for %s: %w", ref.MealieRecipeID, err)
		}
	}
	return len(refs), nil
}

// syncIngredients persists ref's ingredient lines as canonical ingredients and
// recipe_ingredient rows, so ShoppingRequirements (and everything downstream —
// price comparison, wishlist push) has something to resolve. A line whose
// FoodName is still empty after mealie.Client's structured-field-then-brute-
// parser fallback carries nothing usable and is skipped rather than persisted
// as a blank ingredient.
func (s *Recipes) syncIngredients(ctx context.Context, ref mealie.RecipeRef) error {
	refRow, err := s.db.GetRecipeRefByMealieID(ctx, ref.MealieRecipeID)
	if err != nil {
		return fmt.Errorf("resolve recipe ref %q: %w", ref.MealieRecipeID, err)
	}
	for _, line := range ref.Ingredients {
		if line.FoodName == "" {
			continue
		}
		ingID := domain.IngredientIDForName(domain.CanonicalIngredientID(line.FoodName))
		if err := s.db.UpsertIngredient(ctx, persistence.Ingredient{
			ID: ingID, Slug: domain.CanonicalIngredientID(line.FoodName), Display: line.FoodName,
		}); err != nil {
			return fmt.Errorf("upsert ingredient %q: %w", ingID, err)
		}
		ri := persistence.RecipeIngredient{
			RecipeRefID:  refRow.ID,
			IngredientID: ingID,
			Quantity:     line.Quantity,
			Unit:         line.Unit,
		}
		if err := s.db.AddRecipeIngredient(ctx, ri); err != nil {
			return fmt.Errorf("add recipe_ingredient %q: %w", ingID, err)
		}
	}
	return nil
}

// Meals implements the MealsService interface defined in dto.
type Meals struct {
	db       Store
	prefs    dto.PreferencesService
	resolver *ResolveRecipeResolver
}

// NewMeals returns a Meals service backed by db. prefs is used for reaction
// learning (recording preference observations); pass nil to skip.
func NewMeals(db Store, prefs dto.PreferencesService) *Meals {
	return &Meals{db: db, prefs: prefs, resolver: NewResolveRecipeResolver(db, RecipeSourceModeFromEnv())}
}

func (s *Meals) CreateMealEvent(ctx context.Context, in dto.MealEventNew) (dto.MealEventResponse, error) {
	servedOn, err := time.Parse("2006-01-02", in.ServedOn)
	if err != nil {
		return dto.MealEventResponse{}, fmt.Errorf("service: meals create: invalid served_on %q: %w", in.ServedOn, err)
	}

	ref, err := s.db.GetRecipeRefByMealieID(ctx, in.MealieRecipeID)
	if err != nil {
		if errors.Is(err, persistence.ErrNoRows) {
			return dto.MealEventResponse{}, fmt.Errorf("service: meals create: %w: recipe %q not found", dto.ErrNotFound, in.MealieRecipeID)
		}
		return dto.MealEventResponse{}, fmt.Errorf("service: meals create: resolve recipe: %w", err)
	}

	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		return dto.MealEventResponse{}, fmt.Errorf("service: meals create: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	const insertEventQ = `INSERT INTO meal_event (recipe_ref_id, served_on) VALUES ($1, $2) RETURNING id`
	var eventID domain.MealEventID
	if err := tx.QueryRow(ctx, insertEventQ, ref.ID, servedOn).Scan(&eventID); err != nil {
		return dto.MealEventResponse{}, fmt.Errorf("service: meals create: insert event: %w", err)
	}

	for _, rx := range in.Reactions {
		pid, perr := domain.ParsePersonID(rx.PersonID)
		if perr != nil {
			return dto.MealEventResponse{}, fmt.Errorf("service: meals create: parse person id: %w", perr)
		}
		const insertRxQ = `INSERT INTO meal_reaction (meal_event_id, person_id, sentiment, note)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (meal_event_id, person_id) DO UPDATE SET sentiment = EXCLUDED.sentiment,
				note = EXCLUDED.note`
		if _, err := tx.Exec(ctx, insertRxQ, eventID, pid, rx.Sentiment, ""); err != nil {
			return dto.MealEventResponse{}, fmt.Errorf("service: meals create: add reaction: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return dto.MealEventResponse{}, fmt.Errorf("service: meals create: commit tx: %w", err)
	}

	// Record preference observations outside the transaction — non-fatal.
	for _, rx := range in.Reactions {
		pid, _ := domain.ParsePersonID(rx.PersonID)
		obs := persistence.PreferenceObservation{
			PersonID:   pid,
			Tag:        "meal",
			Sentiment:  rx.Sentiment,
			Source:     "reaction",
			ObservedAt: time.Now(),
		}
		if err := s.db.RecordObservation(ctx, obs); err != nil {
			_ = err
		}
	}

	rxns, err := s.db.ListMealReactions(ctx, eventID)
	if err != nil {
		return dto.MealEventResponse{}, fmt.Errorf("service: meals create: read reactions: %w", err)
	}
	out := dto.MealEventResponse{
		ID: eventID.String(), MealieRecipeID: in.MealieRecipeID,
		ServedOn:  in.ServedOn,
		CreatedAt: time.Now(),
		Reactions: make([]dto.MealReactionResponse, 0, len(rxns)),
	}
	for _, r := range rxns {
		out.Reactions = append(out.Reactions, dto.MealReactionResponse{
			PersonID: r.PersonID.String(), Sentiment: r.Sentiment,
		})
	}
	return out, nil
}

func (s *Meals) GetMeal(ctx context.Context, id string) (dto.MealEventResponse, error) {
	eventID, err := domain.ParseMealEventID(id)
	if err != nil {
		return dto.MealEventResponse{}, fmt.Errorf("service: get meal: %w", err)
	}
	event, err := s.db.GetMealEvent(ctx, eventID)
	if err != nil {
		return dto.MealEventResponse{}, fmt.Errorf("service: get meal: %w", err)
	}
	ref, err := s.db.GetRecipeRef(ctx, event.RecipeRefID)
	if err != nil {
		return dto.MealEventResponse{}, fmt.Errorf("service: get meal: resolve recipe: %w", err)
	}
	rxns, err := s.db.ListMealReactions(ctx, event.ID)
	if err != nil {
		return dto.MealEventResponse{}, fmt.Errorf("service: get meal: list reactions: %w", err)
	}
	out := dto.MealEventResponse{
		ID: event.ID.String(), MealieRecipeID: ref.MealieRecipeID,
		ServedOn:  event.ServedOn.Format("2006-01-02"),
		CreatedAt: event.CreatedAt,
		Reactions: make([]dto.MealReactionResponse, 0, len(rxns)),
	}
	for _, r := range rxns {
		out.Reactions = append(out.Reactions, dto.MealReactionResponse{
			PersonID: r.PersonID.String(), Sentiment: r.Sentiment,
		})
	}
	return out, nil
}

func (s *Meals) ListMeals(ctx context.Context, mealieRecipeID, servedOn string) ([]dto.MealEventResponse, error) {
	var recipeRefID domain.RecipeRefID
	if mealieRecipeID != "" {
		ref, err := s.resolver.ResolveRecipeRef(ctx, mealieRecipeID)
		if err != nil {
			if errors.Is(err, persistence.ErrNoRows) {
				return nil, fmt.Errorf("service: list meals: %w: recipe %q not found", dto.ErrNotFound, mealieRecipeID)
			}
			return nil, fmt.Errorf("service: list meals: resolve recipe: %w", err)
		}
		recipeRefID = ref.ID
	}
	events, err := s.db.ListMealEvents(ctx, recipeRefID, servedOn)
	if err != nil {
		return nil, fmt.Errorf("service: list meals: %w", err)
	}
	out := make([]dto.MealEventResponse, 0, len(events))
	for _, event := range events {
		ref, err := s.db.GetRecipeRef(ctx, event.RecipeRefID)
		if err != nil {
			return nil, fmt.Errorf("service: list meals: resolve recipe: %w", err)
		}
		rxns, err := s.db.ListMealReactions(ctx, event.ID)
		if err != nil {
			return nil, fmt.Errorf("service: list meals: list reactions: %w", err)
		}
		resp := dto.MealEventResponse{
			ID: event.ID.String(), MealieRecipeID: ref.MealieRecipeID,
			ServedOn:  event.ServedOn.Format("2006-01-02"),
			CreatedAt: event.CreatedAt,
			Reactions: make([]dto.MealReactionResponse, 0, len(rxns)),
		}
	for _, r := range rxns {
		resp.Reactions = append(resp.Reactions, dto.MealReactionResponse{
			PersonID: r.PersonID.String(), Sentiment: r.Sentiment,
		})
	}
	out = append(out, resp)
	}
	return out, nil
}


