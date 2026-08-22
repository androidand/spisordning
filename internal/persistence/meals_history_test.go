package persistence

import (
	"context"
	"testing"
)

func TestMealHistory_ParticipantRoundTrip(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "meal_participant", "meal_event", "recipe_ref", "person")

	if err := s.UpsertRecipeRef(ctx, RecipeRef{MealieRecipeID: "r-pasta", Title: "Pasta", Effort: 1}); err != nil {
		t.Fatalf("UpsertRecipeRef: %v", err)
	}
	if err := s.CreatePerson(ctx, Person{ID: "p-andreas", Name: "Andreas", Weight: 1}); err != nil {
		t.Fatalf("CreatePerson: %v", err)
	}
	eid, err := s.CreateMealEvent(ctx, "r-pasta", date(t, "2043-01-01"))
	if err != nil {
		t.Fatalf("CreateMealEvent: %v", err)
	}
	if err := s.AddMealParticipant(ctx, eid, "p-andreas"); err != nil {
		t.Fatalf("AddMealParticipant: %v", err)
	}
	// Idempotent: second call succeeds without error.
	if err := s.AddMealParticipant(ctx, eid, "p-andreas"); err != nil {
		t.Fatalf("AddMealParticipant idempotent: %v", err)
	}
	participants, err := s.ListMealParticipants(ctx, eid)
	if err != nil {
		t.Fatalf("ListMealParticipants: %v", err)
	}
	if len(participants) != 1 || participants[0].PersonID != "p-andreas" {
		t.Errorf("participants = %+v", participants)
	}
}

func TestMealHistory_ReviewRoundTrip(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "meal_review", "meal_event", "recipe_ref", "person")

	if err := s.UpsertRecipeRef(ctx, RecipeRef{MealieRecipeID: "r-fish", Title: "Fisk", Effort: 2}); err != nil {
		t.Fatalf("UpsertRecipeRef: %v", err)
	}
	if err := s.CreatePerson(ctx, Person{ID: "p-andreas", Name: "Andreas", Weight: 1}); err != nil {
		t.Fatalf("CreatePerson: %v", err)
	}
	if err := s.CreatePerson(ctx, Person{ID: "p-vera", Name: "Vera", Weight: 1}); err != nil {
		t.Fatalf("CreatePerson: %v", err)
	}
	eid, err := s.CreateMealEvent(ctx, "r-fish", date(t, "2043-01-01"))
	if err != nil {
		t.Fatalf("CreateMealEvent: %v", err)
	}
	if err := s.AddMealReview(ctx, MealReview{MealEventID: eid, PersonID: "p-andreas", Rating: 5, Note: "fantastiskt"}); err != nil {
		t.Fatalf("AddMealReview: %v", err)
	}
	if err := s.AddMealReview(ctx, MealReview{MealEventID: eid, PersonID: "p-vera", Rating: 4}); err != nil {
		t.Fatalf("AddMealReview vera: %v", err)
	}
	reviews, err := s.ListMealReviews(ctx, eid)
	if err != nil {
		t.Fatalf("ListMealReviews: %v", err)
	}
	if len(reviews) != 2 {
		t.Fatalf("expected 2 reviews, got %d", len(reviews))
	}
	// Upsert: update andreas's rating.
	if err := s.AddMealReview(ctx, MealReview{MealEventID: eid, PersonID: "p-andreas", Rating: 4, Note: "jättebra"}); err != nil {
		t.Fatalf("AddMealReview update: %v", err)
	}
	reviews, _ = s.ListMealReviews(ctx, eid)
	if len(reviews) != 2 {
		t.Fatalf("still 2 reviews after upsert, got %d", len(reviews))
	}
	andreas := reviews[0]
	if andreas.PersonID == "p-vera" {
		andreas = reviews[1]
	}
	if andreas.Rating != 4 || andreas.Note != "jättebra" {
		t.Errorf("updated review = %+v", andreas)
	}
}

func TestMealHistory_RecipeRatingAggregation(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "meal_review", "meal_event", "recipe_ref", "person")

	recipeID := "r-stew"
	if err := s.UpsertRecipeRef(ctx, RecipeRef{MealieRecipeID: recipeID, Title: "Gryta", Effort: 2}); err != nil {
		t.Fatalf("UpsertRecipeRef: %v", err)
	}
	if err := s.CreatePerson(ctx, Person{ID: "p-a", Name: "Andreas", Weight: 1}); err != nil {
		t.Fatalf("CreatePerson: %v", err)
	}
	if err := s.CreatePerson(ctx, Person{ID: "p-v", Name: "Vera", Weight: 1}); err != nil {
		t.Fatalf("CreatePerson: %v", err)
	}
	// Two meal events for the same recipe.
	eid1, _ := s.CreateMealEvent(ctx, recipeID, date(t, "2043-01-01"))
	eid2, _ := s.CreateMealEvent(ctx, recipeID, date(t, "2043-01-08"))
	// Three reviews across the two events.
	s.AddMealReview(ctx, MealReview{MealEventID: eid1, PersonID: "p-a", Rating: 5})
	s.AddMealReview(ctx, MealReview{MealEventID: eid1, PersonID: "p-v", Rating: 4})
	s.AddMealReview(ctx, MealReview{MealEventID: eid2, PersonID: "p-a", Rating: 3})

	got, err := s.GetRecipeRating(ctx, recipeID)
	if err != nil {
		t.Fatalf("GetRecipeRating: %v", err)
	}
	if got.ReviewCount != 3 {
		t.Errorf("ReviewCount = %d, want 3", got.ReviewCount)
	}
	// Mean of 5, 4, 3 = 4.0
	if got.Average != 4.0 {
		t.Errorf("Average = %v, want 4.0", got.Average)
	}
}

func TestMealHistory_FavoriteExplicitNotDerived(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "favorite", "meal_review", "meal_event", "recipe_ref", "person")

	recipeID := "r-kids-pasta"
	if err := s.UpsertRecipeRef(ctx, RecipeRef{MealieRecipeID: recipeID, Title: "Barnpasta", Effort: 1}); err != nil {
		t.Fatalf("UpsertRecipeRef: %v", err)
	}
	if err := s.CreatePerson(ctx, Person{ID: "p-kid", Name: "Kid", Weight: 1}); err != nil {
		t.Fatalf("CreatePerson: %v", err)
	}
	eid, _ := s.CreateMealEvent(ctx, recipeID, date(t, "2043-01-01"))
	// Low ratings — but the kid still favorites the recipe.
	s.AddMealReview(ctx, MealReview{MealEventID: eid, PersonID: "p-kid", Rating: 1})
	s.AddMealReview(ctx, MealReview{MealEventID: eid, PersonID: "p-kid", Rating: 2})

	// Favorite is NOT auto-created by low ratings.
	isFav, _ := s.IsPersonFavorite(ctx, "p-kid", recipeID)
	if isFav {
		t.Error("favorite should not be auto-created from low ratings")
	}
	// Explicit favorite survives poor reviews.
	if err := s.AddFavorite(ctx, "p-kid", recipeID); err != nil {
		t.Fatalf("AddFavorite: %v", err)
	}
	isFav, _ = s.IsPersonFavorite(ctx, "p-kid", recipeID)
	if !isFav {
		t.Error("favorite should persist after explicit add")
	}
	favs, _ := s.ListPersonFavorites(ctx, "p-kid")
	if len(favs) != 1 || favs[0].MealieRecipeID != recipeID {
		t.Errorf("favorites = %+v", favs)
	}
	// Remove and verify gone.
	if err := s.RemoveFavorite(ctx, "p-kid", recipeID); err != nil {
		t.Fatalf("RemoveFavorite: %v", err)
	}
	isFav, _ = s.IsPersonFavorite(ctx, "p-kid", recipeID)
	if isFav {
		t.Error("favorite should be gone after explicit remove")
	}
}

func TestMealHistory_FavoritePersonScoped(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "favorite", "recipe_ref", "person")

	recipeID := "r-curry"
	if err := s.UpsertRecipeRef(ctx, RecipeRef{MealieRecipeID: recipeID, Title: "Curry", Effort: 2}); err != nil {
		t.Fatalf("UpsertRecipeRef: %v", err)
	}
	if err := s.CreatePerson(ctx, Person{ID: "p-a", Name: "Andreas", Weight: 1}); err != nil {
		t.Fatalf("CreatePerson: %v", err)
	}
	if err := s.CreatePerson(ctx, Person{ID: "p-v", Name: "Vera", Weight: 1}); err != nil {
		t.Fatalf("CreatePerson: %v", err)
	}
	// Andreas favorites the recipe; Vera does not.
	s.AddFavorite(ctx, "p-a", recipeID)

	isFavA, _ := s.IsPersonFavorite(ctx, "p-a", recipeID)
	isFavV, _ := s.IsPersonFavorite(ctx, "p-v", recipeID)
	if !isFavA {
		t.Error("Andreas should have the favorite")
	}
	if isFavV {
		t.Error("Vera should NOT have the favorite")
	}
	// List by recipe shows only Andreas.
	favs, _ := s.ListRecipeFavorites(ctx, recipeID)
	if len(favs) != 1 || favs[0].PersonID != "p-a" {
		t.Errorf("recipe favorites = %+v", favs)
	}
}

func TestMealHistory_MealEventPlanLink(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "meal_event", "meal_plan", "recipe_ref")

	if err := s.UpsertRecipeRef(ctx, RecipeRef{MealieRecipeID: "r-chicken", Title: "Kyckling", Effort: 2}); err != nil {
		t.Fatalf("UpsertRecipeRef: %v", err)
	}
	pid, _ := s.CreateMealPlan(ctx, date(t, "2043-01-18"))
	eid, _ := s.CreateMealEvent(ctx, "r-chicken", date(t, "2043-01-18"))

	// No link by default.
	planID, planDate, err := s.GetMealEventPlanLink(ctx, eid)
	if err != nil {
		t.Fatalf("GetMealEventPlanLink: %v", err)
	}
	if planID != nil || planDate != nil {
		t.Errorf("expected no link, got plan_id=%v slot_date=%v", planID, planDate)
	}

	// Set link.
	planD := date(t, "2043-01-18")
	if err := s.LinkMealEventToPlan(ctx, eid, &pid, &planD); err != nil {
		t.Fatalf("LinkMealEventToPlan: %v", err)
	}
	planID, planDate, err = s.GetMealEventPlanLink(ctx, eid)
	if err != nil {
		t.Fatalf("GetMealEventPlanLink after link: %v", err)
	}
	if planID == nil || *planID != pid {
		t.Errorf("plan_id = %v, want %d", planID, pid)
	}
	if planDate == nil || !sameDate(*planDate, planD) {
		t.Errorf("plan_slot_date = %v, want %v", planDate, planD)
	}

	// Clear link.
	if err := s.LinkMealEventToPlan(ctx, eid, nil, nil); err != nil {
		t.Fatalf("LinkMealEventToPlan clear: %v", err)
	}
	planID, planDate, err = s.GetMealEventPlanLink(ctx, eid)
	if err != nil {
		t.Fatalf("GetMealEventPlanLink after clear: %v", err)
	}
	if planID != nil || planDate != nil {
		t.Errorf("expected cleared link, got plan_id=%v slot_date=%v", planID, planDate)
	}
}
