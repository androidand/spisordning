# Implement MCP server

## Why

`PLAN.md`'s "AI Provider" section is unambiguous about the shape AI interaction must take: "AI
SHALL call application-layer tools. Never expose unrestricted SQL." Wiring the actual AI
provider (Olla, OpenAI-compatible) is Epic G's territory and out of scope here. But the
*infrastructure* that makes "AI calls application-layer tools" true — an MCP (Model Context
Protocol) server exposing Spisordning's application layer as callable tools rather than a
database connection — is foundational infrastructure and belongs to Epic A alongside the HTTP
API and persistence layer established in `establish-enforced-go-architecture`.

This proposal is written as an ADR-worthy decision record, not just a task list, because it
locks in two choices that are expensive to reverse later: the MCP protocol version and the SDK.

### Decision 1: protocol version — MCP 2026-07-28

The current MCP specification is **2026-07-28**, a major breaking change from earlier stateful
versions. It removes the `initialize`/`initialized` handshake and the `Mcp-Session-Id` header
entirely — the protocol core is now **stateless**; all protocol metadata (version, capabilities)
travels per-request in `_meta.io.modelcontextprotocol/*` JSON-RPC fields. The standard transport
is **Streamable HTTP** (each message is an HTTP POST to a single MCP endpoint; replies arrive as
a JSON object or a request-scoped SSE stream), with stdio also supported for local/subprocess
use. Servers still offer Resources/Prompts/Tools; clients may offer Elicitation. `roots`,
`sampling`, and `logging` are **deprecated** as of this version (supported for roughly 12 more
months for compatibility, but Spisordning's design should not depend on them). A new formal
**extensions framework** exists, with Tasks (async long-running operations with polling/durable
handles), MCP Apps (interactive UI elements), and Skills over MCP as notable extensions — none
are required for the initial tool surface below, but Tasks is worth revisiting once
longer-running operations (e.g. a full weekly plan run) are exposed as tools.

Justification: building against the deprecated stateful handshake now would mean a rewrite
within the ~12-month compatibility window; building against 2026-07-28 costs nothing extra today
since this is a greenfield server.

### Decision 2: SDK — the official Go SDK

`github.com/modelcontextprotocol/go-sdk`, maintained in collaboration with Google, is the
official Go SDK; v1.7.0+ supports 2026-07-28 alongside the older 2025-11-25 / 2025-06-18 /
2025-03-26 / 2024-11-05 versions for compatibility.

Justification: it is the only Go SDK with official protocol backing, it already supports the
target spec version, and its compatibility range means Spisordning is not locked out of talking
to older MCP clients if one is needed later.

### Decision 3: where the server lives — a new `cmd/mcp-server` binary

Recommendation: a **new `cmd/mcp-server` binary alongside `cmd/food-brain`**, sharing the same
`internal/application` package that `establish-enforced-go-architecture`'s HTTP handlers call
into — not a subcommand of `food-brain`, and not a route mounted inside the REST HTTP server.

Reasoning:

- MCP tool implementations and REST handlers calling the *same* `internal/application` functions
  is what structurally guarantees "AI never touches raw SQL" — a shared application layer, not a
  shared process, is the actual invariant that matters.
- A separate binary can run over stdio for local agent tooling (an agent spawning
  `mcp-server` as a subprocess) without standing up the full REST HTTP stack, which
  Streamable-HTTP-only-in-process would preclude.
- The MCP protocol is young (2026-07-28 shipped this cycle) and likely to keep moving; decoupling
  its lifecycle/versioning from the REST API's own stability requirements avoids forcing a
  REST API redeploy every time MCP protocol plumbing changes.
- Cost: a second binary to build and containerize. Accepted — `establish-enforced-go-architecture`
  already sets up the Dockerfile/CI patterns this change reuses for a second target.

## What Changes

- Pin `github.com/modelcontextprotocol/go-sdk` (v1.7.0+) as `food-brain`'s first MCP dependency.
- New `cmd/mcp-server` binary implementing the MCP 2026-07-28 **Streamable HTTP** transport (a
  single POST endpoint, JSON or request-scoped SSE replies) plus **stdio** for local/subprocess
  use.
- An initial tool surface where every tool calls an existing or newly-added
  `internal/application` function — never persistence or SQL directly. Initial tools (subject to
  what `establish-enforced-go-architecture`'s application layer actually exposes by
  implementation time):
  - list recipe candidates for a given day/context
  - record a meal reaction
  - get shopping requirements for a plan
- An end-to-end integration test that drives the server through the Go SDK's client, exercising
  at least one tool call over Streamable HTTP.
- An ADR at `docs/adr/mcp-protocol-2026-07-28-and-go-sdk.md` recording the three decisions above
  (this proposal's content is the ADR's source material; writing the file itself is a task below,
  not performed as part of authoring this proposal).

## Capabilities

### New Capabilities

- `mcp-server`: an MCP server exposing Spisordning's application layer as callable tools over
  the 2026-07-28 Streamable HTTP (and stdio) transport, with a hard invariant that tools never
  execute raw SQL.

### Modified Capabilities

<!-- none — this is new infrastructure alongside the REST API, not a change to existing
     capabilities -->

## Impact

- `go.mod`: new dependency `github.com/modelcontextprotocol/go-sdk`.
- New: `cmd/mcp-server/`, MCP tool definitions (likely `internal/mcptools` or similar, itself a
  thin adapter over `internal/application`), an integration test.
- New: `docs/adr/` directory (does not exist today) with
  `docs/adr/mcp-protocol-2026-07-28-and-go-sdk.md`.
- Depends on `establish-enforced-go-architecture` for the `internal/application` layer and
  Dockerfile/CI patterns this change reuses for a second build target.
- Sets up the infrastructure Epic G's AI-provider work (`integrate-ai` in `PLAN.md`'s likely
  OpenSpec sequence) will consume; does not itself wire up Olla or any AI provider.
