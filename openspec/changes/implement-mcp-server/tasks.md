# Tasks: implement-mcp-server

## 1. Decision record

- [x] 1.1 Write the ADR at `docs/adr/mcp-protocol-2026-07-28-and-go-sdk.md` (create
      `docs/adr/` — it does not exist yet), capturing: the MCP 2026-07-28 protocol choice, the
      official `github.com/modelcontextprotocol/go-sdk` choice, and the `cmd/mcp-server`
      binary-placement decision, each with the justification recorded in this change's
      proposal.

## 2. SDK dependency

- [x] 2.1 Pin `github.com/modelcontextprotocol/go-sdk` at v1.7.0 or later in `go.mod`.
- [x] 2.2 Confirm the pinned version supports the 2026-07-28 spec (not only older 2025-11-25 /
      2025-06-18 / 2025-03-26 / 2024-11-05 compatibility modes).

## 3. Transport

- [x] 3.1 Implement the Streamable HTTP transport: a single MCP POST endpoint accepting
      JSON-RPC requests, replying with either a JSON object or a request-scoped SSE stream, per
      the 2026-07-28 spec.
- [x] 3.2 Implement per-request protocol metadata via `_meta.io.modelcontextprotocol/*` JSON-RPC
      fields — there is no `initialize`/`initialized` handshake and no `Mcp-Session-Id` header
      to implement; the core protocol is stateless.
- [x] 3.3 Implement the stdio transport for local/subprocess use (an agent spawning
      `mcp-server` directly).
- [x] 3.4 Do not build around `roots`, `sampling`, or `logging` — they are deprecated as of
      2026-07-28; note the ~12-month compatibility window in code comments if any deprecated
      capability is touched for interop reasons.

## 4. Tool surface

- [x] 4.1 Define the initial tool set, each tool calling an `internal/application` function and
      nothing else:
      - list recipe candidates for a given day/context
      - record a meal reaction
      - get shopping requirements for a plan
- [x] 4.2 For each tool, define its MCP input/output schema and map errors from the application
      layer to MCP tool-call error responses.
- [x] 4.3 Verify by inspection (or an architecture-lint rule reusing
      `establish-enforced-go-architecture`'s mechanism) that no tool implementation imports a
      persistence or SQL package directly.

## 5. `cmd/mcp-server` binary

- [x] 5.1 Scaffold `cmd/mcp-server` as a new binary alongside `cmd/food-brain`, wiring the
      Streamable HTTP and stdio transports to the tool set from section 4.
- [x] 5.2 Add a Dockerfile (or extend the existing multi-stage build from
      `establish-enforced-go-architecture`) for `cmd/mcp-server` as a second build target.
- [x] 5.3 Add a `mcp-server` service to `docker-compose.yml` alongside `food-brain`.

## 6. Testing

- [x] 6.1 Write an integration test that drives `cmd/mcp-server` end-to-end via the
      `go-sdk`'s own client, over Streamable HTTP, exercising at least one full tool call
      (request → application-layer call → persisted or read effect → tool response).
- [x] 6.2 Write a test asserting a malformed or unauthorized tool call is rejected without
      reaching the application layer.

## 7. Verification & docs

- [x] 7.1 `go build ./... && go test ./...` green including the new `cmd/mcp-server` package and
      its integration test.
- [x] 7.2 CI (from `establish-enforced-go-architecture`) builds and tests `cmd/mcp-server` as
      part of the existing pipeline.
- [x] 7.3 Cross-link this change's ADR from `README.md` or `docs/research/current-state.md`'s
      successor so the protocol-version decision is discoverable.
