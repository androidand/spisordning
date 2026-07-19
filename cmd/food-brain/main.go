// Command food-brain is, for this first slice, a demonstration harness: it runs
// the deterministic scorer and the requirements builder over an in-memory sample
// family and recipe set, with no database, LLM, or network. It exists so the
// core planning logic can be seen end to end (`go run ./cmd/food-brain`) before
// the live Mealie / Olla / willys-adapter integrations are wired in.
package main

import (
	"fmt"
	"time"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/androidand/spisordning/internal/planning"
	"github.com/androidand/spisordning/internal/scoring"
)

func main() {
	day := time.Date(2026, 7, 20, 17, 0, 0, 0, time.UTC) // a tired Monday

	people := []domain.Person{
		{ID: "mum", Name: "Mum", Weight: 1},
		{ID: "dad", Name: "Dad", Weight: 1},
		{ID: "kid", Name: "Vera", Weight: 2}, // the picky one counts double
	}

	prefs := []domain.Preference{
		{PersonID: "kid", Tag: "pasta", Sentiment: domain.Loves, Confidence: 0.9},
		{PersonID: "kid", Tag: "fish", Sentiment: domain.Hates, Confidence: 0.8},
		{PersonID: "mum", Tag: "fish", Sentiment: domain.Loves, Confidence: 0.7},
		{PersonID: "dad", Tag: "stew", Sentiment: domain.Likes, Confidence: 0.6},
	}

	candidates := []domain.Candidate{
		{MealieRecipeID: "r-pasta", Title: "Pasta med köttfärssås", Tags: []string{"pasta", "beef"},
			Ingredients: []string{"pasta", "beef-mince", "onion", "tomato"}, Effort: domain.EffortLow},
		{MealieRecipeID: "r-fish", Title: "Ugnsbakad lax", Tags: []string{"fish"},
			Ingredients: []string{"salmon", "potato", "lemon"}, Effort: domain.EffortMedium},
		{MealieRecipeID: "r-stew", Title: "Köttgryta", Tags: []string{"stew", "beef"},
			Ingredients: []string{"beef", "carrot", "onion"}, Effort: domain.EffortHigh},
		{MealieRecipeID: "r-tacos", Title: "Fredagstacos", Tags: []string{"tacos", "beef"},
			Ingredients: []string{"tortilla", "beef-mince", "corn"}, Effort: domain.EffortLow},
	}

	ctx := domain.PlanContext{
		Day:           day,
		People:        people,
		Preferences:   prefs,
		KitchenEnergy: domain.EffortMedium, // no energy for the high-effort stew
		RecentMealIDs: []domain.RecentMeal{
			{MealieRecipeID: "r-tacos", Served: day.AddDate(0, 0, -2)}, // tacos two days ago
		},
		SchoolLunchTags:     []string{"fish"}, // fish at school today
		CampaignIngredients: map[string]bool{"beef-mince": true, "tortilla": true},
	}

	ranked := scoring.Rank(candidates, ctx, scoring.DefaultWeights())

	fmt.Println("Monday dinner candidates (best first):")
	fmt.Println("──────────────────────────────────────")
	for i, r := range ranked {
		flag := ""
		if !r.Feasible {
			flag = "  [infeasible: " + r.Reason + "]"
		}
		fmt.Printf("%d. %-24s score %+.3f%s\n", i+1, r.Candidate.Title, r.Score, flag)
		fmt.Printf("     pref %+.2f  effort %+.2f  repeat %+.2f  school %+.2f  campaign %+.2f\n",
			r.Breakdown.Preference, r.Breakdown.Effort, r.Breakdown.Repetition,
			r.Breakdown.SchoolDedup, r.Breakdown.Campaign)
	}

	// Pretend the family accepts the top feasible candidate for Monday.
	var winner *scoring.ScoredCandidate
	for i := range ranked {
		if ranked[i].Feasible {
			winner = &ranked[i]
			break
		}
	}
	if winner == nil {
		fmt.Println("\nNo feasible meal — take-away night.")
		return
	}
	fmt.Printf("\nChosen for Monday: %s\n", winner.Candidate.Title)

	// Build canonical shopping requirements from the chosen meal's ingredients.
	chosen := planning.ChosenMeal{MealieRecipeID: winner.Candidate.MealieRecipeID}
	for _, ing := range winner.Candidate.Ingredients {
		chosen.Ingredients = append(chosen.Ingredients, planning.RecipeIngredient{
			IngredientID: ing, Quantity: 1, Unit: "st", PreferredForm: "fresh",
		})
	}
	reqs := planning.BuildRequirements([]planning.ChosenMeal{chosen})

	fmt.Println("\nCanonical shopping requirements (retailer-independent):")
	for _, req := range reqs {
		fmt.Printf("  - %-12s %g %s (prefer %s)\n", req.IngredientID, req.Quantity, req.Unit, req.PreferredForm)
	}
	fmt.Println("\n(next: the willys-adapter resolves these to products and creates a wishlist)")
}
