package service_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/androidand/spisordning/internal/dto"
	"github.com/androidand/spisordning/internal/ingredients"
	"github.com/androidand/spisordning/internal/mealie"
	"github.com/androidand/spisordning/internal/persistence"
	"github.com/androidand/spisordning/internal/service"
)

// slugToUUID derives a deterministic UUID from a slug string so tests can use
// readable slugs ("s-1", "rp-1", "willys") while the domain uses typed UUIDs.
func slugToUUID(slug string) [16]byte {
	sum := sha256.Sum256([]byte("spisordning-test:" + slug))
	var u [16]byte
	copy(u[:], sum[:16])
	u[6] = (u[6] & 0x0f) | 0x40
	u[8] = (u[8] & 0x3f) | 0x80
	return u
}

func testStoreID(slug string) domain.StoreID           { return domain.StoreID(slugToUUID(slug)) }
func testRetailerID(slug string) domain.RetailerID     { return domain.RetailerID(slugToUUID(slug)) }
func testRetailerProductID(slug string) domain.RetailerProductID {
	return domain.RetailerProductID(slugToUUID(slug))
}
func testProductID(slug string) domain.ProductID { return domain.ProductID(slugToUUID(slug)) }

// fakeStore is a minimal in-memory Store for unit testing.
type fakeStore struct {
	people            map[string]persistence.Person
	prefs             []persistence.PersonPreference
	recipes           []persistence.RecipeRef
	plans             map[domain.MealPlanID]persistence.MealPlan
	lots              []persistence.InventoryLot
	locations         map[domain.InventoryLocationID]persistence.InventoryLocation
	stores            []domain.Store
	offers            []domain.StoreProductOffer
	ingredients       map[domain.IngredientID]persistence.Ingredient
	recipeIngredients []persistence.RecipeIngredient
	mealEventRef      domain.RecipeRefID
}

func (f *fakeStore) CreatePerson(ctx context.Context, p persistence.Person) error {
	f.people[p.ID] = p
	return nil
}
func (f *fakeStore) UpdatePerson(ctx context.Context, p persistence.Person) error {
	existing, ok := f.people[p.ID]
	if !ok {
		return persistence.ErrNoRows
	}
	if p.Weight > 0 {
		existing.Weight = p.Weight
	}
	existing.Name = p.Name
	f.people[p.ID] = existing
	return nil
}
func (f *fakeStore) GetPerson(ctx context.Context, id string) (persistence.Person, error) {
	p, ok := f.people[id]
	if !ok {
		return persistence.Person{}, errors.New("not found")
	}
	return p, nil
}
func (f *fakeStore) ListPeople(ctx context.Context) ([]persistence.Person, error) {
	out := make([]persistence.Person, 0, len(f.people))
	for _, p := range f.people {
		out = append(out, p)
	}
	return out, nil
}
func (f *fakeStore) UpsertPreference(ctx context.Context, p persistence.PersonPreference) error {
	p.UpdatedAt = time.Now()
	for i, existing := range f.prefs {
		if existing.PersonID == p.PersonID && existing.Tag == p.Tag {
			f.prefs[i] = p
			return nil
		}
	}
	f.prefs = append(f.prefs, p)
	return nil
}
func (f *fakeStore) ListPreferences(ctx context.Context, personID domain.PersonID) ([]persistence.PersonPreference, error) {
	out := make([]persistence.PersonPreference, 0, len(f.prefs))
	for _, p := range f.prefs {
		if personID != (domain.PersonID{}) && p.PersonID != personID {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}
func (f *fakeStore) ListAllStores(ctx context.Context) ([]domain.Store, error) {
	return f.stores, nil
}
func (f *fakeStore) ListPantryIngredientIDs(ctx context.Context) ([]domain.IngredientID, error) {
	seen := make(map[domain.IngredientID]bool)
	var out []domain.IngredientID
	for _, lot := range f.lots {
		if lot.Quantity > 0 && !seen[lot.IngredientID] {
			seen[lot.IngredientID] = true
			out = append(out, lot.IngredientID)
		}
	}
	return out, nil
}
func (f *fakeStore) ListAllRecipeIngredients(ctx context.Context) ([]persistence.RecipeIngredient, error) {
	return f.recipeIngredients, nil
}
func (f *fakeStore) UpsertIngredientAlias(ctx context.Context, a persistence.IngredientAlias) error { return nil }
func (f *fakeStore) GetIngredientAlias(ctx context.Context, householdID, alias string) (persistence.IngredientAlias, error) {
	return persistence.IngredientAlias{}, persistence.ErrNoRows
}
func (f *fakeStore) ListIngredientAliases(ctx context.Context, householdID string) ([]persistence.IngredientAlias, error) {
	return []persistence.IngredientAlias{}, nil
}
func (f *fakeStore) DeleteIngredientAlias(ctx context.Context, householdID, alias string) error { return nil }
func (f *fakeStore) ResolveIngredientAlias(ctx context.Context, householdID, alias string) (string, error) {
	return "", nil
}
func (f *fakeStore) ListStoreProductOffers(ctx context.Context, storeID domain.StoreID) ([]domain.StoreProductOffer, error) {
	out := make([]domain.StoreProductOffer, 0, len(f.offers))
	for _, o := range f.offers {
		if o.StoreID == storeID {
			out = append(out, o)
		}
	}
	return out, nil
}
func (f *fakeStore) RecordObservation(ctx context.Context, o persistence.PreferenceObservation) error { return nil }
func (f *fakeStore) ListRecipeRefs(ctx context.Context) ([]persistence.RecipeRef, error) {
	out := make([]persistence.RecipeRef, 0, len(f.recipes))
	for _, r := range f.recipes {
		out = append(out, r)
	}
	return out, nil
}
func (f *fakeStore) GetRecipeRef(ctx context.Context, id domain.RecipeRefID) (persistence.RecipeRef, error) {
	for _, r := range f.recipes {
		if r.ID == id {
			return r, nil
		}
	}
	return persistence.RecipeRef{}, persistence.ErrNoRows
}
func (f *fakeStore) GetRecipeRefByMealieID(ctx context.Context, mealieRecipeID string) (persistence.RecipeRef, error) {
	for _, r := range f.recipes {
		if r.MealieRecipeID == mealieRecipeID {
			return r, nil
		}
	}
	return persistence.RecipeRef{}, persistence.ErrNoRows
}
func (f *fakeStore) UpsertIngredient(ctx context.Context, i persistence.Ingredient) error {
	if f.ingredients == nil {
		f.ingredients = map[domain.IngredientID]persistence.Ingredient{}
	}
	f.ingredients[i.ID] = i
	return nil
}
func (f *fakeStore) AddRecipeIngredient(ctx context.Context, ri persistence.RecipeIngredient) error {
	for i, existing := range f.recipeIngredients {
		if existing.RecipeRefID == ri.RecipeRefID && existing.IngredientID == ri.IngredientID {
			f.recipeIngredients[i] = ri
			return nil
		}
	}
	f.recipeIngredients = append(f.recipeIngredients, ri)
	return nil
}
func (f *fakeStore) UpsertRecipeRef(ctx context.Context, r persistence.RecipeRef) error {
	for i, existing := range f.recipes {
		if existing.MealieRecipeID == r.MealieRecipeID {
			if r.ID == (domain.RecipeRefID{}) {
				r.ID = existing.ID
			}
			f.recipes[i] = r
			return nil
		}
	}
	if r.ID == (domain.RecipeRefID{}) {
		r.ID = domain.NewRecipeRefID()
	}
	f.recipes = append(f.recipes, r)
	return nil
}
func (f *fakeStore) CreateMealEvent(ctx context.Context, recipeRefID domain.RecipeRefID, servedOn time.Time, planID *domain.MealPlanID, planSlotDate *time.Time) (domain.MealEventID, error) {
	return domain.NewMealEventID(), nil
}
func (f *fakeStore) AddMealReaction(ctx context.Context, r persistence.MealReaction) error { return nil }
func (f *fakeStore) ListMealReactions(ctx context.Context, eventID domain.MealEventID) ([]persistence.MealReaction, error) {
	return []persistence.MealReaction{{MealEventID: eventID, PersonID: domain.NewPersonID(), Sentiment: 1}}, nil
}
func (f *fakeStore) GetMealPlan(ctx context.Context, id domain.MealPlanID) (persistence.MealPlan, error) {
	p, ok := f.plans[id]
	if !ok {
		return persistence.MealPlan{}, errors.New("not found")
	}
	return p, nil
}
func (f *fakeStore) GetOrCreateMealPlan(ctx context.Context, weekStart time.Time) (persistence.MealPlan, error) {
	for _, p := range f.plans {
		if p.WeekStart.Equal(weekStart) {
			return p, nil
		}
	}
	id := domain.NewMealPlanID()
	p := persistence.MealPlan{ID: id, WeekStart: weekStart, Status: "draft", CreatedAt: time.Now()}
	f.plans[id] = p
	return p, nil
}
func (f *fakeStore) SetMealPlanStatus(ctx context.Context, id domain.MealPlanID, status string) error {
	if p, ok := f.plans[id]; ok {
		p.Status = status
		f.plans[id] = p
	}
	return nil
}
func (f *fakeStore) InsertCandidate(ctx context.Context, c persistence.MealPlanCandidate) error { return nil }
func (f *fakeStore) ListCandidates(ctx context.Context, planID domain.MealPlanID) ([]persistence.MealPlanCandidate, error) {
	return nil, nil
}
func (f *fakeStore) SetDecision(ctx context.Context, d persistence.MealPlanDecision) error { return nil }
func (f *fakeStore) ListDecisions(ctx context.Context, planID domain.MealPlanID) ([]persistence.MealPlanDecision, error) {
	return nil, nil
}
func (f *fakeStore) InsertShoppingRequirement(ctx context.Context, r persistence.ShoppingRequirement) error { return nil }
func (f *fakeStore) ListShoppingRequirements(ctx context.Context, planID domain.MealPlanID) ([]persistence.ShoppingRequirement, error) {
	return nil, nil
}
func (f *fakeStore) UpsertIngredientMapping(ctx context.Context, m persistence.IngredientMapping) error { return nil }
func (f *fakeStore) BeginTx(ctx context.Context) (persistence.Tx, error) {
	return &fakeTx{}, nil
}
func (f *fakeStore) CreateInventoryLocation(ctx context.Context, l persistence.InventoryLocation) error {
	f.locations[l.ID] = l
	return nil
}
func (f *fakeStore) GetInventoryLocation(ctx context.Context, id domain.InventoryLocationID) (persistence.InventoryLocation, error) {
	l, ok := f.locations[id]
	if !ok {
		return persistence.InventoryLocation{}, errors.New("not found")
	}
	return l, nil
}
func (f *fakeStore) ListLotsUnderLocation(ctx context.Context, id domain.InventoryLocationID) ([]persistence.InventoryLot, error) {
	out := make([]persistence.InventoryLot, 0)
	for _, l := range f.lots {
		if l.LocationID == id {
			out = append(out, l)
		}
	}
	return out, nil
}
func (f *fakeStore) ListInventoryLocations(ctx context.Context, householdID string) ([]persistence.InventoryLocation, error) {
	out := make([]persistence.InventoryLocation, 0, len(f.locations))
	for _, l := range f.locations {
		if householdID != "" && l.HouseholdID.String() != householdID {
			continue
		}
		out = append(out, l)
	}
	return out, nil
}
func (f *fakeStore) RecordPurchase(ctx context.Context, ingredientID domain.IngredientID, productID *domain.ProductID, locationID domain.InventoryLocationID, quantity float64, unit string, bestBefore *time.Time, source string) (domain.InventoryLotID, error) {
	id := domain.NewInventoryLotID()
	f.lots = append(f.lots, persistence.InventoryLot{ID: id, IngredientID: ingredientID, ProductID: productID, LocationID: locationID, Quantity: quantity, Unit: unit, Confidence: "EXACT", CreatedAt: time.Now(), UpdatedAt: time.Now()})
	return id, nil
}
func (f *fakeStore) RecordConsume(ctx context.Context, lotID domain.InventoryLotID, quantity float64, estimated bool, source string) error { return nil }
func (f *fakeStore) RecordDiscard(ctx context.Context, lotID domain.InventoryLotID, quantity float64, estimated bool, reason, source string) error {
	for i, l := range f.lots {
		if l.ID != lotID {
			continue
		}
		if quantity <= 0 || quantity > l.Quantity {
			return errors.New("invalid discard quantity")
		}
		l.Quantity -= quantity
		l.UpdatedAt = time.Now()
		f.lots[i] = l
		return nil
	}
	return persistence.ErrNoRows
}
func (f *fakeStore) RecordAdjust(ctx context.Context, lotID domain.InventoryLotID, newQuantity float64, estimated bool, reason, source string) error {
	for i, l := range f.lots {
		if l.ID != lotID {
			continue
		}
		if newQuantity < 0 {
			return errors.New("invalid adjust quantity")
		}
		l.Quantity = newQuantity
		l.UpdatedAt = time.Now()
		f.lots[i] = l
		return nil
	}
	return persistence.ErrNoRows
}
func (f *fakeStore) RecordMarkEmpty(ctx context.Context, lotID domain.InventoryLotID) error {
	for i, l := range f.lots {
		if l.ID != lotID {
			continue
		}
		l.Quantity = 0
		l.UpdatedAt = time.Now()
		f.lots[i] = l
		return nil
	}
	return persistence.ErrNoRows
}
func (f *fakeStore) RecordOpen(ctx context.Context, lotID domain.InventoryLotID, source string) error {
	for i, l := range f.lots {
		if l.ID != lotID {
			continue
		}
		now := time.Now()
		l.OpenedAt = &now
		l.UpdatedAt = now
		f.lots[i] = l
		return nil
	}
	return persistence.ErrNoRows
}
func (f *fakeStore) RecordTransfer(ctx context.Context, lotID domain.InventoryLotID, toLocationID domain.InventoryLocationID, quantity float64, source string) (domain.InventoryLotID, error) {
	for i, l := range f.lots {
		if l.ID != lotID {
			continue
		}
		if quantity <= 0 || quantity > l.Quantity {
			return domain.InventoryLotID{}, errors.New("invalid transfer quantity")
		}
		now := time.Now()
		if quantity == l.Quantity {
			l.LocationID = toLocationID
			l.UpdatedAt = now
			f.lots[i] = l
			return l.ID, nil
		}
		l.Quantity -= quantity
		l.UpdatedAt = now
		f.lots[i] = l
		dest := l
		dest.ID = domain.NewInventoryLotID()
		dest.LocationID = toLocationID
		dest.Quantity = quantity
		dest.CreatedAt = now
		dest.UpdatedAt = now
		f.lots = append(f.lots, dest)
		return dest.ID, nil
	}
	return domain.InventoryLotID{}, persistence.ErrNoRows
}
func (f *fakeStore) GetInventoryLot(ctx context.Context, id domain.InventoryLotID) (persistence.InventoryLot, error) {
	for _, l := range f.lots {
		if l.ID == id {
			return l, nil
		}
	}
	return persistence.InventoryLot{}, persistence.ErrNoRows
}
func (f *fakeStore) GetExternalRecipeSource(ctx context.Context, id string) (persistence.ExternalRecipeSource, error) {
	return persistence.ExternalRecipeSource{}, persistence.ErrNoRows
}
func (f *fakeStore) UpsertExternalRecipeSource(ctx context.Context, src persistence.ExternalRecipeSource) error {
	return nil
}
func (f *fakeStore) SaveImportCandidate(ctx context.Context, c persistence.ImportCandidate) (domain.RecipeImportCandidateID, error) {
	if c.ID == (domain.RecipeImportCandidateID{}) {
		c.ID = domain.NewRecipeImportCandidateID()
	}
	return c.ID, nil
}
func (f *fakeStore) SaveCandidateIngredients(ctx context.Context, candidateID domain.RecipeImportCandidateID, lines []persistence.ImportCandidateIngredient) error {
	return nil
}
func (f *fakeStore) GetImportCandidate(ctx context.Context, id domain.RecipeImportCandidateID) (persistence.ImportCandidate, error) {
	return persistence.ImportCandidate{}, persistence.ErrNoRows
}
func (f *fakeStore) ListImportCandidates(ctx context.Context, status *string) ([]persistence.ImportCandidate, error) {
	return nil, nil
}
func (f *fakeStore) ListCandidateIngredients(ctx context.Context, candidateID domain.RecipeImportCandidateID) ([]persistence.ImportCandidateIngredient, error) {
	return nil, nil
}
func (f *fakeStore) SetCandidateStatus(ctx context.Context, id domain.RecipeImportCandidateID, status string) error {
	return nil
}
func (f *fakeStore) SetCandidatePromoted(ctx context.Context, id domain.RecipeImportCandidateID, variantID domain.RecipeVariantID) error {
	return nil
}
func (f *fakeStore) ListMealEvents(ctx context.Context, recipeRefID domain.RecipeRefID, servedOn string) ([]persistence.MealEvent, error) {
	return nil, nil
}
func (f *fakeStore) GetMealEvent(ctx context.Context, id domain.MealEventID) (persistence.MealEvent, error) {
	refID := f.mealEventRef
	if refID == (domain.RecipeRefID{}) {
		refID = domain.NewRecipeRefID()
	}
	return persistence.MealEvent{ID: id, RecipeRefID: refID, ServedOn: time.Now()}, nil
}
func (f *fakeStore) GetIngredientMapping(ctx context.Context, mealieFoodID string) (persistence.IngredientMapping, error) {
	return persistence.IngredientMapping{MealieFoodID: mealieFoodID, IngredientID: domain.IngredientIDForName("cauliflower")}, nil
}
func (f *fakeStore) ListMealPlans(ctx context.Context) ([]persistence.MealPlan, error) {
	out := make([]persistence.MealPlan, 0, len(f.plans))
	for _, p := range f.plans {
		out = append(out, p)
	}
	return out, nil
}
func (f *fakeStore) CreateRecipeFamily(ctx context.Context, fam persistence.RecipeFamily) error { return nil }
func (f *fakeStore) GetRecipeFamily(ctx context.Context, id domain.RecipeFamilyID) (persistence.RecipeFamily, error) {
	return persistence.RecipeFamily{}, persistence.ErrNoRows
}
func (f *fakeStore) ListRecipeFamilies(ctx context.Context) ([]persistence.RecipeFamily, error) {
	return nil, nil
}
func (f *fakeStore) SetRecipeFamilyDefaultVariant(ctx context.Context, familyID domain.RecipeFamilyID, variantID domain.RecipeVariantID) error {
	return nil
}
func (f *fakeStore) CreateRecipeVariant(ctx context.Context, v persistence.RecipeVariant) error { return nil }
func (f *fakeStore) GetRecipeVariant(ctx context.Context, id domain.RecipeVariantID) (persistence.RecipeVariant, error) {
	return persistence.RecipeVariant{}, persistence.ErrNoRows
}
func (f *fakeStore) ListRecipeVariants(ctx context.Context, familyID domain.RecipeFamilyID) ([]persistence.RecipeVariant, error) {
	return nil, nil
}
func (f *fakeStore) CreateRecipeRevision(ctx context.Context, r persistence.RecipeRevision) (domain.RecipeRevisionID, error) {
	return domain.NewRecipeRevisionID(), nil
}
func (f *fakeStore) GetRecipeRevision(ctx context.Context, id domain.RecipeRevisionID) (persistence.RecipeRevision, error) {
	return persistence.RecipeRevision{}, persistence.ErrNoRows
}
func (f *fakeStore) ListRecipeRevisions(ctx context.Context, variantID domain.RecipeVariantID) ([]persistence.RecipeRevision, error) {
	return nil, nil
}
func (f *fakeStore) AddRecipeRevisionParent(ctx context.Context, child, parent domain.RecipeRevisionID) error { return nil }
func (f *fakeStore) ListRecipeRevisionParents(ctx context.Context, revisionID domain.RecipeRevisionID) ([]domain.RecipeRevisionID, error) {
	return nil, nil
}
func (f *fakeStore) UpsertFavorite(ctx context.Context, scopeType, scopeID string, recipeRefID domain.RecipeRefID) error {
	return nil
}
func (f *fakeStore) DeleteFavorite(ctx context.Context, scopeType, scopeID string, recipeRefID domain.RecipeRefID) error {
	return nil
}
func (f *fakeStore) ListFavoritesForRecipe(ctx context.Context, recipeRefID domain.RecipeRefID) ([]persistence.Favorite, error) {
	return nil, nil
}
func (f *fakeStore) GetRecipeRating(ctx context.Context, recipeRefID domain.RecipeRefID) (persistence.RecipeRating, error) {
	return persistence.RecipeRating{RecipeRefID: recipeRefID, Average: 4.2, ReviewCount: 3}, nil
}
func (f *fakeStore) ListRetailers(ctx context.Context) ([]domain.Retailer, error) {
	return []domain.Retailer{{ID: testRetailerID("willys"), Name: "Willys"}}, nil
}
func (f *fakeStore) ListRetailerProducts(ctx context.Context, retailerID domain.RetailerID) ([]domain.RetailerProduct, error) {
	rpid := testRetailerProductID("rp-1")
	pid := testProductID("prod-1")
	return []domain.RetailerProduct{{ID: rpid, RetailerID: testRetailerID("willys"), ProductID: &pid, DisplayName: "Mjölk 3%"}}, nil
}
func (f *fakeStore) ListCurrentPrices(ctx context.Context) ([]domain.CurrentStoreProductPrice, error) {
	rpid := testRetailerProductID("rp-1")
	oid1 := domain.StoreProductOfferID(slugToUUID("offer-1"))
	oid2 := domain.StoreProductOfferID(slugToUUID("offer-2"))
	return []domain.CurrentStoreProductPrice{
		{OfferID: oid1, StoreID: testStoreID("s-1"), RetailerProductID: rpid, PriceKind: domain.PriceKindRegular, Price: 19.9, ObservedAt: time.Now(), Source: "willys_adapter"},
		{OfferID: oid2, StoreID: testStoreID("s-2"), RetailerProductID: rpid, PriceKind: domain.PriceKindRegular, Price: 17.5, ObservedAt: time.Now(), Source: "willys_adapter"},
	}, nil
}
func (f *fakeStore) ListStores(ctx context.Context, retailerID domain.RetailerID) ([]domain.Store, error) {
	return []domain.Store{{ID: testStoreID("s-1"), RetailerID: retailerID, Name: "Willys Centrum"}}, nil
}
func (f *fakeStore) PriceObservationsForProduct(ctx context.Context, retailerProductID domain.RetailerProductID) ([]domain.PriceObservation, error) {
	return []domain.PriceObservation{{
		ID: domain.NewPriceObservationID(), StoreProductOfferID: domain.StoreProductOfferID(slugToUUID("offer-1")),
		ObservedAt: time.Now(), Price: 19.9, PriceKind: domain.PriceKindRegular, Source: "willys_adapter", CreatedAt: time.Now(),
	}}, nil
}
func (f *fakeStore) PriceObservationsForStore(ctx context.Context, storeID domain.StoreID) ([]domain.PriceObservation, error) {
	return []domain.PriceObservation{{
		ID: domain.NewPriceObservationID(), StoreProductOfferID: domain.StoreProductOfferID(slugToUUID("offer-1")),
		ObservedAt: time.Now(), Price: 19.9, PriceKind: domain.PriceKindRegular, Source: "willys_adapter", CreatedAt: time.Now(),
	}}, nil
}
func (f *fakeStore) GetRecipeSourceRefByFamily(ctx context.Context, familyID domain.RecipeFamilyID) (persistence.RecipeSourceRef, error) {
	return persistence.RecipeSourceRef{}, persistence.ErrNoRows
}
func (f *fakeStore) GetRecipeSourceRefBySource(ctx context.Context, source, sourceRecipeID string) (persistence.RecipeSourceRef, error) {
	return persistence.RecipeSourceRef{}, persistence.ErrNoRows
}
func (f *fakeStore) UpsertRecipeSourceRef(ctx context.Context, r persistence.RecipeSourceRef) error {
	return nil
}
func (f *fakeStore) ListUnmappedMealieRecipes(ctx context.Context) ([]string, error) {
	return nil, nil
}
func (f *fakeStore) GetRecipeFamilyBySlug(ctx context.Context, slug string) (persistence.RecipeFamily, error) {
	return persistence.RecipeFamily{}, persistence.ErrNoRows
}
func (f *fakeStore) ListExpiringLots(ctx context.Context, within time.Duration) ([]persistence.InventoryLot, error) {
	return []persistence.InventoryLot{
		{ID: domain.NewInventoryLotID(), IngredientID: domain.IngredientIDForName("mjolk"), LocationID: domain.NewInventoryLocationID(), Quantity: 1, Unit: "L", Confidence: domain.ConfidenceExact, BestBefore: &[]time.Time{time.Now().Add(24 * time.Hour)}[0]},
	}, nil
}

// fakeTx is a minimal no-op persistence.Tx for tests.
type fakeTx struct{}

func (f *fakeTx) Exec(ctx context.Context, sql string, args ...interface{}) (interface{}, error) {
	return nil, nil
}
func (f *fakeTx) QueryRow(ctx context.Context, sql string, args ...interface{}) persistence.Row {
	return &fakeRow{id: "event-1"}
}
func (f *fakeTx) Commit(ctx context.Context) error { return nil }
func (f *fakeTx) Rollback(ctx context.Context) error { return nil }

type fakeRow struct{ id string }

func (f *fakeRow) Scan(dest ...interface{}) error {
	if len(dest) > 0 {
		if p, ok := dest[0].(*string); ok {
			*p = f.id
		}
	}
	return nil
}

func TestPeopleList(t *testing.T) {
	f := &fakeStore{people: map[string]persistence.Person{
		"p1": {ID: "p1", Name: "Andreas", Weight: 1.0, CreatedAt: time.Now()},
	}}
	svc := service.NewPeople(f)
	out, err := svc.ListPeople(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 person, got %d", len(out))
	}
	if out[0].Name != "Andreas" {
		t.Fatalf("expected Andreas, got %s", out[0].Name)
	}
}

func TestPeopleCreate(t *testing.T) {
	f := &fakeStore{people: make(map[string]persistence.Person)}
	svc := service.NewPeople(f)
	out, err := svc.CreatePerson(context.Background(), dto.PersonInput{Name: "Test"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Name != "Test" {
		t.Fatalf("expected Test, got %s", out.Name)
	}
	if out.Weight != 1.0 {
		t.Fatalf("expected weight 1.0, got %f", out.Weight)
	}
}

func TestPeopleUpdate(t *testing.T) {
	f := &fakeStore{people: map[string]persistence.Person{
		"p1": {ID: "p1", Name: "Old", Weight: 1.0, CreatedAt: time.Now()},
	}}
	svc := service.NewPeople(f)
	out, err := svc.UpdatePerson(context.Background(), "p1", dto.PersonUpdate{Name: "New", Weight: 2.0})
	if err != nil {
		t.Fatal(err)
	}
	if out.Name != "New" || out.Weight != 2.0 {
		t.Fatalf("unexpected person: %+v", out)
	}
}

func TestPeopleUpdate_NotFound(t *testing.T) {
	f := &fakeStore{people: make(map[string]persistence.Person)}
	svc := service.NewPeople(f)
	_, err := svc.UpdatePerson(context.Background(), "missing", dto.PersonUpdate{Name: "New"})
	if !errors.Is(err, dto.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestPeopleUpdate_RequiresName(t *testing.T) {
	f := &fakeStore{people: make(map[string]persistence.Person)}
	svc := service.NewPeople(f)
	_, err := svc.UpdatePerson(context.Background(), "p1", dto.PersonUpdate{})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestPreferencesList(t *testing.T) {
	pid := domain.NewPersonID()
	f := &fakeStore{prefs: []persistence.PersonPreference{
		{PersonID: pid, Tag: "spicy", Sentiment: 1, Confidence: 0.8, UpdatedAt: time.Now()},
	}}
	svc := service.NewPreferences(f)
	out, err := svc.ListPreferences(context.Background(), pid.String())
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 pref, got %d", len(out))
	}
	if out[0].Tag != "spicy" {
		t.Fatalf("expected spicy, got %s", out[0].Tag)
	}
}

func TestSetPreference_HappyPath(t *testing.T) {
	pid := domain.NewPersonID()
	f := &fakeStore{}
	svc := service.NewPreferences(f)
	out, err := svc.SetPreference(context.Background(), dto.SetPreferenceInput{
		PersonID: pid.String(), Tag: "spicy", Sentiment: 2, Confidence: 0.9,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Tag != "spicy" || out.Sentiment != 2 || out.Confidence != 0.9 {
		t.Fatalf("unexpected pref: %+v", out)
	}
	if len(f.prefs) != 1 {
		t.Fatalf("expected 1 stored pref, got %d", len(f.prefs))
	}
}

func TestSetPreference_UpdatesExisting(t *testing.T) {
	pid := domain.NewPersonID()
	f := &fakeStore{prefs: []persistence.PersonPreference{
		{PersonID: pid, Tag: "spicy", Sentiment: 1, Confidence: 0.5, UpdatedAt: time.Now()},
	}}
	svc := service.NewPreferences(f)
	out, err := svc.SetPreference(context.Background(), dto.SetPreferenceInput{
		PersonID: pid.String(), Tag: "spicy", Sentiment: -1, Confidence: 0.7,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Sentiment != -1 {
		t.Fatalf("expected updated sentiment -1, got %d", out.Sentiment)
	}
	if len(f.prefs) != 1 {
		t.Fatalf("expected still 1 pref (upsert), got %d", len(f.prefs))
	}
}

func TestSetPreference_Invalid(t *testing.T) {
	f := &fakeStore{}
	svc := service.NewPreferences(f)
	cases := []struct {
		name string
		in   dto.SetPreferenceInput
	}{
		{"missing person", dto.SetPreferenceInput{Tag: "spicy", Sentiment: 1, Confidence: 0.5}},
		{"missing tag", dto.SetPreferenceInput{PersonID: "p1", Sentiment: 1, Confidence: 0.5}},
		{"sentiment too high", dto.SetPreferenceInput{PersonID: "p1", Tag: "spicy", Sentiment: 3, Confidence: 0.5}},
		{"sentiment too low", dto.SetPreferenceInput{PersonID: "p1", Tag: "spicy", Sentiment: -3, Confidence: 0.5}},
		{"confidence too high", dto.SetPreferenceInput{PersonID: "p1", Tag: "spicy", Sentiment: 1, Confidence: 1.5}},
		{"confidence negative", dto.SetPreferenceInput{PersonID: "p1", Tag: "spicy", Sentiment: 1, Confidence: -0.1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.SetPreference(context.Background(), tc.in)
			if !errors.Is(err, dto.ErrInvalidPreference) {
				t.Fatalf("expected ErrInvalidPreference, got %v", err)
			}
		})
	}
}

func TestRecipesList(t *testing.T) {
	f := &fakeStore{recipes: []persistence.RecipeRef{
		{MealieRecipeID: "r1", Title: "Pasta", Tags: []string{"italian"}, Effort: 1, LastSyncedAt: time.Now()},
	}}
	svc := service.NewRecipes(f, nil)
	out, err := svc.ListRecipes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 recipe, got %d", len(out))
	}
	if out[0].Title != "Pasta" {
		t.Fatalf("expected Pasta, got %s", out[0].Title)
	}
}

func TestRecipesGet(t *testing.T) {
	f := &fakeStore{recipes: []persistence.RecipeRef{
		{MealieRecipeID: "r1", Title: "Pasta", Tags: []string{"italian"}, Effort: 1, LastSyncedAt: time.Now()},
	}}
	svc := service.NewRecipes(f, nil)

	out, err := svc.GetRecipe(context.Background(), "r1")
	if err != nil {
		t.Fatal(err)
	}
	if out.Title != "Pasta" || out.Effort != 1 {
		t.Fatalf("unexpected recipe: %+v", out)
	}

	if _, err := svc.GetRecipe(context.Background(), "missing"); !errors.Is(err, dto.ErrNotFound) {
		t.Fatalf("expected dto.ErrNotFound, got %v", err)
	}
}

func TestRecipesSync(t *testing.T) {
	fakeMealie := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/recipes":
			w.Write([]byte(`{"page":1,"perPage":50,"total":1,"items":[
				{"id":"r-pasta","slug":"pasta","name":"Pasta Bolognese"}]}`))
		case "/api/recipes/pasta":
			w.Write([]byte(`{"id":"r-pasta","slug":"pasta","name":"Pasta Bolognese","totalTime":"20 min",
				"tags":[{"name":"pasta"}],
				"recipeIngredient":[{"quantity":400,"unit":{"name":"g"},"food":{"id":"f1","name":"köttfärs"}}]}`))
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(fakeMealie.Close)

	f := &fakeStore{}
	svc := service.NewRecipes(f, mealie.New(fakeMealie.URL, "tok"))
	n, err := svc.SyncFromMealie(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 synced ref, got %d", n)
	}
	if len(f.recipes) != 1 || f.recipes[0].MealieRecipeID != "r-pasta" {
		t.Fatalf("expected r-pasta upserted, got %+v", f.recipes)
	}
	if len(f.recipeIngredients) != 1 {
		t.Fatalf("expected 1 recipe_ingredient row, got %+v", f.recipeIngredients)
	}
	ri := f.recipeIngredients[0]
	if ri.Quantity != 400 || ri.Unit != "g" {
		t.Fatalf("unexpected recipe_ingredient row: %+v", ri)
	}
	if len(f.ingredients) != 1 {
		t.Fatalf("expected 1 ingredient, got %+v", f.ingredients)
	}

	if _, err := svc.SyncFromMealie(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.recipes) != 1 {
		t.Fatalf("expected idempotent upsert, got %d refs", len(f.recipes))
	}
	if len(f.recipeIngredients) != 1 {
		t.Fatalf("expected idempotent ingredient upsert, got %d rows", len(f.recipeIngredients))
	}
}

// TestRecipesSyncUnstructuredIngredients covers the shape our own recipe
// imports actually produce: a bare "note" with no structured food/unit, which
// mealie.Client.fetchRecipe resolves via Mealie's brute parser
// (/api/parser/ingredients) before service.Recipes ever sees it. This is the
// exact path that silently dropped ingredients before syncIngredients existed
// — recipe_ingredient stayed empty for every recipe imported this way.
func TestRecipesSyncUnstructuredIngredients(t *testing.T) {
	fakeMealie := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/recipes" && r.Method == http.MethodGet:
			w.Write([]byte(`{"page":1,"perPage":50,"total":1,"items":[
				{"id":"r-stroganoff","slug":"korv-stroganoff","name":"Korv Stroganoff"}]}`))
		case r.URL.Path == "/api/recipes/korv-stroganoff":
			w.Write([]byte(`{"id":"r-stroganoff","slug":"korv-stroganoff","name":"Korv Stroganoff","totalTime":"20 min",
				"tags":[{"name":"middag"}],
				"recipeIngredient":[{"quantity":0,"note":"500 g falukorv","unit":null,"food":null}]}`))
		case r.URL.Path == "/api/parser/ingredients" && r.Method == http.MethodPost:
			w.Write([]byte(`[{"ingredient":{"quantity":500,"unit":{"name":"g"},"food":{"name":"falukorv"}}}]`))
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(fakeMealie.Close)

	f := &fakeStore{}
	svc := service.NewRecipes(f, mealie.New(fakeMealie.URL, "tok"))
	if _, err := svc.SyncFromMealie(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.recipeIngredients) != 1 {
		t.Fatalf("expected 1 recipe_ingredient row from the brute-parser fallback, got %+v", f.recipeIngredients)
	}
	ri := f.recipeIngredients[0]
	if ri.Quantity != 500 || ri.Unit != "g" {
		t.Fatalf("unexpected recipe_ingredient row: %+v", ri)
	}
}

func TestRecipesSyncNoClient(t *testing.T) {
	svc := service.NewRecipes(&fakeStore{}, nil)
	if _, err := svc.SyncFromMealie(context.Background()); err == nil {
		t.Fatal("expected error when mealie client is nil")
	}
}

func TestMealsCreate(t *testing.T) {
	f := &fakeStore{recipes: []persistence.RecipeRef{
		{ID: domain.NewRecipeRefID(), MealieRecipeID: "r1", Title: "Pasta", Effort: 1},
	}}
	svc := service.NewMeals(f, nil)
	out, err := svc.CreateMealEvent(context.Background(), dto.MealEventNew{
		MealieRecipeID: "r1", ServedOn: "2025-01-15",
		Reactions: []dto.MealReactionInput{{PersonID: domain.NewPersonID().String(), Sentiment: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.MealieRecipeID != "r1" {
		t.Fatalf("expected r1, got %s", out.MealieRecipeID)
	}
}

func TestPlanningList(t *testing.T) {
	weekStart := time.Date(2025, 1, 13, 0, 0, 0, 0, time.UTC)
	planID := domain.NewMealPlanID()
	f := &fakeStore{plans: map[domain.MealPlanID]persistence.MealPlan{
		planID: {ID: planID, WeekStart: weekStart, Status: "draft", CreatedAt: time.Now()},
	}}
	svc := service.NewPlanning(f, nil)
	out, err := svc.ListPlans(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 plan, got %d", len(out))
	}
	if out[0].Status != "draft" {
		t.Fatalf("expected draft, got %s", out[0].Status)
	}
}

func TestPantryCreateLocation(t *testing.T) {
	f := &fakeStore{locations: make(map[domain.InventoryLocationID]persistence.InventoryLocation)}
	svc := service.NewPantry(f)
	out, err := svc.CreateLocation(context.Background(), dto.PantryLocationNew{Name: "Kitchen", HouseholdID: domain.NewHouseholdID().String()})
	if err != nil {
		t.Fatal(err)
	}
	if out.Name != "Kitchen" {
		t.Fatalf("expected Kitchen, got %s", out.Name)
	}
}

func TestPantryPurchase(t *testing.T) {
	locID := domain.NewInventoryLocationID()
	f := &fakeStore{locations: map[domain.InventoryLocationID]persistence.InventoryLocation{
		locID: {ID: locID, Name: "Kitchen", HouseholdID: domain.NewHouseholdID()},
	}}
	svc := service.NewPantry(f)
	out, err := svc.Purchase(context.Background(), dto.PantryPurchaseInput{
		IngredientID: "cauliflower", Quantity: 1.0, Unit: "piece", LocationID: locID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.IngredientID != "cauliflower" {
		t.Fatalf("expected cauliflower, got %s", out.IngredientID)
	}
}

func TestPlanningGet(t *testing.T) {
	weekStart := time.Date(2025, 1, 13, 0, 0, 0, 0, time.UTC)
	planID := domain.NewMealPlanID()
	f := &fakeStore{plans: map[domain.MealPlanID]persistence.MealPlan{
		planID: {ID: planID, WeekStart: weekStart, Status: "approved", CreatedAt: time.Now()},
	}}
	svc := service.NewPlanning(f, nil)
	out, err := svc.GetPlan(context.Background(), planID.String())
	if err != nil {
		t.Fatal(err)
	}
	if out.Plan.ID != planID.String() || out.Plan.Status != "approved" {
		t.Fatalf("unexpected plan: %+v", out.Plan)
	}
}

func TestPlanningUpdate(t *testing.T) {
	weekStart := time.Date(2025, 1, 13, 0, 0, 0, 0, time.UTC)
	planID := domain.NewMealPlanID()
	f := &fakeStore{plans: map[domain.MealPlanID]persistence.MealPlan{
		planID: {ID: planID, WeekStart: weekStart, Status: "draft", CreatedAt: time.Now()},
	}}
	svc := service.NewPlanning(f, nil)
	out, err := svc.UpdatePlan(context.Background(), planID.String(), dto.MealPlanUpdate{Status: "approved"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != "approved" {
		t.Fatalf("expected approved, got %s", out.Status)
	}
}

func TestPlanningCreate(t *testing.T) {
	f := &fakeStore{plans: make(map[domain.MealPlanID]persistence.MealPlan)}
	svc := service.NewPlanning(f, nil)
	out, err := svc.CreatePlan(context.Background(), "2025-01-13")
	if err != nil {
		t.Fatal(err)
	}
	if out.WeekStart != "2025-01-13" || out.Status != "draft" {
		t.Fatalf("unexpected plan: %+v", out)
	}
}

func TestPantryListLocations(t *testing.T) {
	hhID := domain.NewHouseholdID()
	kitchenID := domain.NewInventoryLocationID()
	freezerID := domain.NewInventoryLocationID()
	f := &fakeStore{locations: map[domain.InventoryLocationID]persistence.InventoryLocation{
		kitchenID: {ID: kitchenID, Name: "Kitchen", HouseholdID: hhID},
		freezerID: {ID: freezerID, Name: "Freezer", HouseholdID: hhID},
	}}
	svc := service.NewPantry(f)
	out, err := svc.ListLocations(context.Background(), hhID.String())
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 locations, got %d", len(out))
	}
}

func TestPantryConsume(t *testing.T) {
	f := &fakeStore{locations: make(map[domain.InventoryLocationID]persistence.InventoryLocation)}
	svc := service.NewPantry(f)
	err := svc.Consume(context.Background(), domain.NewInventoryLotID().String(), dto.PantryConsumeInput{Quantity: 1.0})
	if err != nil {
		t.Fatal(err)
	}
}

func newPantryLedgerFixture(t *testing.T) (*fakeStore, domain.InventoryLocationID, domain.InventoryLocationID, domain.InventoryLotID) {
	t.Helper()
	src := domain.NewInventoryLocationID()
	dst := domain.NewInventoryLocationID()
	lot := domain.NewInventoryLotID()
	f := &fakeStore{
		locations: map[domain.InventoryLocationID]persistence.InventoryLocation{
			src: {ID: src, Name: "Kitchen", HouseholdID: domain.NewHouseholdID()},
			dst: {ID: dst, Name: "Fridge", HouseholdID: domain.NewHouseholdID()},
		},
		lots: []persistence.InventoryLot{{
			ID: lot, IngredientID: domain.IngredientIDForName("mjolk"), LocationID: src,
			Quantity: 2.0, Unit: "L", Confidence: "EXACT",
		}},
	}
	return f, src, dst, lot
}

func TestPantryDiscard(t *testing.T) {
	f, _, _, lot := newPantryLedgerFixture(t)
	svc := service.NewPantry(f)
	out, err := svc.Discard(context.Background(), lot.String(), dto.PantryDiscardInput{Quantity: 1.0, Source: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Quantity != 1.0 {
		t.Fatalf("expected remaining quantity 1.0, got %v", out.Quantity)
	}
}

func TestPantryDiscardInvalid(t *testing.T) {
	f, _, _, lot := newPantryLedgerFixture(t)
	svc := service.NewPantry(f)
	if _, err := svc.Discard(context.Background(), lot.String(), dto.PantryDiscardInput{Quantity: 3.0, Source: "manual"}); !errors.Is(err, dto.ErrInvalid) {
		t.Fatalf("expected dto.ErrInvalid for oversized discard, got %v", err)
	}
	if _, err := svc.Discard(context.Background(), lot.String(), dto.PantryDiscardInput{Quantity: 1.0}); !errors.Is(err, dto.ErrInvalid) {
		t.Fatalf("expected dto.ErrInvalid for missing source, got %v", err)
	}
	if _, err := svc.Discard(context.Background(), "not-a-lot", dto.PantryDiscardInput{Quantity: 1.0, Source: "manual"}); !errors.Is(err, dto.ErrInvalid) {
		t.Fatalf("expected dto.ErrInvalid for malformed lot id, got %v", err)
	}
}

func TestPantryDiscardNotFound(t *testing.T) {
	svc := service.NewPantry(&fakeStore{})
	if _, err := svc.Discard(context.Background(), domain.NewInventoryLotID().String(), dto.PantryDiscardInput{Quantity: 1.0, Source: "manual"}); !errors.Is(err, dto.ErrNotFound) {
		t.Fatalf("expected dto.ErrNotFound, got %v", err)
	}
}

func TestPantryAdjust(t *testing.T) {
	f, _, _, lot := newPantryLedgerFixture(t)
	svc := service.NewPantry(f)
	out, err := svc.Adjust(context.Background(), lot.String(), dto.PantryAdjustInput{Quantity: 0.5, Source: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Quantity != 0.5 {
		t.Fatalf("expected adjusted quantity 0.5, got %v", out.Quantity)
	}
}

func TestPantryAdjustInvalid(t *testing.T) {
	f, _, _, lot := newPantryLedgerFixture(t)
	svc := service.NewPantry(f)
	if _, err := svc.Adjust(context.Background(), lot.String(), dto.PantryAdjustInput{Quantity: -1, Source: "manual"}); !errors.Is(err, dto.ErrInvalid) {
		t.Fatalf("expected dto.ErrInvalid for negative adjustment, got %v", err)
	}
}

func TestPantryMarkEmpty(t *testing.T) {
	f, _, _, lot := newPantryLedgerFixture(t)
	svc := service.NewPantry(f)
	out, err := svc.MarkEmpty(context.Background(), lot.String())
	if err != nil {
		t.Fatal(err)
	}
	if out.Quantity != 0 {
		t.Fatalf("expected empty lot, got %v", out.Quantity)
	}
}

func TestPantryOpen(t *testing.T) {
	f, _, _, lot := newPantryLedgerFixture(t)
	svc := service.NewPantry(f)
	out, err := svc.Open(context.Background(), lot.String(), dto.PantryOpenInput{Source: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	if out.OpenedAt.IsZero() {
		t.Fatal("expected opened_at to be set")
	}
}

func TestPantryOpenInvalid(t *testing.T) {
	f, _, _, lot := newPantryLedgerFixture(t)
	svc := service.NewPantry(f)
	if _, err := svc.Open(context.Background(), lot.String(), dto.PantryOpenInput{}); !errors.Is(err, dto.ErrInvalid) {
		t.Fatalf("expected dto.ErrInvalid for missing source, got %v", err)
	}
}

func TestPantryTransferFull(t *testing.T) {
	f, _, dst, lot := newPantryLedgerFixture(t)
	svc := service.NewPantry(f)
	out, err := svc.Transfer(context.Background(), lot.String(), dto.PantryTransferInput{LocationID: dst.String(), Quantity: 2.0, Source: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	if out.ID != lot.String() || out.LocationID != dst.String() || out.Quantity != 2.0 {
		t.Fatalf("unexpected transferred lot: %+v", out)
	}
}

func TestPantryTransferPartial(t *testing.T) {
	f, _, dst, lot := newPantryLedgerFixture(t)
	svc := service.NewPantry(f)
	out, err := svc.Transfer(context.Background(), lot.String(), dto.PantryTransferInput{LocationID: dst.String(), Quantity: 1.0, Source: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	if out.ID == lot.String() || out.LocationID != dst.String() || out.Quantity != 1.0 {
		t.Fatalf("unexpected partial transfer lot: %+v", out)
	}
}

func TestPantryTransferInvalid(t *testing.T) {
	f, _, dst, lot := newPantryLedgerFixture(t)
	svc := service.NewPantry(f)
	if _, err := svc.Transfer(context.Background(), lot.String(), dto.PantryTransferInput{LocationID: dst.String(), Quantity: 3.0, Source: "manual"}); !errors.Is(err, dto.ErrInvalid) {
		t.Fatalf("expected dto.ErrInvalid for oversized transfer, got %v", err)
	}
	if _, err := svc.Transfer(context.Background(), lot.String(), dto.PantryTransferInput{Quantity: 1.0, Source: "manual"}); !errors.Is(err, dto.ErrInvalid) {
		t.Fatalf("expected dto.ErrInvalid for missing destination, got %v", err)
	}
}

func TestMealsGet(t *testing.T) {
	refID := domain.NewRecipeRefID()
	f := &fakeStore{
		recipes: []persistence.RecipeRef{
			{ID: refID, MealieRecipeID: "r1", Title: "Pasta", Effort: 1},
		},
		mealEventRef: refID,
	}
	svc := service.NewMeals(f, nil)
	mealID := domain.NewMealEventID()
	out, err := svc.GetMeal(context.Background(), mealID.String())
	if err != nil {
		t.Fatal(err)
	}
	if out.ID != mealID.String() {
		t.Fatalf("expected id %s, got %s", mealID.String(), out.ID)
	}
}

func TestMealsList(t *testing.T) {
	f := &fakeStore{}
	svc := service.NewMeals(f, nil)
	out, err := svc.ListMeals(context.Background(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Fatalf("expected 0 meals, got %d", len(out))
	}
}

func TestIngredientsGetMapping(t *testing.T) {
	f := &fakeStore{}
	slvURL := os.Getenv("SLV_BASE_URL")
	var slv *ingredients.Client
	if slvURL != "" {
		slv = ingredients.NewLivsmedelsverket(slvURL)
	}
	svc := service.NewIngredients(f, slv, nil, nil)
	out, err := svc.GetMapping(context.Background(), "mealie-food-1")
	if err != nil {
		t.Fatal(err)
	}
	want := domain.IngredientIDForName("cauliflower").String()
	if out.IngredientID != want {
		t.Fatalf("expected %s, got %s", want, out.IngredientID)
	}
}

func TestIngredientsNutritionByID(t *testing.T) {
	fakeSLV := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/livsmedel/12345/naringsvarden" {
			w.Write([]byte(`[{"namn":"Energi","varde":100,"enhet":"kJ"},{"namn":"Protein","varde":2.5,"enhet":"g"}]`))
			return
		}
		w.WriteHeader(404)
	}))
	t.Cleanup(fakeSLV.Close)

	svc := service.NewIngredients(&fakeStore{}, ingredients.NewLivsmedelsverket(fakeSLV.URL), nil, nil)

	out, err := svc.NutritionByID(context.Background(), "slv-12345")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 nutrients, got %d", len(out))
	}
	if out[0].Name != "Energi" || out[0].Value != 100 || out[0].Unit != "kJ" {
		t.Fatalf("unexpected first nutrient: %+v", out[0])
	}

	if _, err := svc.NutritionByID(context.Background(), "cauliflower"); err == nil {
		t.Fatal("expected error for non-slv ingredient id")
	}
	if _, err := svc.NutritionByID(context.Background(), "slv-notanumber"); err == nil {
		t.Fatal("expected error for malformed slv id")
	}
}

func TestStoresListStores(t *testing.T) {
	sid1 := testStoreID("s-1")
	sid2 := testStoreID("s-2")
	ridICA := testRetailerID("ica")
	ridWillys := testRetailerID("willys")
	f := &fakeStore{stores: []domain.Store{
		{ID: sid1, RetailerID: ridICA, Name: "ICA Lindhagen"},
		{ID: sid2, RetailerID: ridWillys, Name: "Willys Kungsholmen"},
	}}
	svc := service.NewStores(f, nil)

	out, err := svc.ListStores(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 stores, got %d", len(out))
	}
	if out[0].ID != sid1.String() || out[0].RetailerID != ridICA.String() || out[0].Name != "ICA Lindhagen" {
		t.Fatalf("unexpected first store: %+v", out[0])
	}
}

func TestStoresListStoreOffers(t *testing.T) {
	sid1 := testStoreID("s-1")
	sid2 := testStoreID("s-2")
	rpid1 := testRetailerProductID("rp-1")
	rpid2 := testRetailerProductID("rp-2")
	f := &fakeStore{offers: []domain.StoreProductOffer{
		{ID: domain.StoreProductOfferID(slugToUUID("offer-1")), StoreID: sid1, RetailerProductID: rpid1, CurrentlyCarried: true},
		{ID: domain.StoreProductOfferID(slugToUUID("offer-2")), StoreID: sid2, RetailerProductID: rpid2, CurrentlyCarried: false},
	}}
	svc := service.NewStores(f, nil)

	out, err := svc.ListStoreOffers(context.Background(), sid1.String())
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 offer for s-1, got %d", len(out))
	}
	if out[0].ID != domain.StoreProductOfferID(slugToUUID("offer-1")).String() || out[0].RetailerProductID != rpid1.String() || !out[0].CurrentlyCarried {
		t.Fatalf("unexpected offer: %+v", out[0])
	}
}

func TestStoresSearchProductsNoClient(t *testing.T) {
	svc := service.NewStores(&fakeStore{}, nil)
	if _, err := svc.SearchProducts(context.Background(), "mjölk"); err == nil {
		t.Fatal("expected error when MPK client is nil")
	}
	if _, err := svc.SearchProductsByGTIN(context.Background(), "123"); err == nil {
		t.Fatal("expected error when MPK client is nil")
	}
}

func TestRecipeFamilyGetNotFound(t *testing.T) {
	svc := service.NewRecipeFamily(&fakeStore{})
	famID := domain.NewRecipeFamilyID().String()
	varID := domain.NewRecipeVariantID().String()
	revID := domain.NewRecipeRevisionID().String()
	if _, err := svc.GetFamily(context.Background(), famID); !errors.Is(err, dto.ErrNotFound) {
		t.Fatalf("expected dto.ErrNotFound, got %v", err)
	}
	if _, err := svc.ListVariants(context.Background(), famID); !errors.Is(err, dto.ErrNotFound) {
		t.Fatalf("expected dto.ErrNotFound, got %v", err)
	}
	if _, err := svc.ListRevisions(context.Background(), varID); !errors.Is(err, dto.ErrNotFound) {
		t.Fatalf("expected dto.ErrNotFound, got %v", err)
	}
	if _, err := svc.GetRevision(context.Background(), revID); !errors.Is(err, dto.ErrNotFound) {
		t.Fatalf("expected dto.ErrNotFound, got %v", err)
	}
}

func TestRecipeFamilySetDefaultVariantWrongFamily(t *testing.T) {
	// fakeStore returns persistence.ErrNoRows for GetRecipeFamily, so the family lookup
	// fails first and maps to ErrNotFound.
	svc := service.NewRecipeFamily(&fakeStore{})
	famID := domain.NewRecipeFamilyID().String()
	varID := domain.NewRecipeVariantID().String()
	if err := svc.SetDefaultVariant(context.Background(), famID, varID); !errors.Is(err, dto.ErrNotFound) {
		t.Fatalf("expected dto.ErrNotFound, got %v", err)
	}
}

// TestRecipeFamilyCreateFamilySlug exercises slugify indirectly: when no ID is
// supplied, CreateFamily derives one from the name via slugify.
func TestRecipeFamilyCreateFamilySlug(t *testing.T) {
	svc := service.NewRecipeFamily(&fakeStore{})
	out, err := svc.CreateFamily(context.Background(), dto.CreateRecipeFamilyInput{
		Name: "Korvstroganoff",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Name != "Korvstroganoff" {
		t.Fatalf("expected name Korvstroganoff, got %q", out.Name)
	}
}

func TestRecipeFamilyCreateFamilyRequiresName(t *testing.T) {
	svc := service.NewRecipeFamily(&fakeStore{})
	if _, err := svc.CreateFamily(context.Background(), dto.CreateRecipeFamilyInput{ID: "x"}); err == nil {
		t.Fatal("expected error when name is empty")
	}
}

func TestFavoritesGetRecipeRating(t *testing.T) {
	f := &fakeStore{recipes: []persistence.RecipeRef{
		{ID: domain.NewRecipeRefID(), MealieRecipeID: "rec-1", Title: "Test", Effort: 1},
	}}
	svc := service.NewFavorites(f)
	out, err := svc.GetRecipeRating(context.Background(), "rec-1")
	if err != nil {
		t.Fatal(err)
	}
	if out.MealieRecipeID != "rec-1" || out.Average != 4.2 || out.ReviewCount != 3 {
		t.Fatalf("unexpected rating: %+v", out)
	}
}

func TestFavoritesListEmpty(t *testing.T) {
	f := &fakeStore{recipes: []persistence.RecipeRef{
		{ID: domain.NewRecipeRefID(), MealieRecipeID: "rec-1", Title: "Test", Effort: 1},
	}}
	svc := service.NewFavorites(f)
	out, err := svc.ListFavoritesForRecipe(context.Background(), "rec-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Fatalf("expected no favorites, got %d", len(out))
	}
}

func TestPantryListExpiring(t *testing.T) {
	svc := service.NewPantry(&fakeStore{})
	out, err := svc.ListExpiring(context.Background(), 7*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 expiring lot, got %d", len(out))
	}
	if out[0].IngredientID != domain.IngredientIDForName("mjolk").String() || out[0].Quantity != 1 {
		t.Fatalf("unexpected expiring lot: %+v", out[0])
	}
	if out[0].BestBefore.IsZero() {
		t.Fatal("expected a non-zero best_before")
	}
}

func TestPriceIntelligenceCheapestStore(t *testing.T) {
	svc := service.NewPriceIntelligence(&fakeStore{})
	out, err := svc.ListProductPrices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 product group, got %d", len(out))
	}
	g := out[0]
	if g.RetailerProductID != testRetailerProductID("rp-1").String() {
		t.Fatalf("expected rp-1, got %q", g.RetailerProductID)
	}
	if len(g.Prices) != 2 {
		t.Fatalf("expected 2 prices, got %d", len(g.Prices))
	}
	if g.Cheapest == nil {
		t.Fatal("expected a cheapest store")
	}
	if g.Cheapest.StoreID != testStoreID("s-2").String() || g.Cheapest.Price != 17.5 {
		t.Fatalf("expected cheapest s-2 @ 17.5, got %+v", g.Cheapest)
	}
	if g.RetailerName != "Willys" {
		t.Fatalf("expected retailer name Willys, got %q", g.RetailerName)
	}
	if g.DisplayName != "Mjölk 3%" {
		t.Fatalf("expected display name Mjölk 3%%, got %q", g.DisplayName)
	}
}

func TestStoresLocateRanksByDistance(t *testing.T) {
	// Origin: Stockholm city centre (59.3293, 18.0686).
	// s-near is ~1 km away, s-far is ~100 km away, s-nogeo has no position.
	ridWillys := testRetailerID("willys")
	sidFar := testStoreID("s-far")
	sidNear := testStoreID("s-near")
	sidNogeo := testStoreID("s-nogeo")
	f := &fakeStore{
		stores: []domain.Store{
			{ID: sidFar, RetailerID: ridWillys, Name: "Far Store",
				Latitude: ptr64(59.8587), Longitude: ptr64(17.6389)},
			{ID: sidNear, RetailerID: ridWillys, Name: "Near Store",
				Latitude: ptr64(59.3345), Longitude: ptr64(18.0720)},
			{ID: sidNogeo, RetailerID: ridWillys, Name: "No Geo Store"},
		},
	}
	svc := service.NewStores(f, nil)
	out, err := svc.LocateStores(context.Background(), dto.LocateStoresInput{
		Latitude:  ptr64(59.3293),
		Longitude: ptr64(18.0686),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 3 {
		t.Fatalf("expected 3 stores, got %d", len(out))
	}
	// Nearest first; geo-less store last.
	if out[0].ID != sidNear.String() {
		t.Fatalf("expected s-near first, got %q", out[0].ID)
	}
	if out[2].ID != sidNogeo.String() {
		t.Fatalf("expected s-nogeo last, got %q", out[2].ID)
	}
	if out[0].DistanceKm == nil {
		t.Fatal("expected s-near to have a distance")
	}
	if *out[0].DistanceKm > 5 {
		t.Fatalf("expected s-near within 5 km, got %.2f", *out[0].DistanceKm)
	}
	if out[2].DistanceKm != nil {
		t.Fatal("expected s-nogeo to have no distance")
	}
	if out[0].RetailerName != "Willys" {
		t.Fatalf("expected retailer name Willys, got %q", out[0].RetailerName)
	}
}

func TestStoresLocateNoOriginSortsByName(t *testing.T) {
	ridWillys := testRetailerID("willys")
	sidB := testStoreID("s-b")
	sidA := testStoreID("s-a")
	f := &fakeStore{
		stores: []domain.Store{
			{ID: sidB, RetailerID: ridWillys, Name: "Beta"},
			{ID: sidA, RetailerID: ridWillys, Name: "Alpha"},
		},
	}
	svc := service.NewStores(f, nil)
	out, err := svc.LocateStores(context.Background(), dto.LocateStoresInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 stores, got %d", len(out))
	}
	if out[0].ID != sidA.String() || out[1].ID != sidB.String() {
		t.Fatalf("expected Alpha then Beta, got %q then %q", out[0].ID, out[1].ID)
	}
	if out[0].DistanceKm != nil {
		t.Fatal("expected no distance without an origin")
	}
}

func ptr64(v float64) *float64 { return &v }

// fakeTonightProvider is a service.TonightProvider for dashboard tests.
type fakeTonightProvider struct {
	view dto.TonightView
	err  error
}

func (f *fakeTonightProvider) GetTonight(ctx context.Context) (dto.TonightView, error) {
	return f.view, f.err
}

// fakePantryProvider is a service.PantryProvider for dashboard tests.
type fakePantryProvider struct {
	locations []dto.PantryLocation
	expiring  []dto.PantryLot
	err       error
}

func (f *fakePantryProvider) ListLocations(ctx context.Context, householdID string) ([]dto.PantryLocation, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.locations, nil
}

func (f *fakePantryProvider) ListExpiring(ctx context.Context, within time.Duration) ([]dto.PantryLot, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.expiring, nil
}

func TestDashboardAggregatesTonightAndPantry(t *testing.T) {
	// One location with two lots (counted via the db fakeStore), one expiring.
	db := &fakeStore{}
	lotLocID := domain.NewInventoryLocationID()
	db.locations = map[domain.InventoryLocationID]persistence.InventoryLocation{
		lotLocID: {ID: lotLocID, Name: "Fridge", HouseholdID: domain.NewHouseholdID()},
	}
	db.lots = []persistence.InventoryLot{
		{ID: domain.NewInventoryLotID(), IngredientID: domain.NewIngredientID(), LocationID: lotLocID, Quantity: 1, Unit: "L"},
		{ID: domain.NewInventoryLotID(), IngredientID: domain.NewIngredientID(), LocationID: lotLocID, Quantity: 2, Unit: "st"},
	}
	tonight := &fakeTonightProvider{view: dto.TonightView{
		ServedOn: "2026-08-28",
		Recipe:   dto.RecipeRefResponse{MealieRecipeID: "r-1", Title: "Pasta"},
	}}
	pantry := &fakePantryProvider{
		locations: []dto.PantryLocation{{ID: lotLocID.String(), Name: "Fridge"}},
		expiring: []dto.PantryLot{
			{ID: "lot-1", IngredientID: "mjolk", Quantity: 1, Unit: "L", BestBefore: time.Now().Add(24 * time.Hour)},
		},
	}
	svc := service.NewDashboard(db, tonight, pantry)
	out, err := svc.Get(context.Background(), "household-1")
	if err != nil {
		t.Fatal(err)
	}
	if out.Tonight == nil || out.Tonight.Recipe.Title != "Pasta" {
		t.Fatalf("expected tonight Pasta, got %+v", out.Tonight)
	}
	if out.Pantry.Locations != 1 {
		t.Fatalf("expected 1 location, got %d", out.Pantry.Locations)
	}
	if out.Pantry.Lots != 2 {
		t.Fatalf("expected 2 lots, got %d", out.Pantry.Lots)
	}
	if out.Pantry.Expiring != 1 {
		t.Fatalf("expected 1 expiring, got %d", out.Pantry.Expiring)
	}
	if len(out.Expiring) != 1 || out.Expiring[0].IngredientID != "mjolk" {
		t.Fatalf("expected expiring mjolk, got %+v", out.Expiring)
	}
}

func TestDashboardNoMealTonightLeavesTonightNil(t *testing.T) {
	db := &fakeStore{}
	tonight := &fakeTonightProvider{err: dto.ErrNoMealTonight}
	pantry := &fakePantryProvider{}
	svc := service.NewDashboard(db, tonight, pantry)
	out, err := svc.Get(context.Background(), "household-1")
	if err != nil {
		t.Fatal(err)
	}
	if out.Tonight != nil {
		t.Fatalf("expected nil tonight, got %+v", out.Tonight)
	}
}

func TestIngredientAliasCreateRequiresFields(t *testing.T) {
	svc := service.NewIngredientAlias(&fakeStore{})
	if _, err := svc.Create(context.Background(), dto.IngredientAliasNew{Alias: "", IngredientID: "potato"}); err == nil {
		t.Fatal("expected error for empty alias")
	}
	if _, err := svc.Create(context.Background(), dto.IngredientAliasNew{Alias: "potatis", IngredientID: ""}); err == nil {
		t.Fatal("expected error for empty ingredient_id")
	}
}

func TestIngredientAliasResolve(t *testing.T) {
	svc := service.NewIngredientAlias(&fakeStore{})
	id, err := svc.Resolve(context.Background(), "h-1", "potatis")
	if err != nil {
		t.Fatal(err)
	}
	if id != "" {
		t.Fatalf("expected empty resolve for unknown alias, got %q", id)
	}
}

func TestInspirationSuggest_RanksByPantryCoverage(t *testing.T) {
	r1ID := domain.NewRecipeRefID()
	r2ID := domain.NewRecipeRefID()
	flourID := domain.NewIngredientID()
	tomatoID := domain.NewIngredientID()
	sugarID := domain.NewIngredientID()
	lettuceID := domain.NewIngredientID()
	oilID := domain.NewIngredientID()
	f := &fakeStore{
		recipes: []persistence.RecipeRef{
			{ID: r1ID, MealieRecipeID: "r1", Title: "Pasta", Tags: []string{"italian"}, Effort: 1},
			{ID: r2ID, MealieRecipeID: "r2", Title: "Salad", Effort: 1},
		},
		lots: []persistence.InventoryLot{
			{ID: domain.NewInventoryLotID(), IngredientID: flourID, Quantity: 500, Unit: "g"},
			{ID: domain.NewInventoryLotID(), IngredientID: tomatoID, Quantity: 3, Unit: "pcs"},
			{ID: domain.NewInventoryLotID(), IngredientID: lettuceID, Quantity: 1, Unit: "head"},
		},
		recipeIngredients: []persistence.RecipeIngredient{
			// r1 (Pasta): flour + tomato + sugar → 2/3 matched
			{RecipeRefID: r1ID, IngredientID: flourID},
			{RecipeRefID: r1ID, IngredientID: tomatoID},
			{RecipeRefID: r1ID, IngredientID: sugarID},
			// r2 (Salad): lettuce + oil → 1/2 matched
			{RecipeRefID: r2ID, IngredientID: lettuceID},
			{RecipeRefID: r2ID, IngredientID: oilID},
		},
	}
	svc := service.NewInspiration(f)
	out, err := svc.Suggest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 suggestions, got %d", len(out))
	}
	// Pasta (2/3 ≈ 0.67) should rank above Salad (1/2 = 0.5).
	if out[0].MealieRecipeID != "r1" {
		t.Fatalf("expected Pasta first, got %s", out[0].MealieRecipeID)
	}
	if out[0].MatchRatio < 0.66 || out[0].MatchRatio > 0.67 {
		t.Fatalf("unexpected Pasta ratio: %f", out[0].MatchRatio)
	}
	if len(out[0].MissingIngredientIDs) != 1 || out[0].MissingIngredientIDs[0] != sugarID.String() {
		t.Fatalf("unexpected Pasta missing: %+v", out[0].MissingIngredientIDs)
	}
}

func TestInspirationSuggest_OmitsNoMatch(t *testing.T) {
	r1ID := domain.NewRecipeRefID()
	r2ID := domain.NewRecipeRefID()
	flourID := domain.NewIngredientID()
	spiceID := domain.NewIngredientID()
	f := &fakeStore{
		recipes: []persistence.RecipeRef{
			{ID: r1ID, MealieRecipeID: "r1", Title: "Pasta", Effort: 1},
			{ID: r2ID, MealieRecipeID: "r2", Title: "Curry", Effort: 2},
		},
		lots: []persistence.InventoryLot{
			{ID: domain.NewInventoryLotID(), IngredientID: flourID, Quantity: 500, Unit: "g"},
		},
		recipeIngredients: []persistence.RecipeIngredient{
			{RecipeRefID: r1ID, IngredientID: flourID},
			// r2 shares nothing with the pantry.
			{RecipeRefID: r2ID, IngredientID: spiceID},
		},
	}
	svc := service.NewInspiration(f)
	out, err := svc.Suggest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].MealieRecipeID != "r1" {
		t.Fatalf("expected only Pasta, got %+v", out)
	}
}

func TestInspirationSuggest_EmptyPantry(t *testing.T) {
	r1ID := domain.NewRecipeRefID()
	flourID := domain.NewIngredientID()
	f := &fakeStore{
		recipes: []persistence.RecipeRef{
			{ID: r1ID, MealieRecipeID: "r1", Title: "Pasta", Effort: 1},
		},
		recipeIngredients: []persistence.RecipeIngredient{
			{RecipeRefID: r1ID, IngredientID: flourID},
		},
	}
	svc := service.NewInspiration(f)
	out, err := svc.Suggest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Fatalf("expected no suggestions with empty pantry, got %+v", out)
	}
}
