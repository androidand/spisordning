package planning

import "testing"

func TestBuildRequirements_AggregatesSameIngredientAndUnit(t *testing.T) {
	meals := []ChosenMeal{
		{MealieRecipeID: "a", Ingredients: []RecipeIngredient{
			{IngredientID: "onion", Quantity: 2, Unit: "st"},
			{IngredientID: "cream", Quantity: 200, Unit: "g", PreferredForm: "fresh"},
		}},
		{MealieRecipeID: "b", Ingredients: []RecipeIngredient{
			{IngredientID: "onion", Quantity: 1, Unit: "st"},
		}},
	}

	reqs := BuildRequirements(meals)

	if len(reqs) != 2 {
		t.Fatalf("expected 2 requirements, got %d", len(reqs))
	}
	// Sorted: cream, onion.
	if reqs[0].IngredientID != "cream" || reqs[1].IngredientID != "onion" {
		t.Fatalf("expected sorted [cream, onion], got [%s, %s]", reqs[0].IngredientID, reqs[1].IngredientID)
	}
	if reqs[1].Quantity != 3 {
		t.Errorf("expected onion quantity summed to 3, got %.1f", reqs[1].Quantity)
	}
}

func TestBuildRequirements_SameIngredientDifferentUnitsStaySeparate(t *testing.T) {
	meals := []ChosenMeal{
		{MealieRecipeID: "a", Ingredients: []RecipeIngredient{
			{IngredientID: "milk", Quantity: 5, Unit: "dl"},
			{IngredientID: "milk", Quantity: 1, Unit: "l"},
		}},
	}

	reqs := BuildRequirements(meals)
	if len(reqs) != 2 {
		t.Fatalf("milk in dl and l must stay separate; got %d requirements", len(reqs))
	}
}

func TestBuildRequirements_UnionsAcceptableForms(t *testing.T) {
	meals := []ChosenMeal{
		{MealieRecipeID: "a", Ingredients: []RecipeIngredient{
			{IngredientID: "cauliflower", Quantity: 1, Unit: "st", AcceptableForms: []string{"fresh"}, PreferredForm: "fresh"},
		}},
		{MealieRecipeID: "b", Ingredients: []RecipeIngredient{
			{IngredientID: "cauliflower", Quantity: 1, Unit: "st", AcceptableForms: []string{"frozen"}},
		}},
	}

	reqs := BuildRequirements(meals)
	if len(reqs) != 1 {
		t.Fatalf("expected 1 requirement, got %d", len(reqs))
	}
	forms := reqs[0].AcceptableForms
	if len(forms) != 2 || forms[0] != "fresh" || forms[1] != "frozen" {
		t.Errorf("expected unioned sorted forms [fresh frozen], got %v", forms)
	}
	if reqs[0].PreferredForm != "fresh" {
		t.Errorf("expected preferred form 'fresh' preserved, got %q", reqs[0].PreferredForm)
	}
}

func TestBuildRequirements_Deterministic(t *testing.T) {
	meals := []ChosenMeal{
		{MealieRecipeID: "a", Ingredients: []RecipeIngredient{
			{IngredientID: "zucchini", Quantity: 1, Unit: "st"},
			{IngredientID: "apple", Quantity: 4, Unit: "st"},
			{IngredientID: "milk", Quantity: 1, Unit: "l"},
		}},
	}
	first := BuildRequirements(meals)
	for range 5 {
		again := BuildRequirements(meals)
		for i := range first {
			if again[i].IngredientID != first[i].IngredientID {
				t.Fatalf("non-deterministic order: %v", again)
			}
		}
	}
	// Confirm the fixed sort.
	if first[0].IngredientID != "apple" || first[2].IngredientID != "zucchini" {
		t.Errorf("expected apple..zucchini sorted, got %s..%s", first[0].IngredientID, first[2].IngredientID)
	}
}

func TestBuildRequirements_Empty(t *testing.T) {
	if got := BuildRequirements(nil); len(got) != 0 {
		t.Errorf("nil input should yield no requirements, got %d", len(got))
	}
}
