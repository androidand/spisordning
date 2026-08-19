# Research inventory label printing

## Why

The household described a concrete, recurring pain point: food goes into the fridge or (most
often) the freezer, and it becomes hard to remember what's in there and since when, which
directly undermines the point of tracking inventory at all if the physical items aren't legible
without opening Spisordning and searching. The proposed fix is a physical, optional label
printer: print a label carrying human-readable info (what it is, when it was stored) plus a
scannable barcode, stuck on the container/bag itself.

This is genuinely new scope — no printer, label, or barcode-generation research exists anywhere
in this repo. It also introduces a barcode semantic that does not yet exist in this backlog:
`implement-pantry-inventory`'s barcode work (D6) and `research-barcode-scanning-devices` are
both about a **manufacturer's GTIN/EAN**, a product-identifying code that's ambiguous across
multiple lots of the same product. A label this change prints is different in kind: it is
generated *by* Spisordning for one specific `InventoryLot`, so its barcode can encode (or
resolve to) that exact lot directly — unambiguous by construction, and the natural mechanism for
the check-out scan-to-consume loop the household also described ("the milk is empty, scan it,
into recycling" generalizes cleanly to "this freezer bag is empty, scan its label").

Per this backlog's established discipline (research before design before implementation — see
`establish-reference-lab`, `research-and-integrate-ica`), and because printer hardware choice is
a real unknown with cost/compatibility trade-offs, this is a research-first change.

This change **depends on `implement-pantry-inventory`** for the `InventoryLot` shape a label's
content is read from, and is a close sibling of `research-barcode-scanning-devices` (the label's
barcode is read back through the same physical scanning hardware that change researches, on the
check-out path).

**Update (2026-08-19): the household already owns a Bluetooth Brother label printer** and wants
to test against it directly rather than only surveying hardware in the abstract. This grounds
the hardware-survey scope (§2 below) with a concrete first candidate rather than leaving the
choice fully open, and the household has also confirmed printing labels for home-cooked meals
put in the freezer — not just retailer-purchased items — is a primary goal, not an edge case.
`implement-pantry-inventory`'s design has since been amended (`design.md` D8) to resolve the
"how does a home-cooked meal become inventory at all" question this proposal originally flagged
as open: a `RecordPurchase` with `source: 'home_prepared'` creates the lot, no new event kind
needed. This change's own scope is unaffected by that resolution — it still only needs the
resulting `InventoryLot` to exist, whichever `source` created it — but the print-trigger design
below no longer treats the home-cooked-meal case as blocked on an unresolved upstream question.

## What Changes

- Research consumer/small-business thermal label printer options, **starting with the
  household's own Bluetooth Brother printer as the primary/first candidate to test against**
  (connectivity: USB, Bluetooth, or network; other candidates worth surveying as fallback/
  comparison include Niimbot, Dymo, and Zebra desktop printers) and their printing-API surfaces
  (vendor SDK, ESC/POS-style raw commands, or a print-to-image/PDF approach) — for the Brother
  unit specifically, identify the exact model and confirm its Bluetooth printing API/SDK before
  assuming general Brother QL-series documentation applies unmodified.
- Design label content: what a label displays (ingredient/product name, quantity where known,
  stored-on date, best-before/use-by if known, and the storage `InventoryLocation` it's
  destined for) and a scannable code (barcode or QR — evaluate which prints more reliably at
  small label sizes and decodes more reliably off a frosty freezer bag) that resolves to the
  specific `InventoryLot`.
- Define the **lot-reference barcode namespace**: explicitly distinct from a GTIN — it encodes
  (directly or via a short internal token resolved server-side) an `InventoryLot` id, not a
  `Product`. This must not collide with, or be confusable with, GTIN scanning in
  `research-barcode-scanning-devices`' check-out flow — design how the scanning flow tells the
  two apart (e.g. a distinguishable code prefix/format, or a distinct symbology).
- Define **print trigger points**: on a `RecordPurchase` (optionally, not mandatory — a
  printer is described as optional hardware), and an explicit "print label for this lot" action
  independent of purchase timing — this second path is a first-class goal, not a deferred edge
  case, since printing a label for a home-cooked meal portioned and frozen for later is one of
  the household's stated primary use cases. `implement-pantry-inventory` `design.md` D8 already
  resolves how such a lot comes to exist (`RecordPurchase` with `source: 'home_prepared'`); this
  change designs the print action itself, triggerable from any existing lot regardless of how it
  was created.
- Define the **check-out loop**: scanning a lot's printed label resolves directly and
  unambiguously to that `InventoryLot`, feeding `RecordConsume`/`RecordDiscard`/
  `RecordMarkEmpty` — the same command surface `research-barcode-scanning-devices` targets for
  GTIN-based check-out, but without that flow's disambiguation problem.

## Non-Goals

- No printer purchase (the household's Bluetooth Brother unit is the test target already on
  hand) or implementation code — this change is research and design only.
- No redesign of `implement-pantry-inventory`'s event vocabulary or lot model — consumed, not
  re-litigated. The `home_prepared` source and the ingredient-only lot it produces (D8) are
  taken as given here.
- No redesign of GTIN scanning or resolution — `research-barcode-scanning-devices`' and
  `implement-pantry-inventory`'s concern; this change only defines how its own lot-reference
  codes stay distinguishable from GTINs.
- No decision on making the printer mandatory hardware — it is explicitly optional per the
  household's own framing.

## Capabilities

### New Capabilities

- `label-printing`: printer hardware research, label content design, the lot-reference barcode
  namespace, print triggers, and the label-scan-to-consume check-out loop.

### Modified Capabilities

<!-- none — additive; consumes but does not modify pantry-inventory or barcode-scanning -->

## Impact

- Affected code: none yet (research change). A future implementation change would touch
  `internal/httpapi` (print/label-generation endpoints) and depends on whatever scanning
  implementation `research-barcode-scanning-devices` eventually produces for the read side.
- Depends on `implement-pantry-inventory` (`InventoryLot` shape) and is closely coupled to
  `research-barcode-scanning-devices` (shared scan-to-consume loop, shared physical scanning
  hardware, must avoid GTIN/lot-reference code collision).
