# ADR: MCP protocol 2026-07-28, the official Go SDK, and a `cmd/mcp-server` binary

- **Status:** Accepted
- **Date:** 2026-08-20
- **Change:** `openspec/changes/implement-mcp-server`
- **Source material:** `openspec/changes/implement-mcp-server/proposal.md`

## Context

`PLAN.md`'s "AI Provider" section is unambiguous about the shape AI interaction must take:
"AI SHALL call application-layer tools. Never expose unrestricted SQL." Wiring the actual AI
provider (Olla, an OpenAI-compatible endpoint) is Epic G's territory and is out of scope here.
What this change establishes is the *infrastructure* that makes "AI calls application-layer
tools" structurally true: an MCP (Model Context Protocol) server that exposes Spisordning's
application layer as callable tools rather than a database connection.

This is foundational infrastructure, so it belongs to Epic A alongside the HTTP API and the
persistence layer established by `establish-enforced-go-architecture`. The server reuses the
same application layer (`internal/planning`) that the REST handlers in `internal/httpapi` call
into; the shared application layer — not a shared process — is the invariant that keeps AI away
from raw SQL.

Three choices are expensive to reverse later, so they are locked in here: the MCP protocol
version, the SDK, and where the server binary lives.

## Decision 1 — Protocol version: MCP 2026-07-28

The current MCP specification is **2026-07-28**, a major breaking change from the earlier
stateful versions. It removes the `initialize`/`initialized` handshake and the `Mcp-Session-Id`
header entirely: the protocol core is now **stateless**, and all protocol metadata (version,
capabilities) travels per-request in `_meta.io.modelcontextprotocol/*` JSON-RPC fields. The
standard transport is **Streamable HTTP** — each message is an HTTP POST to a single MCP
endpoint, and the reply arrives as either a JSON object or a request-scoped SSE stream. **stdio**
is also supported for local/subprocess use.

`roots`, `sampling`, and `logging` are **deprecated** as of 2026-07-28 (SEP-2577). They remain
functional for roughly twelve months for compatibility, but this server does not depend on them
for its tool surface. A new formal extensions framework also ships (Tasks, MCP Apps, Skills over
MCP); none are required for the initial tool surface, though Tasks is worth revisiting once
longer-running operations (a full weekly plan run) are exposed as tools.

**Justification.** Building against the deprecated stateful handshake now would force a rewrite
inside the ~12-month compatibility window. Building against 2026-07-28 costs nothing extra today
because this is a greenfield server — there is no existing stateful client to stay compatible
with.

## Decision 2 — SDK: the official Go SDK

We use `github.com/modelcontextprotocol/go-sdk` (pinned at v1.7.0), the official Go SDK,
maintained in collaboration with Google. v1.7.0 supports the 2026-07-28 spec alongside the older
2025-11-25 / 2025-06-18 / 2025-03-26 / 2024-11-05 versions for compatibility.

**Justification.** It is the only Go SDK with official protocol backing, it already supports the
target spec version, and its compatibility range means Spisordning is not locked out of talking
to older MCP clients if one is ever needed.

## Decision 3 — Where the server lives: a new `cmd/mcp-server` binary

The MCP server is a **new `cmd/mcp-server` binary alongside `cmd/food-brain`**, not a subcommand
of `food-brain` and not a route mounted inside the REST HTTP server. It shares the same
application layer (`internal/planning`) that the REST handlers call into, reached through a thin
adapter package (`internal/mcptools`) that defines the tool schemas and the service interfaces
the tools call.

**Reasoning.**

- MCP tool implementations and REST handlers calling the *same* application functions is what
  structurally guarantees "AI never touches raw SQL." A shared application layer, not a shared
  process, is the invariant that matters.
- A separate binary can run over stdio for local agent tooling (an agent spawning `mcp-server`
  as a subprocess) without standing up the full REST HTTP stack — which an in-process,
  Streamable-HTTP-only design would preclude.
- The MCP protocol is young (2026-07-28 shipped this cycle) and likely to keep moving. Decoupling
  its lifecycle and versioning from the REST API's own stability requirements avoids forcing a
  REST redeploy every time MCP plumbing changes.
- Cost: a second binary to build and containerize. Accepted — `establish-enforced-go-architecture`
  already sets up the Dockerfile/CI patterns this change reuses for a second target.

## Consequences

- `go.mod` gains `github.com/modelcontextprotocol/go-sdk` (v1.7.0) as the first MCP dependency.
- New `cmd/mcp-server` binary: Streamable HTTP (stateless, per-request `_meta`) plus stdio, with
  a `/health` endpoint.
- New `internal/mcptools` package: a thin adapter that owns the tool input/output schemas, the
  service interfaces the tools call, and the mapping of application-layer errors to MCP
  tool-call errors. It imports the SDK and nothing from `persistence`; the composition root
  (`cmd/mcp-server`) is the only place that bridges `internal/mcptools` to
  `internal/persistence` and `internal/planning`. `internal/architecturetest` classifies
  `internal/mcptools` and enforces that it never imports persistence, clients, httpapi, or cmd.
- The initial tool surface (list recipe candidates, record a meal reaction, get shopping
  requirements) calls application-layer functions only; no tool constructs or executes SQL.
- `roots`, `sampling`, and `logging` are not load-bearing; if any is touched later for interop,
  the ~12-month compatibility window must be noted in code comments.
