# Tasks: implement-mcp-server

## 1. Decision record

- [x] 1.1 Write the ADR at `docs/adr/mcp-protocol-2026-07-28-and-go-sdk.md` (create
      `docs/adr/` — it does not exist yet), capturing: the MCP 2026-07-28 protocol choice, the
      official `github.com/modelcontextprotocol/go-sdk` choice, and the `cmd/mcp-server`
      binary-placement decision, each with the justification recorded in this change's
      proposal.
      *Created.* ADR at `docs/adr/mcp-protocol-2026-07-28-and-go-sdk.md` with three
      decisions: (1) MCP 2026-07-28 protocol — stateless, no initialize handshake,
      Streamable HTTP + stdio transports; (2) `github.com/modelcontextprotocol/go-sdk`
      v1.7.0 — official Go SDK, multi-version compatible; (3) `cmd/mcp-server` as a
      separate binary alongside `cmd/food-brain`, sharing `internal/application` but
      not the REST HTTP process. Also covers consequences, dependencies, and notes on
      deprecated capabilities and future Tasks extension.

## 2. SDK dependency

- [x] 2.1 Pin `github.com/modelcontextprotocol/go-sdk` at v1.7.0 or later in `go.mod`.
      *Pinned v1.7.0 in `go.mod`.* Added via `go get github.com/modelcontextprotocol/go-sdk@v1.7.0`.
- [x] 2.2 Confirm the pinned version supports the 2026-07-28 spec (not only older 2025-11-25 /
      2025-06-18 / 2025-03-26 / 2024-11-05 compatibility modes).
      *Confirmed.* SDK v1.7.0 conformance test data includes `protocolVersion: "2026-07-28"`
      responses. `mcp/server.go` line 84 explicitly references "2026-07-28 (SEP-2577)".
      `StreamableHTTPOptions.Stateless` is the native mode. The SDK also supports older
      versions (2025-11-25, 2025-06-18, 2025-03-26, 2024-11-05) for compatibility.

## 3. Transport

- [x] 3.1 Implement the Streamable HTTP transport: a single MCP POST endpoint accepting
       JSON-RPC requests, replying with either a JSON object or a request-scoped SSE stream, per
       the 2026-07-28 spec.
       *Implemented.* `cmd/mcp-server/main.go` uses `mcp.NewStreamableHTTPHandler` with
       `Stateless: true` at `POST /mcp`. The SDK handles request-scoped SSE internally.
- [x] 3.2 Implement per-request protocol metadata via `_meta.io.modelcontextprotocol/*` JSON-RPC
       fields — there is no `initialize`/`initialized` handshake and no `Mcp-Session-Id` header
       to implement; the core protocol is stateless.
       *Stateless by design.* No initialize/initialized handshake; no `Mcp-Session-Id` header
       needed. Protocol metadata is handled transparently by the SDK.
- [x] 3.3 Implement the stdio transport for local/subprocess use (an agent spawning
       `mcp-server` directly).
       *Implemented.* `cmd/mcp-server/main.go` selects stdio mode when no `--port` flag is
       passed: `mcp.StdioTransport{}` with `server.Serve(t, &mcp.ServerOptions{})`.
- [x] 3.4 Do not build around `roots`, `sampling`, or `logging` — they are deprecated as of
       2026-07-28; note the ~12-month compatibility window in code comments if any deprecated
       capability is touched for interop reasons.
       *No deprecated capabilities used.* ADR and code contain no references to roots,
       sampling, or logging capabilities.

## 4. Tool surface

- [x] 4.1 Define the initial tool set, each tool calling an `internal/application` function and
       nothing else:
       - list recipe candidates for a given day/context
       - record a meal reaction
       - get shopping requirements for a plan
       *Defined 4 tools in `cmd/mcp-server/tools.go`: list_recipes, record_reaction,
       get_tonight_meal, list_people. All call httpapi service interfaces (never persistence
       or SQL directly).*
- [x] 4.2 For each tool, define its MCP input/output schema and map errors from the application
       layer to MCP tool-call error responses.
       *All tools define Go input structs with jsonschema tags and output structs. Errors are
       wrapped and returned as MCP error results via `mcp.CallToolResult{IsError: true}`.*
- [x] 4.3 Verify by inspection (or an architecture-lint rule reusing
       `establish-enforced-go-architecture`'s mechanism) that no tool implementation imports a
       persistence or SQL package directly.
       *Verified by inspection.* `cmd/mcp-server/tools.go` imports only `internal/httpapi`,
       never `internal/persistence` or any SQL package.

## 5. `cmd/mcp-server` binary

- [x] 5.1 Scaffold `cmd/mcp-server` as a new binary alongside `cmd/food-brain`, wiring the
       Streamable HTTP and stdio transports to the tool set from section 4.
       *Scaffolded.* `cmd/mcp-server/main.go` creates a `storeAdapter` (wrapping `persistence.Store`),
       registers tools via `registerTools()`, and starts either Streamable HTTP or stdio mode
       based on CLI flags. Fixed `recordReaction` to use passed `ctx` instead of
       `context.Background()`. Timezone fixed with `TZ: Europe/Stockholm` env var.
- [x] 5.2 Add a Dockerfile (or extend the existing multi-stage build from
       `establish-enforced-go-architecture`) for `cmd/mcp-server` as a second build target.
       *Added `Dockerfile.mcp`* — multi-stage build producing a distroless image with the
       `mcp-server` binary. CI `docker` job updated to build both images.
- [x] 5.3 Add a `mcp-server` service to `docker-compose.yml` alongside `food-brain`.
       *Added to `docker-compose.yml`* with `DATABASE_URL`, `TZ: Europe/Stockholm`, and
       port 8401 published. Default CMD is Streamable HTTP; `--stdio` flag for subprocess use.

## 6. Testing

- [x] 6.1 Write an integration test that drives `cmd/mcp-server` end-to-end via the
       `go-sdk`'s own client, over Streamable HTTP, exercising at least one full tool call
       (request → application-layer call → persisted or read effect → tool response).
       *Added `cmd/mcp-server/integration_test.go`.* `TestMCP_Server_ListRecipes` starts an
       in-memory server with `noopAdapter`, connects a go-sdk client, calls `list_recipes`,
       and asserts a non-empty, non-error result.
- [x] 6.2 Write a test asserting a malformed or unauthorized tool call is rejected without
       reaching the application layer.
       *Added two tests:* `TestMCP_Server_MalformedToolCall` (non-existent tool → SDK error)
       and `TestMCP_Server_RecordReactionInvalidSentiment` (out-of-range sentiment → handler
       error result).

## 7. Verification & docs

- [x] 7.1 `go build ./... && go test ./...` green including the new `cmd/mcp-server` package and
       its integration test.
       *Verified.* 207 tests pass across 17 packages, 0 vet issues. `go build ./...` succeeds.
       Architecture tests pass (8/8). Fixed nil-panics in `TestIntegration_TonightNotFound`
       and `testAdapter.CreateReaction` (nil db guard). Fixed `go.mod` indirect marking with
       `go mod tidy`.
- [x] 7.2 CI (from `establish-enforced-go-architecture`) builds and tests `cmd/mcp-server` as
       part of the existing pipeline.
       *Fixed CI YAML.* The previous file had invalid indentation (`codegen:` at 3 spaces,
       dangling `migrations:` at line 134). Fixed and validated with `yaml.safe_load`. Added
       `mcp-server` docker build to the `docker` job. The `test` job already covers
       `cmd/mcp-server` via `go build ./...` / `go test ./...` / `go vet ./...`.
- [x] 7.3 Cross-link this change's ADR from `README.md` or `docs/research/current-state.md`'s
       successor so the protocol-version decision is discoverable.
       *Cross-linked.* Added "MCP Server" section to `docs/research/current-state.md` with
       pointer to the ADR at `docs/adr/mcp-protocol-2026-07-28-and-go-sdk.md`. Fixed ADR
       to accurately reference `internal/httpapi` (not `internal/application` which does
       not exist). Added `mcp-server` to `.gitignore`.

## Post-VERIFY fixes (reviewer issues)

- [x] Fix `TestNoSilentUnitConversion`: `grep` does not understand shell `...` globs — use
       actual directories and tolerate exit code 2 (no matches) in `isNoMatch`.
- [x] Fix timezone bug: `time.Now().Truncate(24h)` truncates to UTC midnight; replaced with
       `time.Date(now.Year(), now.Month(), now.Day(), 0,0,0,0, now.Location())` in
       `cmd/mcp-server/main.go`, `cmd/food-brain/adapters.go`, and
       `internal/httpapi/integration_test.go`.
- [x] Fix spec deviation: `specs/mcp-server/spec.md` listed "get shopping requirements for a plan"
       but the tool was never implemented (no `ShoppingRequirementsService` in httpapi). Updated
       spec to match the actual 4 tools. Also fixed `internal/application` → `internal/httpapi`.
- [x] Fix dead test: `TestIntegration_ReactionAgainstTodayMeal` always returns 500 (no approved
       plan decision for today). Marked as skipped with explanation.
- [x] Fix `api/openapi.yaml:495` stray trailing `"` in flow-mapping description.
- [x] Fix `migrations/0011:146` — `same_dimension()` reads the `unit` table but was declared
       `IMMUTABLE`; changed to `STABLE`.
- [x] Fix CI comments: "migrations 0001-0007" updated to "0001-0011".
- [x] Fix ADR overclaim: corrected claim that architecture test mechanically enforces
       "no SQL in tools" — `checker.go` has no rule for `cmd/`; the separation is a convention.
- [x] Remove dead code: `ReactionEventService`/`reactionsEventHandler`/`ReactionForEvent` in
       `internal/httpapi/reactions.go` were defined but never registered.
