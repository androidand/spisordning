// Package main is the entry point for the spisordning MCP server.
//
// Usage:
//
//	# HTTP mode (Streamable HTTP transport)
//	MCP_PORT=8081 food-brain mcp serve
//
//	# stdio mode (for local/subprocess use by an MCP client)
//	food-brain mcp
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/androidand/spisordning/internal/httpapi"
	"github.com/androidand/spisordning/internal/ingredients"
	mcppkg "github.com/androidand/spisordning/internal/mcp"
	"github.com/androidand/spisordning/internal/persistence"
	"github.com/androidand/spisordning/internal/service"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	mode := "stdio"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	switch mode {
	case "serve":
		runHTTP()
	case "stdio", "":
		runStdio()
	default:
		fmt.Fprintf(os.Stderr, "unknown mode %q (want: serve, stdio)\n", mode)
		os.Exit(2)
	}
}

func runHTTP() {
	deps := buildDeps()
	srv := mcppkg.NewServer(deps.Recipes, deps.Meals, deps.Planning, deps.Pantry)

	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)

	addr := envOr("MCP_PORT", "8081")
	if host := os.Getenv("MCP_HOST"); host != "" {
		addr = host + ":" + addr
	}

	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)

	fmt.Printf("MCP server listening on %s\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		os.Exit(1)
	}
}

func runStdio() {
	deps := buildDeps()
	srv := mcppkg.NewServer(deps.Recipes, deps.Meals, deps.Planning, deps.Pantry)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Fprintf(os.Stderr, "MCP server started (stdio mode)\n")
	if err := srv.Run(ctx, &mcp.StdioTransport{}); err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		os.Exit(1)
	}
}

func buildDeps() httpapi.Dependencies {
	cfg, err := persistence.FromEnv(os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "⚠ no database configured; MCP tools that need persistence will return errors")
		return httpapi.Dependencies{}
	}

	store, err := persistence.New(context.Background(), cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "⚠ persistence unavailable:", err)
		return httpapi.Dependencies{}
	}

	deps := httpapi.Dependencies{}
	deps.People = service.NewPeople(store)
	deps.Preferences = service.NewPreferences(store)
	deps.Recipes = service.NewRecipes(store)
	deps.Meals = service.NewMeals(store, nil)
	deps.Planning = service.NewPlanning(store)
	deps.Pantry = service.NewPantry(store)

	var slv *ingredients.Client
	if slvURL := os.Getenv("SLV_BASE_URL"); slvURL != "" {
		slv = ingredients.NewLivsmedelsverket(slvURL)
	}
	var mpk *ingredients.MPKClient
	if os.Getenv("MPK_ENABLED") != "" {
		mpk = ingredients.NewMatpriskollen()
	}
	deps.Ingredients = service.NewIngredients(store, slv, nil, mpk)
	deps.Stores = service.NewStores(mpk)
	return deps
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
