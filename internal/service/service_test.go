package service_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/androidand/spisordning/internal/httpapi"
	"github.com/androidand/spisordning/internal/ingredients"
	"github.com/androidand/spisordning/internal/persistence"
	"github.com/androidand/spisordning/internal/service"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// fakeStore is a minimal in-memory Store for unit testing.
type fakeStore struct {
	people    map[string]persistence.Person
	prefs     []persistence.PersonPreference
	recipes   []persistence.RecipeRef
	plans     map[int64]persistence.MealPlan
	lots      []persistence.InventoryLot
	locations map[string]persistence.InventoryLocation
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
func (f *fakeStore) RecordObservation(ctx context.Context, o persistence.PreferenceObservation) error { return nil }
func (f *fakeStore) ListRecipeRefs(ctx context.Context) ([]persistence.RecipeRef, error) {
	out := make([]persistence.RecipeRef, 0, len(f.recipes))
	for _, r := range f.recipes {
		out = append(out, r)
	}
	return out, nil
}
func (f *fakeStore) GetRecipeRef(ctx context.Context, id string) (persistence.RecipeRef, error) {
	return persistence.RecipeRef{}, nil
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
	out, err := svc.CreatePerson(context.Background(), httpapi.PersonInput{Name: "Test"})
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
	svc := service.NewRecipes(f)
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

func TestMealsCreate(t *testing.T) {
	f := &fakeStore{}
	svc := service.NewMeals(f, nil)
	out, err := svc.CreateMealEvent(context.Background(), httpapi.MealEventNew{
		MealieRecipeID: "r1", ServedOn: "2025-01-15",
		Reactions: []httpapi.MealReactionInput{{PersonID: "p1", Sentiment: 1}},
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
	svc := service.NewPlanning(f)
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
	out, err := svc.CreateLocation(context.Background(), httpapi.PantryLocationNew{Name: "Kitchen"})
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
	out, err := svc.Purchase(context.Background(), httpapi.PantryPurchaseInput{
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
	svc := service.NewPlanning(f)
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
	svc := service.NewPlanning(f)
	out, err := svc.UpdatePlan(context.Background(), 1, httpapi.MealPlanUpdate{Status: "approved"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != "approved" {
		t.Fatalf("expected approved, got %s", out.Status)
	}
}

func TestPlanningCreate(t *testing.T) {
	f := &fakeStore{plans: make(map[int64]persistence.MealPlan)}
	svc := service.NewPlanning(f)
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
	err := svc.Consume(context.Background(), 1, httpapi.PantryConsumeInput{Quantity: 1.0})
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
