# pantry-inventory (delta)

## ADDED Requirements

### Requirement: A lot is the canonical model of physical household inventory

The system SHALL represent physical household inventory as `InventoryLot` rows, each scoped to
a `Product`, an `InventoryLocation`, a quantity, and a confidence tier. The system SHALL NOT use
a single mutable quantity field on `Product` (e.g. `products.current_quantity`) as the model of
how much of something a household has.

#### Scenario: Two lots of the same product in different locations are distinct

- **WHEN** a household has milk in the fridge and a backup carton in the freezer
- **THEN** these are represented as two separate `InventoryLot` rows, each with its own
  location, quantity, and confidence
- **AND** no single field on the `Product` row aggregates them into one quantity

### Requirement: Inventory events are an append-only ledger

The system SHALL record every inventory mutation (`PURCHASE`, `CONSUME`, `DISCARD`, `ADJUST`,
`TRANSFER`, `MARK_EMPTY`, `OPEN`) as an immutable `InventoryEvent` row. A correction to a
previously recorded event SHALL be made by recording a new event, never by updating or deleting
an existing one.

#### Scenario: A mis-recorded consumption is corrected by a new event, not an edit

- **WHEN** a household member records consuming 200g of an ingredient but meant 100g
- **THEN** the correction is recorded as a new `ADJUST` event
- **AND** the original `CONSUME` event row is unchanged

#### Scenario: A lot's current state changes only via an event

- **WHEN** any change is made to an `InventoryLot`'s quantity or confidence
- **THEN** that change occurs in the same transaction as an `InventoryEvent` insert
- **AND** no code path updates an `InventoryLot` row without a corresponding event

### Requirement: Inventory confidence is one of four tiers, stored and justified

An `InventoryLot`'s confidence SHALL be exactly one of `EXACT`, `LIKELY`, `ESTIMATED`, or
`UNKNOWN`, stored on the lot for querying. Every `InventoryEvent` that sets or changes a lot's
confidence SHALL record the observation source that justifies it.

#### Scenario: A receipted purchase is exact

- **WHEN** a `PURCHASE` event is recorded with a known quantity from a purchase receipt
- **THEN** the resulting lot's confidence is `EXACT`
- **AND** the event's source records that it came from a purchase receipt

#### Scenario: An untouched opened lot is eligible for decay, not silently downgraded

- **WHEN** a lot is opened (`OPEN` event) and later observed again after significant time
  without an intervening event
- **THEN** its confidence is not rewritten by a background process without a corresponding
  `InventoryEvent`
- **AND** any automatic decay is itself recorded as an `ADJUST` event with an
  `inferred_decay` source

#### Scenario: Querying uncertain inventory is a direct query

- **WHEN** a household wants to see all lots it is no longer sure about
- **THEN** the system answers directly from `InventoryLot.confidence = 'UNKNOWN'`
- **AND** does not require replaying event history at query time to determine confidence

### Requirement: Inventory events use concrete typed references, not generic polymorphism

`InventoryEvent` SHALL reference `InventoryLot`, `Product`, and `InventoryLocation` (source and
destination) through concrete, checkable foreign key columns scoped to what each event kind
needs. The system SHALL NOT use a generic `entity_type`/`entity_id`/`value` shape for inventory
events.

#### Scenario: A transfer references both locations by real foreign key

- **WHEN** a `TRANSFER` event moves a lot from the freezer to the fridge
- **THEN** the event row has a real foreign key to the freezer location and a real foreign key
  to the fridge location
- **AND** no polymorphic entity-reference column is used to express either

### Requirement: A barcode SHALL NOT define product identity

A barcode (GTIN/EAN) provides convenient product identification and SHALL be normalized and
resolved to a `Product` reference. No table in this capability SHALL treat a raw barcode as
identity: `InventoryLot` and `InventoryEvent` SHALL reference `product_id`, never a barcode
value directly.

#### Scenario: Scanning a barcode resolves to a product, not a new identity

- **WHEN** a household member scans a product's barcode while recording a purchase
- **THEN** the barcode is normalized and resolved to an existing or newly registered `Product`
- **AND** the resulting `InventoryLot`/`InventoryEvent` rows reference that `Product` by id, not
  by the barcode

#### Scenario: The same product with two different barcodes still resolves to one product

- **WHEN** two packages of the same product carry different regional barcodes
- **THEN** both barcodes resolve, via `ProductIdentifier`, to the same `Product`
- **AND** no duplicate `Product` is created solely because the barcode differs

#### Scenario: An unresolvable barcode falls back to manual entry, never silent identity invention

- **WHEN** a scanned barcode resolves through no lookup source (existing identifier, Open Food
  Facts, retailer lookup)
- **THEN** the system prompts for manual product entry
- **AND** does not fabricate a `Product` or treat the raw barcode as a stand-in identity
