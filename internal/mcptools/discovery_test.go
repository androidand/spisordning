package mcptools_test

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/androidand/spisordning/internal/mcptools"
)

// fakeDiscovery implements mcptools.DiscoveryService for MCP tool tests.
type fakeDiscovery struct {
	discoverCalls int
	lastURL       string
	lastSourceID  string
	candidate     mcptools.ImportCandidate
	candidates    []mcptools.ImportCandidate
	listCalls     int
	lastStatus    *string
	getCalls      int
	lastGetID     string
	rejectCalls   int
	lastRejectID  string
	promoteCalls  int
	lastPromoteID string
	lastFamilyID  *string
	promoteResult mcptools.PromoteCandidateResult
	err           error
}

func (f *fakeDiscovery) DiscoverFromURL(_ context.Context, in mcptools.DiscoverRecipeInput) (mcptools.ImportCandidate, error) {
	f.discoverCalls++
	f.lastURL = in.URL
	f.lastSourceID = in.SourceID
	return f.candidate, f.err
}

func (f *fakeDiscovery) ListCandidates(_ context.Context, status *string) ([]mcptools.ImportCandidate, error) {
	f.listCalls++
	f.lastStatus = status
	return f.candidates, f.err
}

func (f *fakeDiscovery) GetCandidate(_ context.Context, id string) (mcptools.ImportCandidate, error) {
	f.getCalls++
	f.lastGetID = id
	return f.candidate, f.err
}

func (f *fakeDiscovery) RejectCandidate(_ context.Context, id string) error {
	f.rejectCalls++
	f.lastRejectID = id
	return f.err
}

func (f *fakeDiscovery) PromoteCandidate(_ context.Context, id string, familyID *string) (mcptools.PromoteCandidateResult, error) {
	f.promoteCalls++
	f.lastPromoteID = id
	f.lastFamilyID = familyID
	return f.promoteResult, f.err
}

func TestDiscoverRecipe(t *testing.T) {
	d := &fakeDiscovery{candidate: mcptools.ImportCandidate{ID: "cand-1", Title: "Köttfärssås", Status: "candidate"}}
	cs := connectServer(t, mcptools.Dependencies{Discovery: d})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "discover_recipe",
		Arguments: map[string]any{"url": "https://example.com/recipe", "source_id": "web-jsonld"},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}
	got := structured[mcptools.ImportCandidate](t, res)
	if got.ID != "cand-1" || got.Title != "Köttfärssås" {
		t.Fatalf("unexpected candidate: %+v", got)
	}
	if d.discoverCalls != 1 {
		t.Fatalf("discover called %d times, want 1", d.discoverCalls)
	}
	if d.lastURL != "https://example.com/recipe" || d.lastSourceID != "web-jsonld" {
		t.Fatalf("unexpected input passed: url=%q source_id=%q", d.lastURL, d.lastSourceID)
	}
}

func TestDiscoverRecipe_BlankURL(t *testing.T) {
	d := &fakeDiscovery{}
	cs := connectServer(t, mcptools.Dependencies{Discovery: d})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "discover_recipe",
		Arguments: map[string]any{"url": "   "},
	})
	// A blank url may be rejected at the protocol level (err) or as a tool
	// error (IsError); either way the service must not run.
	if err == nil && !res.IsError {
		t.Fatalf("expected rejection for blank url, got success: %+v", res)
	}
	if d.discoverCalls != 0 {
		t.Fatalf("discover called %d times, want 0", d.discoverCalls)
	}
}

func TestListImportCandidates(t *testing.T) {
	d := &fakeDiscovery{candidates: []mcptools.ImportCandidate{
		{ID: "c1", Title: "A", Status: "candidate"},
		{ID: "c2", Title: "B", Status: "promoted"},
	}}
	cs := connectServer(t, mcptools.Dependencies{Discovery: d})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "list_import_candidates",
		Arguments: map[string]any{"status": "candidate"},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}
	got := structured[[]mcptools.ImportCandidate](t, res)
	if len(got) != 2 || got[0].ID != "c1" || got[1].ID != "c2" {
		t.Fatalf("unexpected candidates: %+v", got)
	}
	if d.listCalls != 1 {
		t.Fatalf("list called %d times, want 1", d.listCalls)
	}
	if d.lastStatus == nil || *d.lastStatus != "candidate" {
		t.Fatalf("unexpected status passed: %v", d.lastStatus)
	}
}

func TestListImportCandidates_NoStatus(t *testing.T) {
	d := &fakeDiscovery{candidates: []mcptools.ImportCandidate{{ID: "c1", Status: "candidate"}}}
	cs := connectServer(t, mcptools.Dependencies{Discovery: d})

	if _, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_import_candidates"}); err != nil {
		t.Fatalf("call: %v", err)
	}
	if d.listCalls != 1 {
		t.Fatalf("list called %d times, want 1", d.listCalls)
	}
	if d.lastStatus != nil {
		t.Fatalf("expected nil status, got %q", *d.lastStatus)
	}
}

func TestGetImportCandidate(t *testing.T) {
	d := &fakeDiscovery{candidate: mcptools.ImportCandidate{ID: "cand-9", Title: "Gnocchi", Status: "candidate"}}
	cs := connectServer(t, mcptools.Dependencies{Discovery: d})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_import_candidate",
		Arguments: map[string]any{"id": "cand-9"},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}
	got := structured[mcptools.ImportCandidate](t, res)
	if got.ID != "cand-9" || got.Title != "Gnocchi" {
		t.Fatalf("unexpected candidate: %+v", got)
	}
	if d.getCalls != 1 || d.lastGetID != "cand-9" {
		t.Fatalf("unexpected get: calls=%d id=%q", d.getCalls, d.lastGetID)
	}
}

func TestGetImportCandidate_BlankID(t *testing.T) {
	d := &fakeDiscovery{}
	cs := connectServer(t, mcptools.Dependencies{Discovery: d})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_import_candidate",
		Arguments: map[string]any{"id": "   "},
	})
	if err == nil && !res.IsError {
		t.Fatalf("expected rejection for blank id, got success: %+v", res)
	}
	if d.getCalls != 0 {
		t.Fatalf("get called %d times, want 0", d.getCalls)
	}
}

func TestRejectImportCandidate(t *testing.T) {
	d := &fakeDiscovery{}
	cs := connectServer(t, mcptools.Dependencies{Discovery: d})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "reject_import_candidate",
		Arguments: map[string]any{"id": "cand-3"},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}
	got := structured[mcptools.RejectCandidateResult](t, res)
	if got.ID != "cand-3" || got.Status != "rejected" {
		t.Fatalf("unexpected result: %+v", got)
	}
	if d.rejectCalls != 1 || d.lastRejectID != "cand-3" {
		t.Fatalf("unexpected reject: calls=%d id=%q", d.rejectCalls, d.lastRejectID)
	}
}

func TestRejectImportCandidate_BlankID(t *testing.T) {
	d := &fakeDiscovery{}
	cs := connectServer(t, mcptools.Dependencies{Discovery: d})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "reject_import_candidate",
		Arguments: map[string]any{"id": "   "},
	})
	if err == nil && !res.IsError {
		t.Fatalf("expected rejection for blank id, got success: %+v", res)
	}
	if d.rejectCalls != 0 {
		t.Fatalf("reject called %d times, want 0", d.rejectCalls)
	}
}

func TestPromoteImportCandidate(t *testing.T) {
	d := &fakeDiscovery{promoteResult: mcptools.PromoteCandidateResult{
		FamilyID: "fam-1", VariantID: "var-1", RevisionID: "rev-1", CandidateStatus: "promoted",
	}}
	cs := connectServer(t, mcptools.Dependencies{Discovery: d})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "promote_import_candidate",
		Arguments: map[string]any{"id": "cand-5", "family_id": "fam-1"},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}
	got := structured[mcptools.PromoteCandidateResult](t, res)
	if got.FamilyID != "fam-1" || got.VariantID != "var-1" || got.RevisionID != "rev-1" || got.CandidateStatus != "promoted" {
		t.Fatalf("unexpected result: %+v", got)
	}
	if d.promoteCalls != 1 || d.lastPromoteID != "cand-5" {
		t.Fatalf("unexpected promote: calls=%d id=%q", d.promoteCalls, d.lastPromoteID)
	}
	if d.lastFamilyID == nil || *d.lastFamilyID != "fam-1" {
		t.Fatalf("expected family_id fam-1, got %v", d.lastFamilyID)
	}
}

func TestPromoteImportCandidate_NoFamily(t *testing.T) {
	d := &fakeDiscovery{promoteResult: mcptools.PromoteCandidateResult{
		FamilyID: "fam-new", VariantID: "var-new", RevisionID: "rev-new", CandidateStatus: "promoted",
	}}
	cs := connectServer(t, mcptools.Dependencies{Discovery: d})

	if _, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "promote_import_candidate",
		Arguments: map[string]any{"id": "cand-6"},
	}); err != nil {
		t.Fatalf("call: %v", err)
	}
	if d.promoteCalls != 1 {
		t.Fatalf("promote called %d times, want 1", d.promoteCalls)
	}
	if d.lastFamilyID != nil {
		t.Fatalf("expected nil family_id, got %q", *d.lastFamilyID)
	}
}

func TestPromoteImportCandidate_BlankID(t *testing.T) {
	d := &fakeDiscovery{}
	cs := connectServer(t, mcptools.Dependencies{Discovery: d})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "promote_import_candidate",
		Arguments: map[string]any{"id": "   "},
	})
	if err == nil && !res.IsError {
		t.Fatalf("expected rejection for blank id, got success: %+v", res)
	}
	if d.promoteCalls != 0 {
		t.Fatalf("promote called %d times, want 0", d.promoteCalls)
	}
}

func TestDiscoveryTools_Registered(t *testing.T) {
	cs := connectServer(t, mcptools.Dependencies{Discovery: &fakeDiscovery{}})
	res, err := cs.ListTools(context.Background(), &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	names := map[string]bool{}
	for _, tool := range res.Tools {
		names[tool.Name] = true
	}
	for _, want := range []string{
		"discover_recipe", "list_import_candidates", "get_import_candidate",
		"reject_import_candidate", "promote_import_candidate",
	} {
		if !names[want] {
			t.Fatalf("missing discovery tool %q; got %v", want, names)
		}
	}
}

func TestDiscoveryTools_NilOmitted(t *testing.T) {
	cs := connectServer(t, mcptools.Dependencies{Planner: &fakePlanner{}})
	res, err := cs.ListTools(context.Background(), &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	for _, tool := range res.Tools {
		switch tool.Name {
		case "discover_recipe", "list_import_candidates", "get_import_candidate",
			"reject_import_candidate", "promote_import_candidate":
			t.Fatalf("unexpected discovery tool %q registered for a nil service", tool.Name)
		}
	}
}
