# ADR: MCP Protocol 2026-07-28 and Go SDK

- **Status**: Proposed
- **Date**: 2026-08-22
- **Related**: `openspec/changes/implement-mcp-server`
- **Supersedes**: none

## Context

`PLAN.md`'s AI section requires that "AI SHALL call application-layer tools. Never expose
unrestricted SQL." The infrastructure that enforces this invariant — an MCP (Model Context
Protocol) server exposing Spisordning's application layer as callable tools — is foundational
and belongs to Epic A alongside the HTTP API and persistence layer.

Two decisions must be locked in early because they are expensive to reverse: the MCP protocol
version and the Go SDK that implements it.

## Decisions

### Decision 1: MCP protocol version — 2026-07-28

We target the **2026-07-28** MCP specification.

This is a major breaking change from earlier stateful versions (2025-11-25, 2025-06-18, etc.).
The key differences:

- **No `initialize`/`initialized` handshake.** The protocol core is now stateless.
- **No `Mcp-Session-Id` header.** All protocol metadata (version, capabilities) travels
  per-request in `_meta.io.modelcontextprotocol/*` JSON-RPC fields.
- **Streamable HTTP** is the standard transport: each message is an HTTP POST to a single
  MCP endpoint; replies arrive as a JSON object or a request-scoped SSE stream.
- **stdio** is also supported for local/subprocess use (agents spawning `mcp-server` as a
  subprocess).
- **Servers still offer Resources/Prompts/Tools; clients may offer Elicitation.**
- **`roots`, `sampling`, and `logging` are deprecated** (supported for ~12 months for
  compatibility, but not load-bearing for Spisordning's design).
- A new **extensions framework** exists (Tasks, MCP Apps, Skills over MCP) — not required
  for the initial tool surface but Tasks is worth revisiting for longer-running operations.

**Justification**: Building against the deprecated stateful handshake would require a
rewrite within the ~12-month compatibility window. Building against 2026-07-28 costs
nothing extra today since this is a greenfield server. The stateless model is simpler and
more aligned with HTTP's native statelessness.

### Decision 2: SDK — the official Go SDK

We use `github.com/modelcontextprotocol/go-sdk` v1.7.0+, maintained in collaboration with
Google, the official Go SDK for the MCP protocol.

**Justification**: It is the only Go SDK with official protocol backing, it supports the
2026-07-28 spec natively, and its compatibility range (also supporting 2025-11-25,
2025-06-18, 2025-03-26, 2024-11-05) means Spisordning is not locked out of talking to
older MCP clients if one is needed later.

Key API surface used by this change:

- `mcp.NewServer(impl, opts)` — create a server instance.
- `mcp.AddTool(server, tool, handler)` — register a tool with automatic JSON Schema
  generation from Go struct tags.
- `server.Run(ctx, transport)` — run the server on a transport.
- `mcp.StdioTransport{}` — stdio transport for local/subprocess use.
- `mcp.NewStreamableHTTPHandler(getServer, opts)` — Streamable HTTP transport as an
  `http.Handler` for integration into any Go HTTP server.
- `mcp.StreamableHTTPOptions{Stateless: true}` — explicitly enable stateless mode.
- `mcp.CallToolRequest`, `mcp.CallToolResult`, `mcp.TextContent` — tool call types.

### Decision 3: Binary placement — `cmd/mcp-server` alongside `cmd/food-brain`

We create a **new `cmd/mcp-server` binary** alongside `cmd/food-brain`, sharing the same
`internal/httpapi` service interfaces that `establish-enforced-go-architecture`'s HTTP
handlers call into.

**Not** a subcommand of `food-brain`. **Not** a route mounted inside the REST HTTP server.

**Justification**:

- MCP tool implementations and REST handlers calling the *same* `internal/httpapi`
  service interfaces is what structurally guarantees "AI never touches raw SQL" — a
  shared application layer, not a shared process, is the actual invariant that matters.
- A separate binary can run over stdio for local agent tooling (an agent spawning
  `mcp-server` as a subprocess) without standing up the full REST HTTP stack.
- The MCP protocol is young (2026-07-28 shipped this cycle) and likely to keep moving;
  decoupling its lifecycle/versioning from the REST API's own stability requirements
  avoids forcing a REST API redeploy every time MCP protocol plumbing changes.
- Cost: a second binary to build and containerize. Accepted — `establish-enforced-go-
  architecture` already sets up the Dockerfile/CI patterns this change reuses for a second
  build target.

## Consequences

### Positive

- **AI safety invariant is structural, not aspirational.** Tools are Go functions that
  call `internal/httpapi` service interfaces — they do not reach persistence or SQL in
  practice. The architecture test enforces this for `internal/` packages but not for
  `cmd/` (the adapter wiring in `cmd/mcp-server/main.go` imports `persistence` directly;
  the separation is a convention enforced by code review).
- **No legacy to maintain.** Starting on 2026-07-28 means no migration from stateful to
  stateless later.
- **Dual transport from day one.** stdio for local agent use, Streamable HTTP for
  containerized/deployed use, both from the same codebase.
- **SDK compatibility.** The Go SDK's multi-version support means we can talk to older
  MCP clients if needed.

### Negative

- **Second binary.** Adds build, test, and container image complexity. Mitigated by the
  existing multi-target Dockerfile pattern from `establish-enforced-go-architecture`.
- **Protocol volatility.** MCP 2026-07-28 is the current spec but will keep evolving.
  We pin to v1.7.0 of the SDK and version-bump only when breaking changes are required.
- **No deprecated capabilities.** We deliberately do not implement `roots`, `sampling`,
  or `logging`. Clients that depend on these will need to use a compatibility layer or
  wait for them to be added later.

## Dependencies

- `establish-enforced-go-architecture` — provides `internal/httpapi` service interfaces,
  Dockerfile patterns, and the architecture test framework this change reuses.
- `github.com/modelcontextprotocol/go-sdk` v1.7.0+ — the official Go SDK for MCP 2026-07-28.

## Notes

- The initial tool surface (list recipes, record a meal reaction, get tonight's meal,
  list people) is intentionally small. Tools are added as `internal/httpapi` service
  interfaces become available.
- Tasks (async long-running operations) are not in the initial surface but the SDK
  supports them — revisit when exposing operations like "run a full weekly plan" that
  may take longer than a single HTTP request.
