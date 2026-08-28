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
	ingredients       map[string]persistence.Ingredient
	recipeIngredients []persistence.RecipeIngredient
}

func (f *fakeStore) CreatePerson(ctx context.Context, p persistence.Person) error {
	f.people[p.ID] = p
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
func (f *fakeStore) UpsertPreference(ctx context.Context, p persistence.PersonPreference) error { return nil }
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
func (f *fakeStore) UpsertIngredient(ctx context.Context, i persistence.Ingredient) error {
	if f.ingredients == nil {
		f.ingredients = map[string]persistence.Ingredient{}
	}
	f.ingredients[i.ID] = i
	return nil
}
func (f *fakeStore) AddRecipeIngredient(ctx context.Context, ri persistence.RecipeIngredient) error {
	for i, existing := range f.recipeIngredients {
		if existing.MealieRecipeID == ri.MealieRecipeID && existing.IngredientID == ri.IngredientID {
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
	if ri.MealieRecipeID != "r-pasta" || ri.IngredientID != "köttfärs" || ri.Quantity != 400 || ri.Unit != "g" {
		t.Fatalf("unexpected recipe_ingredient row: %+v", ri)
	}
	if ing, ok := f.ingredients["köttfärs"]; !ok || ing.Display != "köttfärs" {
		t.Fatalf("expected canonical ingredient %q upserted, got %+v", "köttfärs", f.ingredients)
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
	if ri.IngredientID != "falukorv" || ri.Quantity != 500 || ri.Unit != "g" {
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
