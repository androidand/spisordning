package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/androidand/spisordning/internal/ingredients"
)

// runIngredients is the minimal review surface for task 2.3: it shows the
// curated Swedish-unit → grams → package-size mappings so the household can see
// what's been resolved and what still needs review. Until Postgres persistence
// lands (see establish-enforced-go-architecture) this reads the in-memory seed
// at internal/ingredients.Seed, which mirrors migrations/seed/ingredient_mappings.sql.
func runIngredients() {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "CURATED INGREDIENT MAPPINGS (task 2.3 seed)")
	fmt.Fprintln(w, "Ingredient\tUnit\tg/unit\tPackage\tForm\tReview")
	for _, m := range ingredients.Seed {
		review := "no"
		if m.NeedsReview {
			review = "YES"
		}
		fmt.Fprintf(w, "%s\t%s\t%.0f\t%.0f g\t%s\t%s\n",
			m.Display, m.Unit, m.GramsPerUnit, m.PackageSizeGrams, m.DefaultForm, review)
	}
	_ = w.Flush()

	need := 0
	for _, m := range ingredients.Seed {
		if m.NeedsReview {
			need++
		}
	}
	fmt.Printf("\n%d curated mapping(s); %d need(s) review — re-point each at a real mealie_food_id after a live sync assigns one.\n",
		len(ingredients.Seed), need)
}
