package persistence

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/androidand/spisordning/internal/domain"
)

func TestRecipeSourceRef_UpsertAndGetByFamily(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()

	famID := domain.NewRecipeFamilyID()
	if err := s.CreateRecipeFamily(ctx, RecipeFamily{ID: famID, Slug: "test-sr-1", Name: "Test SR 1"}); err != nil {
		t.Fatalf("create family: %v", err)
	}
	t.Cleanup(func() {
		s.db.Exec(context.Background(), `DELETE FROM recipe_source_ref WHERE recipe_family_id = $1`, famID)
		s.db.Exec(context.Background(), `DELETE FROM recipe_family WHERE id = $1`, famID)
	})

	ref := RecipeSourceRef{
		RecipeFamilyID: famID,
		Source:         "mealie",
		SourceRecipeID: "test-slug-1",
		ImportedBy:     "test",
	}
	if err := s.UpsertRecipeSourceRef(ctx, ref); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := s.GetRecipeSourceRefByFamily(ctx, famID)
	if err != nil {
		t.Fatalf("get by family: %v", err)
	}
	if got.Source != "mealie" || got.SourceRecipeID != "test-slug-1" {
		t.Fatalf("unexpected ref: %+v", got)
	}
	if got.ImportedBy != "test" {
		t.Fatalf("unexpected imported_by: %q", got.ImportedBy)
	}
}

func TestRecipeSourceRef_GetBySource(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()

	famID := domain.NewRecipeFamilyID()
	if err := s.CreateRecipeFamily(ctx, RecipeFamily{ID: famID, Slug: "test-sr-2", Name: "Test SR 2"}); err != nil {
		t.Fatalf("create family: %v", err)
	}
	t.Cleanup(func() {
		s.db.Exec(context.Background(), `DELETE FROM recipe_source_ref WHERE recipe_family_id = $1`, famID)
		s.db.Exec(context.Background(), `DELETE FROM recipe_family WHERE id = $1`, famID)
	})

	ref := RecipeSourceRef{
		RecipeFamilyID: famID,
		Source:         "mealie",
		SourceRecipeID: "test-slug-2",
	}
	if err := s.UpsertRecipeSourceRef(ctx, ref); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := s.GetRecipeSourceRefBySource(ctx, "mealie", "test-slug-2")
	if err != nil {
		t.Fatalf("get by source: %v", err)
	}
	if got.RecipeFamilyID != famID {
		t.Fatalf("unexpected family id: %v != %v", got.RecipeFamilyID, famID)
	}
}

func TestRecipeSourceRef_UniquePerFamily(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()

	famID := domain.NewRecipeFamilyID()
	if err := s.CreateRecipeFamily(ctx, RecipeFamily{ID: famID, Slug: "test-sr-3", Name: "Test SR 3"}); err != nil {
		t.Fatalf("create family: %v", err)
	}
	t.Cleanup(func() {
		s.db.Exec(context.Background(), `DELETE FROM recipe_source_ref WHERE recipe_family_id = $1`, famID)
		s.db.Exec(context.Background(), `DELETE FROM recipe_family WHERE id = $1`, famID)
	})

	ref1 := RecipeSourceRef{RecipeFamilyID: famID, Source: "mealie", SourceRecipeID: "slug-a"}
	if err := s.UpsertRecipeSourceRef(ctx, ref1); err != nil {
		t.Fatalf("upsert 1: %v", err)
	}

	// Upserting a different source for the same family should replace the old ref.
	ref2 := RecipeSourceRef{RecipeFamilyID: famID, Source: "structured", SourceRecipeID: "slug-b"}
	if err := s.UpsertRecipeSourceRef(ctx, ref2); err != nil {
		t.Fatalf("upsert 2: %v", err)
	}

	got, err := s.GetRecipeSourceRefByFamily(ctx, famID)
	if err != nil {
		t.Fatalf("get by family: %v", err)
	}
	if got.Source != "structured" || got.SourceRecipeID != "slug-b" {
		t.Fatalf("expected structured/slug-b, got %s/%s", got.Source, got.SourceRecipeID)
	}

	// The old source ref should be gone.
	_, err = s.GetRecipeSourceRefBySource(ctx, "mealie", "slug-a")
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected ErrNoRows for old ref, got %v", err)
	}
}

func TestRecipeSourceRef_UniquePerSource(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()

	fam1 := domain.NewRecipeFamilyID()
	fam2 := domain.NewRecipeFamilyID()
	if err := s.CreateRecipeFamily(ctx, RecipeFamily{ID: fam1, Slug: "test-sr-4a", Name: "Test SR 4a"}); err != nil {
		t.Fatalf("create family 1: %v", err)
	}
	if err := s.CreateRecipeFamily(ctx, RecipeFamily{ID: fam2, Slug: "test-sr-4b", Name: "Test SR 4b"}); err != nil {
		t.Fatalf("create family 2: %v", err)
	}
	t.Cleanup(func() {
		s.db.Exec(context.Background(), `DELETE FROM recipe_source_ref WHERE recipe_family_id IN ($1, $2)`, fam1, fam2)
		s.db.Exec(context.Background(), `DELETE FROM recipe_family WHERE id IN ($1, $2)`, fam1, fam2)
	})

	ref1 := RecipeSourceRef{RecipeFamilyID: fam1, Source: "mealie", SourceRecipeID: "dup-slug"}
	if err := s.UpsertRecipeSourceRef(ctx, ref1); err != nil {
		t.Fatalf("upsert 1: %v", err)
	}

	// A second family claiming the same source recipe should fail (UNIQUE constraint).
	ref2 := RecipeSourceRef{RecipeFamilyID: fam2, Source: "mealie", SourceRecipeID: "dup-slug"}
	if err := s.UpsertRecipeSourceRef(ctx, ref2); err == nil {
		t.Fatal("expected error for duplicate source_recipe_id, got nil")
	}
}

func TestRecipeSourceRef_ListUnmappedMealieRecipes(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()

	// Create two recipe_refs.
	if err := s.UpsertRecipeRef(ctx, RecipeRef{MealieRecipeID: "mapped-1", Title: "Mapped"}); err != nil {
		t.Fatalf("upsert ref 1: %v", err)
	}
	if err := s.UpsertRecipeRef(ctx, RecipeRef{MealieRecipeID: "unmapped-1", Title: "Unmapped"}); err != nil {
		t.Fatalf("upsert ref 2: %v", err)
	}
	t.Cleanup(func() {
		s.db.Exec(context.Background(), `DELETE FROM recipe_source_ref WHERE source_recipe_id IN ('mapped-1', 'unmapped-1')`)
		s.db.Exec(context.Background(), `DELETE FROM recipe_ref WHERE mealie_recipe_id IN ('mapped-1', 'unmapped-1')`)
	})

	// Map the first one.
	famID := domain.NewRecipeFamilyID()
	if err := s.CreateRecipeFamily(ctx, RecipeFamily{ID: famID, Slug: "test-sr-5", Name: "Test SR 5"}); err != nil {
		t.Fatalf("create family: %v", err)
	}
	t.Cleanup(func() {
		s.db.Exec(context.Background(), `DELETE FROM recipe_source_ref WHERE recipe_family_id = $1`, famID)
		s.db.Exec(context.Background(), `DELETE FROM recipe_family WHERE id = $1`, famID)
	})
	if err := s.UpsertRecipeSourceRef(ctx, RecipeSourceRef{
		RecipeFamilyID: famID, Source: "mealie", SourceRecipeID: "mapped-1",
	}); err != nil {
		t.Fatalf("upsert source ref: %v", err)
	}

	unmapped, err := s.ListUnmappedMealieRecipes(ctx)
	if err != nil {
		t.Fatalf("list unmapped: %v", err)
	}
	if len(unmapped) != 1 || unmapped[0] != "unmapped-1" {
		t.Fatalf("expected [unmapped-1], got %v", unmapped)
	}
}
