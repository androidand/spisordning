package persistence

import (
	"context"
	"testing"
	"time"

	"github.com/androidand/spisordning/internal/domain"
)

// TestImportCandidate_UpsertByExternalID verifies that staging the same
// (source_id, external_id) twice upserts the existing row rather than inserting
// a new one: the row count stays at one, the title is refreshed, and the
// original candidate id is preserved (and returned) across the upsert.
func TestImportCandidate_UpsertByExternalID(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "recipe_import_candidate_ingredient", "recipe_import_candidate", "external_recipe_source")

	if err := s.UpsertExternalRecipeSource(ctx, ExternalRecipeSource{
		ID: "web-jsonld", Name: "Web JSON-LD", Kind: "jsonld_web",
		Decision: "integrate_now", Enabled: true,
	}); err != nil {
		t.Fatalf("UpsertExternalRecipeSource: %v", err)
	}

	externalID := "abc123"
	first := ImportCandidate{
		SourceID: "web-jsonld", SourceURL: "https://example.com/a",
		ExternalID: &externalID, Title: "First", Status: "candidate",
		RawJSONLD: []byte(`{"@type":"Recipe"}`), ImportedAt: time.Now(),
	}
	id1, err := s.SaveImportCandidate(ctx, first)
	if err != nil {
		t.Fatalf("SaveImportCandidate (first): %v", err)
	}
	if id1 == (domain.RecipeImportCandidateID{}) {
		t.Fatal("expected non-empty id from first SaveImportCandidate")
	}

	// Same (source_id, external_id) with a fresh (zero) id and a new title:
	// must update the existing row in place and return its original id.
	second := ImportCandidate{
		SourceID: "web-jsonld", SourceURL: "https://example.com/a",
		ExternalID: &externalID, Title: "Updated", Status: "candidate",
		RawJSONLD: []byte(`{"@type":"Recipe"}`), ImportedAt: time.Now(),
	}
	id2, err := s.SaveImportCandidate(ctx, second)
	if err != nil {
		t.Fatalf("SaveImportCandidate (second): %v", err)
	}
	if id2 != id1 {
		t.Fatalf("second SaveImportCandidate returned %s, want %s (existing row's id on upsert)", id2, id1)
	}

	all, err := s.ListImportCandidates(ctx, nil)
	if err != nil {
		t.Fatalf("ListImportCandidates: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("candidates = %d, want 1 (upsert by external_id)", len(all))
	}
	if all[0].Title != "Updated" {
		t.Fatalf("title = %q, want %q", all[0].Title, "Updated")
	}
	if all[0].ID != id1 {
		t.Fatalf("id = %s, want %s (original id preserved across upsert)", all[0].ID, id1)
	}
}

// TestImportCandidate_UpsertByURL verifies that a candidate staged without an
// external_id is keyed by source_url: re-staging the same URL upserts the
// existing row instead of inserting a duplicate, returning the same id.
func TestImportCandidate_UpsertByURL(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "recipe_import_candidate_ingredient", "recipe_import_candidate", "external_recipe_source")

	if err := s.UpsertExternalRecipeSource(ctx, ExternalRecipeSource{
		ID: "web-jsonld", Name: "Web JSON-LD", Kind: "jsonld_web",
		Decision: "integrate_now", Enabled: true,
	}); err != nil {
		t.Fatalf("UpsertExternalRecipeSource: %v", err)
	}

	first := ImportCandidate{
		SourceID: "web-jsonld", SourceURL: "https://example.com/same-url",
		Title: "First", Status: "candidate",
		RawJSONLD: []byte(`{"@type":"Recipe"}`), ImportedAt: time.Now(),
	}
	id1, err := s.SaveImportCandidate(ctx, first)
	if err != nil {
		t.Fatalf("SaveImportCandidate (first): %v", err)
	}
	if id1 == (domain.RecipeImportCandidateID{}) {
		t.Fatal("expected non-empty id from first SaveImportCandidate")
	}

	second := ImportCandidate{
		SourceID: "web-jsonld", SourceURL: "https://example.com/same-url",
		Title: "Updated", Status: "candidate",
		RawJSONLD: []byte(`{"@type":"Recipe"}`), ImportedAt: time.Now(),
	}
	id2, err := s.SaveImportCandidate(ctx, second)
	if err != nil {
		t.Fatalf("SaveImportCandidate (second): %v", err)
	}
	if id2 != id1 {
		t.Fatalf("second SaveImportCandidate returned %s, want %s (existing row's id on upsert)", id2, id1)
	}

	all, err := s.ListImportCandidates(ctx, nil)
	if err != nil {
		t.Fatalf("ListImportCandidates: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("candidates = %d, want 1 (upsert by source_url)", len(all))
	}
	if all[0].Title != "Updated" {
		t.Fatalf("title = %q, want %q", all[0].Title, "Updated")
	}
	if all[0].ID != id1 {
		t.Fatalf("id = %s, want %s (original id preserved across upsert)", all[0].ID, id1)
	}
}

// TestImportCandidate_IngredientsReplaced verifies that SaveCandidateIngredients
// replaces the full set of lines for a candidate: the final set matches exactly
// what was last saved, in line_no order.
func TestImportCandidate_IngredientsReplaced(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "recipe_import_candidate_ingredient", "recipe_import_candidate", "external_recipe_source")

	if err := s.UpsertExternalRecipeSource(ctx, ExternalRecipeSource{
		ID: "web-jsonld", Name: "Web JSON-LD", Kind: "jsonld_web",
		Decision: "integrate_now", Enabled: true,
	}); err != nil {
		t.Fatalf("UpsertExternalRecipeSource: %v", err)
	}

	cid := domain.NewRecipeImportCandidateID()
	savedID, err := s.SaveImportCandidate(ctx, ImportCandidate{
		ID: cid, SourceID: "web-jsonld", SourceURL: "https://example.com/ings",
		Title: "Ings", Status: "candidate",
		RawJSONLD: []byte(`{"@type":"Recipe"}`), ImportedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("SaveImportCandidate: %v", err)
	}
	if savedID != cid {
		t.Fatalf("SaveImportCandidate returned %s, want %s (explicit id preserved)", savedID, cid)
	}

	q1 := 1.0
	q2 := 2.0
	if err := s.SaveCandidateIngredients(ctx, cid, []ImportCandidateIngredient{
		{CandidateID: cid, LineNo: 1, RawText: "1 dl mjölk", Quantity: &q1, Unit: "dl", NeedsReview: true},
		{CandidateID: cid, LineNo: 2, RawText: "2 ägg", Quantity: &q2, NeedsReview: true},
	}); err != nil {
		t.Fatalf("SaveCandidateIngredients (two): %v", err)
	}
	lines, err := s.ListCandidateIngredients(ctx, cid)
	if err != nil {
		t.Fatalf("ListCandidateIngredients: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("ingredients = %d, want 2", len(lines))
	}

	// Re-save a single line: the previous two are replaced, not appended.
	q3 := 3.0
	if err := s.SaveCandidateIngredients(ctx, cid, []ImportCandidateIngredient{
		{CandidateID: cid, LineNo: 1, RawText: "3 msk smör", Quantity: &q3, Unit: "msk", NeedsReview: true},
	}); err != nil {
		t.Fatalf("SaveCandidateIngredients (one): %v", err)
	}
	lines, err = s.ListCandidateIngredients(ctx, cid)
	if err != nil {
		t.Fatalf("ListCandidateIngredients: %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("ingredients = %d, want 1 (replaced)", len(lines))
	}
	if lines[0].RawText != "3 msk smör" {
		t.Fatalf("raw_text = %q, want %q", lines[0].RawText, "3 msk smör")
	}
}
