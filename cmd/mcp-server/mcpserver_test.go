package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/androidand/spisordning/internal/mcptools"
)

// fakes redeclared here (not shared with the mcptools unit tests) so the
// integration test can drive the real cmd/mcp-server composition root over
// Streamable HTTP without a database. They implement the mcptools service
// interfaces.

type fakePlanner struct {
	calls   int
	dinners []mcptools.PlannedSlot
}

func (f *fakePlanner) PlanDinners(_ context.Context, _ time.Time, _ int) ([]mcptools.PlannedSlot, error) {
	f.calls++
	return f.dinners, nil
}

func (f *fakePlanner) PlanSlots(_ context.Context, _ time.Time, _ int, _ []string) ([]mcptools.PlannedSlot, error) {
	f.calls++
	return f.dinners, nil
}

type fakeReactions struct {
	calls int
	last  mcptools.RecordReactionInput
}

func (f *fakeReactions) RecordReaction(_ context.Context, in mcptools.RecordReactionInput) (mcptools.RecordReactionResult, error) {
	f.calls++
	f.last = in
	return mcptools.RecordReactionResult{
		MealEventID: "meal-7", Recipe: in.Recipe, ServedOn: in.ServedOn, PersonID: in.PersonID, Sentiment: in.Sentiment,
	}, nil
}

type fakeRequirements struct {
	calls int
	reqs  []mcptools.ShoppingRequirement
}

func (f *fakeRequirements) ShoppingRequirements(_ context.Context, _ []string) ([]mcptools.ShoppingRequirement, error) {
	f.calls++
	return f.reqs, nil
}

// startServer builds the real MCP server with the given deps and serves it over
// Streamable HTTP on a throwaway httptest server, returning the MCP endpoint URL.
func startServer(t *testing.T, deps mcptools.Dependencies) string {
	t.Helper()
	server := newMCPServer(deps, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(newMCPHandler(server))
	t.Cleanup(ts.Close)
	return ts.URL + "/mcp"
}

// connectClient connects a real MCP client over the Streamable HTTP transport.
func connectClient(t *testing.T, endpoint string) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "integration-client", Version: "v0"}, nil)
	transport := &mcp.StreamableClientTransport{
		Endpoint:             endpoint,
		HTTPClient:           &http.Client{},
		DisableStandaloneSSE: true,
	}
	cs, err := client.Connect(context.Background(), transport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func structured[T any](t *testing.T, res *mcp.CallToolResult) T {
	t.Helper()
	var out T
	b, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal structured content: %v", err)
	}
	return out
}

func toolNames(tools []*mcp.Tool) []string {
	out := make([]string, 0, len(tools))
	for _, tool := range tools {
		out = append(out, tool.Name)
	}
	return out
}

// TestIntegration_StreamableHTTP drives the real server end-to-end over
// Streamable HTTP via the SDK client: connect, list tools, call a tool, and
// assert both the response and the side effect on the fake service.
func TestIntegration_StreamableHTTP(t *testing.T) {
	planner := &fakePlanner{dinners: []mcptools.PlannedSlot{{Date: "2026-08-20", Slot: "dinner", Recipe: "r1", Title: "Pasta", Score: 0.9}}}
	reactions := &fakeReactions{}
	reqs := &fakeRequirements{reqs: []mcptools.ShoppingRequirement{{Ingredient: "tomato", Quantity: 4, Unit: "pcs"}}}
	deps := mcptools.Dependencies{Planner: planner, Reactions: reactions, Requirements: reqs}

	cs := connectClient(t, startServer(t, deps))

	list, err := cs.ListTools(context.Background(), &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	if len(list.Tools) != 3 {
		t.Fatalf("expected 3 tools, got %d: %v", len(list.Tools), toolNames(list.Tools))
	}

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "record_meal_reaction",
		Arguments: map[string]any{
			"recipe": "r1", "served_on": "2026-08-20", "person_id": "p1", "sentiment": 2,
		},
	})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}
	got := structured[mcptools.RecordReactionResult](t, res)
	if got.MealEventID != "meal-7" || got.Recipe != "r1" || got.Sentiment != 2 {
		t.Fatalf("unexpected result: %+v", got)
	}
	if reactions.calls != 1 {
		t.Fatalf("fake reactions called %d times, want 1", reactions.calls)
	}
	if reactions.last.Recipe != "r1" || reactions.last.PersonID != "p1" || reactions.last.Sentiment != 2 {
		t.Fatalf("unexpected side effect on fake service: %+v", reactions.last)
	}
}

// TestIntegration_MalformedCallRejected verifies a malformed tool call is
// rejected without reaching the application layer, over the real transport.
func TestIntegration_MalformedCallRejected(t *testing.T) {
	reactions := &fakeReactions{}
	cs := connectClient(t, startServer(t, mcptools.Dependencies{Reactions: reactions}))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "record_meal_reaction",
		Arguments: map[string]any{
			"recipe": "r1", "served_on": "2026-08-20", "person_id": "p1", "sentiment": "not-a-number",
		},
	})
	if err != nil {
		t.Fatalf("unexpected protocol error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected tool error for malformed call, got %+v", res)
	}
	if reactions.calls != 0 {
		t.Fatalf("fake reactions called %d times, want 0 (malformed call must not reach the application layer)", reactions.calls)
	}
}

// TestIntegration_SevenDayPlanWithAllSlots requests a 7-day plan over MCP with
// all three slot kinds and asserts dinner+breakfast+snack candidates are
// returned for each date. It also records a breakfast reaction and verifies
// the slot is passed through.
func TestIntegration_SevenDayPlanWithAllSlots(t *testing.T) {
	// Build a fake planner that returns all three slot kinds for 7 days.
	var slots []mcptools.PlannedSlot
	for i := 0; i < 7; i++ {
		date := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i).Format("2006-01-02")
		slots = append(slots,
			mcptools.PlannedSlot{Date: date, Slot: "dinner", Recipe: fmt.Sprintf("dinner-%d", i), Title: fmt.Sprintf("Dinner %d", i), Score: 0.9},
			mcptools.PlannedSlot{Date: date, Slot: "breakfast", Recipe: fmt.Sprintf("breakfast-%d", i), Title: fmt.Sprintf("Breakfast %d", i), Score: 0.8},
			mcptools.PlannedSlot{Date: date, Slot: "snack", Recipe: fmt.Sprintf("snack-%d", i), Title: fmt.Sprintf("Snack %d", i), Score: 0.7},
		)
	}
	planner := &fakePlanner{dinners: slots}
	reactions := &fakeReactions{}
	deps := mcptools.Dependencies{Planner: planner, Reactions: reactions}

	cs := connectClient(t, startServer(t, deps))

	// Request a 7-day plan with all slot kinds.
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "list_recipe_candidates",
		Arguments: map[string]any{
			"date":  "2026-08-17",
			"days":  7,
			"slots": []string{"dinner", "breakfast", "snack"},
		},
	})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}
	got := structured[[]mcptools.PlannedSlot](t, res)
	if len(got) != 21 {
		t.Fatalf("expected 21 slots (7 days x 3 kinds), got %d", len(got))
	}

	// Verify each date has all three slot kinds.
	for i := 0; i < 7; i++ {
		date := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i).Format("2006-01-02")
		for _, kind := range []string{"dinner", "breakfast", "snack"} {
			found := false
			for _, s := range got {
				if s.Date == date && s.Slot == kind {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("missing %s slot for %s", kind, date)
			}
		}
	}

	// Record a breakfast reaction and verify the slot is passed through.
	res2, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "record_meal_reaction",
		Arguments: map[string]any{
			"recipe": "breakfast-0", "served_on": "2026-08-17", "person_id": "p1", "sentiment": 1, "slot": "breakfast",
		},
	})
	if err != nil {
		t.Fatalf("call record_meal_reaction: %v", err)
	}
	if res2.IsError {
		t.Fatalf("unexpected tool error: %+v", res2)
	}
	if reactions.calls != 1 {
		t.Fatalf("fake reactions called %d times, want 1", reactions.calls)
	}
	if reactions.last.Slot != "breakfast" {
		t.Errorf("slot = %q, want breakfast", reactions.last.Slot)
	}
}
