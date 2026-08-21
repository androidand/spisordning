# barcode-scanning (delta)

## ADDED Requirements

### Requirement: Barcode scanning is researched before it is designed or built

The system SHALL NOT implement mobile camera scanning or dedicated scanner hardware support
until this change's research (mobile scanning approaches, dedicated HID scanner hardware,
check-in flow, check-out flow) is documented in `docs/research/barcode-scanning-devices.md`.

#### Scenario: No scanning code lands ahead of the research doc

- **WHEN** a future change proposes implementing camera-based or hardware-scanner barcode input
- **THEN** it cites `docs/research/barcode-scanning-devices.md` as its grounding
- **AND** it does not invent scanning-device behavior unverified by that research

### Requirement: Scanning supports both check-in and check-out of inventory

The barcode-scanning capability SHALL support two distinct workflows: check-in (a scan
identifies a product and leads to `RecordPurchase`) and check-out (a scan identifies an existing
`InventoryLot` and leads to `RecordConsume`, `RecordDiscard`, or `RecordMarkEmpty`). Neither
workflow SHALL be designed as a special case of the other, since check-out requires lot
disambiguation that check-in does not.

#### Scenario: A check-in scan leads toward recording a purchase

- **WHEN** a household member scans a product while putting groceries away
- **THEN** the flow resolves toward `implement-pantry-inventory`'s `RecordPurchase` command,
  using its existing `LookupBarcode` resolution chain

#### Scenario: A check-out scan leads toward recording consumption or disposal

- **WHEN** a household member scans an empty product's barcode while discarding it
- **THEN** the flow resolves toward `implement-pantry-inventory`'s `RecordMarkEmpty` (or
  `RecordConsume`/`RecordDiscard` as appropriate) against a specific, disambiguated
  `InventoryLot`

### Requirement: Two independent scanning input methods are supported

The barcode-scanning capability SHALL support both a mobile device's camera and a dedicated
barcode scanner (USB or Bluetooth HID) as input methods, since the household has described
distinct use cases for each (a phone always at hand vs. faster/more reliable repeated scanning
during a full check-in session). Neither input method SHALL be a hard prerequisite for the
other.

#### Scenario: A household member with only a phone can still scan

- **WHEN** no dedicated scanner hardware is present
- **THEN** camera-based scanning via a mobile device remains fully functional

#### Scenario: A dedicated scanner requires no bespoke driver integration

- **WHEN** a supported USB or Bluetooth HID barcode scanner is used
- **THEN** it is treated as keyboard input into the active scan-target field, requiring no
  device-specific driver code in Spisordning
