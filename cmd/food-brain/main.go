// Command food-brain is the Food Brain CLI.
//
//	food-brain demo         — in-memory demonstration of the scoring pipe (no services)
//	food-brain plan         — live weekly plan: Mealie → scorer (+Skolmaten, +Olla) →
//	                          shopping requirements → willys-adapter (optional wishlist)
//	                          --write-tonight writes the ambient projection for HA
//	food-brain serve        — HTTP server (api/openapi.yaml); /health and /people routes
//	                          are wired, more as the contract is implemented (tasks 3.3+).
//	                          Persistence-backed handlers require a Postgres connection
//	                          (POSTGRES_* / DATABASE_URL); without one, /health still serves.
//	food-brain tonight      — ambient surface: show tonight's meal + record one-tap
//	                          reactions (task 5.2; driven by Home Assistant / homeops)
//	food-brain ingredients  — review surface: show the curated Swedish-unit → grams →
//	                          package-size ingredient mappings (task 2.3)
//	food-brain sync-offers  — sync retailer campaign/offer data for campaign-aware planning
//	food-brain sync         — sync external data: `sync recipes` refreshes the
//	                          recipe_ref cache from Mealie (task 4.4);
//	                          `sync nutrition <slv-nummer>...` fetches SLV
//	                          nutrition (task 8.4); `sync prices -store ...`
//	                          records retailer offers into the price tables
//	                          (task 9.4)
//	food-brain migrate      — apply/check the embedded schema migrations (Goose):
//	                          `migrate up [--seed]` applies, `migrate status` reports
//
// Running with no arguments is equivalent to `demo`.
package main

import (
	"fmt"
	"os"

	"github.com/androidand/spisordning/internal/config"
	"github.com/androidand/spisordning/internal/httpapi"
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
	case "serve":
		addr := config.Load().HTTPAddr
		deps := buildDependencies()
		if err := httpapi.Serve(addr, deps); err != nil {
			fmt.Fprintln(os.Stderr, "❌", err)
			os.Exit(1)
		}
	case "tonight":
		if err := runTonight(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "❌", err)
			os.Exit(1)
		}
	case "ingredients":
		runIngredients()
	case "sync-offers":
		if err := runSyncOffers(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "❌", err)
			os.Exit(1)
		}
	case "sync":
		if err := runSync(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "❌", err)
			os.Exit(1)
		}
	case "migrate":
		if err := runMigrate(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "❌", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q (want: demo, plan, serve, tonight, ingredients, sync-offers, sync, migrate)\n", cmd)
		os.Exit(2)
	}
}
