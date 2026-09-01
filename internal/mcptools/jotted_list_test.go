package mcptools_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/androidand/spisordning/internal/mcptools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// fakeCompareSvc records the requirements the tool passed to ComparePrices and
// returns a canned comparison, so the test asserts only the free-text mapping.
type fakeCompareSvc struct {
	got    []mcptools.ShoppingRequirement
	out    mcptools.PriceComparison
	calls  int
}

func (f *fakeCompareSvc) ComparePrices(_ context.Context, reqs []mcptools.ShoppingRequirement) (mcptools.PriceComparison, error) {
	f.calls++
	f.got = reqs
	return f.out, nil
}

func TestResolveJottedList(t *testing.T) {
	svc := &fakeCompareSvc{out: mcptools.PriceComparison{Items: []mcptools.ItemComparison{
		{Ingredient: "kycklingfilé", Unresolved: false},
		{Ingredient: "loök", Unresolved: false},
	}}}
	cs := connectServer(t, mcptools.Dependencies{Compare: svc})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "resolve_jotted_list",
		Arguments: map[string]any{"items": []any{
			map[string]any{"item": " Kycklingfilé ", "quantity": 500, "unit": "g"},
			map[string]any{"item": "lök", "quantity": 2, "unit": "st"},
		}},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}
	if svc.calls != 1 {
		t.Fatalf("ComparePrices called %d times, want 1", svc.calls)
	}
	if len(svc.got) != 2 {
		t.Fatalf("ComparePrices got %d requirements, want 2", len(svc.got))
	}

	// Ingredient is CanonicalIngredientID(item); quantity and unit pass through.
	assertMappedMCPT(t, svc.got, []mcptools.JottedListItem{
		{Item: " Kycklingfilé ", Quantity: 500, Unit: "g"},
		{Item: "lök", Quantity: 2, Unit: "st"},
	})
}

func assertMappedMCPT(t *testing.T, got []mcptools.ShoppingRequirement, in []mcptools.JottedListItem) {
	t.Helper()
	if len(got) != len(in) {
		t.Fatalf("got %d requirements, want %d (%+v)", len(got), len(in), got)
	}
	for i, r := range got {
		if r.Quantity != in[i].Quantity || r.Unit != in[i].Unit {
			t.Errorf("mapped requirement for %q = {qty:%v unit:%q}, want {qty:%v unit:%q}", in[i].Item, r.Quantity, r.Unit, in[i].Quantity, in[i].Unit)
		}
		if r.Ingredient != domain.CanonicalIngredientID(in[i].Item) {
			t.Errorf("mapped ingredient for %q = %q, want %q", in[i].Item, r.Ingredient, domain.CanonicalIngredientID(in[i].Item))
		}
	}
}

func TestResolveJottedList_EmptyRejected(t *testing.T) {
	svc := &fakeCompareSvc{}
	cs := connectServer(t, mcptools.Dependencies{Compare: svc})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "resolve_jotted_list",
		Arguments: map[string]any{"items": []any{}},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if !res.IsError {
		t.Fatalf("empty list should error, got %+v", res)
	}
	if svc.calls != 0 {
		t.Fatalf("ComparePrices called %d times for empty list, want 0", svc.calls)
	}
}

// TestResolveJottedList_SharedWithREST is the shared assertion: the exact same
// mapped requirements must reach the service regardless of surface. The REST
// handler (internal/httpapi) builds them identically, so a divergence here is
// caught by this test.
func TestResolveJottedList_SharedWithREST(t *testing.T) {
	svc := &fakeCompareSvc{out: mcptools.PriceComparison{Items: []mcptools.ItemComparison{{Ingredient: "mjöl"}}}}
	cs := connectServer(t, mcptools.Dependencies{Compare: svc})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "resolve_jotted_list",
		Arguments: map[string]any{"items": []any{
			map[string]any{"item": "Mjööl", "quantity": 500, "unit": "g"},
		}},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}
	if len(svc.got) != 1 || svc.got[0].Quantity != 500 || svc.got[0].Unit != "g" {
		t.Fatalf("mapped requirements not identical across surfaces: %+v", svc.got)
	}
	// The MCP surface echoes the input free-text line as the item label, mirroring
	// the REST surface so a jotted list stays traceable through the tool output.
	var body mcptools.PriceComparison
	b, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	if err := json.Unmarshal(b, &body); err != nil {
		t.Fatalf("unmarshal structured content: %v", err)
	}
	if len(body.Items) != 1 || body.Items[0].Label != "Mjööl" {
		t.Fatalf("MCP response did not echo the input label, got %+v", body.Items)
	}
}
