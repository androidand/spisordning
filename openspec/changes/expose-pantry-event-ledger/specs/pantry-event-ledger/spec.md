# pantry-event-ledger (delta)

## ADDED Requirements

### Requirement: Pantry lots can be reduced through discard, adjust, and mark-empty

The system SHALL expose operations to reduce a pantry lot's quantity through an explicit discard,
an observed-count adjustment, or a mark-empty action. Each operation SHALL append the
corresponding inventory event and update the lot's quantity atomically.

#### Scenario: Discarding part of a lot

- **WHEN** a client calls `POST /pantry/lots/{id}/discard` with a quantity less than the lot's
  current quantity
- **THEN** the lot's quantity is reduced by the discarded amount
- **AND** a `DISCARD` event is appended with the provided reason and source

#### Scenario: Adjusting a lot to an observed quantity

- **WHEN** a client calls `POST /pantry/lots/{id}/adjust` with a new quantity
- **THEN** the lot's quantity is set to the observed value
- **AND** an `ADJUST` event is appended whose `quantity_delta` is the difference between the
  observed value and the lot's prior quantity

#### Scenario: Marking a lot empty

- **WHEN** a client calls `POST /pantry/lots/{id}/mark-empty`
- **THEN** the lot's quantity is set to zero
- **AND** a `CONSUME` event is appended with source `mark_empty`

### Requirement: Pantry lots can be opened

The system SHALL expose an operation to record that a sealed pantry lot has been opened. The
operation SHALL set the lot's `opened_at` timestamp and append an `OPEN` event without changing
the lot's quantity or confidence.

#### Scenario: Opening a sealed lot

- **WHEN** a client calls `POST /pantry/lots/{id}/open`
- **THEN** the lot's `opened_at` is set to the current time
- **AND** an `OPEN` event is appended
- **AND** the lot's quantity and confidence are unchanged

### Requirement: Pantry lots can be transferred between locations

The system SHALL expose an operation to move a pantry lot's quantity (all or part) to a different
location. A full-quantity transfer SHALL move the lot in place; a partial transfer SHALL decrement
the source lot and create a new lot at the destination.

#### Scenario: Transferring the full quantity of a lot

- **WHEN** a client calls `POST /pantry/lots/{id}/transfer` with a quantity equal to the lot's
  current quantity
- **THEN** the lot's location is updated to the destination
- **AND** a `TRANSFER` event is appended
- **AND** no new lot is created

#### Scenario: Transferring part of a lot

- **WHEN** a client calls `POST /pantry/lots/{id}/transfer` with a quantity less than the lot's
  current quantity
- **THEN** the source lot's quantity is reduced by the transferred amount
- **AND** a new lot is created at the destination with the transferred quantity
- **AND** a `TRANSFER` event is appended

### Requirement: The HTTP API exposes the full pantry event ledger

The system SHALL expose HTTP routes for discard, adjust, mark-empty, open, and transfer in addition
to the existing purchase and consume routes. Each route SHALL delegate to the pantry service and
return the updated lot.

#### Scenario: Discarding a lot over HTTP

- **WHEN** a client calls `POST /pantry/lots/{id}/discard` with a valid body
- **THEN** the response includes the updated lot
- **AND** the lot's quantity reflects the discard

#### Scenario: Transferring a lot over HTTP

- **WHEN** a client calls `POST /pantry/lots/{id}/transfer` with a valid destination and quantity
- **THEN** the response includes the destination lot
- **AND** the source lot's quantity is updated if the transfer was partial

### Requirement: The UI exposes pantry lot lifecycle actions

The web UI SHALL provide actions on each pantry lot row for discard, adjust, mark empty, open, and
transfer, in addition to the existing consume action.

#### Scenario: Discarding a lot from the UI

- **WHEN** a user clicks the discard action on a pantry lot and confirms the quantity and reason
- **THEN** the UI calls `POST /pantry/lots/{id}/discard`
- **AND** the lot list refreshes to show the updated quantity