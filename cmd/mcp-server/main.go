// Command mcp-server exposes Spisordning's application layer as MCP (Model
// Context Protocol) tools. It is a separate binary from food-brain (see
// docs/adr/mcp-protocol-2026-07-28-and-go-sdk.md) so it can run over stdio for
// local agent tooling without standing up the REST HTTP stack, and so its
// lifecycle is decoupled from the REST API's.
//
//	mcp-server            — Streamable HTTP (stateless) on SPISORNING_MCP_ADDR (default :8081)
//	mcp-server --stdio    — stdio transport, for an agent spawning it as a subprocess
//
// The server degrades gracefully: without a Postgres connection (POSTGRES_* /
// DATABASE_URL) it serves /health but registers no tools.
package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/androidand/spisordning/internal/config"
	"github.com/androidand/spisordning/internal/mcptools"
	"github.com/androidand/spisordning/internal/mealie"
	"github.com/androidand/spisordning/internal/persistence"
	"github.com/androidand/spisordning/internal/service"
)

func main() {
	stdio := flag.Bool("stdio", false, "serve over stdio instead of Streamable HTTP")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	deps := buildMCPDeps(logger)
	server := newMCPServer(deps, logger)

	if *stdio {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
			logger.Error("stdio server failed", "error", err)
			os.Exit(1)
		}
		return
	}

	cfg := config.Load()
	addr := cfg.SpisordningMCPAddr
	httpServer := &http.Server{Addr: addr, Handler: newMCPHandler(server)}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	logger.Info("mcp-server listening", "addr", addr, "transport", "streamable-http")
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("http server failed", "error", err)
		os.Exit(1)
	}
}

// buildMCPDeps wires the persistence-backed services the tools call. It degrades
// gracefully: if Postgres isn't configured or unreachable, no tools are
// registered (the server still serves /health).
func buildMCPDeps(logger *slog.Logger) mcptools.Dependencies {
	var deps mcptools.Dependencies

	appCfg := config.Load()

	pgCfg, err := persistence.FromEnv(os.Getenv)
	if err != nil {
		logger.Warn("no database configured (POSTGRES_PASSWORD/DATABASE_URL unset); registering no tools")
		return deps
	}

	ctx := context.Background()
	store, err := persistence.New(ctx, pgCfg)
	if err != nil {
		logger.Warn("persistence unavailable; registering no tools", "error", err)
		return deps
	}

	adapter := mcpStoreAdapter{
		db:        store,
		willysURL: appCfg.AdapterURL,
		icaURL:    appCfg.ICAAdapterURL,
		hemkopURL: appCfg.HemkopAdapterURL,
		cfg:       appCfg,
	}
	if appCfg.HasMealie() {
		adapter.recipes = service.NewRecipes(store, mealie.New(appCfg.MealieBaseURL, appCfg.MealieAPIToken))
	} else {
		logger.Warn("no Mealie instance configured (MEALIE_BASE_URL/MEALIE_API_TOKEN unset); structure_recipe not registered")
	}
	adapter.discovery = service.NewDiscovery(store, service.NewRecipeFamily(store), nil)
	deps.Planner = adapter
	deps.Reactions = adapter
	deps.Requirements = adapter
	deps.ShoppingList = adapter
	deps.Compare = adapter
	deps.Wishlist = adapter
	deps.Discovery = adapter
	if adapter.recipes != nil {
		deps.RecipeStructuring = adapter
	}
	return deps
}

// newMCPServer builds the MCP server implementation and registers the tool set.
func newMCPServer(deps mcptools.Dependencies, logger *slog.Logger) *mcp.Server {
	server := mcp.NewServer(
		&mcp.Implementation{Name: mcptools.Name, Version: mcptools.Version},
		&mcp.ServerOptions{Logger: logger},
	)
	mcptools.RegisterTools(server, deps)
	return server
}

// newMCPHandler builds the HTTP handler: /health for liveness and /mcp for the
// stateless Streamable HTTP transport.
func newMCPHandler(server *mcp.Server) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/mcp", mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{Stateless: true}))
	return mux
}
