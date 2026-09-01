package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/androidand/spisordning/internal/domain"
)

// fakeJottedCompareSvc records the requirements it received and returns a
// canned PriceComparison, so the handler test asserts only the free-text
// mapping (and not the comparison itself).
type fakeJottedCompareSvc struct {
	got        []CompareRequirement
	compareOut PriceComparison
}

func (f *fakeJottedCompareSvc) ComparePrices(ctx context.Context, reqs []CompareRequirement) (PriceComparison, error) {
	f.got = reqs
	return f.compareOut, nil
}

func TestSuggestJottedList_MapsItemsToCanonicalRequirements(t *testing.T) {
	svc := &fakeJottedCompareSvc{compareOut: PriceComparison{Items: []ItemComparison{
		{Ingredient: "kycklingfilé", Unresolved: false},
		{Ingredient: "loök", Unresolved: false},
	}}}
	mux := newMux(t, Dependencies{PriceComparison: svc})

	rec := doPost(t, mux, "/shopping/suggest", `{"items":[{"item":" Kycklingfilé ","quantity":500,"unit":"g"},{"item":"lök","quantity":2,"unit":"st"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	if len(svc.got) != 2 {
		t.Fatalf("sent %d requirements, want 2", len(svc.got))
	}

	// The mapped ingredient is CanonicalIngredientID(item); Quantity/Unit pass
	// through. Deriving the expected name from the same canonicalizer keeps the
	// assertion focused on "name is mapped and amount survives" rather than on
	// the exact Unicode normalization of diaeresis.
	assertMapped(t, svc.got, []item{{label: " Kycklingfilé ", qty: 500, unit: "g"}, {label: "lök", qty: 2, unit: "st"}}, []item{{label: " Kycklingfilé ", qty: 500, unit: "g"}, {label: "lök", qty: 2, unit: "st"}})
}

func assertMapped(t *testing.T, got []CompareRequirement, in []item, want []item) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("sent %d requirements, want %d (%+v)", len(got), len(want), got)
	}
	for i, r := range got {
		if want[i].qty != r.Quantity || want[i].unit != r.Unit {
			t.Errorf("mapped requirement for %q = {qty:%v unit:%q}, want {qty:%v unit:%q}", in[i].label, r.Quantity, r.Unit, want[i].qty, want[i].unit)
		}
		if r.Ingredient != domain.CanonicalIngredientID(in[i].label) {
			t.Errorf("mapped ingredient for %q = %q, want %q", in[i].label, r.Ingredient, domain.CanonicalIngredientID(in[i].label))
		}
	}
}

type item struct {
	label string
	qty   float64
	unit  string
}

func TestSuggestJottedList_EmptyListRejected(t *testing.T) {
	svc := &fakeJottedCompareSvc{}
	mux := newMux(t, Dependencies{PriceComparison: svc})

	rec := doPost(t, mux, "/shopping/suggest", `{"items":[]}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty list status = %d, want 400", rec.Code)
	}
	if len(svc.got) != 0 {
		t.Fatalf("service should not be called for an empty list, got %+v", svc.got)
	}
}

func TestSuggestJottedList_UnresolvedLinePreserved(t *testing.T) {
	svc := &fakeJottedCompareSvc{compareOut: PriceComparison{Items: []ItemComparison{
		{Ingredient: "bokhyllsmjölk", Unresolved: true},
	}}}
	mux := newMux(t, Dependencies{PriceComparison: svc})

	rec := doPost(t, mux, "/shopping/suggest", `{"items":[{"item":"bokhyllsmjölk","quantity":1,"unit":"l"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}

	var got PriceComparison
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got.Items) != 1 || !got.Items[0].Unresolved {
		t.Fatalf("unresolved line not preserved: %+v", got.Items)
	}
}

func TestSuggestJottedList_MalformedJSONRejected(t *testing.T) {
	svc := &fakeJottedCompareSvc{}
	mux := newMux(t, Dependencies{PriceComparison: svc})

	rec := doPost(t, mux, "/shopping/suggest", `{"items":`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed JSON status = %d, want 400", rec.Code)
	}
	if len(svc.got) != 0 {
		t.Fatalf("service should not be called on malformed JSON, got %+v", svc.got)
	}
}

// TestSuggestJottedList_SharedWithMCP is the shared assertion: the exact same
// mapped requirements must reach the service regardless of surface. The MCP
// handler (internal/mcptools) builds them identically, so a regression in one
// surface is caught here.
func TestSuggestJottedList_SharedWithMCP(t *testing.T) {
	svc := &fakeJottedCompareSvc{compareOut: PriceComparison{Items: []ItemComparison{{Ingredient: "mjöl"}}}}
	mux := newMux(t, Dependencies{PriceComparison: svc})

	// "Mjööl" (mixed case, leading char) must map to the canonical "mjöl".
	rec := doPost(t, mux, "/shopping/suggest", `{"items":[{"item":"Mjööl","quantity":500,"unit":"g"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	assertMapped(t, svc.got, []item{{label: "Mjööl", qty: 500, unit: "g"}}, []item{{label: "Mjööl", qty: 500, unit: "g"}})

	// The response body round-trips through writeJSON as the same shape /compare
	// returns (items with ingredient + unresolved). This is the "response body
	// matches the compare endpoint" spec scenario.
	var body PriceComparison
	mustJSON(t, rec.Body.Bytes(), &body)
	if len(body.Items) != 1 || body.Items[0].Ingredient != "mjöl" {
		t.Fatalf("response body did not carry the mapped ingredient: %+v", body.Items)
	}
	// The free-text line is echoed back as the item label (design + openapi
	// contract), so a jotted list can match a result row to what was written.
	if body.Items[0].Label != "Mjööl" {
		t.Fatalf("response body did not echo the input label, got %q", body.Items[0].Label)
	}
}
