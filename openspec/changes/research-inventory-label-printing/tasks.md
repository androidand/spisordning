# Tasks: research-inventory-label-printing

## 1. Upstream dependencies

- [ ] 1.1 Confirm `implement-pantry-inventory`'s `InventoryLot` shape (ingredient/product,
      quantity, location, best-before, confidence — including the graduated-specificity D8
      refinement) is stable enough to design label content against
- [ ] 1.2 Read `research-barcode-scanning-devices`' proposal before starting — this change's
      lot-reference barcode must be designed to coexist with, not collide with, that change's
      GTIN check-out scanning

## 2. Printer hardware research

- [ ] 2.1 Identify the exact model of the household's existing Bluetooth Brother label printer
      and confirm its printing API/SDK (Brother's own SDK, generic Bluetooth-printing library,
      or raw command protocol) — this is the primary test target, not a hypothetical candidate
- [ ] 2.2 Test-print a minimal label against the household's actual printer once 2.1 identifies
      the API surface, to validate the connectivity/API assumption before designing further
      against it
- [ ] 2.3 Survey other consumer/small-business thermal label printer options as fallback/
      comparison only (Niimbot, Dymo, Zebra desktop printers) — not required if the household's
      own Brother unit proves sufficient, but useful context for the recommendation
- [ ] 2.4 Evaluate label stock cost/availability and freezer-durability (a label must survive
      frost, moisture, and handling — confirm this is a real differentiator, not assumed;
      confirm what label stock the household's specific Brother model accepts)
- [ ] 2.5 Evaluate Bluetooth pairing reliability in a household kitchen setting for the specific
      printer on hand

## 3. Label content design

- [ ] 3.1 Define label content fields: ingredient/product name, quantity (where known),
      stored-on date, best-before/use-by (where known), destination `InventoryLocation`
- [ ] 3.2 Evaluate barcode vs. QR code for the scannable element — reliability at small label
      sizes, decode reliability off frosty/curved freezer bag surfaces, information density
      needed (a QR can carry a full lot reference directly; a barcode may need a shorter
      internal token resolved server-side)
- [ ] 3.3 Design the label layout (a mockup, not implementation) balancing legibility of
      human-readable text against printable label size constraints from the surveyed hardware

## 4. Lot-reference barcode namespace design

- [ ] 4.1 Define how a lot-reference code is generated (direct `InventoryLot` id encoding vs. a
      short opaque token resolved via lookup) and its lifecycle (does it need to remain valid
      only while the lot exists, or persist for audit purposes after `MARK_EMPTY`)
- [ ] 4.2 Design how this code is distinguishable from a GTIN at scan time (code
      prefix/format, distinct symbology, or a scan-context toggle) — coordinate directly with
      `research-barcode-scanning-devices`' check-out flow design so the two proposals agree on
      one mechanism, not two incompatible ones
- [ ] 4.3 Confirm this namespace does not violate `implement-pantry-inventory`'s "a barcode
      SHALL NOT define product identity" invariant — a lot-reference code identifies a specific
      *lot* (a physical instance), not a *product*, which is a different claim; document why
      this is a legitimate, distinct use, not an exception to that invariant

## 5. Print trigger points

- [ ] 5.1 Design the optional print-on-`RecordPurchase` trigger (printer absence must not block
      or complicate a purchase being recorded — printing is strictly additive)
- [ ] 5.2 Design an explicit "print label for this lot" action independent of purchase timing —
      treat this as a primary goal (freezer meal labeling), not a secondary trigger: it must work
      for any existing `InventoryLot` regardless of how it was created
- [ ] 5.3 Confirm the home-cooked/frozen-meal path end to end: `RecordPurchase` with
      `source: 'home_prepared'` (`implement-pantry-inventory` `design.md` D8) creates the lot,
      then the "print label for this lot" action (5.2) prints it — this is resolved upstream, no
      longer an open scope gap; this task is to verify the two designs actually connect cleanly

## 6. Check-out loop design

- [ ] 6.1 Design the scan-label → resolve-`InventoryLot` → `RecordConsume`/`RecordDiscard`/
      `RecordMarkEmpty` flow, confirming it requires no disambiguation step (unlike
      `research-barcode-scanning-devices`' GTIN check-out flow, by construction)

## 7. Recommendation & docs

- [ ] 7.1 Produce `docs/research/inventory-label-printing.md` recording findings, the printer
      hardware recommendation, label content/layout design, and the lot-reference barcode
      namespace design
- [ ] 7.2 `openspec validate research-inventory-label-printing`
