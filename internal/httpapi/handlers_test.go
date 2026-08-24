package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/androidand/spisordning/internal/dto"
	"github.com/oapi-codegen/runtime/types"
)

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

func TestCreateMealEvent_ServiceError(t *testing.T) {
	svc := &fakeMealsSvc{err: errSentinel("dupe")}
	mux := newMux(t, Dependencies{Meals: svc})
	rec := doPost(t, mux, "/meals", `{"mealie_recipe_id":"r-1","served_on":"2026-08-19"}`)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

// ---- Tonight fakes + tests ----

type fakeTonightSvc struct {
	view TonightView
	err  error
}

func (f *fakeTonightSvc) GetTonight(ctx context.Context) (TonightView, error) {
	if f.err != nil {
		return TonightView{}, f.err
	}
	return f.view, nil
}

func TestGetTonight_HappyPath(t *testing.T) {
	svc := &fakeTonightSvc{view: TonightView{
		ServedOn: "2026-08-21",
		Recipe:   dto.RecipeRefResponse{MealieRecipeID: "r-1", Title: "Pasta Bolognese", Tags: []string{"pasta"}, Effort: 2},
		Reactions: []dto.MealReactionResponse{{PersonID: "p1", Sentiment: 2}},
	}}
	mux := newMux(t, Dependencies{Tonight: svc})

	rec := doGet(t, mux, "/tonight")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got TonightView
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.ServedOn != "2026-08-21" || got.Recipe.Title != "Pasta Bolognese" {
		t.Fatalf("unexpected tonight view: %+v", got)
	}
	if len(got.Reactions) != 1 || got.Reactions[0].Sentiment != 2 {
		t.Fatalf("unexpected reactions: %+v", got.Reactions)
	}
}

func TestGetTonight_NotFound(t *testing.T) {
	svc := &fakeTonightSvc{err: ErrNoMealTonight}
	mux := newMux(t, Dependencies{Tonight: svc})

	rec := doGet(t, mux, "/tonight")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	var body errorBody
	mustJSON(t, rec.Body.Bytes(), &body)
	if body.Message == "" {
		t.Fatal("expected error message")
	}
}

func TestGetTonight_ServiceError(t *testing.T) {
	svc := &fakeTonightSvc{err: errSentinel("db error")}
	mux := newMux(t, Dependencies{Tonight: svc})

	rec := doGet(t, mux, "/tonight")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

// ---- Reactions fakes + tests ----

type fakeReactionsSvc struct {
	created dto.MealReactionResponse
	err     error
}

func (f *fakeReactionsSvc) CreateReaction(ctx context.Context, in ReactionNew) (dto.MealReactionResponse, error) {
	if f.err != nil {
		return dto.MealReactionResponse{}, f.err
	}
	return f.created, nil
}

func TestCreateReaction_HappyPath(t *testing.T) {
	svc := &fakeReactionsSvc{created: dto.MealReactionResponse{PersonID: "p1", Sentiment: 2}}
	mux := newMux(t, Dependencies{Reactions: svc})

	rec := doPost(t, mux, "/reactions", `{"person_id":"p1","sentiment":2}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body)
	}
	var got dto.MealReactionResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.PersonID != "p1" || got.Sentiment != 2 {
		t.Fatalf("unexpected reaction: %+v", got)
	}
}

func TestCreateReaction_Validation(t *testing.T) {
	mux := newMux(t, Dependencies{Reactions: &fakeReactionsSvc{}})

	cases := []struct {
		name, body string
		wantCode   int
	}{
		{"missing_person", `{"sentiment":1}`, http.StatusBadRequest},
		{"bad_sentiment_high", `{"person_id":"p1","sentiment":3}`, http.StatusBadRequest},
		{"bad_sentiment_low", `{"person_id":"p1","sentiment":-3}`, http.StatusBadRequest},
		{"bad_json", `not-json`, http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := doPost(t, mux, "/reactions", c.body)
			if rec.Code != c.wantCode {
				t.Errorf("%s: status = %d, want %d", c.name, rec.Code, c.wantCode)
			}
		})
	}
}

func TestCreateReaction_ServiceError(t *testing.T) {
	svc := &fakeReactionsSvc{err: errSentinel("db error")}
	mux := newMux(t, Dependencies{Reactions: svc})

	rec := doPost(t, mux, "/reactions", `{"person_id":"p1","sentiment":1}`)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

// ---- Plans fakes + tests ----

type fakePlansSvc struct {
	result PlanRunResult
	err    error
	// Plan stubs
	plans           []PlanResponse
	planView        PlanView
	planErr         error
	candidates      []PlanCandidateResponse
	candidatesErr   error
	shoppingReqs    []ShoppingRequirementResponse
	shoppingErr     error
	decisions       []PlanDecisionInput
	decisionsErr    error
}

func (f *fakePlansSvc) RunPlan(ctx context.Context, in PlanRunInput) (PlanRunResult, error) {
	if f.err != nil {
		return PlanRunResult{}, f.err
	}
	return f.result, nil
}

func (f *fakePlansSvc) ListPlans(ctx context.Context) ([]PlanResponse, error) {
	if f.planErr != nil {
		return nil, f.planErr
	}
	return f.plans, nil
}

func (f *fakePlansSvc) CreatePlan(ctx context.Context, weekStart time.Time) (PlanResponse, error) {
	if f.planErr != nil {
		return PlanResponse{}, f.planErr
	}
	return PlanResponse{ID: 1, WeekStart: types.Date{Time: weekStart}, Status: "draft"}, nil
}

func (f *fakePlansSvc) GetPlan(ctx context.Context, planID int64) (PlanView, error) {
	if f.planErr != nil {
		return PlanView{}, f.planErr
	}
	return f.planView, nil
}

func (f *fakePlansSvc) UpdatePlan(ctx context.Context, planID int64, status string) (PlanResponse, error) {
	if f.planErr != nil {
		return PlanResponse{}, f.planErr
	}
	return PlanResponse{ID: int(planID), Status: status}, nil
}

func (f *fakePlansSvc) SetDecisions(ctx context.Context, planID int64, decisions []PlanDecisionInput) error {
	return f.decisionsErr
}

func (f *fakePlansSvc) ListCandidates(ctx context.Context, planID int64) ([]PlanCandidateResponse, error) {
	if f.candidatesErr != nil {
		return nil, f.candidatesErr
	}
	return f.candidates, nil
}

func (f *fakePlansSvc) InsertCandidates(ctx context.Context, candidates []PlanCandidateInput) error {
	return nil
}

func (f *fakePlansSvc) ListShoppingRequirements(ctx context.Context, planID int64) ([]ShoppingRequirementResponse, error) {
	if f.shoppingErr != nil {
		return nil, f.shoppingErr
	}
	return f.shoppingReqs, nil
}

func TestRunPlan_HappyPath(t *testing.T) {
	svc := &fakePlansSvc{result: PlanRunResult{
		Status:  "accepted",
		Message: "planned 7 dinners",
	}}
	mux := newMux(t, Dependencies{Plans: svc})

	rec := doPost(t, mux, "/plans/run", `{}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body: %s", rec.Code, rec.Body)
	}
	var got PlanRunResult
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.Status != "accepted" || got.Message != "planned 7 dinners" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestRunPlan_EmptyBody(t *testing.T) {
	svc := &fakePlansSvc{result: PlanRunResult{Status: "accepted", Message: "ok"}}
	mux := newMux(t, Dependencies{Plans: svc})

	rec := doPost(t, mux, "/plans/run", "")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rec.Code)
	}
}

func TestRunPlan_ServiceError(t *testing.T) {
	svc := &fakePlansSvc{err: errSentinel("mealie unavailable")}
	mux := newMux(t, Dependencies{Plans: svc})

	rec := doPost(t, mux, "/plans/run", `{}`)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	var got PlanRunResult
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.Status != "failed" {
		t.Fatalf("status = %q, want failed", got.Status)
	}
}
