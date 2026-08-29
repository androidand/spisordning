## ADDED Requirements

### Requirement: Plan/resolve progress is streamable over SSE
`internal/httpapi` SHALL expose a Server-Sent Events endpoint that streams progress events while a
plan/resolve operation runs, so a client isn't blind for the multi-minute duration observed with
today's synchronous CLI/HTTP path.

#### Scenario: A client watches resolve progress in real time
- **WHEN** a client opens the SSE progress endpoint for a running plan/resolve operation
- **THEN** it receives an event per item as it resolves (or fails to resolve), rather than only a
  final result after the whole operation completes

#### Scenario: The plain REST endpoint is unaffected
- **WHEN** a client calls the existing synchronous plan endpoint without using SSE
- **THEN** it behaves exactly as it does today — SSE is an additive alternative, not a replacement
