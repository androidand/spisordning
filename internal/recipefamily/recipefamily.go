// Package recipefamily is the in-memory, persistence-free domain core for the
// git-like recipe hierarchy (see
// openspec/changes/implement-recipe-family-and-revisions/design.md):
// RecipeFamily → RecipeVariant → RecipeRevision, with revision parentage held as
// a DAG.
//
// The invariants it encodes, mirroring the spec and design.md:
//
//   - A RecipeRevision is immutable: the type has no update path; a correction
//     is a NEW Revision (new ID, the old one as a parent).
//   - A RecipeVariant belongs to exactly one Family (Variant.FamilyID is a
//     single field by construction).
//   - A RecipeRevision belongs to exactly one Variant (Revision.VariantID is a
//     single field by construction).
//   - Revision parentage never cycles: Graph.AddEdge rejects any edge that would
//     make a revision its own ancestor (the application-layer check the schema
//     cannot express).
//
// The package holds no transport or persistence concerns, so its invariants can
// be exercised as pure functions in unit tests.
package recipefamily

import (
	"fmt"
	"sort"
	"time"

	"github.com/androidand/spisordning/internal/domain"
)

// Family is a conceptual dish ("Korvstroganoff"). It groups the variants that are
// recognizably the same dish and pins which one is shown expanded by default.
type Family struct {
	ID          string
	Name        string
	Description string
	// DefaultVariantID is the manually-pinned expanded variant ("" when unset).
	// It is set only by an explicit command and is never derived from ratings or
	// any computed signal (design.md, "Decisions — default_variant_id").
	DefaultVariantID string
}

// Variant is one recognizable fork/style/source of a family ("Andreas version",
// "ICA version"). It is the unit a household cooks and rates. A variant belongs
// to exactly one family: FamilyID is a single field, so the "exactly one family"
// invariant holds by construction.
type Variant struct {
	ID                string
	FamilyID          string
	Title             string
	SourceAttribution string
	Archived          bool
}

// Revision is one immutable snapshot of a variant's content. It is the unit of
// history: a correction is a NEW Revision (new ID, old one as parent), never a
// mutation of an existing one. A revision belongs to exactly one variant:
// VariantID is a single field, so the "exactly one variant" invariant holds by
// construction. There is deliberately no method that mutates Ingredients or
// Steps after a Revision is created.
//
// ID is int64 to mirror recipe_revision.id BIGSERIAL; Family/Variant ids are
// string slugs (TEXT) because they are user-facing names, not sequences.
type Revision struct {
	ID          int64
	VariantID   string
	Servings    int
	Description string
	Ingredients []domain.Ingredient
	Steps       []string
	CreatedAt   time.Time
}

// Graph is the in-memory projection of the recipe_revision_parent edge table:
// each child revision ID maps to its parent revision IDs. It enforces the DAG
// invariant (no cycles) at edge-insertion time, mirroring the application-layer
// cycle check the schema cannot express.
type Graph struct {
	parents map[int64][]int64 // child -> parents
}

// NewGraph returns an empty lineage graph.
func NewGraph() *Graph {
	return &Graph{parents: make(map[int64][]int64)}
}

// AddEdge records that child was derived from parent. It is idempotent for an
// edge that already exists. It rejects a self-edge and any edge that would create
// a cycle (child already an ancestor of parent), so a Graph can never hold a
// cycle; in those cases it returns an error and leaves the graph unchanged.
func (g *Graph) AddEdge(child, parent int64) error {
	if child == parent {
		return fmt.Errorf("recipefamily: revision %d cannot be its own parent", child)
	}
	for _, p := range g.parents[child] {
		if p == parent {
			return nil // already recorded
		}
	}
	if g.isAncestor(child, parent) {
		return fmt.Errorf("recipefamily: making %d a parent of %d would create a cycle", parent, child)
	}
	g.parents[child] = append(g.parents[child], parent)
	return nil
}

// Parents returns the direct parent revision IDs of id (nil when it has none).
// The returned slice is a copy, so callers can never mutate the graph through
// it.
func (g *Graph) Parents(id int64) []int64 {
	return append([]int64(nil), g.parents[id]...)
}

// Ancestors returns every revision that id transitively derives from (its full
// history), deduplicated and sorted ascending for deterministic output. This is
// the in-memory form of the WITH RECURSIVE ancestor query over
// recipe_revision_parent.
func (g *Graph) Ancestors(id int64) []int64 {
	seen := make(map[int64]bool)
	var stack []int64
	stack = append(stack, g.parents[id]...)
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[n] {
			continue
		}
		seen[n] = true
		stack = append(stack, g.parents[n]...)
	}
	out := make([]int64, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// isAncestor reports whether candidate is an ancestor of id (id transitively
// derives from candidate).
func (g *Graph) isAncestor(candidate, id int64) bool {
	for _, a := range g.Ancestors(id) {
		if a == candidate {
			return true
		}
	}
	return false
}

// FamilyView is the read model behind the Visual Recipe Family Requirement UI:
// the family name, the title of the default (expanded) variant, and the titles
// of the remaining variants listed as alternates.
type FamilyView struct {
	FamilyName   string
	DefaultTitle string
	Alternates   []string
}

// BuildFamilyView projects a family and its variants into the shape the family
// view renders: the default variant expanded, the rest listed as alternates. It
// is the in-memory analog of the SQL in design.md's "Tracing the Visual Recipe
// Family Requirement UI". Variants are matched to the family by FamilyID; the
// alternates are ordered by title for a stable listing.
func BuildFamilyView(f Family, variants []Variant) FamilyView {
	view := FamilyView{FamilyName: f.Name, Alternates: []string{}}
	for _, v := range variants {
		if v.FamilyID != f.ID {
			continue
		}
		if v.ID == f.DefaultVariantID {
			view.DefaultTitle = v.Title
			continue
		}
		view.Alternates = append(view.Alternates, v.Title)
	}
	sort.Strings(view.Alternates)
	return view
}
