package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/androidand/spisordning/internal/mealie"
	"github.com/androidand/spisordning/internal/persistence"
)

// ImportMealieRecipe imports a single Mealie recipe into the native
// recipe_family hierarchy (design.md D3). It is idempotent: re-running for
// the same Mealie slug returns the existing family id when the source ref
// already exists.
//
// The import creates:
//   - a recipe_family (slug = Mealie slug; conflict policy links to existing)
//   - a default recipe_variant (source_attribution = "mealie:<slug>")
//   - one recipe_revision carrying ingredients
//   - a recipe_source_ref row (source="mealie", source_recipe_id=Mealie slug)
func (s *Recipes) ImportMealieRecipe(ctx context.Context, mealieRecipeID string) (domain.RecipeFamilyID, error) {
	// Check if already imported.
	if ref, err := s.db.GetRecipeSourceRefBySource(ctx, "mealie", mealieRecipeID); err == nil {
		return ref.RecipeFamilyID, nil
	} else if !isNoRows(err) {
		return domain.RecipeFamilyID{}, fmt.Errorf("import: lookup source ref: %w", err)
	}

	// Fetch the Mealie recipe content.
	if s.mealie == nil {
		return domain.RecipeFamilyID{}, fmt.Errorf("import: no Mealie client configured")
	}
	refs, err := s.mealie.SyncRecipes(ctx)
	if err != nil {
		return domain.RecipeFamilyID{}, fmt.Errorf("import: sync mealie: %w", err)
	}
	var mr *mealie.RecipeRef
	for i := range refs {
		if refs[i].MealieRecipeID == mealieRecipeID || refs[i].Slug == mealieRecipeID {
			mr = &refs[i]
			break
		}
	}
	if mr == nil {
		return domain.RecipeFamilyID{}, fmt.Errorf("import: mealie recipe %q not found", mealieRecipeID)
	}

	// Conflict policy: if a native family with the same slug exists, link to it.
	slug := mr.Slug
	if slug == "" {
		slug = mealieRecipeID
	}
	family, err := s.db.GetRecipeFamilyBySlug(ctx, slug)
	if err != nil && !isNoRows(err) {
		return domain.RecipeFamilyID{}, fmt.Errorf("import: lookup family by slug: %w", err)
	}
	if isNoRows(err) {
		family = persistence.RecipeFamily{
			ID:   domain.NewRecipeFamilyID(),
			Slug: slug,
			Name: mr.Title,
		}
		if err := s.db.CreateRecipeFamily(ctx, family); err != nil {
			return domain.RecipeFamilyID{}, fmt.Errorf("import: create family %q: %w", slug, err)
		}
	}

	// Create the default variant with provenance marker.
	variant := persistence.RecipeVariant{
		ID:                domain.NewRecipeVariantID(),
		Slug:              slug + "-default",
		FamilyID:          family.ID,
		Title:             mr.Title,
		SourceAttribution: "mealie:" + mealieRecipeID,
	}
	if err := s.db.CreateRecipeVariant(ctx, variant); err != nil {
		return domain.RecipeFamilyID{}, fmt.Errorf("import: create variant for %q: %w", slug, err)
	}

	// Build the revision from Mealie content.
	ingredients := make([]domain.Ingredient, 0, len(mr.Ingredients))
	for _, ing := range mr.Ingredients {
		name := strings.TrimSpace(ing.FoodName)
		if name == "" {
			name = strings.TrimSpace(ing.Note)
		}
		if name == "" {
			continue
		}
		ingredients = append(ingredients, domain.Ingredient{
			IngredientID: domain.CanonicalIngredientID(name),
			Quantity:     ing.Quantity,
			Unit:         ing.Unit,
			RawText:      ing.Note,
		})
	}

	revision := persistence.RecipeRevision{
		ID:          domain.NewRecipeRevisionID(),
		VariantID:   variant.ID,
		Ingredients: ingredients,
	}
	if _, err := s.db.CreateRecipeRevision(ctx, revision); err != nil {
		return domain.RecipeFamilyID{}, fmt.Errorf("import: create revision for %q: %w", slug, err)
	}

	// Pin the default variant.
	if err := s.db.SetRecipeFamilyDefaultVariant(ctx, family.ID, variant.ID); err != nil {
		return domain.RecipeFamilyID{}, fmt.Errorf("import: set default variant for %q: %w", slug, err)
	}

	// Register the source ref.
	ref := persistence.RecipeSourceRef{
		RecipeFamilyID: family.ID,
		Source:         "mealie",
		SourceRecipeID: mealieRecipeID,
		ImportedBy:     "import-recipes",
	}
	if err := s.db.UpsertRecipeSourceRef(ctx, ref); err != nil {
		return domain.RecipeFamilyID{}, fmt.Errorf("import: upsert source ref for %q: %w", mealieRecipeID, err)
	}

	return family.ID, nil
}

// ImportAllMealieRecipes imports every Mealie recipe that doesn't yet have a
// source ref. Returns the number of recipes imported.
func (s *Recipes) ImportAllMealieRecipes(ctx context.Context) (int, error) {
	if s.mealie == nil {
		return 0, fmt.Errorf("import: no Mealie client configured")
	}
	unmapped, err := s.db.ListUnmappedMealieRecipes(ctx)
	if err != nil {
		return 0, fmt.Errorf("import: list unmapped: %w", err)
	}
	imported := 0
	for _, slug := range unmapped {
		if _, err := s.ImportMealieRecipe(ctx, slug); err != nil {
			return imported, fmt.Errorf("import: %q: %w", slug, err)
		}
		imported++
	}
	return imported, nil
}

// isNoRows reports whether err is a "not found" error from the persistence layer.
func isNoRows(err error) bool {
	if err == nil {
		return false
	}
	return err == persistence.ErrNoRows || strings.Contains(err.Error(), "no rows")
}
