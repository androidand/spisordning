// `food-brain sync nutrition` fetches SLV nutrition for one or more SLV nummers
// and prints it as JSON (task 8.4). There is no nutrition table to persist into
// yet, so this is the fetch edge — the same shape as `sync-offers` until a
// nutrition schema lands.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/androidand/spisordning/internal/ingredients"
)

func runSyncNutrition(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: food-brain sync nutrition <slv-nummer> [slv-nummer...]")
	}

	slvURL := os.Getenv("SLV_BASE_URL")
	if slvURL == "" {
		return fmt.Errorf("sync nutrition: SLV_BASE_URL must be set")
	}
	slv := ingredients.NewLivsmedelsverket(slvURL)

	ctx := context.Background()
	out := make(map[string][]ingredients.Nutrient, len(args))
	for _, arg := range args {
		nummer, err := strconv.Atoi(arg)
		if err != nil {
			return fmt.Errorf("sync nutrition: invalid slv nummer %q", arg)
		}
		nutr, err := slv.LookupNutrition(ctx, nummer, ingredients.SprakSwedish)
		if err != nil {
			return fmt.Errorf("sync nutrition: nummer %d: %w", nummer, err)
		}
		out[strconv.Itoa(nummer)] = nutr
	}

	encoded, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("sync nutrition: marshal: %w", err)
	}
	fmt.Printf("✅ nutrition for %d nummer(s):\n%s\n", len(out), string(encoded))
	return nil
}
