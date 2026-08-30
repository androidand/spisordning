# Tasks: expose-pantry-event-ledger

## 1. DTOs and service interface

- [ ] 1.1 Add `PantryDiscardInput` to `internal/dto/pantry.go` with `quantity`, `estimated`,
  `reason`, and `source`.
- [ ] 1.2 Add `PantryAdjustInput` to `internal/dto/pantry.go` with `quantity`, `estimated`,
  `reason`, and `source`.
- [ ] 1.3 Add `PantryOpenInput` to `internal/dto/pantry.go` with `source`.
- [ ] 1.4 Add `PantryTransferInput` to `internal/dto/pantry.go` with `location_id`, `quantity`,
  and `source`.
- [ ] 1.5 Extend `dto.PantryService` with:
  - `Discard(ctx, lotID string, in PantryDiscardInput) (PantryLot, error)`
  - `Adjust(ctx, lotID string, in PantryAdjustInput) (PantryLot, error)`
  - `MarkEmpty(ctx, lotID string) (PantryLot, error)`
  - `Open(ctx, lotID string, in PantryOpenInput) (PantryLot, error)`
  - `Transfer(ctx, lotID string, in PantryTransferInput) (PantryLot, error)`

## 2. Service implementation

- [ ] 2.1 Implement `Pantry.Discard` in `internal/service/pantry.go`:
  - parse the lot ID,
  - call `s.db.RecordDiscard`,
  - read the updated lot with `s.db.GetInventoryLot`,
  - return the DTO.
- [ ] 2.2 Implement `Pantry.Adjust`:
  - parse the lot ID,
  - call `s.db.RecordAdjust`,
  - read and return the updated lot.
- [ ] 2.3 Implement `Pantry.MarkEmpty`:
  - parse the lot ID,
  - call `s.db.RecordMarkEmpty`,
  - read and return the updated lot.
- [ ] 2.4 Implement `Pantry.Open`:
  - parse the lot ID,
  - call `s.db.RecordOpen`,
  - read and return the updated lot.
- [ ] 2.5 Implement `Pantry.Transfer`:
  - parse the lot ID and destination location ID,
  - call `s.db.RecordTransfer`,
  - read and return the destination lot.

## 3. HTTP handlers

- [ ] 3.1 Add `discard`, `adjust`, `markEmpty`, `open`, and `transfer` handlers to
  `internal/httpapi/pantry.go`.
- [ ] 3.2 Each handler SHALL decode the request body (where applicable), call the service, and
  return the updated lot with `200 OK`.
- [ ] 3.3 Map not-found errors to `404` and validation errors to `400`.
- [ ] 3.4 Register the new routes in `internal/httpapi/people.go` under the existing
  `deps.Pantry != nil` block:
  - `POST /pantry/lots/{id}/discard`
  - `POST /pantry/lots/{id}/adjust`
  - `POST /pantry/lots/{id}/mark-empty`
  - `POST /pantry/lots/{id}/open`
  - `POST /pantry/lots/{id}/transfer`

## 4. OpenAPI and codegen

- [ ] 4.1 Add the five new paths to `api/openapi.yaml` under the `pantry` tag.
- [ ] 4.2 Add request schemas: `PantryDiscardNew`, `PantryAdjustNew`, `PantryOpenNew`,
  `PantryTransferNew`.
- [ ] 4.3 Add a `200` response for each path returning `PantryLot`.
- [ ] 4.4 Run `make generate-openapi` and commit `internal/openapi/types.gen.go`.
- [ ] 4.5 Run the web client codegen and commit `web/src/generated/spisordning.ts`.

## 5. Frontend

- [ ] 5.1 Add discard, adjust, mark-empty, open, and transfer mutations to
  `web/src/pages/PantryPage.tsx`.
- [ ] 5.2 Add compact action buttons to each lot row.
- [ ] 5.3 Use prompts or inline inputs for the operations that need extra input (discard quantity
  + reason, adjust quantity, transfer destination + quantity).
- [ ] 5.4 Send `source: "manual"` for all user-initiated actions.
- [ ] 5.5 Invalidate the lot and expiring queries after each successful action.

## 6. Tests

- [ ] 6.1 Add service tests for `Discard`, `Adjust`, `MarkEmpty`, `Open`, and `Transfer` using a
  fake store.
- [ ] 6.2 Add HTTP handler tests for the five new routes, covering success, validation failure,
  and not-found.
- [ ] 6.3 Ensure existing pantry tests still pass.

## 7. Verification

- [ ] 7.1 `go build ./...` succeeds.
- [ ] 7.2 `go vet ./...` succeeds.
- [ ] 7.3 `go test ./...` succeeds.
- [ ] 7.4 `make verify-codegen` succeeds.
- [ ] 7.5 `openspec validate expose-pantry-event-ledger` succeeds.