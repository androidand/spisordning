## Context

`internal/persistence/pantry.go` already implements the pantry event ledger as
ledger-plus-projection:

- every quantity-changing command appends an `inventory_event` row and updates the `inventory_lot`
  projection in one transaction,
- `RecordDiscard`, `RecordAdjust`, `RecordMarkEmpty`, `RecordOpen`, and `RecordTransfer` are
  already implemented and tested at the persistence layer,
- `service.Pantry` currently wraps only `Purchase` and `Consume`,
- `httpapi` currently routes only `/pantry/lots/purchase` and `/pantry/lots/{id}/consume`.

The gap is purely exposure: DTOs, service methods, HTTP handlers, OpenAPI schemas, and UI actions.

## Goals / Non-Goals

**Goals:**
- Make every pantry lot lifecycle operation reachable from the HTTP API and the web UI.
- Preserve the existing event semantics: confidence, source, reason, estimated, and quantity
  guards stay in the persistence layer.
- Keep the service layer thin: parse/validate IDs, call persistence, map errors, and return DTOs.
- Regenerate the Go and web client types from the updated OpenAPI contract.

**Non-Goals:**
- Not adding a read endpoint for the raw `inventory_event` history (the projection is the product
  surface; event history is an audit concern).
- Not changing `Consume`'s existing signature or route.
- Not adding MCP tools for these operations.
- Not adding batch operations.
- Not changing the pantry's role in planning or inspiration.

## Decisions

### D1: New service methods return the updated lot

The new operations return the lot's state after the event is applied:

| Method | Returns |
|---|---|
| `Discard` | updated `PantryLot` |
| `Adjust` | updated `PantryLot` |
| `MarkEmpty` | updated `PantryLot` |
| `Open` | updated `PantryLot` |
| `Transfer` | destination `PantryLot` |

This lets the UI refresh a single lot's row without re-fetching the whole location. For
`Transfer`, the returned lot is the destination lot (which may be the same lot ID for a full
transfer, or a new lot ID for a partial transfer).

### D2: Request bodies mirror the persistence command parameters

| Route | Body |
|---|---|
| `POST /pantry/lots/{id}/discard` | `{ quantity, estimated, reason, source }` |
| `POST /pantry/lots/{id}/adjust` | `{ quantity, estimated, reason, source }` |
| `POST /pantry/lots/{id}/mark-empty` | none |
| `POST /pantry/lots/{id}/open` | `{ source }` |
| `POST /pantry/lots/{id}/transfer` | `{ location_id, quantity, source }` |

`source` is required in the body, matching the existing `Consume` and `Purchase` routes. The UI
sends `"manual"` for user-initiated actions.

### D3: Persistence remains authoritative; the service classifies errors

The persistence layer remains the authority for the ledger's invariants (quantity must be positive,
transfer quantity cannot exceed the lot's quantity, adjust target must be non-negative). The service
performs a best-effort pre-read and pre-validation only to classify missing-lot and invalid-input
errors before invoking the persistence command. The handler maps `dto.ErrInvalid` to 400 and
`dto.ErrNotFound` to 404.

### D4: The UI adds compact per-lot actions

The pantry page's lot rows gain a small action set:

- **Consume** (existing)
- **Discard** — prompt for quantity and reason
- **Adjust** — prompt for the observed quantity
- **Mark empty** — one click
- **Open** — one click
- **Transfer** — prompt for destination location and quantity

The actions are intentionally simple and do not expose the `source` field (the UI always sends
`"manual"`).

## Risks / Trade-offs

- **More per-lot actions can clutter the UI.** Mitigated by keeping them compact and using
  prompts for the few operations that need extra input.
- **Returning the updated lot requires an extra read after each event.** This is acceptable at
  household scale and keeps the response shape consistent.
- **No raw event history endpoint.** Users cannot audit the ledger through the API. This is a
  deliberate scope boundary; the projection is sufficient for the product surface today.