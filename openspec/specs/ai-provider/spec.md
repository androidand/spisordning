# ai-provider

## Purpose

Define an AI provider abstraction for any OpenAI-compatible chat-completion endpoint so
Spisordning's deterministic core never depends on one AI deployment, and AI output can vary
presentation/ordering only within bounds deterministic logic already established. The local
Olla proxy is the primary implementation; AI-driven data access goes through application-layer
tools, never raw SQL.
## Requirements
### Requirement: The AI provider abstraction SHALL target any OpenAI-compatible endpoint

The system SHALL define an AI provider interface that any OpenAI-compatible chat-completion endpoint can satisfy, parameterized by base URL, model, and timeout rather than hard-coded to one deployment. The local Olla proxy SHALL be the primary, initial implementation, but SHALL NOT be the only shape the interface can accept.

#### Scenario: A second provider implementation satisfies the same interface

- **WHEN** a test-only fake OpenAI-compatible provider is implemented against the interface
- **THEN** it can be substituted for the Olla client at any call site without changing caller code
- **AND** existing behavior contracts (e.g. `Explain`, `ProposeOrder`) are exercised identically
  through both

#### Scenario: Olla remains the default, already-deployed backend

- **WHEN** the system is configured with no explicit provider override
- **THEN** it uses the local Olla proxy at its existing configured endpoint
- **AND** no behavior regresses relative to `internal/llm/olla.go`'s current, tested contract

### Requirement: AI SHALL only call application-layer tools, never raw SQL

The AI provider abstraction SHALL NOT expose, accept, or construct a direct database connection or raw SQL execution path. Any action an AI-driven feature takes against Spisordning's data SHALL go through application-layer tools (owned by Epic A's `implement-mcp-server`), never through unrestricted SQL access. This is the same rule as `implement-mcp-server`'s "MCP tools never execute raw SQL" requirement; the two SHALL be kept in lockstep and neither redefines what counts as an application-layer tool.

#### Scenario: No SQL execution path exists in the provider package

- **WHEN** the `internal/llm` package (or its successor) is inspected
- **THEN** it contains no database driver import, no `*sql.DB`, and no SQL string construction
- **AND** any data access an AI-driven feature needs is expressed as a call to an
  application-layer tool, not a query the AI provider builds itself

#### Scenario: This invariant is phrased consistently with the MCP tool surface's own invariant

- **WHEN** this capability's "tools only, never raw SQL" requirement is compared against Epic A's
  `implement-mcp-server` equivalent requirement
- **THEN** both state the same rule in compatible language
- **AND** neither independently redefines what counts as an application-layer tool

### Requirement: The AI provider is additive, never load-bearing

An AI provider response SHALL NOT determine feasibility, override a deterministic decision, or be the sole source of truth for any Spisordning domain outcome. Its output MAY vary presentation, phrasing, or ordering within a set already established as valid by deterministic logic.

#### Scenario: Provider failure degrades gracefully

- **WHEN** the configured AI provider is unreachable or returns an unparseable response
- **THEN** the calling feature falls back to its deterministic behavior
- **AND** no request fails solely because the AI provider was unavailable

#### Scenario: Provider output cannot introduce a result deterministic logic rejected

- **WHEN** an AI provider proposes a variation outside the bounds already established as feasible
  by deterministic logic
- **THEN** the system rejects that variation before it takes effect