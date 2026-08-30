package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/androidand/spisordning/internal/dto"
	"github.com/androidand/spisordning/internal/service"
	"github.com/oapi-codegen/runtime/types"
)

// ---- Pantry fakes + tests ----

type fakePantrySvc struct {
	locations []dto.PantryLocation
	lots      []dto.PantryLot
	purchased dto.PantryLot
	updated   dto.PantryLot
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

func (f *fakePantrySvc) Consume(ctx context.Context, lotID string, in dto.PantryConsumeInput) error {
	return f.err
}

func (f *fakePantrySvc) ListExpiring(ctx context.Context, within time.Duration) ([]dto.PantryLot, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.lots, nil
}

func (f *fakePantrySvc) Discard(ctx context.Context, lotID string, in dto.PantryDiscardInput) (dto.PantryLot, error) {
	if f.err != nil {
		return dto.PantryLot{}, f.err
	}
	return f.updated, nil
}

func (f *fakePantrySvc) Adjust(ctx context.Context, lotID string, in dto.PantryAdjustInput) (dto.PantryLot, error) {
	if f.err != nil {
		return dto.PantryLot{}, f.err
	}
	return f.updated, nil
}

func (f *fakePantrySvc) MarkEmpty(ctx context.Context, lotID string) (dto.PantryLot, error) {
	if f.err != nil {
		return dto.PantryLot{}, f.err
	}
	return f.updated, nil
}

func (f *fakePantrySvc) Open(ctx context.Context, lotID string, in dto.PantryOpenInput) (dto.PantryLot, error) {
	if f.err != nil {
		return dto.PantryLot{}, f.err
	}
	return f.updated, nil
}

func (f *fakePantrySvc) Transfer(ctx context.Context, lotID string, in dto.PantryTransferInput) (dto.PantryLot, error) {
	if f.err != nil {
		return dto.PantryLot{}, f.err
	}
	return f.updated, nil
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
		{ID: "lot-1", IngredientID: "cauliflower", LocationID: "kitchen", Quantity: 2.0, Unit: "piece"},
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
	svc := &fakePantrySvc{purchased: dto.PantryLot{ID: "lot-1", IngredientID: "cauliflower", Quantity: 1.0}}
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
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestDiscard_HappyPath(t *testing.T) {
	svc := &fakePantrySvc{updated: dto.PantryLot{ID: "lot-1", Quantity: 1.0}}
	mux := newMux(t, Dependencies{Pantry: svc})

	rec := doPost(t, mux, "/pantry/lots/lot-1/discard", `{"quantity":1.0,"source":"manual"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got dto.PantryLot
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.ID != "lot-1" || got.Quantity != 1.0 {
		t.Fatalf("unexpected lot: %+v", got)
	}
}

func TestDiscard_ZeroQuantity(t *testing.T) {
	mux := newMux(t, Dependencies{Pantry: &fakePantrySvc{}})

	rec := doPost(t, mux, "/pantry/lots/lot-1/discard", `{"quantity":0,"source":"manual"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", rec.Code, rec.Body)
	}
}

func TestDiscard_NotFound(t *testing.T) {
	svc := &fakePantrySvc{err: dto.ErrNotFound}
	mux := newMux(t, Dependencies{Pantry: svc})

	rec := doPost(t, mux, "/pantry/lots/missing/discard", `{"quantity":1.0,"source":"manual"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body: %s", rec.Code, rec.Body)
	}
}

func TestAdjust_HappyPath(t *testing.T) {
	svc := &fakePantrySvc{updated: dto.PantryLot{ID: "lot-1", Quantity: 0.5}}
	mux := newMux(t, Dependencies{Pantry: svc})

	rec := doPost(t, mux, "/pantry/lots/lot-1/adjust", `{"quantity":0.5,"source":"manual"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got dto.PantryLot
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.Quantity != 0.5 {
		t.Fatalf("unexpected lot: %+v", got)
	}
}

func TestAdjust_NegativeQuantity(t *testing.T) {
	mux := newMux(t, Dependencies{Pantry: &fakePantrySvc{}})

	rec := doPost(t, mux, "/pantry/lots/lot-1/adjust", `{"quantity":-1,"source":"manual"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", rec.Code, rec.Body)
	}
}

func TestMarkEmpty_HappyPath(t *testing.T) {
	svc := &fakePantrySvc{updated: dto.PantryLot{ID: "lot-1", Quantity: 0}}
	mux := newMux(t, Dependencies{Pantry: svc})

	rec := doPost(t, mux, "/pantry/lots/lot-1/mark-empty", ``)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got dto.PantryLot
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.Quantity != 0 {
		t.Fatalf("unexpected lot: %+v", got)
	}
}

func TestOpen_InvalidSource(t *testing.T) {
	svc := &fakePantrySvc{err: dto.ErrInvalid}
	mux := newMux(t, Dependencies{Pantry: svc})

	rec := doPost(t, mux, "/pantry/lots/lot-1/open", `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", rec.Code, rec.Body)
	}
}

func TestTransfer_HappyPath(t *testing.T) {
	svc := &fakePantrySvc{updated: dto.PantryLot{ID: "lot-2", LocationID: "fridge", Quantity: 1.0}}
	mux := newMux(t, Dependencies{Pantry: svc})

	rec := doPost(t, mux, "/pantry/lots/lot-1/transfer", `{"location_id":"fridge","quantity":1.0,"source":"manual"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got dto.PantryLot
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.ID != "lot-2" || got.LocationID != "fridge" {
		t.Fatalf("unexpected lot: %+v", got)
	}
}

func TestTransfer_MissingLocation(t *testing.T) {
	mux := newMux(t, Dependencies{Pantry: &fakePantrySvc{}})

	rec := doPost(t, mux, "/pantry/lots/lot-1/transfer", `{"quantity":1.0,"source":"manual"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", rec.Code, rec.Body)
	}
}

func TestTransfer_ZeroQuantity(t *testing.T) {
	mux := newMux(t, Dependencies{Pantry: &fakePantrySvc{}})

	rec := doPost(t, mux, "/pantry/lots/lot-1/transfer", `{"location_id":"fridge","quantity":0,"source":"manual"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", rec.Code, rec.Body)
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

func (f *fakeIngredientsSvc) NutritionByID(ctx context.Context, id string) ([]dto.IngredientNutrient, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.nutrients, nil
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
	stores   []dto.Store
	offers   []dto.StoreOffer
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

func (f *fakeStoresSvc) ListStores(ctx context.Context) ([]dto.Store, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.stores, nil
}

func (f *fakeStoresSvc) LocateStores(ctx context.Context, input dto.LocateStoresInput) ([]dto.Store, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.stores, nil
}

func (f *fakeStoresSvc) ListStoreOffers(ctx context.Context, storeID string) ([]dto.StoreOffer, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.offers, nil
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

func TestListStores_HappyPath(t *testing.T) {
	svc := &fakeStoresSvc{stores: []dto.Store{{ID: "s-1", RetailerID: "ica", Name: "ICA"}}}
	mux := newMux(t, Dependencies{Stores: svc})

	rec := doGet(t, mux, "/stores")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got []dto.Store
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].ID != "s-1" {
		t.Fatalf("unexpected stores: %+v", got)
	}
}

func TestListStores_WithOrigin(t *testing.T) {
	lat := 59.3293
	lon := 18.0686
	svc := &fakeStoresSvc{stores: []dto.Store{
		{ID: "s-1", RetailerID: "ica", Name: "ICA", Latitude: &lat, Longitude: &lon, DistanceKm: &[]float64{1.2}[0]},
	}}
	mux := newMux(t, Dependencies{Stores: svc})

	rec := doGet(t, mux, "/stores?latitude=59.3293&longitude=18.0686")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got []dto.Store
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].DistanceKm == nil || *got[0].DistanceKm != 1.2 {
		t.Fatalf("unexpected stores: %+v", got)
	}
}

func TestListStores_BadLatitude(t *testing.T) {
	mux := newMux(t, Dependencies{Stores: &fakeStoresSvc{}})

	rec := doGet(t, mux, "/stores?latitude=notanumber")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", rec.Code, rec.Body)
	}
}

func TestListStoreOffers_HappyPath(t *testing.T) {
	svc := &fakeStoresSvc{offers: []dto.StoreOffer{{ID: "offer-7", StoreID: "s-1", RetailerProductID: "rp-1", CurrentlyCarried: true}}}
	mux := newMux(t, Dependencies{Stores: svc})

	rec := doGet(t, mux, "/stores/s-1/offers")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got []dto.StoreOffer
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].ID != "offer-7" {
		t.Fatalf("unexpected offers: %+v", got)
	}
}

func TestListStores_Error(t *testing.T) {
	svc := &fakeStoresSvc{err: errSentinel("boom")}
	mux := newMux(t, Dependencies{Stores: svc})

	rec := doGet(t, mux, "/stores")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
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

func (f *fakeMealsSvc) GetMeal(ctx context.Context, id string) (dto.MealEventResponse, error) {
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
		{ID: "meal-1", MealieRecipeID: "r-1", ServedOn: "2026-08-19", Reactions: []dto.MealReactionResponse{{PersonID: "p1", Sentiment: 1}}},
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
	svc := &fakeMealsSvc{meal: dto.MealEventResponse{ID: "meal-42", MealieRecipeID: "r-1", ServedOn: "2026-08-19"}}
	mux := newMux(t, Dependencies{Meals: svc})

	rec := doGet(t, mux, "/meals/meal-42")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got dto.MealEventResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.ID != "meal-42" || got.MealieRecipeID != "r-1" {
		t.Fatalf("unexpected meal: %+v", got)
	}
}

func TestGetMeal_BadID(t *testing.T) {
	mux := newMux(t, Dependencies{Meals: &fakeMealsSvc{}})

	rec := doGet(t, mux, "/meals/abc")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
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
	view dto.TonightView
	err  error
}

func (f *fakeTonightSvc) GetTonight(ctx context.Context) (dto.TonightView, error) {
	if f.err != nil {
		return dto.TonightView{}, f.err
	}
	return f.view, nil
}

func TestGetTonight_HappyPath(t *testing.T) {
	svc := &fakeTonightSvc{view: dto.TonightView{
		ServedOn: "2026-08-21",
		Recipe:   dto.RecipeRefResponse{MealieRecipeID: "r-1", Title: "Pasta Bolognese", Tags: []string{"pasta"}, Effort: 2},
		Reactions: []dto.MealReactionResponse{{PersonID: "p1", Sentiment: 2}},
	}}
	mux := newMux(t, Dependencies{Tonight: svc})

	rec := doGet(t, mux, "/tonight")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got dto.TonightView
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.ServedOn != "2026-08-21" || got.Recipe.Title != "Pasta Bolognese" {
		t.Fatalf("unexpected tonight view: %+v", got)
	}
	if len(got.Reactions) != 1 || got.Reactions[0].Sentiment != 2 {
		t.Fatalf("unexpected reactions: %+v", got.Reactions)
	}
}

func TestGetTonight_NotFound(t *testing.T) {
	svc := &fakeTonightSvc{err: dto.ErrNoMealTonight}
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

// ---- Dashboard fakes + tests ----

type fakeDashboardSvc struct {
	out dto.Dashboard
	err error
}

func (f *fakeDashboardSvc) Get(ctx context.Context, householdID string) (dto.Dashboard, error) {
	if f.err != nil {
		return dto.Dashboard{}, f.err
	}
	return f.out, nil
}

func TestGetDashboard_HappyPath(t *testing.T) {
	svc := &fakeDashboardSvc{out: dto.Dashboard{
		Pantry:   dto.DashboardPantry{Locations: 2, Lots: 5, Expiring: 1},
		Expiring: []dto.DashboardExpiringLot{{IngredientID: "mjolk", Quantity: 1, Unit: "L"}},
	}}
	mux := newMux(t, Dependencies{Dashboard: svc})

	rec := doGet(t, mux, "/widgets/dashboard?householdId=h-1")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got dto.Dashboard
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.Pantry.Locations != 2 || got.Pantry.Lots != 5 || got.Pantry.Expiring != 1 {
		t.Fatalf("unexpected pantry: %+v", got.Pantry)
	}
	if len(got.Expiring) != 1 || got.Expiring[0].IngredientID != "mjolk" {
		t.Fatalf("unexpected expiring: %+v", got.Expiring)
	}
}

func TestGetDashboard_Error(t *testing.T) {
	svc := &fakeDashboardSvc{err: errSentinel("boom")}
	mux := newMux(t, Dependencies{Dashboard: svc})

	rec := doGet(t, mux, "/widgets/dashboard")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

// ---- Ingredient alias fakes + tests ----

type fakeAliasSvc struct {
	aliases []dto.IngredientAlias
	err     error
}

func (f *fakeAliasSvc) List(ctx context.Context, householdID string) ([]dto.IngredientAlias, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.aliases, nil
}

func (f *fakeAliasSvc) Create(ctx context.Context, in dto.IngredientAliasNew) (dto.IngredientAlias, error) {
	if f.err != nil {
		return dto.IngredientAlias{}, f.err
	}
	if in.Alias == "" || in.IngredientID == "" {
		return dto.IngredientAlias{}, fmt.Errorf("%w: alias is required", dto.ErrInvalidAlias)
	}
	return dto.IngredientAlias{Alias: in.Alias, IngredientID: in.IngredientID}, nil
}

func (f *fakeAliasSvc) Delete(ctx context.Context, householdID, alias string) error {
	return f.err
}

func (f *fakeAliasSvc) Resolve(ctx context.Context, householdID, alias string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return "potato", nil
}

func TestListAliases_HappyPath(t *testing.T) {
	svc := &fakeAliasSvc{aliases: []dto.IngredientAlias{{Alias: "potatis", IngredientID: "potato"}}}
	mux := newMux(t, Dependencies{IngredientAlias: svc})

	rec := doGet(t, mux, "/ingredient-aliases?householdId=h-1")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got []dto.IngredientAlias
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].Alias != "potatis" {
		t.Fatalf("unexpected aliases: %+v", got)
	}
}

func TestCreateAlias_Invalid(t *testing.T) {
	svc := &fakeAliasSvc{}
	mux := newMux(t, Dependencies{IngredientAlias: svc})

	rec := doPost(t, mux, "/ingredient-aliases", `{"alias": "", "ingredient_id": "potato"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", rec.Code, rec.Body)
	}
}

func TestResolveAlias_HappyPath(t *testing.T) {
	svc := &fakeAliasSvc{}
	mux := newMux(t, Dependencies{IngredientAlias: svc})

	rec := doGet(t, mux, "/ingredient-aliases/resolve/potatis")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got map[string]string
	mustJSON(t, rec.Body.Bytes(), &got)
	if got["ingredient_id"] != "potato" {
		t.Fatalf("unexpected resolve: %+v", got)
	}
}

// ---- Preferences fakes + tests ----

type fakePrefsSvc struct {
	prefs []dto.PersonPreferenceResponse
	err   error
}

func (f *fakePrefsSvc) ListPreferences(ctx context.Context, personID string) ([]dto.PersonPreferenceResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.prefs, nil
}

func (f *fakePrefsSvc) SetPreference(ctx context.Context, in dto.SetPreferenceInput) (dto.PersonPreferenceResponse, error) {
	if f.err != nil {
		return dto.PersonPreferenceResponse{}, f.err
	}
	if in.PersonID == "" || in.Tag == "" {
		return dto.PersonPreferenceResponse{}, fmt.Errorf("%w: person_id and tag are required", dto.ErrInvalidPreference)
	}
	return dto.PersonPreferenceResponse{PersonID: in.PersonID, Tag: in.Tag, Sentiment: in.Sentiment, Confidence: in.Confidence}, nil
}

func TestSetPreference_HappyPath(t *testing.T) {
	svc := &fakePrefsSvc{}
	mux := newMux(t, Dependencies{Preferences: svc})

	rec := doPost(t, mux, "/preferences", `{"person_id": "p1", "tag": "spicy", "sentiment": 2, "confidence": 0.9}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body)
	}
	var got dto.PersonPreferenceResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.Tag != "spicy" || got.Sentiment != 2 {
		t.Fatalf("unexpected pref: %+v", got)
	}
}

func TestSetPreference_Invalid(t *testing.T) {
	svc := &fakePrefsSvc{}
	mux := newMux(t, Dependencies{Preferences: svc})

	rec := doPost(t, mux, "/preferences", `{"tag": "spicy", "sentiment": 2, "confidence": 0.9}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", rec.Code, rec.Body)
	}
}

// ---- People update fakes + tests ----

type fakePeopleSvc struct {
	people map[string]dto.PersonResponse
}

func (f *fakePeopleSvc) ListPeople(ctx context.Context) ([]dto.PersonResponse, error) {
	out := make([]dto.PersonResponse, 0, len(f.people))
	for _, p := range f.people {
		out = append(out, p)
	}
	return out, nil
}

func (f *fakePeopleSvc) GetPerson(ctx context.Context, id string) (dto.PersonResponse, error) {
	p, ok := f.people[id]
	if !ok {
		return dto.PersonResponse{}, fmt.Errorf("%w: person %s not found", ErrNotFound, id)
	}
	return p, nil
}

func (f *fakePeopleSvc) CreatePerson(ctx context.Context, in dto.PersonInput) (dto.PersonResponse, error) {
	id := "new-" + in.Name
	p := dto.PersonResponse{ID: id, Name: in.Name, Weight: in.Weight}
	f.people[id] = p
	return p, nil
}

func (f *fakePeopleSvc) UpdatePerson(ctx context.Context, id string, in dto.PersonUpdate) (dto.PersonResponse, error) {
	p, ok := f.people[id]
	if !ok {
		return dto.PersonResponse{}, fmt.Errorf("%w: person %s not found", ErrNotFound, id)
	}
	p.Name = in.Name
	if in.Weight > 0 {
		p.Weight = in.Weight
	}
	f.people[id] = p
	return p, nil
}

func TestUpdatePerson_HappyPath(t *testing.T) {
	svc := &fakePeopleSvc{people: map[string]dto.PersonResponse{
		"p1": {ID: "p1", Name: "Old", Weight: 1.0},
	}}
	mux := newMux(t, Dependencies{People: svc})

	rec := doPatch(t, mux, "/people/p1", `{"name": "New", "weight": 2.0}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got dto.PersonResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.Name != "New" || got.Weight != 2.0 {
		t.Fatalf("unexpected person: %+v", got)
	}
}

func TestUpdatePerson_NotFound(t *testing.T) {
	svc := &fakePeopleSvc{people: make(map[string]dto.PersonResponse)}
	mux := newMux(t, Dependencies{People: svc})

	rec := doPatch(t, mux, "/people/missing", `{"name": "New"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body: %s", rec.Code, rec.Body)
	}
}

// ---- Inspiration fakes + tests ----

type fakeInspirationSvc struct {
	suggestions []dto.InspirationSuggestion
	err         error
}

func (f *fakeInspirationSvc) Suggest(ctx context.Context) ([]dto.InspirationSuggestion, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.suggestions, nil
}

func TestSuggest_HappyPath(t *testing.T) {
	svc := &fakeInspirationSvc{suggestions: []dto.InspirationSuggestion{
		{MealieRecipeID: "r1", Title: "Pasta", MatchRatio: 1.0},
	}}
	mux := newMux(t, Dependencies{Inspiration: svc})

	rec := doGet(t, mux, "/inspiration")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got []dto.InspirationSuggestion
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].Title != "Pasta" {
		t.Fatalf("unexpected suggestions: %+v", got)
	}
}

func TestSuggest_Error(t *testing.T) {
	svc := &fakeInspirationSvc{err: fmt.Errorf("boom")}
	mux := newMux(t, Dependencies{Inspiration: svc})

	rec := doGet(t, mux, "/inspiration")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body: %s", rec.Code, rec.Body)
	}
}

// ---- Grocy fakes + tests ----

type fakeGrocySvc struct {
	status    dto.GrocyStatus
	products  []dto.GrocyProduct
	stock     []dto.GrocyStockEntry
	items     []dto.GrocyShoppingItem
	err       error
	added     bool
	consumed  bool
	itemAdded bool
}

func (f *fakeGrocySvc) Status(ctx context.Context) (dto.GrocyStatus, error) {
	if f.err != nil {
		return dto.GrocyStatus{}, f.err
	}
	return f.status, nil
}

func (f *fakeGrocySvc) ListProducts(ctx context.Context) ([]dto.GrocyProduct, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.products, nil
}

func (f *fakeGrocySvc) ListStock(ctx context.Context) ([]dto.GrocyStockEntry, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.stock, nil
}

func (f *fakeGrocySvc) ListShoppingList(ctx context.Context) ([]dto.GrocyShoppingItem, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.items, nil
}

func (f *fakeGrocySvc) AddStock(ctx context.Context, productID int, amount float64, bestBefore string) error {
	if f.err != nil {
		return f.err
	}
	f.added = true
	return nil
}

func (f *fakeGrocySvc) ConsumeStock(ctx context.Context, productID int, amount float64) error {
	if f.err != nil {
		return f.err
	}
	f.consumed = true
	return nil
}

func (f *fakeGrocySvc) AddShoppingItem(ctx context.Context, productID int, note string, amount float64) error {
	if f.err != nil {
		return f.err
	}
	f.itemAdded = true
	return nil
}

func TestGrocyStatus_NotConfigured(t *testing.T) {
	svc := &fakeGrocySvc{status: dto.GrocyStatus{Configured: false}}
	mux := newMux(t, Dependencies{Grocy: svc})

	rec := doGet(t, mux, "/grocy/status")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got dto.GrocyStatus
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.Configured {
		t.Fatalf("expected not configured, got %+v", got)
	}
}

func TestGrocyListProducts_HappyPath(t *testing.T) {
	svc := &fakeGrocySvc{products: []dto.GrocyProduct{{ID: 1, Name: "Milk"}}}
	mux := newMux(t, Dependencies{Grocy: svc})

	rec := doGet(t, mux, "/grocy/products")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got []dto.GrocyProduct
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].Name != "Milk" {
		t.Fatalf("unexpected products: %+v", got)
	}
}

func TestGrocyAddStock_HappyPath(t *testing.T) {
	svc := &fakeGrocySvc{}
	mux := newMux(t, Dependencies{Grocy: svc})

	rec := doPost(t, mux, "/grocy/stock/add", `{"product_id": 1, "amount": 2}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	if !svc.added {
		t.Fatal("expected AddStock to be called")
	}
}

func TestGrocyAddStock_NotConfigured(t *testing.T) {
	svc := &fakeGrocySvc{err: service.ErrGrocyNotConfigured}
	mux := newMux(t, Dependencies{Grocy: svc})

	rec := doPost(t, mux, "/grocy/stock/add", `{"product_id": 1, "amount": 2}`)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body: %s", rec.Code, rec.Body)
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
	// progressDelay simulates a slow adapter: each progress event is delayed
	// by this amount, so the SSE stream emits events incrementally over time.
	progressDelay time.Duration
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

func (f *fakePlansSvc) RunPlanWithProgress(ctx context.Context, in PlanRunInput, progress func(PlanProgress)) (PlanRunResult, error) {
	if f.err != nil {
		return PlanRunResult{}, f.err
	}
	for _, phase := range []string{"planning", "resolving", "wishlist"} {
		if f.progressDelay > 0 {
			select {
			case <-time.After(f.progressDelay):
			case <-ctx.Done():
				return PlanRunResult{}, ctx.Err()
			}
		}
		progress(PlanProgress{Phase: phase, Message: phase, At: time.Now()})
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
	return PlanResponse{ID: "plan-1", WeekStart: types.Date{Time: weekStart}, Status: "draft"}, nil
}

func (f *fakePlansSvc) GetPlan(ctx context.Context, planID string) (PlanView, error) {
	if f.planErr != nil {
		return PlanView{}, f.planErr
	}
	return f.planView, nil
}

func (f *fakePlansSvc) UpdatePlan(ctx context.Context, planID string, status string) (PlanResponse, error) {
	if f.planErr != nil {
		return PlanResponse{}, f.planErr
	}
	return PlanResponse{ID: planID, Status: status}, nil
}

func (f *fakePlansSvc) SetDecisions(ctx context.Context, planID string, decisions []PlanDecisionInput) error {
	return f.decisionsErr
}

func (f *fakePlansSvc) ListCandidates(ctx context.Context, planID string) ([]PlanCandidateResponse, error) {
	if f.candidatesErr != nil {
		return nil, f.candidatesErr
	}
	return f.candidates, nil
}

func (f *fakePlansSvc) InsertCandidates(ctx context.Context, candidates []PlanCandidateInput) error {
	return nil
}

func (f *fakePlansSvc) ListShoppingRequirements(ctx context.Context, planID string) ([]ShoppingRequirementResponse, error) {
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
