package mcptools_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/androidand/spisordning/internal/mcptools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// fakeCoverageSvc returns a canned report and records the input, so the test
// asserts the thin handler (validation + passthrough) rather than the
// application-layer coverage computation, which is unit-tested elsewhere.
type fakeCoverageSvc struct {
	out   mcptools.CoverageReport
	got   mcptools.CheckCoverageInput
	calls int
}

func (f *fakeCoverageSvc) CheckCoverage(_ context.Context, in mcptools.CheckCoverageInput) (mcptools.CoverageReport, error) {
	f.calls++
	f.got = in
	return f.out, nil
}

func TestCheckShoppingCoverage(t *testing.T) {
	svc := &fakeCoverageSvc{out: mcptools.CoverageReport{
		ShortCount: 1, MissingCount: 1, NotPlanDerived: 1,
		Lines: []mcptools.CoverageLine{
			{IngredientID: "mjölk", IngredientName: "jölk", Unit: "l", Status: "covered", Required: 1, Supplied: 1},
			{IngredientID: "mjöl", IngredientName: "mhjörningsmjöl", Unit: "g", Status: "short", Required: 500, Supplied: 200, Shortfall: 300},
			{IngredientID: "flour", IngredientName: "mhjörningsmjöl", Unit: "g", Status: "missing", Required: 250, Supplied: 0},
		},
	}}
	cs := connectServer(t, mcptools.Dependencies{Coverage: svc})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "check_shopping_coverage",
		Arguments: map[string]any{
			"shopping_list_id": "list-123",
			"plan_id":          "plan-456",
		},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}
	if svc.calls != 1 {
		t.Fatalf("CheckCoverage called %d times, want 1", svc.calls)
	}
	if svc.got.ShoppingListID != "list-123" || svc.got.PlanID != "plan-456" {
		t.Fatalf("service got %+v, want list-123/plan-456", svc.got)
	}

	var body mcptools.CoverageReport
	b, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := json.Unmarshal(b, &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.ShortCount != 1 || body.MissingCount != 1 || body.NotPlanDerived != 1 {
		t.Fatalf("summary mismatch: %+v", body)
	}
	if len(body.Lines) != 3 {
		t.Fatalf("got %d lines, want 3 (%+v)", len(body.Lines), body.Lines)
	}
	if body.Lines[1].Status != "short" || body.Lines[1].Shortfall != 300 {
		t.Fatalf("short line mismatch: %+v", body.Lines[1])
	}
	if body.Lines[2].Status != "missing" {
		t.Fatalf("missing line status = %q, want missing", body.Lines[2].Status)
	}
}

func TestCheckShoppingCoverage_MissingInputRejected(t *testing.T) {
	svc := &fakeCoverageSvc{}
	cs := connectServer(t, mcptools.Dependencies{Coverage: svc})

	// plan_id omitted.
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "check_shopping_coverage",
		Arguments: map[string]any{"shopping_list_id": "list-123"},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if !res.IsError {
		t.Fatalf("missing plan_id should error, got %+v", res)
	}
	if svc.calls != 0 {
		t.Fatalf("CheckCoverage called %d times for invalid input, want 0", svc.calls)
	}
}
