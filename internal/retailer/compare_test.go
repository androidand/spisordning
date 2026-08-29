package retailer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/androidand/spisordning/internal/domain"
)

// mockResolveServer returns a /resolve stub that always answers with the given
// resolutions, in order.
func mockResolveServer(t *testing.T, resolutions []map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"resolutions": resolutions})
	}))
}

// mockErrorServer returns a /resolve stub that always answers with the given
// HTTP status and a JSON error, simulating an unavailable/stale adapter.
func mockErrorServer(t *testing.T, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "session expired"})
	}))
}

func cauliflowerReq() []domain.ShoppingRequirement {
	return []domain.ShoppingRequirement{{IngredientID: "cauliflower", Quantity: 500, Unit: "g"}}
}

func TestCompare_BothResolve_CheaperWins(t *testing.T) {
	willys := mockResolveServer(t, []map[string]any{{
		"ingredientId":      "cauliflower",
		"retailerProductId": "willys-123",
		"productName":       "Blomkål Klass 1",
		"priceValue":        29.9,
		"price":             "29,90 kr",
	}})
	defer willys.Close()
	ica := mockResolveServer(t, []map[string]any{{
		"ingredientId":      "cauliflower",
		"retailerProductId": "ica-456",
		"productName":       "Blomkål",
		"priceValue":        24.9,
		"price":             "24,90 kr",
	}})
	defer ica.Close()

	cmp := Compare(context.Background(), cauliflowerReq(), nil, willys.URL, ica.URL, "")
	if len(cmp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(cmp.Items))
	}
	item := cmp.Items[0]
	if item.Unresolved {
		t.Errorf("item should be resolved")
	}
	if len(item.Results) != 3 {
		t.Fatalf("expected 3 retailer results, got %d", len(item.Results))
	}
	w, i, h := item.Results[0], item.Results[1], item.Results[2]
	if w.Retailer != RetailerWillys || i.Retailer != RetailerICA || h.Retailer != RetailerHemkop {
		t.Fatalf("results must follow RetailerOrder (willys, ica, hemkop): %+v", item.Results)
	}
	if !w.Available || !i.Available {
		t.Errorf("willys and ica should be available: %+v", item.Results)
	}
	if h.Available {
		t.Errorf("hemkop should be unavailable (no adapter): %+v", h)
	}
	if item.Cheapest == nil {
		t.Fatal("expected a cheapest result")
	}
	if item.Cheapest.Retailer != RetailerICA {
		t.Errorf("expected ICA to be cheapest, got %v", item.Cheapest.Retailer)
	}
	if *item.Cheapest.PriceValue != 24.9 {
		t.Errorf("expected cheapest price 24.9, got %v", *item.Cheapest.PriceValue)
	}
}

func TestCompare_ICAStale_DegradesGracefully(t *testing.T) {
	willys := mockResolveServer(t, []map[string]any{{
		"ingredientId":      "cauliflower",
		"retailerProductId": "willys-123",
		"productName":       "Blomkål Klass 1",
		"priceValue":        29.9,
		"price":             "29,90 kr",
	}})
	defer willys.Close()
	ica := mockErrorServer(t, http.StatusBadGateway)
	defer ica.Close()

	cmp := Compare(context.Background(), cauliflowerReq(), nil, willys.URL, ica.URL, "")
	item := cmp.Items[0]
	if item.Unresolved {
		t.Errorf("item should still be resolved via Willys")
	}
	w, i, h := item.Results[0], item.Results[1], item.Results[2]
	if !w.Available {
		t.Errorf("Willys should be available")
	}
	if i.Available {
		t.Errorf("ICA should be unavailable after a 502")
	}
	if h.Available {
		t.Errorf("Hemkop should be unavailable (no adapter)")
	}
	if item.Cheapest == nil || item.Cheapest.Retailer != RetailerWillys {
		t.Errorf("expected Willys to be the only cheapest, got %+v", item.Cheapest)
	}
}

func TestCompare_NeitherResolves_Unresolved(t *testing.T) {
	noMatch := []map[string]any{{
		"ingredientId":      "saffron",
		"retailerProductId": nil,
		"matchType":         "none",
		"needsReview":       true,
	}}
	willys := mockResolveServer(t, noMatch)
	defer willys.Close()
	ica := mockResolveServer(t, noMatch)
	defer ica.Close()

	cmp := Compare(context.Background(),
		[]domain.ShoppingRequirement{{IngredientID: "saffron", Quantity: 1, Unit: "g"}},
		nil, willys.URL, ica.URL, "")
	item := cmp.Items[0]
	if !item.Unresolved {
		t.Errorf("item should be unresolved")
	}
	if item.Cheapest != nil {
		t.Errorf("unresolved item must have no cheapest, got %+v", item.Cheapest)
	}
	for _, r := range item.Results {
		if r.Available {
			t.Errorf("no retailer should be available: %+v", r)
		}
	}
}

func TestCompare_ResolvedWithoutPrice_NotCheapestCandidate(t *testing.T) {
	willys := mockResolveServer(t, []map[string]any{{
		"ingredientId":      "cauliflower",
		"retailerProductId": "willys-123",
		"productName":       "Blomkål Klass 1",
		// resolved, but no priceValue/price
	}})
	defer willys.Close()
	ica := mockResolveServer(t, []map[string]any{{
		"ingredientId":      "cauliflower",
		"retailerProductId": "ica-456",
		"productName":       "Blomkål",
		"priceValue":        24.9,
		"price":             "24,90 kr",
	}})
	defer ica.Close()

	cmp := Compare(context.Background(), cauliflowerReq(), nil, willys.URL, ica.URL, "")
	item := cmp.Items[0]
	if item.Unresolved {
		t.Errorf("item should be resolved (both matched a product)")
	}
	w, i, h := item.Results[0], item.Results[1], item.Results[2]
	if !w.Available || !i.Available {
		t.Errorf("willys and ica should be available (both matched): %+v", item.Results)
	}
	if h.Available {
		t.Errorf("hemkop should be unavailable (no adapter)")
	}
	if w.PriceValue != nil {
		t.Errorf("Willys resolved without a price, expected nil PriceValue")
	}
	if item.Cheapest == nil || item.Cheapest.Retailer != RetailerICA {
		t.Errorf("expected ICA (the only priced one) to be cheapest, got %+v", item.Cheapest)
	}
}
