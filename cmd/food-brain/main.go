// Command food-brain is the Food Brain CLI.
//
//	food-brain demo         — in-memory demonstration of the scoring pipe (no services)
//	food-brain plan         — live weekly plan: Mealie → scorer (+Skolmaten, +Olla) →
//	                          shopping requirements → willys-adapter (optional wishlist)
//	food-brain ingredients  — review surface: show the curated Swedish-unit → grams →
//	                          package-size ingredient mappings (task 2.3)
//
// Running with no arguments is equivalent to `demo`.
package main

import (
	"fmt"
	"os"
)

func main() {
	cmd := "demo"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	switch cmd {
	case "demo":
		runDemo()
	case "plan":
		if err := runPlan(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "❌", err)
			os.Exit(1)
		}
	case "ingredients":
		runIngredients()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q (want: demo, plan, ingredients)\n", cmd)
		os.Exit(2)
	}
}
