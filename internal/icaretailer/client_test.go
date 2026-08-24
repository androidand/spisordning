package icaretailer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolve_RoundTrip(t *testing.T) {
	var captured struct {
		Terms []string `json:"terms"`
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/resolve" || r.Method != http.MethodPost {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"resolutions": []map[string]any{{
				"matchType":         "search",
				"productCode":       "7340039192756",
				"productName":       "Mjölk 3,2%",
				"packages":          1,
				"confidence":        0.92,
				"needsReview":       false,
				"quantityUncertain": false,
				"retailer":          "ica",
			}},
		})
	}))
	defer srv.Close()

	got, err := New(srv.URL).Resolve(context.Background(), []string{"mjölk"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(captured.Terms) != 1 || captured.Terms[0] != "mjölk" {
		t.Errorf("unexpected terms sent: %v", captured.Terms)
	}
	if len(got) != 1 || got[0].ProductCode != "7340039192756" {
		t.Fatalf("unexpected resolution: %+v", got)
	}
	if got[0].NeedsReview {
		t.Errorf("high-confidence resolution should not need review")
	}
	if got[0].Retailer != "ica" {
		t.Errorf("expected retailer 'ica', got %q", got[0].Retailer)
	}
}

func TestBarcodeLookup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/barcode/7340039192756" || r.Method != http.MethodGet {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"matchType":         "barcode",
			"productCode":       "7340039192756",
			"productName":       "Kex Choklad",
			"packages":          1,
			"confidence":        1.0,
			"needsReview":       false,
			"quantityUncertain": false,
			"retailer":          "ica",
		})
	}))
	defer srv.Close()

	got, err := New(srv.URL).BarcodeLookup(context.Background(), "7340039192756")
	if err != nil {
		t.Fatalf("BarcodeLookup: %v", err)
	}
	if got.MatchType != "barcode" {
		t.Errorf("expected matchType 'barcode', got %q", got.MatchType)
	}
	if got.ProductName != "Kex Choklad" {
		t.Errorf("unexpected product name: %q", got.ProductName)
	}
}

func TestCreateShoppingList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/shopping-lists" || r.Method != http.MethodPost {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body CreateShoppingListRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Name != "Vecka 32" || len(body.Items) != 1 {
			t.Errorf("unexpected body: %+v", body)
		}
		_ = json.NewEncoder(w).Encode(CreatedList{ExternalListID: "ica-list-123", Name: body.Name})
	}))
	defer srv.Close()

	got, err := New(srv.URL).CreateShoppingList(context.Background(), "Vecka 32", []ShoppingListItem{
		{Label: "Mjölk", Quantity: 1, Unit: "l"},
	})
	if err != nil {
		t.Fatalf("CreateShoppingList: %v", err)
	}
	if got.ExternalListID != "ica-list-123" {
		t.Errorf("unexpected external list id: %q", got.ExternalListID)
	}
}

func TestSyncShoppingList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/shopping-lists/ica-list-123/sync" || r.Method != http.MethodPost {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(CreatedList{ExternalListID: "ica-list-123", Name: "Vecka 32"})
	}))
	defer srv.Close()

	got, err := New(srv.URL).SyncShoppingList(context.Background(), "ica-list-123", "Vecka 32", nil)
	if err != nil {
		t.Fatalf("SyncShoppingList: %v", err)
	}
	if got.ExternalListID != "ica-list-123" {
		t.Errorf("unexpected external list id: %q", got.ExternalListID)
	}
}

func TestGetBonusBalance(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bonus" || r.Method != http.MethodGet {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(BonusBalance{
			Balance:         125.50,
			Vouchers:        []string{"10% nästa köp"},
			DiscountSummary: "Lovisa medlem",
		})
	}))
	defer srv.Close()

	got, err := New(srv.URL).GetBonusBalance(context.Background())
	if err != nil {
		t.Fatalf("GetBonusBalance: %v", err)
	}
	if got.Balance != 125.50 {
		t.Errorf("unexpected balance: %f", got.Balance)
	}
	if len(got.Vouchers) != 1 || got.Vouchers[0] != "10% nästa köp" {
		t.Errorf("unexpected vouchers: %v", got.Vouchers)
	}
}

func TestSearchProducts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" || r.Method != http.MethodPost {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body ProductSearchRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Query != "kex" {
			t.Errorf("unexpected query: %q", body.Query)
		}
		_ = json.NewEncoder(w).Encode(ProductSearchResponse{
			Hits: []ProductHit{
				{ProductCode: "7340039192756", ProductName: "Kex Choklad", Price: 18.90, Available: true},
				{ProductCode: "7340039192763", ProductName: "Kex Vanilj", Price: 18.90, Available: true},
			},
		})
	}))
	defer srv.Close()

	got, err := New(srv.URL).SearchProducts(context.Background(), "kex")
	if err != nil {
		t.Fatalf("SearchProducts: %v", err)
	}
	if len(got.Hits) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(got.Hits))
	}
	if got.Hits[0].ProductCode != "7340039192756" {
		t.Errorf("unexpected first hit: %+v", got.Hits[0])
	}
}

func TestGetProduct(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/products/7340039192756" || r.Method != http.MethodGet {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(ProductDetail{
			ProductCode: "7340039192756",
			ProductName: "Kex Choklad",
			Price:       18.90,
			Available:   true,
		})
	}))
	defer srv.Close()

	got, err := New(srv.URL).GetProduct(context.Background(), "7340039192756")
	if err != nil {
		t.Fatalf("GetProduct: %v", err)
	}
	if got.ProductName != "Kex Choklad" {
		t.Errorf("unexpected product name: %q", got.ProductName)
	}
	if got.Price != 18.90 {
		t.Errorf("unexpected price: %f", got.Price)
	}
}

func TestAdapterErrorSurfaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "ica session expired"})
	}))
	defer srv.Close()

	_, err := New(srv.URL).Resolve(context.Background(), []string{"mjölk"})
	if err == nil {
		t.Fatal("expected error from 502 response")
	}
}
