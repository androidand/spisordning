package service

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/androidand/spisordning/internal/persistence"
)

// RecipeSourceMode controls how recipe resolution works during the
// Mealie → recipe_family migration (design.md D2).
type RecipeSourceMode string

const (
	SourceNative  RecipeSourceMode = "native"
	SourceDual    RecipeSourceMode = "dual"
	SourceMealie  RecipeSourceMode = "mealie"
)

// RecipeSourceModeFromEnv reads the RECIPE_SOURCE env var. Defaults to "native"
// (the post-migration end state). Set RECIPE_SOURCE=mealie to restore the
// pre-migration behavior during rollback.
func RecipeSourceModeFromEnv() RecipeSourceMode {
	switch v := os.Getenv("RECIPE_SOURCE"); v {
	case "mealie":
		return SourceMealie
	case "dual":
		return SourceDual
	default:
		return SourceNative
	}
}

// ResolvedRecipe is the result of resolving a recipe reference through the
// source-of-truth flag.
type ResolvedRecipe struct {
	// FamilyID is the native recipe_family id (zero when unresolved).
	FamilyID *persistence.RecipeFamily
	// Source is which source served the resolution: "native", "mealie", or "unmapped".
	Source string
	// MealieRecipeID is the original Mealie slug (for observability).
	MealieRecipeID string
}

// ResolveRecipeResolver resolves recipe references through the source-of-truth
// flag. It is the single entry point every consumer (planner, recommender,
// shopping, reactions, favorites) uses to look up recipe content.
type ResolveRecipeResolver struct {
	db   Store
	mode RecipeSourceMode
}

// NewResolveRecipeResolver creates a resolver with the given mode.
func NewResolveRecipeResolver(db Store, mode RecipeSourceMode) *ResolveRecipeResolver {
	return &ResolveRecipeResolver{db: db, mode: mode}
}

// ResolveRecipe resolves a Mealie recipe slug to its native recipe_family (if
// mapped) or falls back to Mealie depending on the source mode.
//
//   - mode=mealie: always returns Source="mealie" (current behavior).
//   - mode=dual: tries native first (via recipe_source_ref); falls back to
//     "mealie" when no mapping exists.
//   - mode=native: tries native first; if no mapping exists, returns
//     Source="unmapped" (surfaced, never silently dropped).
func (r *ResolveRecipeResolver) ResolveRecipe(ctx context.Context, mealieRecipeID string) (ResolvedRecipe, error) {
	if r.mode == SourceMealie {
		return ResolvedRecipe{Source: "mealie", MealieRecipeID: mealieRecipeID}, nil
	}

	// Try to resolve via recipe_source_ref.
	ref, err := r.db.GetRecipeSourceRefBySource(ctx, "mealie", mealieRecipeID)
	if err == nil {
		family, ferr := r.db.GetRecipeFamily(ctx, ref.RecipeFamilyID)
		if ferr == nil {
			return ResolvedRecipe{
				FamilyID:       &family,
				Source:         "native",
				MealieRecipeID: mealieRecipeID,
			}, nil
		}
	} else if !errors.Is(err, persistence.ErrNoRows) {
		return ResolvedRecipe{}, fmt.Errorf("resolve recipe: lookup source ref: %w", err)
	}

	if r.mode == SourceDual {
		return ResolvedRecipe{Source: "mealie", MealieRecipeID: mealieRecipeID}, nil
	}

	// mode=native: no mapping found — surface it, don't drop it.
	return ResolvedRecipe{Source: "unmapped", MealieRecipeID: mealieRecipeID}, nil
}

// ResolveBatch resolves multiple Mealie recipe slugs in one pass.
// Returns a map of MealieRecipeID → ResolvedRecipe. Errors are cumulative:
// a single failure returns the partial results collected so far plus the error.
func (r *ResolveRecipeResolver) ResolveBatch(ctx context.Context, mealieRecipeIDs []string) (map[string]ResolvedRecipe, error) {
	out := make(map[string]ResolvedRecipe, len(mealieRecipeIDs))
	for _, id := range mealieRecipeIDs {
		res, err := r.ResolveRecipe(ctx, id)
		if err != nil {
			return out, fmt.Errorf("resolve batch: %w", err)
		}
		out[id] = res
	}
	return out, nil
}

// ResolveRecipeByFamilyID resolves a native recipe_family id directly.
// Returns the family and its source ref (if any).
func (r *ResolveRecipeResolver) ResolveRecipeByFamilyID(ctx context.Context, familyID domain.RecipeFamilyID) (ResolvedRecipe, error) {
	family, err := r.db.GetRecipeFamily(ctx, familyID)
	if err != nil {
		return ResolvedRecipe{}, fmt.Errorf("resolve recipe by family: %w", err)
	}
	res := ResolvedRecipe{FamilyID: &family, Source: "native"}
	if ref, rerr := r.db.GetRecipeSourceRefByFamily(ctx, familyID); rerr == nil {
		res.MealieRecipeID = ref.SourceRecipeID
	}
	return res, nil
}

// ResolveRecipeRef resolves a Mealie recipe slug to a persistence.RecipeRef
// (the cached reference row). This is the workhorse for consumers that need
// the RecipeRefID (favorites, plan decisions, meal events) — it goes through
// the source-of-truth flag to decide whether to look up the ref directly
// (mealie mode) or via the native family mapping (native/dual mode).
//
// In all modes the recipe_ref cache row is the final destination: the
// in-flight reference columns (meal_plan.recipe_ref_id, favorite.recipe_ref_id,
// meal_event.recipe_ref_id) all point at recipe_ref, not recipe_family.
// The resolver's job is to ensure the correct recipe_ref row is found even
// when the caller only has a Mealie slug.
func (r *ResolveRecipeResolver) ResolveRecipeRef(ctx context.Context, mealieRecipeID string) (persistence.RecipeRef, error) {
	if r.mode == SourceMealie {
		ref, err := r.db.GetRecipeRefByMealieID(ctx, mealieRecipeID)
		if err != nil {
			return persistence.RecipeRef{}, fmt.Errorf("resolve recipe ref: %w", err)
		}
		return ref, nil
	}

	// native/dual: try the source ref mapping first to confirm the recipe
	// is known, then fall through to the recipe_ref cache (which is still
	// the row that in-flight references point at).
	_, err := r.db.GetRecipeSourceRefBySource(ctx, "mealie", mealieRecipeID)
	if err == nil {
		// Mapped: the recipe_ref cache should have a row for this Mealie id.
		ref, rerr := r.db.GetRecipeRefByMealieID(ctx, mealieRecipeID)
		if rerr == nil {
			return ref, nil
		}
		// No cache row yet — surface as unmapped in native mode.
		if r.mode == SourceNative {
			return persistence.RecipeRef{}, fmt.Errorf("resolve recipe ref: %q is mapped to a native family but has no recipe_ref cache row (run `food-brain sync recipes`)", mealieRecipeID)
		}
		// dual: fall through to direct lookup (which will also fail, but
		// the error message is clearer coming from the direct path).
	} else if !errors.Is(err, persistence.ErrNoRows) {
		return persistence.RecipeRef{}, fmt.Errorf("resolve recipe ref: lookup source ref: %w", err)
	}

	// Direct lookup (dual fallback, or no mapping found).
	ref, err := r.db.GetRecipeRefByMealieID(ctx, mealieRecipeID)
	if err != nil {
		if r.mode == SourceNative && errors.Is(err, persistence.ErrNoRows) {
			return persistence.RecipeRef{}, fmt.Errorf("resolve recipe ref: %q not found in recipe_ref and not mapped to a native family (unmapped under RECIPE_SOURCE=native)", mealieRecipeID)
		}
		return persistence.RecipeRef{}, fmt.Errorf("resolve recipe ref: %w", err)
	}
	return ref, nil
}
