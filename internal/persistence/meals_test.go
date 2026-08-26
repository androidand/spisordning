package persistence

import (
	"context"
	"testing"
)

func TestMealsAndPreferences_ParticipantAndReviewRoundTrip(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "meal_review", "meal_participant", "meal_reaction", "meal_event", "recipe_ref", "person")

	// Seed a person + recipe so FKs resolve.
	if err := s.CreatePerson(ctx, Person{ID: "p-andreas", Name: "Andreas", Weight: 1.0}); err != nil {
		t.Fatalf("CreatePerson: %v", err)
	}
	if err := s.UpsertRecipeRef(ctx, RecipeRef{MealieRecipeID: "r-pasta", Title: "Pasta", Effort: 2}); err != nil {
		t.Fatalf("UpsertRecipeRef: %v", err)
	}
	served := date(t, "2026-08-15")
	eid, err := s.CreateMealEvent(ctx, "r-pasta", served, nil, nil)
	if err != nil {
		t.Fatalf("CreateMealEvent: %v", err)
	}

	// Add a participant (attends without reacting).
	if err := s.AddMealParticipant(ctx, MealParticipant{MealEventID: eid, PersonID: "p-andreas"}); err != nil {
		t.Fatalf("AddMealParticipant: %v", err)
	}
	parts, err := s.ListMealParticipants(ctx, eid)
	if err != nil {
		t.Fatalf("ListMealParticipants: %v", err)
	}
	if len(parts) != 1 || parts[0].PersonID != "p-andreas" {
		t.Errorf("participants = %+v", parts)
	}

	// Add a reaction (coarse, directional).
	if err := s.AddMealReaction(ctx, MealReaction{MealEventID: eid, PersonID: "p-andreas", Sentiment: 2, Note: "gott"}); err != nil {
		t.Fatalf("AddMealReaction: %v", err)
	}

	// Add a review (considered, 1-5).
	if err := s.UpsertMealReview(ctx, MealReview{MealEventID: eid, PersonID: "p-andreas", Rating: 5, Note: "fantastiskt"}); err != nil {
		t.Fatalf("UpsertMealReview: %v", err)
	}
	reviews, err := s.ListMealReviews(ctx, eid)
	if err != nil {
		t.Fatalf("ListMealReviews: %v", err)
	}
	if len(reviews) != 1 || reviews[0].Rating != 5 {
		t.Errorf("reviews = %+v", reviews)
	}

	// Upsert overwrites the rating.
	if err := s.UpsertMealReview(ctx, MealReview{MealEventID: eid, PersonID: "p-andreas", Rating: 4, Note: "jättebra"}); err != nil {
		t.Fatalf("UpsertMealReview second: %v", err)
	}
	reviews, err = s.ListMealReviews(ctx, eid)
	if err != nil {
		t.Fatalf("ListMealReviews after upsert: %v", err)
	}
	if len(reviews) != 1 || reviews[0].Rating != 4 || reviews[0].Note != "jättebra" {
		t.Errorf("upserted review = %+v", reviews)
	}
}

func TestMealsAndPreferences_RecipeRatingAggregation(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "meal_review", "meal_event", "recipe_ref", "person")

	// Two people with different weights.
	if err := s.CreatePerson(ctx, Person{ID: "p-mum", Name: "Mum", Weight: 1.0}); err != nil {
		t.Fatalf("CreatePerson: %v", err)
	}
	if err := s.CreatePerson(ctx, Person{ID: "p-kid", Name: "Kid", Weight: 2.0}); err != nil {
		t.Fatalf("CreatePerson: %v", err)
	}
	if err := s.UpsertRecipeRef(ctx, RecipeRef{MealieRecipeID: "r-pasta", Title: "Pasta", Effort: 2}); err != nil {
		t.Fatalf("UpsertRecipeRef: %v", err)
	}

	// Two meal events for the same recipe.
	e1, _ := s.CreateMealEvent(ctx, "r-pasta", date(t, "2026-08-01"), nil, nil)
	e2, _ := s.CreateMealEvent(ctx, "r-pasta", date(t, "2026-08-08"), nil, nil)

	// Mum (weight 1) rates both 5; kid (weight 2) rates both 3.
	// Weighted avg = (1*5 + 2*3 + 1*5 + 2*3) / (1+2+1+2) = 22/6 ≈ 3.667.
	s.UpsertMealReview(ctx, MealReview{MealEventID: e1, PersonID: "p-mum", Rating: 5})
	s.UpsertMealReview(ctx, MealReview{MealEventID: e1, PersonID: "p-kid", Rating: 3})
	s.UpsertMealReview(ctx, MealReview{MealEventID: e2, PersonID: "p-mum", Rating: 5})
	s.UpsertMealReview(ctx, MealReview{MealEventID: e2, PersonID: "p-kid", Rating: 3})

	rating, err := s.GetRecipeRating(ctx, "r-pasta")
	if err != nil {
		t.Fatalf("GetRecipeRating: %v", err)
	}
	if rating.ReviewCount != 4 {
		t.Errorf("review count = %d, want 4", rating.ReviewCount)
	}
	if rating.Average < 3.66 || rating.Average > 3.68 {
		t.Errorf("average = %v, want ~3.667", rating.Average)
	}

	// Rating for a recipe with no reviews is zero-valued.
	empty, err := s.GetRecipeRating(ctx, "r-never")
	if err != nil {
		t.Fatalf("GetRecipeRating empty: %v", err)
	}
	if empty.ReviewCount != 0 || empty.Average != 0 {
		t.Errorf("empty rating = %+v", empty)
	}
}

func TestMealsAndPreferences_FavoriteSurvivesLowReviews(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "favorite", "meal_review", "meal_event", "recipe_ref", "person")

	if err := s.CreatePerson(ctx, Person{ID: "p-kid", Name: "Kid", Weight: 1.0}); err != nil {
		t.Fatalf("CreatePerson: %v", err)
	}
	if err := s.UpsertRecipeRef(ctx, RecipeRef{MealieRecipeID: "r-comfort", Title: "Mac & Cheese", Effort: 1}); err != nil {
		t.Fatalf("UpsertRecipeRef: %v", err)
	}

	// Kid favorited the recipe (deliberate comfort choice).
	if err := s.UpsertFavorite(ctx, "p-kid", "", "r-comfort"); err != nil {
		t.Fatalf("UpsertFavorite: %v", err)
	}

	// Then left several low reviews (maybe it was undercooked each time).
	e1, _ := s.CreateMealEvent(ctx, "r-comfort", date(t, "2026-08-01"), nil, nil)
	e2, _ := s.CreateMealEvent(ctx, "r-comfort", date(t, "2026-08-08"), nil, nil)
	s.UpsertMealReview(ctx, MealReview{MealEventID: e1, PersonID: "p-kid", Rating: 2})
	s.UpsertMealReview(ctx, MealReview{MealEventID: e2, PersonID: "p-kid", Rating: 1})

	// Favorite persists despite low average rating — explicit preference != observed rating.
	favs, err := s.ListFavoritesForRecipe(ctx, "r-comfort")
	if err != nil {
		t.Fatalf("ListFavoritesForRecipe: %v", err)
	}
	if len(favs) != 1 {
		t.Fatalf("expected 1 favorite, got %d", len(favs))
	}
	if favs[0].PersonID == nil || *favs[0].PersonID != "p-kid" || favs[0].MealieRecipeID != "r-comfort" {
		t.Errorf("unexpected favorite: %+v", favs[0])
	}

	// Recipe rating is low, but favorite still exists.
	rating, _ := s.GetRecipeRating(ctx, "r-comfort")
	if rating.Average > 1.5 || rating.ReviewCount != 2 {
		t.Errorf("average = %v (count=%d), expected low", rating.Average, rating.ReviewCount)
	}
}

func TestMealsAndPreferences_PlanLink(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "meal_event", "meal_plan_decision", "meal_plan", "recipe_ref")

	// Seed a recipe.
	if err := s.UpsertRecipeRef(ctx, RecipeRef{MealieRecipeID: "r-pasta", Title: "Pasta", Effort: 2}); err != nil {
		t.Fatalf("UpsertRecipeRef: %v", err)
	}

	// Create a plan for a week.
	weekStart := date(t, "2026-08-17") // Monday
	planID, err := s.CreateMealPlan(ctx, weekStart)
	if err != nil {
		t.Fatalf("CreateMealPlan: %v", err)
	}

	// Add a decision for Wednesday.
	decisionDate := weekStart.AddDate(0, 0, 2) // Wednesday
	if err := s.SetDecision(ctx, MealPlanDecision{
		PlanID:         planID,
		SlotDate:       decisionDate,
		MealieRecipeID: "r-pasta",
	}); err != nil {
		t.Fatalf("SetDecision: %v", err)
	}

	// Create the actual meal linked to that decision.
	served := decisionDate
	eid, err := s.CreateMealEvent(ctx, "r-pasta", served, &planID, &served)
	if err != nil {
		t.Fatalf("CreateMealEvent with plan link: %v", err)
	}

	// Ad-hoc meal (no plan link) still works.
	eid2, err := s.CreateMealEvent(ctx, "r-pasta", served, nil, nil)
	if err != nil {
		t.Fatalf("CreateMealEvent ad-hoc: %v", err)
	}
	if eid2 == eid {
		t.Fatal("ad-hoc and planned events should have different ids")
	}
}

func TestMealsAndPreferences_ParticipantUniqueness(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "meal_participant", "meal_event", "recipe_ref", "person")

	if err := s.CreatePerson(ctx, Person{ID: "p-a", Name: "A", Weight: 1.0}); err != nil {
		t.Fatalf("CreatePerson: %v", err)
	}
	if err := s.UpsertRecipeRef(ctx, RecipeRef{MealieRecipeID: "r-pasta", Title: "Pasta", Effort: 2}); err != nil {
		t.Fatalf("UpsertRecipeRef: %v", err)
	}
	eid, _ := s.CreateMealEvent(ctx, "r-pasta", date(t, "2026-08-15"), nil, nil)

	// First add is fine.
	if err := s.AddMealParticipant(ctx, MealParticipant{MealEventID: eid, PersonID: "p-a"}); err != nil {
		t.Fatalf("first AddMealParticipant: %v", err)
	}
	// Duplicate add is idempotent (ON CONFLICT DO NOTHING).
	if err := s.AddMealParticipant(ctx, MealParticipant{MealEventID: eid, PersonID: "p-a"}); err != nil {
		t.Fatalf("duplicate AddMealParticipant: %v", err)
	}
	parts, err := s.ListMealParticipants(ctx, eid)
	if err != nil {
		t.Fatalf("ListMealParticipants: %v", err)
	}
	if len(parts) != 1 {
		t.Errorf("expected 1 participant row (idempotent), got %d", len(parts))
	}
}

func TestMealsAndPreferences_FavoriteScopeInvariant(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "favorite", "person", "household", "recipe_ref")

	if err := s.CreatePerson(ctx, Person{ID: "p-a", Name: "A", Weight: 1.0}); err != nil {
		t.Fatalf("CreatePerson: %v", err)
	}
	if err := s.CreateHousehold(ctx, Household{ID: "h-home", Name: "Home"}); err != nil {
		t.Fatalf("CreateHousehold: %v", err)
	}
	if err := s.UpsertRecipeRef(ctx, RecipeRef{MealieRecipeID: "r-pizza", Title: "Pizza", Effort: 2}); err != nil {
		t.Fatalf("UpsertRecipeRef: %v", err)
	}

	// Person-scoped favorite.
	if err := s.UpsertFavorite(ctx, "p-a", "", "r-pizza"); err != nil {
		t.Fatalf("UpsertFavorite person-scoped: %v", err)
	}
	// Household-scoped favorite for the same recipe.
	if err := s.UpsertFavorite(ctx, "", "h-home", "r-pizza"); err != nil {
		t.Fatalf("UpsertFavorite household-scoped: %v", err)
	}

	favs, err := s.ListFavoritesForRecipe(ctx, "r-pizza")
	if err != nil {
		t.Fatalf("ListFavoritesForRecipe: %v", err)
	}
	if len(favs) != 2 {
		t.Fatalf("expected 2 favorites (person + household), got %d", len(favs))
	}

	// Delete the person-scoped one; household favorite remains.
	if err := s.DeleteFavorite(ctx, "p-a", "", "r-pizza"); err != nil {
		t.Fatalf("DeleteFavorite: %v", err)
	}
	favs, err = s.ListFavoritesForRecipe(ctx, "r-pizza")
	if err != nil {
		t.Fatalf("ListFavoritesForRecipe after delete: %v", err)
	}
	if len(favs) != 1 || favs[0].HouseholdID == nil || *favs[0].HouseholdID != "h-home" {
		t.Errorf("expected household favorite to remain, got %+v", favs)
	}
}
