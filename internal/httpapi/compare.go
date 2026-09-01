package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
)

// CompareRequirement is one canonical, retailer-independent shopping line to
// compare across retailers (mirrors the MCP compare_shopping_prices tool input).
type CompareRequirement struct {
	Ingredient      string   `json:"ingredient"`
	Quantity        float64  `json:"quantity"`
	Unit            string   `json:"unit"`
	AcceptableForms []string `json:"acceptable_forms,omitempty"`
	PreferredForm   string   `json:"preferred_form,omitempty"`
}

// CompareInput is the request body for POST /compare.
type CompareInput struct {
	Requirements []CompareRequirement `json:"requirements"`
}

// RetailerPriceResult is one retailer's outcome for a single requirement.
type RetailerPriceResult struct {
	Retailer    string   `json:"retailer"`
	Available   bool     `json:"available"`
	ProductID   *string  `json:"product_id,omitempty"`
	ProductName *string  `json:"product_name,omitempty"`
	PriceValue  *float64 `json:"price_value,omitempty"`
	Price       *string  `json:"price,omitempty"`
	// Error is set when the retailer's resolve call failed entirely (e.g. a
	// stale session) so the caller can say why it is unavailable.
	Error string `json:"error,omitempty"`
}

// ItemComparison is the cross-retailer comparison for one requirement.
type ItemComparison struct {
	Ingredient string                `json:"ingredient"`
	// Label echoes the original free-text line that produced this comparison.
	Label    string                `json:"label,omitempty"`
	Results    []RetailerPriceResult `json:"results"`
	Cheapest   *RetailerPriceResult  `json:"cheapest,omitempty"`
	Unresolved bool                  `json:"unresolved"`
}

// PriceComparison is the response body for POST /compare: the full cross-retailer
// price comparison for a set of requirements.
type PriceComparison struct {
	Items []ItemComparison `json:"items"`
}

// PriceComparisonService compares prices across retailers for a set of
// requirements. A stale or unavailable retailer degrades to available:false per
// item instead of failing the call. The cmd composition root supplies an
// implementation backed by internal/retailer.Compare.
type PriceComparisonService interface {
	ComparePrices(ctx context.Context, reqs []CompareRequirement) (PriceComparison, error)
}

type compareHandler struct {
	svc PriceComparisonService
}

func (h *compareHandler) compare(w http.ResponseWriter, r *http.Request) {
	var in CompareInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	if len(in.Requirements) == 0 {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "at least one requirement is required"})
		return
	}
	out, err := h.svc.ComparePrices(r.Context(), in.Requirements)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "compare prices: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}
