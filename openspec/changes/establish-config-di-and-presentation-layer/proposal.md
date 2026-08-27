## Why

`establish-enforced-go-architecture` already put a real, mechanically-enforced 4-layer core in
place (`domain → application → persistence`, `httpapi → application → domain`, infra clients on
the side, checked by `internal/architecturetest` on every `go test ./...`). That foundation is
sound and stays as-is. But three concrete gaps remain against the household's actual target shape
(config/DI, auth-tier modeling, and interface breadth), each grounded in something observed
live today, not speculative:

1. **No dedicated configuration layer.** `os.Getenv(...)` is called ad hoc, directly inside two
   separate composition roots (`cmd/food-brain/adapters.go` and `cmd/mcp-server/adapters.go`),
   duplicating the same parsing logic twice with no shared validation, no single source of truth,
   and nothing injectable for tests. `~/dev/tengil/internal/config` (a single `Config` struct built
   once at startup, injected everywhere) is the concrete reference pattern the household wants
   mirrored here.
2. **Multi-tier retailer auth isn't modeled anywhere in Go.** ICA genuinely has two auth levels —
   `ica-client/src/auth/oauth2.ts` (basic, programmatic OAuth2/PKCE) vs.
   `ica-client/src/auth/ecom-session.ts` (elevated, requires a human-driven Playwright web-login to
   capture commerce-API cookies) — and `expose-shopping-price-and-notes-bridge`'s D3 already had to
   special-case ICA session staleness for exactly this reason. Today that split is invisible above
   the TS adapter layer; spisordning's `internal/icaretailer` treats `ica-adapter` as one opaque
   service with no concept of "this call needs elevated auth" or where a manually-refreshed
   credential would live.
3. **Presentation layer is narrower than the household's stated need.** REST (`internal/httpapi`),
   MCP (`internal/mcptools`), and CLI (`cmd/food-brain`) are real. SSE, WebSockets, and a SPA/React
   frontend do not exist. Today's actual session experience is direct evidence for at least one of
   these: watching `food-brain plan -create-wishlist` resolve ~20 items through a rate-limited
   retailer adapter with zero progress feedback for minutes is exactly the kind of gap SSE closes.

## What Changes

- Add `internal/config`: one `Config` struct, loaded once per binary from environment variables
  (env-only, not YAML — see design.md D1 for why this deliberately does *not* copy tengil's
  file-based config), replacing the duplicated `os.Getenv` calls in both composition roots.
- Add an `AuthTier` concept (`internal/retailer` or a new small `internal/auth` domain package —
  see design.md D2) modeling basic-vs-elevated retailer auth generically, with ICA as the first
  real case; the `Config` layer owns where a manually-refreshed elevated credential is read from.
- Add SSE support to `internal/httpapi` for long-running operations (starting with plan/resolve
  progress) — concrete, motivated by today's observed UX gap, not speculative.
- Explicitly **defer** WebSockets — no concrete use case exists yet (no live multi-user
  collaboration requirement today); revisit if one emerges rather than building it speculatively.
- Scope (design + first slice, not a full build-out) a SPA/React frontend: the API contract it
  consumes already mostly exists (`api/openapi.yaml` + MCP); this change defines the frontend
  project shape and its first read-only slice (this week's plan + shopping list), not a
  feature-complete app.

## Capabilities

### New Capabilities
- `application-config`: a single injected configuration source of truth per binary, replacing
  scattered `os.Getenv` calls.
- `retailer-auth-tiers`: a generic basic/elevated auth-tier model for retailer clients, with ICA as
  the grounding case.
- `sse-progress-streaming`: Server-Sent Events for long-running operations (plan/resolve), exposed
  alongside the existing REST API.
- `web-frontend-first-slice`: the SPA/React project's shape and its first read-only view.

### Modified Capabilities
- None in `openspec/specs/` today model configuration, auth tiers, or a frontend, so this is
  additive.

## Impact

- `cmd/food-brain/adapters.go`, `cmd/mcp-server/adapters.go`: replace scattered `os.Getenv` calls
  with `config.Load()` + field access.
- `internal/icaretailer`, `internal/retailer`: add auth-tier awareness.
- `internal/httpapi`: add an SSE endpoint for plan/resolve progress.
- New: `internal/config`, a small `internal/auth` (or extended `internal/retailer`), and a new
  top-level frontend project (location TBD in design.md).
- Does not touch the meal-planning, shopping/price, or deployment changes already in flight
  (`complete-live-meal-planning`, `expose-shopping-price-and-notes-bridge`,
  `deploy-food-brain-to-proxmox`) — this change is orthogonal infrastructure they can land
  independently of.
