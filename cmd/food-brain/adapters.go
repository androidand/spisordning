// Package main is the composition root: it owns the only edge that may import both
// the persistence layer and the httpapi layer, wiring service implementations
// into the httpapi.Dependencies struct.
//
// Business logic lives in internal/service; this file is deliberately thin —
// it only constructs services and passes the persistence.Store to them.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/androidand/spisordning/internal/httpapi"
	"github.com/androidand/spisordning/internal/persistence"
	"github.com/androidand/spisordning/internal/service"
)

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

	deps.People = service.NewPeople(store)
	deps.Preferences = service.NewPreferences(store)
	deps.Recipes = service.NewRecipes(store)
	deps.Meals = service.NewMeals(store, nil)
	return deps
}
