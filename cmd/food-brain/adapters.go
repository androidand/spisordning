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
	"github.com/androidand/spisordning/internal/ingredients"
	"github.com/androidand/spisordning/internal/persistence"
	"github.com/androidand/spisordning/internal/service"
)

// buildDependencies wires the persistence-backed services the HTTP layer exposes.
// It degrades gracefully: if Postgres isn't configured or unreachable, only the
// /health endpoint is served (resource routes are nil-guarded in RegisterHandlers).
// External client services (ingredients, stores) are wired only when their
// environment variables are set; missing clients result in nil service entries.
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
	deps.Planning = service.NewPlanning(store)
	deps.Pantry = service.NewPantry(store)

	// External clients are optional — only wired when configured.
	var slv *ingredients.Client
	if slvURL := os.Getenv("SLV_BASE_URL"); slvURL != "" {
		slv = ingredients.NewLivsmedelsverket(slvURL)
	}
	var dabas *ingredients.DabasClient
	if os.Getenv("DABAS_ENABLED") != "" {
		dabas = ingredients.NewDabas()
	}
	var mpk *ingredients.MPKClient
	if os.Getenv("MPK_ENABLED") != "" {
		mpk = ingredients.NewMatpriskollen()
	}
	deps.Ingredients = service.NewIngredients(store, slv, dabas, mpk)
	deps.Stores = service.NewStores(mpk)
	return deps
}
