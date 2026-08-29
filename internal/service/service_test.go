package service_test

import (
	"context"
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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// fakeStore is a minimal in-memory Store for unit testing.
type fakeStore struct {
	people            map[string]persistence.Person
	prefs             []persistence.PersonPreference
	recipes           []persistence.RecipeRef
	plans             map[int64]persistence.MealPlan
	lots              []persistence.InventoryLot
	locations         map[string]persistence.InventoryLocation
	stores            []domain.Store
	offers            []domain.StoreProductOffer
	recipeIngredients []persistence.RecipeIngredient
}

func (f *fakeStore) CreatePerson(ctx context.Context, p persistence.Person) error {
	f.people[p.ID] = p
	return nil
}
func (f *fakeStore) UpdatePerson(ctx context.Context, p persistence.Person) error {
	existing, ok := f.people[p.ID]
	if !ok {
		return pgx.ErrNoRows
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
func (f *fakeStore) ListPreferences(ctx context.Context, personID string) ([]persistence.PersonPreference, error) {
	out := make([]persistence.PersonPreference, 0, len(f.prefs))
	for _, p := range f.prefs {
		if personID != "" && p.PersonID != personID {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}
func (f *fakeStore) ListAllStores(ctx context.Context) ([]domain.Store, error) {
	return f.stores, nil
}
func (f *fakeStore) ListPantryIngredientIDs(ctx context.Context) ([]string, error) {
	seen := make(map[string]bool)
	var out []string
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
	return persistence.IngredientAlias{}, pgx.ErrNoRows
}
func (f *fakeStore) ListIngredientAliases(ctx context.Context, householdID string) ([]persistence.IngredientAlias, error) {
	return []persistence.IngredientAlias{}, nil
}
func (f *fakeStore) DeleteIngredientAlias(ctx context.Context, householdID, alias string) error { return nil }
func (f *fakeStore) ResolveIngredientAlias(ctx context.Context, householdID, alias string) (string, error) {
	return "", nil
}
func (f *fakeStore) ListStoreProductOffers(ctx context.Context, storeID string) ([]domain.StoreProductOffer, error) {
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
func (f *fakeStore) GetRecipeRef(ctx context.Context, id string) (persistence.RecipeRef, error) {
	for _, r := range f.recipes {
		if r.MealieRecipeID == id {
			return r, nil
		}
	}
	return persistence.RecipeRef{}, pgx.ErrNoRows
}
func (f *fakeStore) UpsertRecipeRef(ctx context.Context, r persistence.RecipeRef) error {
	for i, existing := range f.recipes {
		if existing.MealieRecipeID == r.MealieRecipeID {
			f.recipes[i] = r
			return nil
		}
	}
	f.recipes = append(f.recipes, r)
	return nil
}
func (f *fakeStore) CreateMealEvent(ctx context.Context, mealieRecipeID string, servedOn time.Time, planID *int64, planSlotDate *time.Time) (int64, error) {
	return 1, nil
}
func (f *fakeStore) AddMealReaction(ctx context.Context, r persistence.MealReaction) error { return nil }
func (f *fakeStore) ListMealReactions(ctx context.Context, eventID int64) ([]persistence.MealReaction, error) {
	return []persistence.MealReaction{{MealEventID: eventID, PersonID: "p1", Sentiment: 1}}, nil
}
func (f *fakeStore) GetMealPlan(ctx context.Context, id int64) (persistence.MealPlan, error) {
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
	id := int64(len(f.plans) + 1)
	p := persistence.MealPlan{ID: id, WeekStart: weekStart, Status: "draft", CreatedAt: time.Now()}
	f.plans[id] = p
	return p, nil
}
func (f *fakeStore) SetMealPlanStatus(ctx context.Context, id int64, status string) error {
	if p, ok := f.plans[id]; ok {
		p.Status = status
		f.plans[id] = p
	}
	return nil
}
func (f *fakeStore) InsertCandidate(ctx context.Context, c persistence.MealPlanCandidate) error { return nil }
func (f *fakeStore) ListCandidates(ctx context.Context, planID int64) ([]persistence.MealPlanCandidate, error) {
	return nil, nil
}
func (f *fakeStore) SetDecision(ctx context.Context, d persistence.MealPlanDecision) error { return nil }
func (f *fakeStore) ListDecisions(ctx context.Context, planID int64) ([]persistence.MealPlanDecision, error) {
	return nil, nil
}
func (f *fakeStore) InsertShoppingRequirement(ctx context.Context, r persistence.ShoppingRequirement) error { return nil }
func (f *fakeStore) ListShoppingRequirements(ctx context.Context, planID int64) ([]persistence.ShoppingRequirement, error) {
	return nil, nil
}
func (f *fakeStore) UpsertIngredientMapping(ctx context.Context, m persistence.IngredientMapping) error { return nil }
func (f *fakeStore) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return &fakeTx{}, nil
}
func (f *fakeStore) CreateInventoryLocation(ctx context.Context, l persistence.InventoryLocation) error {
	f.locations[l.ID] = l
	return nil
}
func (f *fakeStore) GetInventoryLocation(ctx context.Context, id string) (persistence.InventoryLocation, error) {
	l, ok := f.locations[id]
	if !ok {
		return persistence.InventoryLocation{}, errors.New("not found")
	}
	return l, nil
}
func (f *fakeStore) ListLotsUnderLocation(ctx context.Context, id string) ([]persistence.InventoryLot, error) {
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
		if householdID != "" && l.HouseholdID != householdID {
			continue
		}
		out = append(out, l)
	}
	return out, nil
}
func (f *fakeStore) RecordPurchase(ctx context.Context, ingredientID, productID, locationID string, quantity float64, unit string, bestBefore *time.Time, source string) (int64, error) {
	id := int64(len(f.lots) + 1)
	f.lots = append(f.lots, persistence.InventoryLot{ID: id, IngredientID: ingredientID, ProductID: productID, LocationID: locationID, Quantity: quantity, Unit: unit, Confidence: "EXACT", CreatedAt: time.Now(), UpdatedAt: time.Now()})
	return id, nil
}
func (f *fakeStore) RecordConsume(ctx context.Context, lotID int64, quantity float64, estimated bool, source string) error { return nil }
func (f *fakeStore) GetInventoryLot(ctx context.Context, id int64) (persistence.InventoryLot, error) {
	for _, l := range f.lots {
		if l.ID == id {
			return l, nil
		}
	}
	return persistence.InventoryLot{}, errors.New("not found")
}
func (f *fakeStore) ListMealEvents(ctx context.Context, mealieRecipeID, servedOn string) ([]persistence.MealEvent, error) {
	return nil, nil
}
func (f *fakeStore) GetMealEvent(ctx context.Context, id int64) (persistence.MealEvent, error) {
	return persistence.MealEvent{ID: id, MealieRecipeID: "r-1", ServedOn: time.Now()}, nil
}
func (f *fakeStore) GetIngredientMapping(ctx context.Context, mealieFoodID string) (persistence.IngredientMapping, error) {
	return persistence.IngredientMapping{MealieFoodID: mealieFoodID, IngredientID: "cauliflower"}, nil
}
func (f *fakeStore) ListMealPlans(ctx context.Context) ([]persistence.MealPlan, error) {
	out := make([]persistence.MealPlan, 0, len(f.plans))
	for _, p := range f.plans {
		out = append(out, p)
	}
	return out, nil
}
func (f *fakeStore) CreateRecipeFamily(ctx context.Context, fam persistence.RecipeFamily) error { return nil }
func (f *fakeStore) GetRecipeFamily(ctx context.Context, id string) (persistence.RecipeFamily, error) {
	return persistence.RecipeFamily{}, pgx.ErrNoRows
}
func (f *fakeStore) ListRecipeFamilies(ctx context.Context) ([]persistence.RecipeFamily, error) {
	return nil, nil
}
func (f *fakeStore) SetRecipeFamilyDefaultVariant(ctx context.Context, familyID, variantID string) error {
	return nil
}
func (f *fakeStore) CreateRecipeVariant(ctx context.Context, v persistence.RecipeVariant) error { return nil }
func (f *fakeStore) GetRecipeVariant(ctx context.Context, id string) (persistence.RecipeVariant, error) {
	return persistence.RecipeVariant{}, pgx.ErrNoRows
}
func (f *fakeStore) ListRecipeVariants(ctx context.Context, familyID string) ([]persistence.RecipeVariant, error) {
	return nil, nil
}
func (f *fakeStore) CreateRecipeRevision(ctx context.Context, r persistence.RecipeRevision) (int64, error) {
	return 1, nil
}
func (f *fakeStore) GetRecipeRevision(ctx context.Context, id int64) (persistence.RecipeRevision, error) {
	return persistence.RecipeRevision{}, pgx.ErrNoRows
}
func (f *fakeStore) ListRecipeRevisions(ctx context.Context, variantID string) ([]persistence.RecipeRevision, error) {
	return nil, nil
}
func (f *fakeStore) AddRecipeRevisionParent(ctx context.Context, child, parent int64) error { return nil }
func (f *fakeStore) ListRecipeRevisionParents(ctx context.Context, revisionID int64) ([]int64, error) {
	return nil, nil
}
func (f *fakeStore) UpsertFavorite(ctx context.Context, personID, householdID, mealieRecipeID string) error {
	return nil
}
func (f *fakeStore) DeleteFavorite(ctx context.Context, personID, householdID, mealieRecipeID string) error {
	return nil
}
func (f *fakeStore) ListFavoritesForRecipe(ctx context.Context, mealieRecipeID string) ([]persistence.Favorite, error) {
	return nil, nil
}
func (f *fakeStore) GetRecipeRating(ctx context.Context, mealieRecipeID string) (persistence.RecipeRating, error) {
	return persistence.RecipeRating{MealieRecipeID: mealieRecipeID, Average: 4.2, ReviewCount: 3}, nil
}
func (f *fakeStore) ListRetailers(ctx context.Context) ([]domain.Retailer, error) {
	return []domain.Retailer{{ID: "willys", Name: "Willys"}}, nil
}
func (f *fakeStore) ListRetailerProducts(ctx context.Context, retailerID string) ([]domain.RetailerProduct, error) {
	return []domain.RetailerProduct{{ID: "rp-1", RetailerID: retailerID, ProductID: "prod-1", DisplayName: "Mjölk 3%"}}, nil
}
func (f *fakeStore) ListCurrentPrices(ctx context.Context) ([]domain.CurrentStoreProductPrice, error) {
	return []domain.CurrentStoreProductPrice{
		{OfferID: 1, StoreID: "s-1", RetailerProductID: "rp-1", PriceKind: domain.PriceKindRegular, Price: 19.9, ObservedAt: time.Now(), Source: "willys_adapter"},
		{OfferID: 2, StoreID: "s-2", RetailerProductID: "rp-1", PriceKind: domain.PriceKindRegular, Price: 17.5, ObservedAt: time.Now(), Source: "willys_adapter"},
	}, nil
}
func (f *fakeStore) ListExpiringLots(ctx context.Context, within time.Duration) ([]persistence.InventoryLot, error) {
	return []persistence.InventoryLot{
		{ID: 1, IngredientID: "mjolk", LocationID: "l-1", Quantity: 1, Unit: "L", Confidence: domain.ConfidenceExact, BestBefore: &[]time.Time{time.Now().Add(24 * time.Hour)}[0]},
	}, nil
}

// fakeTx is a minimal no-op pgx.Tx for tests.
type fakeTx struct{}

func (f *fakeTx) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("OK"), nil
}
func (f *fakeTx) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return &fakeRow{id: int64(1)}
}
func (f *fakeTx) Commit(ctx context.Context) error { return nil }
func (f *fakeTx) Rollback(ctx context.Context) error { return nil }
func (f *fakeTx) Begin(ctx context.Context) (pgx.Tx, error) { return f, nil }
func (f *fakeTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) { return 0, nil }
func (f *fakeTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults { return &fakeBatch{} }
func (f *fakeTx) LargeObjects() pgx.LargeObjects { return pgx.LargeObjects{} }
func (f *fakeTx) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) { return nil, nil }
func (f *fakeTx) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) { return nil, nil }
func (f *fakeTx) Conn() *pgx.Conn { return nil }

type fakeRow struct{ id int64 }

func (f *fakeRow) Scan(dest ...interface{}) error {
	if len(dest) > 0 {
		if p, ok := dest[0].(*int64); ok {
			*p = f.id
		}
	}
	return nil
}

type fakeBatch struct{}

func (f *fakeBatch) Exec() (pgconn.CommandTag, error) { return pgconn.NewCommandTag("OK"), nil }
func (f *fakeBatch) Query() (pgx.Rows, error) { return nil, nil }
func (f *fakeBatch) QueryRow() pgx.Row { return nil }
func (f *fakeBatch) Close() error { return nil }

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
	f := &fakeStore{prefs: []persistence.PersonPreference{
		{PersonID: "p1", Tag: "spicy", Sentiment: 1, Confidence: 0.8, UpdatedAt: time.Now()},
	}}
	svc := service.NewPreferences(f)
	out, err := svc.ListPreferences(context.Background(), "p1")
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
	f := &fakeStore{}
	svc := service.NewPreferences(f)
	out, err := svc.SetPreference(context.Background(), dto.SetPreferenceInput{
		PersonID: "p1", Tag: "spicy", Sentiment: 2, Confidence: 0.9,
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
	f := &fakeStore{prefs: []persistence.PersonPreference{
		{PersonID: "p1", Tag: "spicy", Sentiment: 1, Confidence: 0.5, UpdatedAt: time.Now()},
	}}
	svc := service.NewPreferences(f)
	out, err := svc.SetPreference(context.Background(), dto.SetPreferenceInput{
		PersonID: "p1", Tag: "spicy", Sentiment: -1, Confidence: 0.7,
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

	if _, err := svc.SyncFromMealie(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.recipes) != 1 {
		t.Fatalf("expected idempotent upsert, got %d refs", len(f.recipes))
	}
}

func TestRecipesSyncNoClient(t *testing.T) {
	svc := service.NewRecipes(&fakeStore{}, nil)
	if _, err := svc.SyncFromMealie(context.Background()); err == nil {
		t.Fatal("expected error when mealie client is nil")
	}
}

func TestMealsCreate(t *testing.T) {
	f := &fakeStore{}
	svc := service.NewMeals(f, nil)
	out, err := svc.CreateMealEvent(context.Background(), dto.MealEventNew{
		MealieRecipeID: "r1", ServedOn: "2025-01-15",
		Reactions: []dto.MealReactionInput{{PersonID: "p1", Sentiment: 1}},
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
	f := &fakeStore{plans: map[int64]persistence.MealPlan{
		1: {ID: 1, WeekStart: weekStart, Status: "draft", CreatedAt: time.Now()},
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
	f := &fakeStore{locations: make(map[string]persistence.InventoryLocation)}
	svc := service.NewPantry(f)
	out, err := svc.CreateLocation(context.Background(), dto.PantryLocationNew{Name: "Kitchen"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Name != "Kitchen" {
		t.Fatalf("expected Kitchen, got %s", out.Name)
	}
}

func TestPantryPurchase(t *testing.T) {
	f := &fakeStore{locations: make(map[string]persistence.InventoryLocation)}
	svc := service.NewPantry(f)
	out, err := svc.Purchase(context.Background(), dto.PantryPurchaseInput{
		IngredientID: "cauliflower", Quantity: 1.0, Unit: "piece", LocationID: "kitchen",
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
	f := &fakeStore{plans: map[int64]persistence.MealPlan{
		1: {ID: 1, WeekStart: weekStart, Status: "approved", CreatedAt: time.Now()},
	}}
	svc := service.NewPlanning(f, nil)
	out, err := svc.GetPlan(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if out.Plan.ID != 1 || out.Plan.Status != "approved" {
		t.Fatalf("unexpected plan: %+v", out.Plan)
	}
}

func TestPlanningUpdate(t *testing.T) {
	weekStart := time.Date(2025, 1, 13, 0, 0, 0, 0, time.UTC)
	f := &fakeStore{plans: map[int64]persistence.MealPlan{
		1: {ID: 1, WeekStart: weekStart, Status: "draft", CreatedAt: time.Now()},
	}}
	svc := service.NewPlanning(f, nil)
	out, err := svc.UpdatePlan(context.Background(), 1, dto.MealPlanUpdate{Status: "approved"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != "approved" {
		t.Fatalf("expected approved, got %s", out.Status)
	}
}

func TestPlanningCreate(t *testing.T) {
	f := &fakeStore{plans: make(map[int64]persistence.MealPlan)}
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
	f := &fakeStore{locations: map[string]persistence.InventoryLocation{
		"kitchen": {ID: "kitchen", Name: "Kitchen", HouseholdID: "h1"},
		"freezer": {ID: "freezer", Name: "Freezer", HouseholdID: "h1"},
	}}
	svc := service.NewPantry(f)
	out, err := svc.ListLocations(context.Background(), "h1")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 locations, got %d", len(out))
	}
}

func TestPantryConsume(t *testing.T) {
	f := &fakeStore{locations: make(map[string]persistence.InventoryLocation)}
	svc := service.NewPantry(f)
	err := svc.Consume(context.Background(), 1, dto.PantryConsumeInput{Quantity: 1.0})
	if err != nil {
		t.Fatal(err)
	}
}

func TestMealsGet(t *testing.T) {
	f := &fakeStore{}
	svc := service.NewMeals(f, nil)
	out, err := svc.GetMeal(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if out.ID != 42 {
		t.Fatalf("expected id 42, got %d", out.ID)
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
	if out.IngredientID != "cauliflower" {
		t.Fatalf("expected cauliflower, got %s", out.IngredientID)
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
	f := &fakeStore{stores: []domain.Store{
		{ID: "s-1", RetailerID: "ica", Name: "ICA Lindhagen"},
		{ID: "s-2", RetailerID: "willys", Name: "Willys Kungsholmen"},
	}}
	svc := service.NewStores(f, nil)

	out, err := svc.ListStores(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 stores, got %d", len(out))
	}
	if out[0].ID != "s-1" || out[0].RetailerID != "ica" || out[0].Name != "ICA Lindhagen" {
		t.Fatalf("unexpected first store: %+v", out[0])
	}
}

func TestStoresListStoreOffers(t *testing.T) {
	f := &fakeStore{offers: []domain.StoreProductOffer{
		{ID: 1, StoreID: "s-1", RetailerProductID: "rp-1", CurrentlyCarried: true},
		{ID: 2, StoreID: "s-2", RetailerProductID: "rp-2", CurrentlyCarried: false},
	}}
	svc := service.NewStores(f, nil)

	out, err := svc.ListStoreOffers(context.Background(), "s-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 offer for s-1, got %d", len(out))
	}
	if out[0].ID != 1 || out[0].RetailerProductID != "rp-1" || !out[0].CurrentlyCarried {
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
	if _, err := svc.GetFamily(context.Background(), "missing"); !errors.Is(err, dto.ErrNotFound) {
		t.Fatalf("expected dto.ErrNotFound, got %v", err)
	}
	if _, err := svc.ListVariants(context.Background(), "missing"); !errors.Is(err, dto.ErrNotFound) {
		t.Fatalf("expected dto.ErrNotFound, got %v", err)
	}
	if _, err := svc.ListRevisions(context.Background(), "missing"); !errors.Is(err, dto.ErrNotFound) {
		t.Fatalf("expected dto.ErrNotFound, got %v", err)
	}
	if _, err := svc.GetRevision(context.Background(), 99); !errors.Is(err, dto.ErrNotFound) {
		t.Fatalf("expected dto.ErrNotFound, got %v", err)
	}
}

func TestRecipeFamilySetDefaultVariantWrongFamily(t *testing.T) {
	// fakeStore returns pgx.ErrNoRows for GetRecipeFamily, so the family lookup
	// fails first and maps to ErrNotFound.
	svc := service.NewRecipeFamily(&fakeStore{})
	if err := svc.SetDefaultVariant(context.Background(), "fam", "var"); !errors.Is(err, dto.ErrNotFound) {
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
	if out.ID != "korvstroganoff" {
		t.Fatalf("expected slug id korvstroganoff, got %q", out.ID)
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
	svc := service.NewFavorites(&fakeStore{})
	out, err := svc.GetRecipeRating(context.Background(), "rec-1")
	if err != nil {
		t.Fatal(err)
	}
	if out.MealieRecipeID != "rec-1" || out.Average != 4.2 || out.ReviewCount != 3 {
		t.Fatalf("unexpected rating: %+v", out)
	}
}

func TestFavoritesListEmpty(t *testing.T) {
	svc := service.NewFavorites(&fakeStore{})
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
	if out[0].IngredientID != "mjolk" || out[0].Quantity != 1 {
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
	if g.RetailerProductID != "rp-1" {
		t.Fatalf("expected rp-1, got %q", g.RetailerProductID)
	}
	if len(g.Prices) != 2 {
		t.Fatalf("expected 2 prices, got %d", len(g.Prices))
	}
	if g.Cheapest == nil {
		t.Fatal("expected a cheapest store")
	}
	if g.Cheapest.StoreID != "s-2" || g.Cheapest.Price != 17.5 {
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
	f := &fakeStore{
		stores: []domain.Store{
			{ID: "s-far", RetailerID: "willys", Name: "Far Store",
				Latitude: ptr64(59.8587), Longitude: ptr64(17.6389)},
			{ID: "s-near", RetailerID: "willys", Name: "Near Store",
				Latitude: ptr64(59.3345), Longitude: ptr64(18.0720)},
			{ID: "s-nogeo", RetailerID: "willys", Name: "No Geo Store"},
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
	if out[0].ID != "s-near" {
		t.Fatalf("expected s-near first, got %q", out[0].ID)
	}
	if out[2].ID != "s-nogeo" {
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
	f := &fakeStore{
		stores: []domain.Store{
			{ID: "s-b", RetailerID: "willys", Name: "Beta"},
			{ID: "s-a", RetailerID: "willys", Name: "Alpha"},
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
	if out[0].ID != "s-a" || out[1].ID != "s-b" {
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
	db.lots = []persistence.InventoryLot{
		{ID: 1, IngredientID: "mjolk", LocationID: "l-1", Quantity: 1, Unit: "L"},
		{ID: 2, IngredientID: "ost", LocationID: "l-1", Quantity: 2, Unit: "st"},
	}
	tonight := &fakeTonightProvider{view: dto.TonightView{
		ServedOn: "2026-08-28",
		Recipe:   dto.RecipeRefResponse{MealieRecipeID: "r-1", Title: "Pasta"},
	}}
	pantry := &fakePantryProvider{
		locations: []dto.PantryLocation{{ID: "l-1", Name: "Fridge"}},
		expiring: []dto.PantryLot{
			{ID: 1, IngredientID: "mjolk", Quantity: 1, Unit: "L", BestBefore: time.Now().Add(24 * time.Hour)},
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
	f := &fakeStore{
		recipes: []persistence.RecipeRef{
			{MealieRecipeID: "r1", Title: "Pasta", Tags: []string{"italian"}, Effort: 1},
			{MealieRecipeID: "r2", Title: "Salad", Effort: 1},
		},
		lots: []persistence.InventoryLot{
			{ID: 1, IngredientID: "flour", Quantity: 500, Unit: "g"},
			{ID: 2, IngredientID: "tomato", Quantity: 3, Unit: "pcs"},
			{ID: 3, IngredientID: "lettuce", Quantity: 1, Unit: "head"},
		},
		recipeIngredients: []persistence.RecipeIngredient{
			// r1 (Pasta): flour + tomato + sugar → 2/3 matched
			{MealieRecipeID: "r1", IngredientID: "flour"},
			{MealieRecipeID: "r1", IngredientID: "tomato"},
			{MealieRecipeID: "r1", IngredientID: "sugar"},
			// r2 (Salad): lettuce + oil → 1/2 matched
			{MealieRecipeID: "r2", IngredientID: "lettuce"},
			{MealieRecipeID: "r2", IngredientID: "oil"},
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
	if len(out[0].MissingIngredientIDs) != 1 || out[0].MissingIngredientIDs[0] != "sugar" {
		t.Fatalf("unexpected Pasta missing: %+v", out[0].MissingIngredientIDs)
	}
}

func TestInspirationSuggest_OmitsNoMatch(t *testing.T) {
	f := &fakeStore{
		recipes: []persistence.RecipeRef{
			{MealieRecipeID: "r1", Title: "Pasta", Effort: 1},
			{MealieRecipeID: "r2", Title: "Curry", Effort: 2},
		},
		lots: []persistence.InventoryLot{
			{ID: 1, IngredientID: "flour", Quantity: 500, Unit: "g"},
		},
		recipeIngredients: []persistence.RecipeIngredient{
			{MealieRecipeID: "r1", IngredientID: "flour"},
			// r2 shares nothing with the pantry.
			{MealieRecipeID: "r2", IngredientID: "spice"},
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
	f := &fakeStore{
		recipes: []persistence.RecipeRef{
			{MealieRecipeID: "r1", Title: "Pasta", Effort: 1},
		},
		recipeIngredients: []persistence.RecipeIngredient{
			{MealieRecipeID: "r1", IngredientID: "flour"},
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
