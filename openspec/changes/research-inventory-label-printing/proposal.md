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

## What Changes

- Research consumer/small-business thermal label printer options (connectivity: USB, Bluetooth,
  or network; common candidates worth surveying include Brother QL-series, Niimbot, Dymo, and
  Zebra desktop printers — nothing pre-decided, survey before recommending) and their
  printing-API surfaces (vendor SDK, ESC/POS-style raw commands, or a print-to-image/PDF
  approach).
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
  printer is described as optional hardware), and/or an explicit "print label for this lot"
  action independent of purchase timing (e.g. printing a label for a home-cooked meal frozen
  for later, which isn't a `PURCHASE` at all — flag this as a scope question, since
  `implement-pantry-inventory`'s event vocabulary doesn't currently have a kind for "portioned
  and frozen a cooked meal").
- Define the **check-out loop**: scanning a lot's printed label resolves directly and
  unambiguously to that `InventoryLot`, feeding `RecordConsume`/`RecordDiscard`/
  `RecordMarkEmpty` — the same command surface `research-barcode-scanning-devices` targets for
  GTIN-based check-out, but without that flow's disambiguation problem.

## Non-Goals

- No printer purchase or implementation code — this change is research and design only.
- No redesign of `implement-pantry-inventory`'s event vocabulary or lot model — consumed, not
  re-litigated (though the "frozen home-cooked meal" scope question above is flagged for that
  change or a future one to resolve, not decided here).
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
