package retailer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/androidand/spisordning/internal/domain"
)

func TestResolveRequirements_RoundTrip(t *testing.T) {
	var captured struct {
		Requirements []map[string]any `json:"requirements"`
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
				"ingredientId":      "cauliflower",
				"retailerProductId": "willys-123",
				"productName":       "Blomkål Klass 1",
				"packages":          1,
				"resolvedQuantity":  650.0,
				"matchType":         "exact",
				"confidence":        0.94,
				"needsReview":       false,
			}},
		})
	}))
	defer srv.Close()

	reqs := []domain.ShoppingRequirement{{
		IngredientID:    "cauliflower",
		Quantity:        500,
		Unit:            "g",
		AcceptableForms: []string{"fresh", "frozen"},
		PreferredForm:   "fresh",
	}}
	terms := SearchTerms{"cauliflower": "blomkål"}

	got, err := New(srv.URL).ResolveRequirements(context.Background(), reqs, terms)
	if err != nil {
		t.Fatalf("ResolveRequirements: %v", err)
	}

	// The adapter received the canonical shape with the Swedish search term.
	if len(captured.Requirements) != 1 {
		t.Fatalf("expected 1 requirement sent, got %d", len(captured.Requirements))
	}
	sent := captured.Requirements[0]
	if sent["searchTerm"] != "blomkål" || sent["ingredientId"] != "cauliflower" {
		t.Errorf("unexpected payload: %v", sent)
	}
	if _, hasRetailerID := sent["retailerProductId"]; hasRetailerID {
		t.Errorf("requirement payload must never carry a retailer product id")
	}

	if len(got) != 1 || got[0].RetailerProductID == nil || *got[0].RetailerProductID != "willys-123" {
		t.Fatalf("unexpected resolutions: %+v", got)
	}
	if got[0].NeedsReview {
		t.Errorf("high-confidence resolution should not need review")
	}
}

func TestResolveRequirements_PreservesReviewFlag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"resolutions": []map[string]any{{
				"ingredientId":      "saffron",
				"retailerProductId": nil,
				"packages":          0,
				"matchType":         "none",
				"confidence":        0.0,
				"needsReview":       true,
			}},
		})
	}))
	defer srv.Close()

	got, err := New(srv.URL).ResolveRequirements(context.Background(),
		[]domain.ShoppingRequirement{{IngredientID: "saffron", Quantity: 1, Unit: "g"}}, nil)
	if err != nil {
		t.Fatalf("ResolveRequirements: %v", err)
	}
	if !got[0].NeedsReview || got[0].RetailerProductID != nil {
		t.Errorf("unresolved item must keep needsReview and nil product id: %+v", got[0])
	}
}

func TestResolveRequirements_PriceFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"resolutions": []map[string]any{
				{
					"ingredientId":      "cauliflower",
					"retailerProductId": "willys-123",
					"productName":       "Blomkål Klass 1",
					"packages":          1,
					"matchType":         "exact",
					"confidence":        0.94,
					"needsReview":       false,
					"priceValue":        29.9,
					"price":             "29,90 kr",
				},
				{
					"ingredientId":      "saffron",
					"retailerProductId": nil,
					"packages":          0,
					"matchType":         "none",
					"confidence":        0.0,
					"needsReview":       true,
				},
			},
		})
	}))
	defer srv.Close()

	got, err := New(srv.URL).ResolveRequirements(context.Background(),
		[]domain.ShoppingRequirement{
			{IngredientID: "cauliflower", Quantity: 500, Unit: "g"},
			{IngredientID: "saffron", Quantity: 1, Unit: "g"},
		}, nil)
	if err != nil {
		t.Fatalf("ResolveRequirements: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 resolutions, got %d", len(got))
	}
	// A priced resolution carries both the numeric (comparable) and display price.
	if got[0].PriceValue == nil || *got[0].PriceValue != 29.9 {
		t.Errorf("expected priceValue 29.9, got %+v", got[0].PriceValue)
	}
	if got[0].Price == nil || *got[0].Price != "29,90 kr" {
		t.Errorf("expected price \"29,90 kr\", got %+v", got[0].Price)
	}
	// An unresolved resolution leaves price absent (nil), not zero.
	if got[1].PriceValue != nil || got[1].Price != nil {
		t.Errorf("unresolved resolution must have nil price, got %+v", got[1])
	}
}

func TestCreateShoppingList_RoundTrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/shopping-lists" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		var body struct {
			Name  string             `json:"name"`
			Items []ShoppingListItem `json:"items"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Name != "Vecka 30" || len(body.Items) != 2 {
			t.Errorf("unexpected body: %+v", body)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(CreatedList{WishlistID: "9639791045159", Name: body.Name})
	}))
	defer srv.Close()

	got, err := New(srv.URL).CreateShoppingList(context.Background(), "Vecka 30", []ShoppingListItem{
		{ProductCode: "willys-123", Quantity: 1},
		{ProductCode: "willys-456", Quantity: 2},
	})
	if err != nil {
		t.Fatalf("CreateShoppingList: %v", err)
	}
	if got.WishlistID != "9639791045159" {
		t.Errorf("unexpected wishlist id %q", got.WishlistID)
	}
}

func TestAdapterErrorSurfaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "willys session expired"})
	}))
	defer srv.Close()

	_, err := New(srv.URL).ResolveRequirements(context.Background(),
		[]domain.ShoppingRequirement{{IngredientID: "x", Quantity: 1, Unit: "g"}}, nil)
	if err == nil {
		t.Fatal("expected error from 502 response")
	}
}

// TestNewICA_DifferentPrefix verifies that ICA clients use a distinct error
// prefix so failures are attributable to the right adapter.
func TestNewICA_DifferentPrefix(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "ica session expired"})
	}))
	defer srv.Close()

	_, err := NewICA(srv.URL).ResolveRequirements(context.Background(),
		[]domain.ShoppingRequirement{{IngredientID: "x", Quantity: 1, Unit: "g"}}, nil)
	if err == nil {
		t.Fatal("expected error from 502 response")
	}
	if !strings.Contains(err.Error(), "ica-adapter") {
		t.Errorf("expected error to be prefixed with 'ica-adapter', got: %v", err)
	}
}

// ── ICA-specific tests ──────────────────────────────────────────────────────

func TestLookupBarcode_RoundTrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/barcode/7320103456789" || r.Method != http.MethodGet {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"gtin":          "7320103456789",
			"name":          "Yes Original Handdiskmedel",
			"articleId":     12345,
			"articleGroupId": 678,
		})
	}))
	defer srv.Close()

	got, err := NewICA(srv.URL).LookupBarcode(context.Background(), "7320103456789")
	if err != nil {
		t.Fatalf("LookupBarcode: %v", err)
	}
	if got.GTIN == nil || *got.GTIN != "7320103456789" {
		t.Errorf("unexpected gtin: %+v", got)
	}
	if got.Name == nil || *got.Name != "Yes Original Handdiskmedel" {
		t.Errorf("unexpected name: %+v", got)
	}
}

func TestLookupBarcode_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := NewICA(srv.URL).LookupBarcode(context.Background(), "0000000000000")
	if err == nil {
		t.Fatal("expected error from 404 response")
	}
}

func TestGetBonusBalance_RoundTrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bonus" || r.Method != http.MethodGet {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"balance":         125.50,
			"vouchers":        2,
			"discountSummary": "5% rabatt denna vecka",
		})
	}))
	defer srv.Close()

	got, err := NewICA(srv.URL).GetBonusBalance(context.Background())
	if err != nil {
		t.Fatalf("GetBonusBalance: %v", err)
	}
	if got.Balance != 125.50 {
		t.Errorf("unexpected balance: %f", got.Balance)
	}
	if got.Vouchers != 2 {
		t.Errorf("unexpected vouchers: %d", got.Vouchers)
	}
}

func TestSyncShoppingList_RoundTrip(t *testing.T) {
	var captured ShoppingListSyncPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/shopping-lists/my-list/sync" || r.Method != http.MethodPost {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&captured)
		_ = json.NewEncoder(w).Encode(map[string]any{"offlineId": "my-list", "rows": []any{}})
	}))
	defer srv.Close()

	got, err := NewICA(srv.URL).SyncShoppingList(context.Background(), "my-list", ShoppingListSyncPayload{
		OfflineID: "my-list",
		Created: []ShoppingListSyncDelta{{
			ProductName: "Blomkål",
			ProductEan:  "7320103456789",
			Quantity:    1,
			Unit:        "st",
		}},
		Deleted: []string{"old-row-id"},
	})
	if err != nil {
		t.Fatalf("SyncShoppingList: %v", err)
	}
	if captured.OfflineID != "my-list" {
		t.Errorf("unexpected offlineId: %s", captured.OfflineID)
	}
	if len(captured.Created) != 1 || captured.Created[0].ProductName != "Blomkål" {
		t.Errorf("unexpected created rows: %+v", captured.Created)
	}
	if len(captured.Deleted) != 1 || captured.Deleted[0] != "old-row-id" {
		t.Errorf("unexpected deleted rows: %+v", captured.Deleted)
	}
	if got.OfflineID != "my-list" {
		t.Errorf("unexpected response offlineId: %s", got.OfflineID)
	}
}
