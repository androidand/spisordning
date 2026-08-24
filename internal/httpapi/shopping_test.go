package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ---- Shopping list fakes + tests ----

type fakeShoppingListSvc struct {
	lists []ShoppingListResponse
	err   error
}

func (f *fakeShoppingListSvc) ListShoppingLists(_ context.Context) ([]ShoppingListResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.lists, nil
}

func (f *fakeShoppingListSvc) CreateShoppingList(_ context.Context, in ShoppingListInput) (ShoppingListResponse, error) {
	if f.err != nil {
		return ShoppingListResponse{}, f.err
	}
	return ShoppingListResponse{ID: 1, Name: in.Name, Status: "active", CreatedAt: time.Now()}, nil
}

func (f *fakeShoppingListSvc) GetShoppingList(_ context.Context, listID int64) (ShoppingListResponse, error) {
	if f.err != nil {
		return ShoppingListResponse{}, f.err
	}
	return ShoppingListResponse{ID: int(listID), Name: "Test", Status: "active", CreatedAt: time.Now()}, nil
}

func (f *fakeShoppingListSvc) ArchiveShoppingList(_ context.Context, listID int64) error {
	if f.err != nil {
		return f.err
	}
	return nil
}

func TestListShoppingLists_HappyPath(t *testing.T) {
	svc := &fakeShoppingListSvc{
		lists: []ShoppingListResponse{{ID: 1, Name: "Vecka 30", Status: "active"}},
	}
	mux := newMux(t, Dependencies{ShoppingLists: svc})
	rec := doGet(t, mux, "/shopping-lists")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got []ShoppingListResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].Name != "Vecka 30" {
		t.Fatalf("unexpected lists: %+v", got)
	}
}

func TestCreateShoppingList_HappyPath(t *testing.T) {
	svc := &fakeShoppingListSvc{}
	mux := newMux(t, Dependencies{ShoppingLists: svc})
	body, _ := json.Marshal(map[string]string{"name": "Vecka 31"})
	rec := doPost(t, mux, "/shopping-lists", string(body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	var got ShoppingListResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.Name != "Vecka 31" || got.Status != "active" {
		t.Fatalf("unexpected list: %+v", got)
	}
}

func TestCreateShoppingList_MissingName(t *testing.T) {
	svc := &fakeShoppingListSvc{}
	mux := newMux(t, Dependencies{ShoppingLists: svc})
	rec := doPost(t, mux, "/shopping-lists", `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestGetShoppingList_HappyPath(t *testing.T) {
	svc := &fakeShoppingListSvc{}
	mux := newMux(t, Dependencies{ShoppingLists: svc})
	rec := doGet(t, mux, "/shopping-lists/42")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got ShoppingListResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.ID != 42 {
		t.Fatalf("unexpected id: %d", got.ID)
	}
}

func TestGetShoppingList_NotFound(t *testing.T) {
	svc := &fakeShoppingListSvc{err: ErrNotFound}
	mux := newMux(t, Dependencies{ShoppingLists: svc})
	rec := doGet(t, mux, "/shopping-lists/999")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestArchiveShoppingList_HappyPath(t *testing.T) {
	svc := &fakeShoppingListSvc{}
	mux := newMux(t, Dependencies{ShoppingLists: svc})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/shopping-lists/42", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

// ---- Shopping list item fakes + tests ----

type fakeShoppingListItemSvc struct {
	items []ShoppingListItemResponse
	err   error
}

func (f *fakeShoppingListItemSvc) ListShoppingListItems(_ context.Context, listID int64) ([]ShoppingListItemResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.items, nil
}

func (f *fakeShoppingListItemSvc) AddShoppingListItem(_ context.Context, listID int64, in ShoppingListItemInput) (ShoppingListItemResponse, error) {
	if f.err != nil {
		return ShoppingListItemResponse{}, f.err
	}
	return ShoppingListItemResponse{ID: 1, ShoppingListID: int(listID), Quantity: in.Quantity, Unit: in.Unit, Checked: false}, nil
}

func (f *fakeShoppingListItemSvc) ToggleShoppingListItem(_ context.Context, listID, itemID int64, checked bool) (ShoppingListItemResponse, error) {
	if f.err != nil {
		return ShoppingListItemResponse{}, f.err
	}
	return ShoppingListItemResponse{ID: int(itemID), ShoppingListID: int(listID), Checked: checked}, nil
}

func (f *fakeShoppingListItemSvc) DeleteShoppingListItem(_ context.Context, listID, itemID int64) error {
	if f.err != nil {
		return f.err
	}
	return nil
}

func TestListShoppingListItems_HappyPath(t *testing.T) {
	svc := &fakeShoppingListItemSvc{
		items: []ShoppingListItemResponse{{ID: 1, ShoppingListID: 1, Quantity: 500, Unit: "g", Checked: false}},
	}
	mux := newMux(t, Dependencies{ShoppingListItems: svc})
	rec := doGet(t, mux, "/shopping-lists/1/items")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got []ShoppingListItemResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].Quantity != 500 {
		t.Fatalf("unexpected items: %+v", got)
	}
}

func TestAddShoppingListItem_HappyPath(t *testing.T) {
	svc := &fakeShoppingListItemSvc{}
	mux := newMux(t, Dependencies{ShoppingListItems: svc})
	body, _ := json.Marshal(map[string]interface{}{"quantity": 500, "unit": "g", "label": "chicken"})
	rec := doPost(t, mux, "/shopping-lists/1/items", string(body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	var got ShoppingListItemResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.Quantity != 500 || got.Unit != "g" {
		t.Fatalf("unexpected item: %+v", got)
	}
}

func TestAddShoppingListItem_InvalidQuantity(t *testing.T) {
	svc := &fakeShoppingListItemSvc{}
	mux := newMux(t, Dependencies{ShoppingListItems: svc})
	body, _ := json.Marshal(map[string]interface{}{"quantity": 0, "unit": "g", "label": "x"})
	rec := doPost(t, mux, "/shopping-lists/1/items", string(body))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestAddShoppingListItem_MissingIdentifiers(t *testing.T) {
	svc := &fakeShoppingListItemSvc{}
	mux := newMux(t, Dependencies{ShoppingListItems: svc})
	body, _ := json.Marshal(map[string]interface{}{"quantity": 1, "unit": "g"})
	rec := doPost(t, mux, "/shopping-lists/1/items", string(body))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestToggleShoppingListItem_HappyPath(t *testing.T) {
	svc := &fakeShoppingListItemSvc{}
	mux := newMux(t, Dependencies{ShoppingListItems: svc})
	body, _ := json.Marshal(map[string]bool{"checked": true})
	rec := doPost(t, mux, "/shopping-lists/1/items/5", string(body))
	// Use PATCH via doPost replacement
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/shopping-lists/1/items/5", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got ShoppingListItemResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if !got.Checked {
		t.Fatalf("expected checked=true")
	}
}

func TestDeleteShoppingListItem_HappyPath(t *testing.T) {
	svc := &fakeShoppingListItemSvc{}
	mux := newMux(t, Dependencies{ShoppingListItems: svc})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/shopping-lists/1/items/5", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}

// ---- Push / cart fakes + tests ----

type fakeShoppingPushSvc struct {
	binding RetailerListBindingResponse
	carts   []ShoppingCartResponse
	err     error
}

func (f *fakeShoppingPushSvc) PushShoppingList(_ context.Context, listID int64, retailer string) (RetailerListBindingResponse, error) {
	if f.err != nil {
		return RetailerListBindingResponse{}, f.err
	}
	return RetailerListBindingResponse{ID: 1, ShoppingListID: int(listID), Retailer: retailer, ExternalListID: "wl-123", SyncDirection: "outbound"}, nil
}

func (f *fakeShoppingPushSvc) ListShoppingCarts(_ context.Context, listID int64) ([]ShoppingCartResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.carts, nil
}

func (f *fakeShoppingPushSvc) ToCart(_ context.Context, listID int64, retailer string) (ShoppingCartResponse, error) {
	if f.err != nil {
		return ShoppingCartResponse{}, f.err
	}
	if len(f.carts) == 0 {
		return ShoppingCartResponse{}, errors.New("no carts")
	}
	return f.carts[0], nil
}

func TestPushShoppingList_HappyPath(t *testing.T) {
	svc := &fakeShoppingPushSvc{}
	mux := newMux(t, Dependencies{ShoppingPush: svc})
	rec := doPost(t, mux, "/shopping-lists/1/push?retailer=willys", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got RetailerListBindingResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.Retailer != "willys" || got.ExternalListID != "wl-123" {
		t.Fatalf("unexpected binding: %+v", got)
	}
}

func TestListShoppingCarts_HappyPath(t *testing.T) {
	svc := &fakeShoppingPushSvc{
		carts: []ShoppingCartResponse{{ID: 1, RetailerListBindingID: 1, Status: "created"}},
	}
	mux := newMux(t, Dependencies{ShoppingPush: svc})
	rec := doGet(t, mux, "/shopping-lists/1/carts")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got []ShoppingCartResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].Status != "created" {
		t.Fatalf("unexpected carts: %+v", got)
	}
}

func TestToCart_HappyPath(t *testing.T) {
	svc := &fakeShoppingPushSvc{
		carts: []ShoppingCartResponse{{ID: 1, RetailerListBindingID: 1, Status: "created"}},
	}
	mux := newMux(t, Dependencies{ShoppingPush: svc})
	rec := doPost(t, mux, "/shopping-lists/1/push/to-cart?retailer=willys", "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	var got ShoppingCartResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.ID != 1 {
		t.Fatalf("unexpected cart: %+v", got)
	}
}

// ---- Order fakes + tests ----

type fakeOrderSvc struct {
	orders []OrderResponse
	view   OrderViewResponse
	items  []OrderItemResponse
	err    error
}

func (f *fakeOrderSvc) ListOrders(_ context.Context, retailer *string, cartID *int64) ([]OrderResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := make([]OrderResponse, 0, len(f.orders))
	for _, o := range f.orders {
		if retailer != nil && o.Retailer != *retailer {
			continue
		}
		if cartID != nil && (o.ShoppingCartID == nil || int64(*o.ShoppingCartID) != *cartID) {
			continue
		}
		out = append(out, o)
	}
	return out, nil
}

func (f *fakeOrderSvc) GetOrder(_ context.Context, orderID int64) (OrderViewResponse, error) {
	if f.err != nil {
		return OrderViewResponse{}, f.err
	}
	return f.view, nil
}

func (f *fakeOrderSvc) ListOrderItems(_ context.Context, orderID int64) ([]OrderItemResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.items, nil
}

func TestListOrders_HappyPath(t *testing.T) {
	svc := &fakeOrderSvc{
		orders: []OrderResponse{{ID: 1, Retailer: "willys", Source: "manual"}},
	}
	mux := newMux(t, Dependencies{Orders: svc})
	rec := doGet(t, mux, "/orders")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got []OrderResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].Retailer != "willys" {
		t.Fatalf("unexpected orders: %+v", got)
	}
}

func TestGetOrder_HappyPath(t *testing.T) {
	svc := &fakeOrderSvc{
		view: OrderViewResponse{
			Order: OrderResponse{ID: 1, Retailer: "willys", Source: "manual"},
			Items: []OrderItemResponse{{ID: 1, OrderID: 1, RetailerProductID: "prod-1", Quantity: 2}},
		},
	}
	mux := newMux(t, Dependencies{Orders: svc})
	rec := doGet(t, mux, "/orders/1")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got OrderViewResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.Order.ID != 1 || len(got.Items) != 1 {
		t.Fatalf("unexpected view: %+v", got)
	}
}

func TestListOrderItems_HappyPath(t *testing.T) {
	svc := &fakeOrderSvc{
		items: []OrderItemResponse{{ID: 1, OrderID: 1, RetailerProductID: "prod-1", Quantity: 2}},
	}
	mux := newMux(t, Dependencies{Orders: svc})
	rec := doGet(t, mux, "/orders/1/items")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got []OrderItemResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].RetailerProductID != "prod-1" {
		t.Fatalf("unexpected items: %+v", got)
	}
}

func TestListOrders_FilterByRetailer(t *testing.T) {
	svc := &fakeOrderSvc{
		orders: []OrderResponse{{ID: 1, Retailer: "willys"}, {ID: 2, Retailer: "ica"}},
	}
	mux := newMux(t, Dependencies{Orders: svc})
	rec := doGet(t, mux, "/orders?retailer=willys")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got []OrderResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].Retailer != "willys" {
		t.Fatalf("unexpected orders: %+v", got)
	}
}
