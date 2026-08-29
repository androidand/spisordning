# Spisordning Architecture

## Overview

Spisordning is a self-hosted household food knowledge system with a clean layered
architecture. The Go codebase is organized into distinct layers with strict import
boundaries enforced by `internal/architecturetest`.

## Layer Diagram

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              Presentation                               │
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────────┐  │
│  │   web/ (React)   │  │  cmd/food-brain  │  │    cmd/mcp-server    │  │
│  │   HTTP Client    │  │      (CLI)       │  │      (MCP)           │  │
│  └────────┬─────────┘  └────────┬─────────┘  └──────────┬───────────┘  │
└───────────┼──────────────────────┼───────────────────────┼──────────────┘
            │                      │                       │
┌───────────▼──────────────────────▼───────────────────────▼──────────────┐
│                              HTTP API                                    │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │                    internal/httpapi                               │   │
│  │  (handlers, routing, SSE streaming, OpenAPI)                     │   │
│  └──────────────────────────────────────────────────────────────────┘   │
└───────────────────────────────────┬──────────────────────────────────────┘
                                    │
┌───────────────────────────────────▼──────────────────────────────────────┐
│                              Service Layer                                │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │                      internal/service                             │   │
│  │  (business logic, planning, recommendations, price intelligence) │   │
│  └──────────────────────────────────────────────────────────────────┘   │
└───────────────────────────────────┬──────────────────────────────────────┘
                                    │
┌───────────────────────────────────▼──────────────────────────────────────┐
│                              Data Access                                  │
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────────┐   │
│  │ internal/persist │  │  internal/mealie │  │   internal/grocy     │   │
│  │   (Postgres)     │  │   (Mealie API)   │  │   (Grocy API)        │   │
│  └──────────────────┘  └──────────────────┘  └──────────────────────┘   │
└──────────────────────────────────────────────────────────────────────────┘
```

## Package Structure

```
spisordning/
├── cmd/
│   ├── food-brain/          # CLI + HTTP server entrypoint
│   └── mcp-server/          # MCP server entrypoint
├── internal/
│   ├── architecturetest/    # Import boundary enforcement
│   ├── config/              # Environment-based configuration
│   ├── dto/                 # Data transfer objects + service interfaces
│   ├── grocy/               # Grocy client
│   ├── httpapi/             # HTTP handlers, routing, SSE
│   ├── mealie/              # Mealie client
│   ├── mcp/                 # MCP server
│   ├── persistence/         # Postgres data access
│   ├── retailer/            # Willys retailer client
│   ├── icaretailer/         # ICA retailer client
│   └── service/             # Business logic
├── web/                     # React frontend
├── api/
│   └── openapi.yaml         # OpenAPI specification
├── db/
│   └── migrations/          # Goose migrations
└── docs/                    # Documentation
```

## Key Design Decisions

### 1. Layered Architecture with Strict Boundaries

The codebase enforces a clean layered architecture:

- **Presentation** (`cmd/`, `web/`): User-facing entrypoints
- **HTTP API** (`internal/httpapi`): Request handling, routing, SSE streaming
- **Service** (`internal/service`): Business logic, planning, recommendations
- **Data Access** (`internal/persistence`, `internal/mealie`, `internal/grocy`): External data sources

Import boundaries are enforced by `internal/architecturetest` which runs as part of
`go test ./...`.

### 2. Dependency Injection via Service Interfaces

The `internal/dto` package defines service interfaces (e.g., `RecipesService`,
`MealsService`, `PlanningService`, `PantryService`). Both the HTTP API and MCP
server depend on these interfaces, not concrete implementations. This enables:

- Testability (mock implementations in tests)
- Flexibility (swap implementations without changing consumers)
- Clear separation of concerns

### 3. Environment-Based Configuration

All configuration is read from environment variables via `internal/config`. The
`config.Load()` function is called once at startup and the resulting `Config`
struct is passed to all components that need configuration.

### 4. SSE for Long-Running Operations

Long-running operations (e.g., meal planning) use Server-Sent Events (SSE) to
stream progress to the client. The `POST /plans/run/stream` endpoint emits
phase-level progress events (`started`, `planning`, `resolving`, `wishlist`,
`done`) and the final result.

### 5. OpenAPI-First API Design

The HTTP API is defined in `api/openapi.yaml` first, then implemented in Go.
The React frontend uses a hand-written typed client generated from the OpenAPI
spec.

## External Dependencies

| Service | Purpose | Client Package |
|---------|---------|----------------|
| Postgres 19 | Primary data store | `internal/persistence` |
| Mealie | Recipe management | `internal/mealie` |
| Grocy | Pantry/inventory | `internal/grocy` |
| Willys Adapter | Retailer integration | `internal/retailer` |
| ICA Adapter | Retailer integration | `internal/icaretailer` |

## Deployment

- **Local dev**: `docker-compose.yml` (Postgres + willys-adapter + food-brain + mcp-server)
- **Production**: Tengil (`~/dev/tengil`) with pre-pulled GHCR images
- **CI**: GitHub Actions builds and publishes `ghcr.io/androidand/spisordning/food-brain`
