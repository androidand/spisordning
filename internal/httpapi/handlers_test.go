package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/androidand/spisordning/internal/dto"
)

// ---- Planning fakes + tests ----

type fakePlanningSvc struct {
	plans      []dto.MealPlan
	plan       dto.MealPlanView
	updated    dto.MealPlan
	decisions  []dto.MealPlanDecision
	reqs       []dto.ShoppingRequirement
	err        error
}

func (f *fakePlanningSvc) ListPlans(ctx context.Context) ([]dto.MealPlan, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.plans, nil
}

func (f *fakePlanningSvc) CreatePlan(ctx context.Context, weekStart string) (dto.MealPlan, error) {
	if f.err != nil {
		return dto.MealPlan{}, f.err
	}
	return dto.MealPlan{ID: 1, WeekStart: weekStart, Status: "draft", CreatedAt: time.Now()}, nil
}

func (f *fakePlanningSvc) GetPlan(ctx context.Context, id int64) (dto.MealPlanView, error) {
	if f.err != nil {
		return dto.MealPlanView{}, f.err
	}
	return f.plan, nil
}

func (f *fakePlanningSvc) UpdatePlan(ctx context.Context, id int64, in dto.MealPlanUpdate) (dto.MealPlan, error) {
	if f.err != nil {
		return dto.MealPlan{}, f.err
	}
	return f.updated, nil
}

func (f *fakePlanningSvc) SetDecisions(ctx context.Context, planID int64, in []dto.MealPlanDecision) ([]dto.MealPlanDecision, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.decisions, nil
}

func (f *fakePlanningSvc) ListShoppingRequirements(ctx context.Context, planID int64) ([]dto.ShoppingRequirement, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.reqs, nil
}

func TestListPlans_HappyPath(t *testing.T) {
	svc := &fakePlanningSvc{plans: []dto.MealPlan{
		{ID: 1, WeekStart: "2026-01-13", Status: "draft", CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
	}}
	mux := newMux(t, Dependencies{Planning: svc})

	rec := doGet(t, mux, "/plans")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got []dto.MealPlan
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].ID != 1 || got[0].Status != "draft" {
		t.Fatalf("unexpected plan: %+v", got)
	}
}

func TestCreatePlan_HappyPath(t *testing.T) {
	mux := newMux(t, Dependencies{Planning: &fakePlanningSvc{}})

	rec := doPost(t, mux, "/plans", `{"week_start":"2026-01-13"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body)
	}
	var got dto.MealPlan
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.WeekStart != "2026-01-13" || got.Status != "draft" {
		t.Fatalf("unexpected plan: %+v", got)
	}
}

func TestCreatePlan_MissingWeekStart(t *testing.T) {
	mux := newMux(t, Dependencies{Planning: &fakePlanningSvc{}})

	rec := doPost(t, mux, "/plans", `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestGetPlan_HappyPath(t *testing.T) {
	svc := &fakePlanningSvc{plan: dto.MealPlanView{
		Plan: dto.MealPlan{ID: 42, WeekStart: "2026-01-13", Status: "approved"},
	}}
	mux := newMux(t, Dependencies{Planning: svc})

	rec := doGet(t, mux, "/plans/42")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got dto.MealPlanView
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.Plan.ID != 42 || got.Plan.Status != "approved" {
		t.Fatalf("unexpected plan view: %+v", got)
	}
}

func TestUpdatePlan_HappyPath(t *testing.T) {
	svc := &fakePlanningSvc{updated: dto.MealPlan{ID: 1, WeekStart: "2026-01-13", Status: "approved"}}
	mux := newMux(t, Dependencies{Planning: svc})

	rec := doPatch(t, mux, "/plans/1", `{"status":"approved"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
}

func TestListShoppingRequirements_HappyPath(t *testing.T) {
	svc := &fakePlanningSvc{reqs: []dto.ShoppingRequirement{
		{ID: 1, IngredientID: "cauliflower", Quantity: 1.0, Unit: "piece"},
	}}
	mux := newMux(t, Dependencies{Planning: svc})

	rec := doGet(t, mux, "/plans/1/shopping-requirements")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got []dto.ShoppingRequirement
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].IngredientID != "cauliflower" {
		t.Fatalf("unexpected reqs: %+v", got)
	}
}

func TestGetPlan_BadID(t *testing.T) {
	mux := newMux(t, Dependencies{Planning: &fakePlanningSvc{}})

	rec := doGet(t, mux, "/plans/abc")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// ---- Pantry fakes + tests ----

type fakePantrySvc struct {
	locations []dto.PantryLocation
	lots      []dto.PantryLot
	purchased dto.PantryLot
	err       error
}

func (f *fakePantrySvc) ListLocations(ctx context.Context, householdID string) ([]dto.PantryLocation, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.locations, nil
}

func (f *fakePantrySvc) CreateLocation(ctx context.Context, in dto.PantryLocationNew) (dto.PantryLocation, error) {
	if f.err != nil {
		return dto.PantryLocation{}, f.err
	}
	return dto.PantryLocation{ID: "loc-1", Name: in.Name, HouseholdID: in.HouseholdID}, nil
}

func (f *fakePantrySvc) ListLots(ctx context.Context, locationID string) ([]dto.PantryLot, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.lots, nil
}

func (f *fakePantrySvc) Purchase(ctx context.Context, in dto.PantryPurchaseInput) (dto.PantryLot, error) {
	if f.err != nil {
		return dto.PantryLot{}, f.err
	}
	return f.purchased, nil
}

func (f *fakePantrySvc) Consume(ctx context.Context, lotID int64, in dto.PantryConsumeInput) error {
	return f.err
}

func TestListLocations_HappyPath(t *testing.T) {
	svc := &fakePantrySvc{locations: []dto.PantryLocation{
		{ID: "kitchen", Name: "Kitchen", HouseholdID: "h1"},
	}}
	mux := newMux(t, Dependencies{Pantry: svc})

	rec := doGet(t, mux, "/pantry/locations")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got []dto.PantryLocation
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].Name != "Kitchen" {
		t.Fatalf("unexpected locations: %+v", got)
	}
}

func TestCreateLocation_HappyPath(t *testing.T) {
	mux := newMux(t, Dependencies{Pantry: &fakePantrySvc{}})

	rec := doPost(t, mux, "/pantry/locations", `{"name":"Fridge","household_id":"h1"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body)
	}
	var got dto.PantryLocation
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.Name != "Fridge" || got.ID != "loc-1" {
		t.Fatalf("unexpected location: %+v", got)
	}
}

func TestCreateLocation_EmptyName(t *testing.T) {
	svc := &fakePantrySvc{err: fmt.Errorf("name is required")}
	mux := newMux(t, Dependencies{Pantry: svc})

	rec := doPost(t, mux, "/pantry/locations", `{"name":""}`)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body: %s", rec.Code, rec.Body)
	}
}

func TestListLots_HappyPath(t *testing.T) {
	svc := &fakePantrySvc{lots: []dto.PantryLot{
		{ID: 1, IngredientID: "cauliflower", LocationID: "kitchen", Quantity: 2.0, Unit: "piece"},
	}}
	mux := newMux(t, Dependencies{Pantry: svc})

	rec := doGet(t, mux, "/pantry/locations/kitchen/lots")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got []dto.PantryLot
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].IngredientID != "cauliflower" {
		t.Fatalf("unexpected lots: %+v", got)
	}
}

func TestPurchase_HappyPath(t *testing.T) {
	svc := &fakePantrySvc{purchased: dto.PantryLot{ID: 1, IngredientID: "cauliflower", Quantity: 1.0}}
	mux := newMux(t, Dependencies{Pantry: svc})

	rec := doPost(t, mux, "/pantry/lots/purchase", `{"ingredient_id":"cauliflower","quantity":1.0,"unit":"piece","location_id":"kitchen"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body)
	}
	var got dto.PantryLot
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.IngredientID != "cauliflower" {
		t.Fatalf("unexpected lot: %+v", got)
	}
}

func TestPurchase_ZeroQuantity(t *testing.T) {
	mux := newMux(t, Dependencies{Pantry: &fakePantrySvc{}})

	rec := doPost(t, mux, "/pantry/lots/purchase", `{"ingredient_id":"cauliflower","quantity":0,"unit":"piece","location_id":"kitchen"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestConsume_HappyPath(t *testing.T) {
	mux := newMux(t, Dependencies{Pantry: &fakePantrySvc{}})

	rec := doPost(t, mux, "/pantry/lots/1/consume", `{"quantity":1.0}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
}

func TestConsume_BadLotID(t *testing.T) {
	mux := newMux(t, Dependencies{Pantry: &fakePantrySvc{}})

	rec := doPost(t, mux, "/pantry/lots/abc/consume", `{"quantity":1.0}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// ---- Ingredients fakes + tests ----

type fakeIngredientsSvc struct {
	foods     []dto.Ingredient
	nutrients []dto.IngredientNutrient
	products  []dto.IngredientProduct
	mapping   dto.IngredientMapping
	err       error
}

func (f *fakeIngredientsSvc) SearchFood(ctx context.Context, query string, limit int) ([]dto.Ingredient, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.foods, nil
}

func (f *fakeIngredientsSvc) LookupNutrition(ctx context.Context, nummer int) ([]dto.IngredientNutrient, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.nutrients, nil
}

func (f *fakeIngredientsSvc) SearchDabas(ctx context.Context, query string) ([]dto.IngredientProduct, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.products, nil
}

func (f *fakeIngredientsSvc) SearchMatpriskollen(ctx context.Context, query string) ([]dto.IngredientProduct, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.products, nil
}

func (f *fakeIngredientsSvc) ResolveMapping(ctx context.Context, mealieFoodID string, in dto.IngredientMappingResolve) (dto.IngredientMapping, error) {
	if f.err != nil {
		return dto.IngredientMapping{}, f.err
	}
	return dto.IngredientMapping{MealieFoodID: mealieFoodID, IngredientID: in.IngredientID, NeedsReview: false}, nil
}

func (f *fakeIngredientsSvc) GetMapping(ctx context.Context, mealieFoodID string) (dto.IngredientMapping, error) {
	if f.err != nil {
		return dto.IngredientMapping{}, f.err
	}
	return f.mapping, nil
}

func TestSearchFood_HappyPath(t *testing.T) {
	svc := &fakeIngredientsSvc{foods: []dto.Ingredient{
		{ID: "cauliflower", Display: "Cauliflower", Source: "slv"},
	}}
	mux := newMux(t, Dependencies{Ingredients: svc})

	rec := doGet(t, mux, "/ingredients/search?q=cauliflower")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got []dto.Ingredient
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].ID != "cauliflower" {
		t.Fatalf("unexpected food: %+v", got)
	}
}

func TestLookupNutrition_HappyPath(t *testing.T) {
	svc := &fakeIngredientsSvc{nutrients: []dto.IngredientNutrient{
		{Name: "Energy", Value: 0.9, Unit: "kWh"},
	}}
	mux := newMux(t, Dependencies{Ingredients: svc})

	rec := doGet(t, mux, "/ingredients/nutrition/12345")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got []dto.IngredientNutrient
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].Name != "Energy" {
		t.Fatalf("unexpected nutrient: %+v", got)
	}
}

func TestLookupNutrition_BadNummer(t *testing.T) {
	mux := newMux(t, Dependencies{Ingredients: &fakeIngredientsSvc{}})

	rec := doGet(t, mux, "/ingredients/nutrition/abc")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestSearchDabas_HappyPath(t *testing.T) {
	svc := &fakeIngredientsSvc{products: []dto.IngredientProduct{
		{Key: "dabas-123", Name: "Milk", GTIN: "7340012345678"},
	}}
	mux := newMux(t, Dependencies{Ingredients: svc})

	rec := doGet(t, mux, "/ingredients/dabas/search?q=milk")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got []dto.IngredientProduct
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].Name != "Milk" {
		t.Fatalf("unexpected product: %+v", got)
	}
}

func TestSearchMatpriskollen_HappyPath(t *testing.T) {
	svc := &fakeIngredientsSvc{products: []dto.IngredientProduct{
		{Key: "mpk-123", Name: "Bread", GTIN: "7340098765432"},
	}}
	mux := newMux(t, Dependencies{Ingredients: svc})

	rec := doGet(t, mux, "/ingredients/matpriskollen/search?q=bread")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got []dto.IngredientProduct
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].Name != "Bread" {
		t.Fatalf("unexpected product: %+v", got)
	}
}

func TestResolveMapping_HappyPath(t *testing.T) {
	mux := newMux(t, Dependencies{Ingredients: &fakeIngredientsSvc{}})

	body := `{"ingredient_id":"cauliflower","acceptable_forms":["head","florets"]}`
	rec := doPatch(t, mux, "/ingredient-mappings/mealie-food-1", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got dto.IngredientMapping
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.MealieFoodID != "mealie-food-1" || got.IngredientID != "cauliflower" {
		t.Fatalf("unexpected mapping: %+v", got)
	}
}

func TestResolveMapping_MissingIngredientID(t *testing.T) {
	mux := newMux(t, Dependencies{Ingredients: &fakeIngredientsSvc{}})

	rec := doPatch(t, mux, "/ingredient-mappings/mealie-food-1", `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// ---- Stores fakes + tests ----

type fakeStoresSvc struct {
	products []dto.IngredientProduct
	err      error
}

func (f *fakeStoresSvc) SearchProducts(ctx context.Context, query string) ([]dto.IngredientProduct, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.products, nil
}

func (f *fakeStoresSvc) SearchProductsByGTIN(ctx context.Context, gtin string) ([]dto.IngredientProduct, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.products, nil
}

func TestSearchProducts_HappyPath(t *testing.T) {
	svc := &fakeStoresSvc{products: []dto.IngredientProduct{
		{Key: "mpk-1", Name: "Tomatoes", Brand: "Brand X"},
	}}
	mux := newMux(t, Dependencies{Stores: svc})

	rec := doGet(t, mux, "/products/search?q=tomatoes")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got []dto.IngredientProduct
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].Name != "Tomatoes" {
		t.Fatalf("unexpected product: %+v", got)
	}
}

func TestSearchByGTIN_HappyPath(t *testing.T) {
	svc := &fakeStoresSvc{products: []dto.IngredientProduct{
		{Key: "mpk-1", GTIN: "7340011111111", Name: "Tomatoes"},
	}}
	mux := newMux(t, Dependencies{Stores: svc})

	rec := doGet(t, mux, "/products/by-gtin?gtin=7340011111111")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got []dto.IngredientProduct
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].GTIN != "7340011111111" {
		t.Fatalf("unexpected product: %+v", got)
	}
}

func TestSearchByGTIN_MissingGTIN(t *testing.T) {
	mux := newMux(t, Dependencies{Stores: &fakeStoresSvc{}})

	rec := doGet(t, mux, "/products/by-gtin")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// ---- Meals fakes + tests (extra) ----

type fakeMealsSvc struct {
	created dto.MealEventResponse
	meal    dto.MealEventResponse
	meals   []dto.MealEventResponse
	err     error
}

func (f *fakeMealsSvc) CreateMealEvent(ctx context.Context, in dto.MealEventNew) (dto.MealEventResponse, error) {
	if f.err != nil {
		return dto.MealEventResponse{}, f.err
	}
	return f.created, nil
}

func (f *fakeMealsSvc) GetMeal(ctx context.Context, id int64) (dto.MealEventResponse, error) {
	if f.err != nil {
		return dto.MealEventResponse{}, f.err
	}
	return f.meal, nil
}

func (f *fakeMealsSvc) ListMeals(ctx context.Context, mealieRecipeID, servedOn string) ([]dto.MealEventResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.meals, nil
}

func TestListMeals_HappyPath(t *testing.T) {
	svc := &fakeMealsSvc{meals: []dto.MealEventResponse{
		{ID: 1, MealieRecipeID: "r-1", ServedOn: "2026-08-19", Reactions: []dto.MealReactionResponse{{PersonID: "p1", Sentiment: 1}}},
	}}
	mux := newMux(t, Dependencies{Meals: svc})

	rec := doGet(t, mux, "/meals")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got []dto.MealEventResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].MealieRecipeID != "r-1" {
		t.Fatalf("unexpected meal: %+v", got)
	}
}

func TestGetMeal_HappyPath(t *testing.T) {
	svc := &fakeMealsSvc{meal: dto.MealEventResponse{ID: 42, MealieRecipeID: "r-1", ServedOn: "2026-08-19"}}
	mux := newMux(t, Dependencies{Meals: svc})

	rec := doGet(t, mux, "/meals/42")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got dto.MealEventResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.ID != 42 || got.MealieRecipeID != "r-1" {
		t.Fatalf("unexpected meal: %+v", got)
	}
}

func TestGetMeal_BadID(t *testing.T) {
	mux := newMux(t, Dependencies{Meals: &fakeMealsSvc{}})

	rec := doGet(t, mux, "/meals/abc")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// Ensure strconv is used (it is via the mux routing).
var _ = strconv.Itoa
