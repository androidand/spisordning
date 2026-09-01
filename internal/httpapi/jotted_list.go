package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/androidand/spisordning/internal/domain"
)

// JottedListItem is one free-text line a person jotted for shopping. It maps
// onto a CompareRequirement by normalizing Item to a canonical ingredient name;
// Quantity and Unit pass through to the comparison unchanged.
type JottedListItem struct {
	Item     string  `json:"item"`
	Quantity float64 `json:"quantity"`
	Unit     string  `json:"unit"`
}

// JottedListInput is the request body for POST /shopping/suggest: a free-text
// shopping list to price across retailers.
type JottedListInput struct {
	Items []JottedListItem `json:"items"`
}

// jottedListHandler maps a jotted free-text list onto canonical compare
// requirements and returns the existing PriceComparison unchanged. It is a
// thin adapter over PriceComparisonService.ComparePrices: the only logic here
// is the free-text-to-ingredient-name mapping.
type jottedListHandler struct {
	svc PriceComparisonService
}

func (h *jottedListHandler) suggest(w http.ResponseWriter, r *http.Request) {
	var in JottedListInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	if len(in.Items) == 0 {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "at least one item is required"})
		return
	}
	reqs := make([]CompareRequirement, 0, len(in.Items))
	labels := make([]string, 0, len(in.Items))
	for _, it := range in.Items {
		reqs = append(reqs, CompareRequirement{
			Ingredient: domain.CanonicalIngredientID(it.Item),
			Quantity:   it.Quantity,
			Unit:       it.Unit,
		})
		labels = append(labels, it.Item)
	}
	out, err := h.svc.ComparePrices(r.Context(), reqs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "compare prices: "+err.Error())
		return
	}
	echoLabels(out.Items, labels)
	writeJSON(w, http.StatusOK, out)
}

// echoLabels sets each result item's Label from the corresponding input free-text
// line, so a jotted list keeps a match between what the person wrote and the
// returned row. Labels beyond the item count are ignored.
func echoLabels(items []ItemComparison, labels []string) {
	for i := range items {
		if i < len(labels) {
			items[i].Label = labels[i]
		}
	}
}
