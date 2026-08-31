// `food-brain sync recipes` refreshes the local recipe_ref cache from Mealie
// (task 4.4). Mealie stays the source of truth; this only writes the cached
// references the HTTP API and planner read from Postgres.
package main

import (
	"context"
	"fmt"

	"github.com/androidand/spisordning/internal/config"
	"github.com/androidand/spisordning/internal/mealie"
	"github.com/androidand/spisordning/internal/service"
)

// runSync dispatches `food-brain sync <kind> [args...]`.
func runSync(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: food-brain sync <recipes|import|nutrition|prices> [args...]")
	}
	switch args[0] {
	case "recipes":
		return runSyncRecipes(context.Background())
	case "import":
		return runImportRecipes(context.Background())
	case "nutrition":
		return runSyncNutrition(args[1:])
	case "prices":
		return runSyncPrices(args[1:])
	default:
		return fmt.Errorf("unknown sync target %q (want: recipes, import, nutrition, prices)", args[0])
	}
}

func runSyncRecipes(ctx context.Context) error {
	cfg := config.Load()
	if cfg.MealieBaseURL == "" || cfg.MealieAPIToken == "" {
		return fmt.Errorf("sync recipes: MEALIE_BASE_URL and MEALIE_API_TOKEN must be set")
	}
	store, err := openStore(ctx)
	if err != nil {
		return err
	}
	if store == nil {
		return fmt.Errorf("sync recipes: no database configured (set POSTGRES_PASSWORD or DATABASE_URL)")
	}
	svc := service.NewRecipes(store, mealie.New(cfg.MealieBaseURL, cfg.MealieAPIToken))
	n, err := svc.SyncFromMealie(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("synced %d recipe refs from Mealie\n", n)
	return nil
}

// runImportRecipes runs the one-way Mealie → recipe_family import (design.md D3).
func runImportRecipes(ctx context.Context) error {
	cfg := config.Load()
	if cfg.MealieBaseURL == "" || cfg.MealieAPIToken == "" {
		return fmt.Errorf("import: MEALIE_BASE_URL and MEALIE_API_TOKEN must be set")
	}
	store, err := openStore(ctx)
	if err != nil {
		return err
	}
	if store == nil {
		return fmt.Errorf("import: no database configured (set POSTGRES_PASSWORD or DATABASE_URL)")
	}
	svc := service.NewRecipes(store, mealie.New(cfg.MealieBaseURL, cfg.MealieAPIToken))
	n, err := svc.ImportAllMealieRecipes(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("imported %d recipes from Mealie to recipe_family\n", n)
	return nil
}
