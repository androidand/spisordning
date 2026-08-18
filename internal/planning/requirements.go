// Package planning is the planner's core: it plans the week's dinners from
// candidates (PlanWeek, see week.go) and turns chosen meals into canonical,
// retailer-independent shopping requirements. It deliberately knows nothing
// about Willys or any retailer: a requirement names a canonical ingredient and
// an amount, and the retailer adapter is responsible for resolving that to an
// actual product.
package planning

import (
	"sort"

	"github.com/androidand/spisordning/internal/domain"
)

// ChosenMeal is a decided slot: the recipe plus its canonical ingredient lines.
type ChosenMeal struct {
	MealieRecipeID string
	Ingredients    []domain.Ingredient
}

// ShoppingRequirement is the retailer-independent output. It intentionally
// carries no retailer product id — that mapping is the adapter's job.
type ShoppingRequirement struct {
	IngredientID    string
	Quantity        float64
	Unit            string
	AcceptableForms []string
	PreferredForm   string
}

// BuildRequirements aggregates the ingredient lines of all chosen meals into one
// requirement per (ingredient, unit) pair, summing quantities. Requirements for
// the same ingredient in different units are kept separate (the adapter/mapping
// layer handles unit reconciliation, not this pure aggregation). Output is
// sorted by ingredient then unit for a deterministic result.
func BuildRequirements(meals []ChosenMeal) []ShoppingRequirement {
	type key struct{ ingredient, unit string }
	agg := map[key]*ShoppingRequirement{}

	for _, meal := range meals {
		for _, line := range meal.Ingredients {
			k := key{line.IngredientID, line.Unit}
			req, ok := agg[k]
			if !ok {
				req = &ShoppingRequirement{
					IngredientID:    line.IngredientID,
					Unit:            line.Unit,
					AcceptableForms: append([]string(nil), line.AcceptableForms...),
					PreferredForm:   line.PreferredForm,
				}
				agg[k] = req
			}
			req.Quantity += line.Quantity
			req.AcceptableForms = unionForms(req.AcceptableForms, line.AcceptableForms)
			// Keep the first non-empty preferred form; conflicts are unlikely at
			// the ingredient level and can be reviewed later.
			if req.PreferredForm == "" {
				req.PreferredForm = line.PreferredForm
			}
		}
	}

	out := make([]ShoppingRequirement, 0, len(agg))
	for _, req := range agg {
		out = append(out, *req)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IngredientID != out[j].IngredientID {
			return out[i].IngredientID < out[j].IngredientID
		}
		return out[i].Unit < out[j].Unit
	})
	return out
}

func unionForms(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, list := range [][]string{a, b} {
		for _, f := range list {
			if f == "" || seen[f] {
				continue
			}
			seen[f] = true
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}
