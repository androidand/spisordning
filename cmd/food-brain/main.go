// Command food-brain is the Food Brain CLI.
//
//	food-brain demo         — in-memory demonstration of the scoring pipe (no services)
//	food-brain plan         — live weekly plan: Mealie → scorer (+Skolmaten, +Olla) →
//	                          shopping requirements → willys-adapter (optional wishlist)
//	food-brain serve        — HTTP server (api/openapi.yaml); /health and /people routes
//	                          are wired, more as the contract is implemented (tasks 3.3+).
//	                          Persistence-backed handlers require a Postgres connection
//	                          (POSTGRES_* / DATABASE_URL); without one, /health still serves.
//	food-brain ingredients  — review surface: show the curated Swedish-unit → grams →
//	                          package-size ingredient mappings (task 2.3)
//
// Running with no arguments is equivalent to `demo`.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/androidand/spisordning/internal/httpapi"
	"github.com/androidand/spisordning/internal/persistence"
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
		addr := envDefault("SPISORNING_ADDR", ":8080")
		deps := buildDependencies()
		if err := httpapi.Serve(addr, deps); err != nil {
			fmt.Fprintln(os.Stderr, "❌", err)
			os.Exit(1)
		}
	case "ingredients":
		runIngredients()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q (want: demo, plan, serve, ingredients)\n", cmd)
		os.Exit(2)
	}
}

// buildDependencies wires the persistence-backed services the HTTP layer exposes.
// It degrades gracefully: if Postgres isn't configured or unreachable, only the
// /health endpoint is served (resource routes are nil-guarded in RegisterHandlers).
func buildDependencies() httpapi.Dependencies {
	deps := httpapi.Dependencies{}

	cfg, err := persistence.FromEnv(os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "⚠ no database configured (POSTGRES_PASSWORD/DATABASE_URL unset); serving /health only")
		return deps
	}

	ctx := context.Background()
	store, err := persistence.New(ctx, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "⚠ persistence unavailable:", err)
		return deps
	}
	deps.People = personAdapter{db: store}
	return deps
}

func envDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
