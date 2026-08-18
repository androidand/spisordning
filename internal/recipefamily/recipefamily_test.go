package recipefamily

import (
	"reflect"
	"testing"

	"github.com/androidand/spisordning/internal/domain"
)

// contains reports whether ids contains v.
func contains(ids []int64, v int64) bool {
	for _, id := range ids {
		if id == v {
			return true
		}
	}
	return false
}

// assertSingleStringField asserts that typ has a field named name whose type is a
// single string (not a slice/set). This is how the "belongs to exactly one"
// invariants are encoded structurally: a variant has one FamilyID, a revision has
// one VariantID.
func assertSingleStringField(t *testing.T, typ reflect.Type, name string) {
	t.Helper()
	f, ok := typ.FieldByName(name)
	if !ok {
		t.Fatalf("%s has no field %q", typ.Name(), name)
	}
	if f.Type.Kind() != reflect.String {
		t.Fatalf("%s.%s is %s, want a single string (exactly one)", typ.Name(), name, f.Type)
	}
}

// TestRevisionHasNoUpdatePath encodes "a RecipeRevision is immutable once
// created": the type exposes no exported method, so there is no update path. The
// only way to change a revision's content is to create a new Revision.
func TestRevisionHasNoUpdatePath(t *testing.T) {
	typ := reflect.TypeOf(&Revision{})
	for i := 0; i < typ.NumMethod(); i++ {
		if m := typ.Method(i); m.IsExported() {
			t.Fatalf("Revision has an exported method %q; a revision must have no update path", m.Name)
		}
	}
}

// TestCorrectionCreatesNewRevision is the spec scenario "Correcting a recipe
// creates a new revision": a correction is a new revision whose parent is the
// original; the original's stored content is unchanged.
func TestCorrectionCreatesNewRevision(t *testing.T) {
	a1 := Revision{ID: 1, VariantID: "v1", Ingredients: []domain.Ingredient{{RawText: "300 ml cream"}}, Steps: []string{"boil"}}
	a2 := Revision{ID: 2, VariantID: "v1", Ingredients: []domain.Ingredient{{RawText: "200 ml cream"}}, Steps: []string{"boil"}}

	g := NewGraph()
	if err := g.AddEdge(2, 1); err != nil {
		t.Fatalf("AddEdge(A2, A1): %v", err)
	}
	if a1.Ingredients[0].RawText != "300 ml cream" {
		t.Fatalf("original revision mutated: got %q, want %q", a1.Ingredients[0].RawText, "300 ml cream")
	}
	if a2.Ingredients[0].RawText != "200 ml cream" {
		t.Fatalf("corrected revision wrong: got %q, want %q", a2.Ingredients[0].RawText, "200 ml cream")
	}
	if !contains(g.Ancestors(2), 1) {
		t.Fatalf("A2 should derive from A1; ancestors(A2)=%v", g.Ancestors(2))
	}
}

// TestVariantBelongsToOneFamily encodes "a RecipeVariant belongs to exactly one
// RecipeFamily": Variant.FamilyID is a single string by construction.
func TestVariantBelongsToOneFamily(t *testing.T) {
	assertSingleStringField(t, reflect.TypeOf(Variant{}), "FamilyID")
}

// TestRevisionBelongsToOneVariant encodes "a RecipeRevision belongs to exactly
// one RecipeVariant": Revision.VariantID is a single string by construction.
func TestRevisionBelongsToOneVariant(t *testing.T) {
	assertSingleStringField(t, reflect.TypeOf(Revision{}), "VariantID")
}

// TestCycleRejection encodes "revision parentage never cycles": an edge that
// would make a revision its own ancestor is rejected, as is a self-edge.
func TestCycleRejection(t *testing.T) {
	g := NewGraph()
	if err := g.AddEdge(2, 1); err != nil { // 2 derives from 1
		t.Fatalf("AddEdge(2,1): %v", err)
	}
	if err := g.AddEdge(3, 2); err != nil { // 3 derives from 2
		t.Fatalf("AddEdge(3,2): %v", err)
	}
	// 1 deriving from 3 would create the cycle 1 -> 3 -> 2 -> 1.
	if err := g.AddEdge(1, 3); err == nil {
		t.Fatal("expected cycle rejection for AddEdge(1,3)")
	}
	if err := g.AddEdge(1, 1); err == nil {
		t.Fatal("expected self-edge rejection for AddEdge(1,1)")
	}
	// The graph is unchanged by the rejected edges.
	if !reflect.DeepEqual(g.Ancestors(3), []int64{1, 2}) {
		t.Fatalf("ancestors(3) after rejected edges = %v, want [1 2]", g.Ancestors(3))
	}
}

// TestMultiParentMergeLineage encodes the merge mechanism: a revision may have
// two parents, and its ancestor set is the union of both lines (deduplicated).
func TestMultiParentMergeLineage(t *testing.T) {
	g := NewGraph()
	if err := g.AddEdge(2, 1); err != nil { // line A: 2 -> 1
		t.Fatalf("AddEdge(2,1): %v", err)
	}
	if err := g.AddEdge(3, 1); err != nil { // line B: 3 -> 1 (branch)
		t.Fatalf("AddEdge(3,1): %v", err)
	}
	if err := g.AddEdge(4, 2); err != nil { // merge: 4 derives from 2 and 3
		t.Fatalf("AddEdge(4,2): %v", err)
	}
	if err := g.AddEdge(4, 3); err != nil {
		t.Fatalf("AddEdge(4,3): %v", err)
	}
	if !contains(g.Parents(4), 2) || !contains(g.Parents(4), 3) {
		t.Fatalf("parents(4) = %v, want both 2 and 3", g.Parents(4))
	}
	if !reflect.DeepEqual(g.Ancestors(4), []int64{1, 2, 3}) {
		t.Fatalf("ancestors(4) = %v, want [1 2 3]", g.Ancestors(4))
	}
}

// TestParentsReturnsCopy confirms Parents hands out a copy, not the internal
// slice: mutating the returned slice must never change the graph.
func TestParentsReturnsCopy(t *testing.T) {
	g := NewGraph()
	if err := g.AddEdge(2, 1); err != nil {
		t.Fatalf("AddEdge(2,1): %v", err)
	}
	got := g.Parents(2)
	got[0] = 99
	if g.Parents(2)[0] != 1 {
		t.Fatalf("mutating the returned Parents slice mutated the graph: %v", g.Parents(2))
	}
}

// TestKorvstroganoffWorkedExample reproduces the design.md Step 7 tree and the
// Visual Recipe Family Requirement UI projection: one family, four variants,
// Andreas' three-revision history, a cross-variant fork (A3 -> C1), and the
// default-variant pin.
func TestKorvstroganoffWorkedExample(t *testing.T) {
	f := Family{ID: "F1", Name: "Korvstroganoff", DefaultVariantID: "V1"}
	variants := []Variant{
		{ID: "V1", FamilyID: "F1", Title: "Andreas version", SourceAttribution: "household"},
		{ID: "V2", FamilyID: "F1", Title: "ICA version", SourceAttribution: "ICA Kök"},
		{ID: "V3", FamilyID: "F1", Title: "Köket version", SourceAttribution: "Köket"},
		{ID: "V4", FamilyID: "F1", Title: "Child-friendly", SourceAttribution: "household"},
	}

	g := NewGraph()
	// Revision IDs: A1=1, A2=2, A3=3 (V1), C1=4 (V4). I1/K1 have no parents.
	for _, e := range [][2]int64{{2, 1}, {3, 2}, {4, 3}} { // A1->A2->A3, A3->C1
		if err := g.AddEdge(e[0], e[1]); err != nil {
			t.Fatalf("AddEdge(%d,%d): %v", e[0], e[1], err)
		}
	}

	// The cross-variant fork: C1 (V4) derives from A3 (V1). The DAG is over
	// revisions, not variants — C1's full history spans the fork.
	if !reflect.DeepEqual(g.Ancestors(4), []int64{1, 2, 3}) {
		t.Fatalf("ancestors(C1) = %v, want [1 2 3]", g.Ancestors(4))
	}
	if !reflect.DeepEqual(g.Parents(4), []int64{3}) {
		t.Fatalf("parents(C1) = %v, want [3] (A3)", g.Parents(4))
	}

	// UI projection: default variant expanded, the rest listed as alternates.
	view := BuildFamilyView(f, variants)
	if view.FamilyName != "Korvstroganoff" {
		t.Fatalf("FamilyName = %q, want %q", view.FamilyName, "Korvstroganoff")
	}
	if view.DefaultTitle != "Andreas version" {
		t.Fatalf("DefaultTitle = %q, want %q", view.DefaultTitle, "Andreas version")
	}
	want := []string{"Child-friendly", "ICA version", "Köket version"}
	if !reflect.DeepEqual(view.Alternates, want) {
		t.Fatalf("Alternates = %v, want %v", view.Alternates, want)
	}
}

// TestBuildFamilyViewFiltersByFamily confirms the view only lists variants of
// the given family, so a variant from another family never leaks into the list.
func TestBuildFamilyViewFiltersByFamily(t *testing.T) {
	f := Family{ID: "F1", Name: "Korvstroganoff", DefaultVariantID: "V1"}
	variants := []Variant{
		{ID: "V1", FamilyID: "F1", Title: "Andreas version"},
		{ID: "X1", FamilyID: "F2", Title: "Other family variant"},
	}
	view := BuildFamilyView(f, variants)
	if containsStrings(view.Alternates, "Other family variant") {
		t.Fatalf("variant from another family leaked into the view: %v", view.Alternates)
	}
	if len(view.Alternates) != 0 {
		t.Fatalf("expected no alternates, got %v", view.Alternates)
	}
}

func containsStrings(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}
