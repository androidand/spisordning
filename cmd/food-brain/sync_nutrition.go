// `food-brain sync nutrition` fetches SLV nutrition and, with --slv-full, persists
// the complete SLV dataset (foods + nutrients) plus Dabas product mappings into
// the nutrition tables (research-nutrition-data-sources tasks 5/6). Without flags
// it does a one-off lookup and prints JSON, matching the `sync nutrition <nummer>`
// edge shape.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/androidand/spisordning/internal/config"
	"github.com/androidand/spisordning/internal/ingredients"
	"github.com/androidand/spisordning/internal/service"
)

func runSyncNutrition(args []string) error {
	// No flags: one-off lookup, prints JSON (the `sync nutrition <nummer>` edge).
	if len(args) > 0 && args[0][0] != '-' {
		return runSyncNutritionLookup(args)
	}

	fs := flag.NewFlagSet("sync nutrition", flag.ExitOnError)
	slvFull := fs.Bool("slv-full", false, "persist the complete SLV dataset (foods + nutrients) into the nutrition tables")
	dabasQueries := fs.String("dabas", "", "comma-separated Dabas search queries to map products (e.g. \"mjolk,brod\")")
	statusOnly := fs.Bool("status", false, "print the last full-sync status per source instead of running a sync")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg := config.Load()
	slv := ingredients.NewLivsmedelsverket(cfg.SLVBaseURL)
	dabas := ingredients.NewDabas()

	store, err := openStore(context.Background())
	if err != nil {
		return err
	}
	if store == nil {
		return fmt.Errorf("sync nutrition: no database configured (set POSTGRES_PASSWORD or DATABASE_URL)")
	}

	ns := service.NewNutritionSync(store, slv, dabas)
	ctx := context.Background()

	if *statusOnly {
		return printSyncStatus(ctx, ns)
	}

	if *slvFull && *dabasQueries == "" {
		return fmt.Errorf("sync nutrition: pass --slv-full, --dabas <queries>, or both")
	}

	var results []service.SyncResult
	if *slvFull {
		r, err := ns.SyncSLV(ctx)
		if err != nil {
			return err
		}
		results = append(results, r)
	}
	if *dabasQueries != "" {
		r, err := ns.SyncDabas(ctx, splitCSV(*dabasQueries)...)
		if err != nil {
			return err
		}
		results = append(results, r)
	}

	for _, r := range results {
		if r.Skipped {
			fmt.Printf("%s: skipped (client not configured)\n", r.Source)
			continue
		}
		fmt.Printf("%s: wrote %d records at %s\n", r.Source, r.Written, r.SyncedAt.Format("2006-01-02 15:04:05"))
	}
	return nil
}

// runSyncNutritionLookup is the one-off lookup edge: fetches nutrition for the
// given SLV nummers and prints it as JSON.
func runSyncNutritionLookup(args []string) error {
	cfg := config.Load()
	if cfg.SLVBaseURL == "" {
		return fmt.Errorf("sync nutrition: SLV_BASE_URL must be set")
	}
	slv := ingredients.NewLivsmedelsverket(cfg.SLVBaseURL)

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
	fmt.Printf("nutrition for %d nummer(s):\n%s\n", len(out), string(encoded))
	return nil
}

func printSyncStatus(ctx context.Context, ns *service.NutritionSync) error {
	for _, source := range []string{"slv", "dabas"} {
		st, ok, err := ns.SyncStatusFor(ctx, source)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Printf("%s: never synced\n", source)
			continue
		}
		fmt.Printf("%s: last synced %s (%d records)\n", source, st.LastSynced.Format("2006-01-02 15:04:05"), st.RecordCount)
	}
	return nil
}

// splitCSV splits a comma-separated list, trimming whitespace and dropping empties.
func splitCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
