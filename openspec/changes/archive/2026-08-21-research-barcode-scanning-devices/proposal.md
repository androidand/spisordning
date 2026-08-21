# Research barcode scanning devices

## Why

`implement-pantry-inventory`'s Barcode scope (that change's `design.md` D6) already covers
*resolving* a GTIN once you have one — normalization, `ProductIdentifier` lookup, Open Food
Facts, retailer fallback, manual entry. It deliberately does not cover how a barcode gets *into*
the system in the first place: no camera scanning, no dedicated hardware scanner, no scan-driven
UI/workflow. That gap is real and household-requested: the described use case is standing in a
physical store (no online order, so no automatic product resolution) and scanning a product to
both identify it and record it into inventory, plus the mirror case — scanning something on the
way *out* of the pantry ("the milk is empty, into recycling") to record a `CONSUME`/`DISCARD`/
`MARK_EMPTY` without manual lookup.

This is genuinely unresearched: no scanning-hardware or scanning-UI investigation exists
anywhere in this repo (`docs/research/` has no scanner-related document), and two real
alternatives exist that behave very differently — a phone's camera (software-only, always
available, needs a decode library and camera-permission UX) and a dedicated barcode scanner
(hardware cost, but typically acts as a keyboard-wedge HID device needing no special driver, and
is faster/more reliable for repeated use, e.g. checking in a whole grocery haul at once). Per
`PLAN.md`'s own discipline elsewhere in this backlog (research before design, design before
implementation — see `establish-reference-lab`, `research-and-integrate-ica`), this is a
research-first change: its deliverable is a documented recommendation, not scanning code.

This change **depends on `implement-pantry-inventory`** landing first (or far enough along) for
two reasons: (1) its own Barcode/GTIN-resolution logic (`LookupBarcode`) is the thing a scan
result gets handed to on check-in, and (2) its `RecordConsume`/`RecordDiscard`/
`RecordMarkEmpty` commands are exactly what a check-out scan needs to call. This change does not
redefine or duplicate any of that — it is strictly "how does a barcode reach the system," never
"what happens once it has."

## What Changes

- Research mobile/camera-based scanning: available browser/PWA barcode-decode approaches
  (e.g. the `BarcodeDetector` Web API where supported, or a JS decode library as fallback),
  camera-permission UX, and reliability in poor lighting/reflective packaging (a real Grocy/
  household pain point worth checking against, not assuming away).
- Research dedicated barcode scanner hardware: USB and Bluetooth HID ("keyboard wedge") scanners
  — confirm the assumption that these need no custom driver/integration beyond capturing
  keyboard input on a barcode-entry field, survey a couple of concrete affordable models, and
  identify any gotchas (scan-then-Enter framing, multi-symbology support, pairing stability).
- Define the **check-in flow**: scan → `LookupBarcode` (from `implement-pantry-inventory`) →
  either an existing `Product` resolves and a `RecordPurchase` is offered pre-filled, or manual
  fallback triggers per that change's D6. This change designs the UI/workflow shape; it does not
  redesign the resolution chain itself.
- Define the **check-out flow**: scan → resolve which `InventoryLot` the scanned item
  corresponds to (this is the genuinely new design question this change owns — a GTIN alone is
  ambiguous when multiple lots of the same product exist, e.g. two milk cartons; disambiguation
  by location and/or recency is in scope to design) → offer `RecordConsume`/`RecordDiscard`/
  `RecordMarkEmpty` against the resolved lot.
- Explicitly out of scope for the check-out flow: resolving a *label-printed* barcode (that
  barcode encodes a lot reference directly, not a GTIN, and is disambiguation-free by
  construction) — that is `research-inventory-label-printing`'s concern; this change's check-out
  scanning is for a package's own manufacturer barcode, which is ambiguous by nature and needs
  the disambiguation step above.

## Non-Goals

- No scanning implementation code — this change is research and design only, producing a
  recommendation and a `tasks.md` design for a later implementation change to pick up.
- No redesign of GTIN normalization/resolution (`implement-pantry-inventory` D6) — consumed, not
  re-litigated.
- No redesign of the inventory event commands (`RecordPurchase`/`RecordConsume`/etc.) —
  consumed, not re-litigated.
- No label-printer hardware research or lot-reference-barcode design — that is
  `research-inventory-label-printing`.
- No decision on whether a native mobile app is ever built — scope here is what's achievable via
  the browser/PWA surface Spisordning already targets, plus commodity HID hardware.

## Capabilities

### New Capabilities

- `barcode-scanning`: how a barcode reaches Spisordning in the first place — mobile camera
  scanning, dedicated HID scanner hardware, and the check-in/check-out workflows that consume a
  scan result.

### Modified Capabilities

<!-- none — additive; consumes but does not modify pantry-inventory -->

## Impact

- Affected code: none yet (research change). A future implementation change would touch
  `internal/httpapi` (scan-result endpoints) and a not-yet-existing frontend/PWA surface.
- Depends on `implement-pantry-inventory` (barcode resolution, inventory event commands).
- Feeds a future implementation change, and is a sibling to `research-inventory-label-printing`
  (shares the check-out/consume workflow; label-printed barcodes are a distinct, unambiguous
  namespace scanned through the same physical hardware).
