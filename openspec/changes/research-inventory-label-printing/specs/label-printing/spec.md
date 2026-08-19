# label-printing (delta)

## ADDED Requirements

### Requirement: Label printing is researched and designed before it is implemented

The system SHALL NOT implement printer integration, label generation, or a lot-reference
barcode namespace until this change's research is documented in
`docs/research/inventory-label-printing.md`.

#### Scenario: No printing code lands ahead of the research doc

- **WHEN** a future change proposes implementing label printing or lot-reference barcode
  generation
- **THEN** it cites `docs/research/inventory-label-printing.md` as its grounding

### Requirement: A printed label's barcode identifies a specific lot, not a product

A label printed by this capability SHALL encode a reference to a specific `InventoryLot`, not a
manufacturer GTIN/EAN. This is a distinct namespace from product-identifying barcodes and SHALL
be distinguishable from them at scan time, so that check-out scanning never confuses a
label-printed lot reference with a package's own GTIN.

#### Scenario: Scanning a printed label resolves directly to one lot, no disambiguation

- **WHEN** a household member scans a label this system printed for a specific frozen meal
- **THEN** the scan resolves directly to that one `InventoryLot`
- **AND** no disambiguation step (as GTIN scanning may require across multiple lots of the same
  product) is needed

#### Scenario: A label-printed code is never mistaken for a GTIN

- **WHEN** a scan is received during check-out
- **THEN** the system can determine from the code itself whether it is a lot-reference code or a
  manufacturer GTIN
- **AND** routes to lot resolution or GTIN resolution accordingly, never guessing

### Requirement: Printer hardware is optional and its absence never blocks inventory recording

Label printing SHALL be optional household hardware. The absence of a configured printer SHALL
NOT block, degrade, or complicate any `implement-pantry-inventory` command (`RecordPurchase`,
`RecordConsume`, etc.) — printing is strictly an additive action layered on top of an existing
lot.

#### Scenario: A purchase is recorded normally with no printer configured

- **WHEN** a household has no label printer configured
- **THEN** `RecordPurchase` completes exactly as it would for a household that does have one

### Requirement: A printed label surfaces what the household needs to recognize stored food later

A label SHALL include, at minimum, a human-readable identification of the contents and the date
it was stored, so a household member can recognize and date freezer/fridge contents without
opening Spisordning.

#### Scenario: A frozen meal's label is legible without scanning

- **WHEN** a household member looks at a label on a container in the freezer
- **THEN** they can read what it is and when it was stored directly off the label, without
  needing to scan it or open the app
