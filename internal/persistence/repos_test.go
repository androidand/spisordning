package persistence

import (
	"context"
	"testing"
	"time"
)

// truncateTable resets the tables an integration test touches so round-trips
// are independent of ordering. Called only when a DB is present.
func truncateTables(t *testing.T, ctx context.Context, s *Store, tables ...string) {
	t.Helper()
	if len(tables) == 0 {
		return
	}
	var q string
	for i, t2 := range tables {
		if i > 0 {
			q += ", "
		}
		q += " \"" + t2 + "\""
	}
	if _, err := s.db.Exec(ctx, "TRUNCATE"+q+" CASCADE"); err != nil {
		t.Fatalf("truncate %s: %v", q, err)
	}
}

func TestPeople_PreferencesAndObservations(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "preference_observation", "person_preference", "person")

	p := Person{ID: "p-test-people", Name: "Test Person", Weight: 2}
	if err := s.CreatePerson(ctx, p); err != nil {
		t.Fatalf("CreatePerson: %v", err)
	}
	got, err := s.GetPerson(ctx, "p-test-people")
	if err != nil {
		t.Fatalf("GetPerson: %v", err)
	}
	if got.Name != "Test Person" || got.Weight != 2 {
		t.Errorf("GetPerson = %+v", got)
	}

	if err := s.UpsertPreference(ctx, PersonPreference{
		PersonID: "p-test-people", Tag: "pasta", Sentiment: 2, Confidence: 0.9,
	}); err != nil {
		t.Fatalf("UpsertPreference: %v", err)
	}
	prefs, err := s.ListPreferences(ctx, "p-test-people")
	if err != nil {
		t.Fatalf("ListPreferences: %v", err)
	}
	if len(prefs) != 1 || prefs[0].Tag != "pasta" || prefs[0].Sentiment != 2 {
		t.Errorf("preferences = %+v", prefs)
	}
}

func TestRecipes_IngredientsAndMappings(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "recipe_ingredient", "ingredient_mapping", "ingredient", "recipe_ref")

	// Seed an ingredient + mapping, then a recipe referencing it.
	if err := s.UpsertIngredient(ctx, Ingredient{ID: "köttfärs", Display: "Köttfärs"}); err != nil {
		t.Fatalf("UpsertIngredient: %v", err)
	}
	if err := s.UpsertIngredientMapping(ctx, IngredientMapping{
		MealieFoodID: "mf-1", IngredientID: "köttfärs", GramsPerUnit: 500, DefaultForm: "fresh",
	}); err != nil {
		t.Fatalf("UpsertIngredientMapping: %v", err)
	}
	if err := s.UpsertRecipeRef(ctx, RecipeRef{
		MealieRecipeID: "r-pasta", Title: "Pasta", Tags: []string{"pasta"}, Effort: 2,
	}); err != nil {
		t.Fatalf("UpsertRecipeRef: %v", err)
	}
	if err := s.AddRecipeIngredient(ctx, RecipeIngredient{
		MealieRecipeID: "r-pasta", IngredientID: "köttfärs", Quantity: 400, Unit: "g",
	}); err != nil {
		t.Fatalf("AddRecipeIngredient: %v", err)
	}

	refs, err := s.GetRecipeRef(ctx, "r-pasta")
	if err != nil {
		t.Fatalf("GetRecipeRef: %v", err)
	}
	if refs.Title != "Pasta" || len(refs.Tags) != 1 || refs.Tags[0] != "pasta" {
		t.Errorf("GetRecipeRef = %+v", refs)
	}

	all, err := s.ListRecipeRefs(ctx)
	if err != nil {
		t.Fatalf("ListRecipeRefs: %v", err)
	}
	if len(all) != 1 || all[0].MealieRecipeID != "r-pasta" || all[0].Effort != 2 {
		t.Errorf("ListRecipeRefs = %+v", all)
	}
	ing, err := s.GetIngredient(ctx, "köttfärs")
	if err != nil {
		t.Fatalf("GetIngredient: %v", err)
	}
	if ing.Display != "Köttfärs" {
		t.Errorf("GetIngredient = %+v", ing)
	}
	rings, err := s.ListRecipeIngredients(ctx, "r-pasta")
	if err != nil {
		t.Fatalf("ListRecipeIngredients: %v", err)
	}
	if len(rings) != 1 || rings[0].Quantity != 400 {
		t.Errorf("recipe ingredients = %+v", rings)
	}

	// resolve + re-fetch mapping
	if err := s.UpsertIngredientMapping(ctx, IngredientMapping{
		MealieFoodID: "mf-1", IngredientID: "köttfärs", GramsPerUnit: 500, DefaultForm: "fresh", NeedsReview: false,
	}); err != nil {
		t.Fatalf("UpsertIngredientMapping resolve: %v", err)
	}
	needs, err := s.ListNeedsReviewMappings(ctx)
	if err != nil {
		t.Fatalf("ListNeedsReviewMappings: %v", err)
	}
	if len(needs) != 0 {
		t.Errorf("expected 0 needs-review after resolve, got %d", len(needs))
	}
}

func TestMeals_ReactionsAndConstraints(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "meal_reaction", "meal_event", "planning_constraint", "recipe_ref")

	// meal_event needs a recipe_ref FK
	if err := s.UpsertRecipeRef(ctx, RecipeRef{MealieRecipeID: "r-fisk", Title: "Ugnslax", Effort: 2}); err != nil {
		t.Fatalf("UpsertRecipeRef: %v", err)
	}
	served := time.Now().AddDate(0, 0, -3)
	eid, err := s.CreateMealEvent(ctx, "r-fisk", served, nil)
	if err != nil {
		t.Fatalf("CreateMealEvent: %v", err)
	}
	if err := s.AddMealReaction(ctx, MealReaction{MealEventID: eid, PersonID: "p-kid", Sentiment: 2, Note: "gott"}); err != nil {
		t.Fatalf("AddMealReaction: %v", err)
	}
	rxns, err := s.ListMealReactions(ctx, eid)
	if err != nil {
		t.Fatalf("ListMealReactions: %v", err)
	}
	if len(rxns) != 1 || rxns[0].Note != "gott" || rxns[0].Sentiment != 2 {
		t.Errorf("reactions = %+v", rxns)
	}

	if _, err := s.CreatePlanningConstraint(ctx, PlanningConstraint{Kind: "avoid_tag", Value: "fisk", Active: true}); err != nil {
		t.Fatalf("CreatePlanningConstraint: %v", err)
	}
}

func TestEffortProfile(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "effort_profile")
	if err := s.UpsertEffortProfile(ctx, EffortProfile{Weekday: 1, KitchenEnergy: 2}); err != nil {
		t.Fatalf("UpsertEffortProfile: %v", err)
	}
}
