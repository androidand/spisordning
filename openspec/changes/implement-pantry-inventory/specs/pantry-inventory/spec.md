# pantry-inventory (delta)

## ADDED Requirements

### Requirement: A lot is the canonical model of physical household inventory

The system SHALL represent physical household inventory as `InventoryLot` rows, each scoped to
an `Ingredient` (always) and optionally a `Product`, an `InventoryLocation`, a quantity, and a
confidence tier. The system SHALL NOT use a single mutable quantity field on `Product` (e.g.
`products.current_quantity`) as the model of how much of something a household has.

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

### Requirement: An inventory lot may be recorded at ingredient-level or product-level specificity

An `InventoryLot` SHALL always reference an `Ingredient`. It MAY additionally reference a
specific `Product`; when it does not, the lot is valid at ingredient-only specificity, and the
system SHALL NOT invent or guess a `Product` to satisfy it. A `Product` MAY be attached to an
existing ingredient-only lot at any later time without altering the lot's quantity, location, or
confidence. A `PURCHASE` event whose source is a completed online shopping order SHALL always
carry a specific `Product` reference, since the retailer resolution that produced the order
already determined it.

#### Scenario: A quick manual entry needs only an ingredient

- **WHEN** a household member records "we have mjölk" without selecting a specific product
- **THEN** an `InventoryLot` is created referencing the `Ingredient` for milk
- **AND** the lot's `Product` reference is left unset
- **AND** no `Product` row is fabricated to fill it in

#### Scenario: A lot is refined from generic to specific after the fact

- **WHEN** a household member later identifies exactly which milk product the ingredient-only
  lot refers to
- **THEN** the lot is updated to reference that `Product`
- **AND** the lot's quantity, location, and confidence are unchanged by this refinement

#### Scenario: An online order always creates a product-level lot

- **WHEN** a completed online shopping order's line item creates a `PURCHASE` event
- **THEN** the resulting `InventoryLot` references the specific `Product` the order resolved
- **AND** the lot is never created at ingredient-only specificity for this source

#### Scenario: A home-cooked meal can be recorded as inventory without a retailer purchase

- **WHEN** a household member portions and freezes a home-cooked meal
- **THEN** a `PURCHASE` event with a `home_prepared` source creates the `InventoryLot`
- **AND** the lot is created at ingredient-only specificity, since no retailer `Product` exists
  for a home-cooked dish

### Requirement: Attaching a Product to a lot is scoped by the lot's Ingredient

When refining an ingredient-only `InventoryLot` to a specific `Product`, the system SHALL
present candidate products scoped to the lot's `Ingredient` — products already mapped to that
ingredient, or products matched by name against the ingredient's canonical name — rather than an
unscoped search across the entire product catalog.

#### Scenario: Refining a milk lot only offers milk products

- **WHEN** a household member refines an ingredient-only lot for "mjölk"
- **THEN** the candidate products offered are those already linked to the milk `Ingredient`, or
  matched by name to "mjölk"
- **AND** unrelated products elsewhere in the catalog are not offered

### Requirement: Inventory locations may be typed and nested

The system SHALL allow an `InventoryLocation` to optionally carry a `location_type` (e.g.
cupboard, drawer, fridge, freezer, basement, balcony, breadbox, or other) as a non-authoritative
hint, and to optionally reference a parent `InventoryLocation` to represent physical nesting
(e.g. a freezer inside a basement, a drawer inside a fridge). Location type SHALL NOT be treated
as identity: multiple locations may share the same type. The system SHALL prevent a location
from being configured as its own ancestor.

#### Scenario: A household with two fridges keeps them as distinct locations

- **WHEN** a household registers a kitchen fridge and a garage fridge, both typed `FRIDGE`
- **THEN** both exist as separate `InventoryLocation` rows
- **AND** no system behavior conflates them because they share a type

#### Scenario: A location can be nested inside another

- **WHEN** a household registers a chest freezer located in the basement
- **THEN** the chest freezer's `InventoryLocation` references the basement's `InventoryLocation`
  as its parent
- **AND** querying everything stored "in the basement" includes lots recorded directly against
  the chest freezer

#### Scenario: A location cannot become its own ancestor

- **WHEN** a household member attempts to set a location's parent to itself or to one of its own
  descendants
- **THEN** the system rejects the change
- **AND** the location hierarchy remains a valid tree
