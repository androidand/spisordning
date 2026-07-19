package retailer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/androidand/spisordning/internal/planning"
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

	reqs := []planning.ShoppingRequirement{{
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
		[]planning.ShoppingRequirement{{IngredientID: "saffron", Quantity: 1, Unit: "g"}}, nil)
	if err != nil {
		t.Fatalf("ResolveRequirements: %v", err)
	}
	if !got[0].NeedsReview || got[0].RetailerProductID != nil {
		t.Errorf("unresolved item must keep needsReview and nil product id: %+v", got[0])
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
		[]planning.ShoppingRequirement{{IngredientID: "x", Quantity: 1, Unit: "g"}}, nil)
	if err == nil {
		t.Fatal("expected error from 502 response")
	}
}
