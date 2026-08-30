# Expose pantry event ledger

## Why

The pantry persistence layer already implements the full event ledger:

- `RecordPurchase`
- `RecordConsume`
- `RecordDiscard`
- `RecordAdjust`
- `RecordMarkEmpty`
- `RecordOpen`
- `RecordTransfer`

But the service and HTTP layers only expose a subset:

- `Purchase`
- `Consume`

This means a user cannot discard spoiled food, correct an observed quantity, mark a lot empty,
record that a sealed lot was opened, or move a lot between locations through the API or the web
UI. The underlying behavior exists and is tested at the persistence layer, but it is unreachable
from the product.

This change closes that gap by exposing the remaining ledger operations through the service, HTTP
API, OpenAPI, and frontend.

## What Changes

- Extend `dto.PantryService` with:
  - `Discard(ctx, lotID, in)`
  - `Adjust(ctx, lotID, in)`
  - `MarkEmpty(ctx, lotID)`
  - `Open(ctx, lotID, in)`
  - `Transfer(ctx, lotID, in)`
- Extend `service.Pantry` to implement those methods against the existing persistence layer.
- Add HTTP routes:
  - `POST /pantry/lots/{id}/discard`
  - `POST /pantry/lots/{id}/adjust`
  - `POST /pantry/lots/{id}/mark-empty`
  - `POST /pantry/lots/{id}/open`
  - `POST /pantry/lots/{id}/transfer`
- Add OpenAPI request schemas and regenerate Go + web client types.
- Extend the frontend pantry page with actions for discard, adjust, mark empty, open, and
  transfer.

## Capabilities

### New Capabilities

- `pantry-event-ledger`: the full set of pantry lot lifecycle operations is reachable through the
  service, HTTP API, and UI.

## Impact

- **Affected code:**
  - `internal/dto/pantry.go`
  - `internal/service/pantry.go`
  - `internal/httpapi/pantry.go`
  - `internal/httpapi/people.go`
  - `api/openapi.yaml`
  - `internal/openapi/types.gen.go`
  - `web/src/generated/spisordning.ts`
  - `web/src/pages/PantryPage.tsx`
- **No persistence changes:** `internal/persistence/pantry.go` already implements the required
  behavior.
- **No domain changes:** the event kinds, confidence rules, and source semantics already exist.
- **No MCP changes:** this change is about the household product surface, not the MCP tool surface.
- **No planner changes:** pantry reads used by planning/inspiration continue to work as before.