# mcp-server (delta)

## ADDED Requirements

### Requirement: MCP server implements the 2026-07-28 Streamable HTTP transport

The system SHALL expose an MCP server implementing the 2026-07-28 MCP specification's
Streamable HTTP transport (a single POST endpoint accepting JSON-RPC requests, replying with a
JSON object or a request-scoped SSE stream, with per-request protocol metadata carried in
`_meta.io.modelcontextprotocol/*` fields rather than a stateful `initialize` handshake), and
SHALL additionally support the stdio transport for local/subprocess use.

#### Scenario: A tool call over Streamable HTTP succeeds without a handshake

- **WHEN** an MCP client sends a JSON-RPC tool-call request as an HTTP POST to the MCP endpoint,
  with protocol metadata in `_meta.io.modelcontextprotocol/*`
- **THEN** the server processes the request without requiring a prior `initialize` call or a
  `Mcp-Session-Id` header
- **AND** the response arrives as a JSON object or an SSE stream scoped to that request

#### Scenario: The stdio transport serves a local client

- **WHEN** `cmd/mcp-server` is launched as a subprocess communicating over stdio
- **THEN** it accepts and responds to JSON-RPC MCP requests over stdin/stdout using the same
  tool set as the Streamable HTTP transport

### Requirement: MCP tools never execute raw SQL

MCP tools SHALL only call `internal/httpapi` service interfaces to read or mutate Spisordning
state. MCP tools SHALL NOT execute raw SQL or call persistence-layer code directly.

#### Scenario: A tool call is satisfied entirely through the application layer

- **WHEN** an MCP tool (e.g. "record a meal reaction") is invoked
- **THEN** the tool implementation calls an `internal/httpapi` service interface
- **AND** no SQL statement is constructed or executed within the tool implementation itself

#### Scenario: A hypothetical raw-SQL tool is rejected at review/lint time

- **WHEN** a tool implementation is added that imports a persistence or database driver package
  directly
- **THEN** this is treated as a violation of the MCP server's core invariant and SHALL be
  rejected before merge (via code review and, where the architecture-enforcement tooling from
  `establish-enforced-go-architecture` can be extended to cover `cmd/mcp-server`, via CI)

### Requirement: Deprecated stateful capabilities are not load-bearing

The system SHALL NOT depend on `roots`, `sampling`, or `logging` for its initial tool surface,
as these are deprecated in the 2026-07-28 specification.

#### Scenario: Core tool functionality works without deprecated capabilities

- **WHEN** the initial tool set (list recipes, record a meal reaction, get tonight's meal,
  list people) is exercised by a client that does not implement `roots`, `sampling`,
  or `logging`
- **THEN** all four tools function correctly
