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

type fakeDiscovery struct {
	calls       int
	lastURL     string
	lastStatus  *string
	lastID      string
	lastFamily  *string
	candidate   mcptools.ImportCandidate
	candidates  []mcptools.ImportCandidate
	promoteRes  mcptools.PromoteCandidateResult
}

func (f *fakeDiscovery) DiscoverFromURL(_ context.Context, in mcptools.DiscoverRecipeInput) (mcptools.ImportCandidate, error) {
	f.calls++
	f.lastURL = in.URL
	return f.candidate, nil
}

func (f *fakeDiscovery) ListCandidates(_ context.Context, status *string) ([]mcptools.ImportCandidate, error) {
	f.calls++
	f.lastStatus = status
	return f.candidates, nil
}

func (f *fakeDiscovery) GetCandidate(_ context.Context, id string) (mcptools.ImportCandidate, error) {
	f.calls++
	f.lastID = id
	return f.candidate, nil
}

func (f *fakeDiscovery) RejectCandidate(_ context.Context, id string) error {
	f.calls++
	f.lastID = id
	return nil
}

func (f *fakeDiscovery) PromoteCandidate(_ context.Context, id string, familyID *string) (mcptools.PromoteCandidateResult, error) {
	f.calls++
	f.lastID = id
	f.lastFamily = familyID
	return f.promoteRes, nil
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

// TestIntegration_DiscoveryTools drives the five recipe-discovery tools end-to-end
// over Streamable HTTP via the SDK client: list the tools, discover a URL, and
// promote a candidate, asserting both the responses and the side effects on the
// fake service.
func TestIntegration_DiscoveryTools(t *testing.T) {
	candidate := mcptools.ImportCandidate{
		ID: "cand-1", SourceID: "web-jsonld", SourceURL: "https://example.com/recipe",
		Title: "Pasta", Status: "candidate", ImportedAt: time.Now(),
		Ingredients: []mcptools.DiscoveryIngredient{{LineNo: 1, RawText: "200 g spaghetti", Quantity: ptr(200.0), Unit: "g", NeedsReview: false}},
	}
	discovery := &fakeDiscovery{
		candidate:  candidate,
		candidates: []mcptools.ImportCandidate{candidate},
		promoteRes: mcptools.PromoteCandidateResult{FamilyID: "fam-1", VariantID: "var-1", RevisionID: "rev-1", CandidateStatus: "promoted"},
	}
	deps := mcptools.Dependencies{Discovery: discovery}

	cs := connectClient(t, startServer(t, deps))

	list, err := cs.ListTools(context.Background(), &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	names := toolNames(list.Tools)
	for _, want := range []string{"discover_recipe", "list_import_candidates", "get_import_candidate", "reject_import_candidate", "promote_import_candidate"} {
		if !contains(names, want) {
			t.Fatalf("expected tool %q to be registered, got %v", want, names)
		}
	}

	// discover_recipe stages a URL and returns the candidate.
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "discover_recipe",
		Arguments: map[string]any{"url": "https://example.com/recipe"},
	})
	if err != nil {
		t.Fatalf("call discover_recipe: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}
	got := structured[mcptools.ImportCandidate](t, res)
	if got.ID != "cand-1" || got.Title != "Pasta" {
		t.Fatalf("unexpected discover result: %+v", got)
	}
	if discovery.lastURL != "https://example.com/recipe" {
		t.Fatalf("fake discovery lastURL = %q, want the requested URL", discovery.lastURL)
	}

	// list_import_candidates with a status filter passes the filter through.
	res, err = cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "list_import_candidates",
		Arguments: map[string]any{"status": "candidate"},
	})
	if err != nil {
		t.Fatalf("call list_import_candidates: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}
	gotList := structured[[]mcptools.ImportCandidate](t, res)
	if len(gotList) != 1 || gotList[0].ID != "cand-1" {
		t.Fatalf("unexpected list result: %+v", gotList)
	}
	if discovery.lastStatus == nil || *discovery.lastStatus != "candidate" {
		t.Fatalf("fake discovery lastStatus = %v, want candidate", discovery.lastStatus)
	}

	// reject_import_candidate marks the candidate rejected.
	res, err = cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "reject_import_candidate",
		Arguments: map[string]any{"id": "cand-1"},
	})
	if err != nil {
		t.Fatalf("call reject_import_candidate: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}
	gotReject := structured[mcptools.RejectCandidateResult](t, res)
	if gotReject.ID != "cand-1" || gotReject.Status != "rejected" {
		t.Fatalf("unexpected reject result: %+v", gotReject)
	}

	// promote_import_candidate promotes the candidate and returns the family/variant/revision.
	res, err = cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "promote_import_candidate",
		Arguments: map[string]any{"id": "cand-1"},
	})
	if err != nil {
		t.Fatalf("call promote_import_candidate: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}
	gotPromote := structured[mcptools.PromoteCandidateResult](t, res)
	if gotPromote.FamilyID != "fam-1" || gotPromote.VariantID != "var-1" || gotPromote.RevisionID != "rev-1" || gotPromote.CandidateStatus != "promoted" {
		t.Fatalf("unexpected promote result: %+v", gotPromote)
	}
	if discovery.lastFamily != nil {
		t.Fatalf("fake discovery lastFamily = %v, want nil (no family_id given)", discovery.lastFamily)
	}
}

func ptr(v float64) *float64 { return &v }

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
