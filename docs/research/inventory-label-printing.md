# Inventory label printing research (tasks.md 2.1–7.1)

Researched 2026-08-20 for `openspec/changes/research-inventory-label-printing`.

This document answers the question that change owns: **how does Spisordning produce a physical,
scannable label for a specific `InventoryLot`, and how is that label read back without colliding
with manufacturer GTIN scanning?**

It consumes, but does not redesign:

- `implement-pantry-inventory`'s `InventoryLot` shape, event commands, and barcode invariant
  (`design.md` D6/D7/D8/D9).
- `research-barcode-scanning-devices`' scan-input recommendation (camera first, HID scanner
  second, manual entry always available).

The household's existing printer is the **Brother P-touch P300BT**. The household has also
confirmed that labeling home-cooked meals portioned and frozen for later is a primary use case,
not an edge case.

**Research-only limitation:** no physical test print was performed for this pass. The
household's Brother P300BT is currently loaned out and unavailable, and the household has directed
that the physical test be deferred to the future implementation change. The Brother unit was also
not visible on the research machine (`system_profiler SPBluetoothDataType`, `lpstat -v`, and
`system_profiler SPUSBDataType` showed no Brother device). Research is therefore complete without
a physical print test; direct-printing API selection must remain unvalidated until implementation.

---

## Executive recommendation

1. **Use the household's Brother P-touch P300BT as the primary target.** It is already owned,
   it prints narrow laminated `TZe` labels suitable for freezer/fridge containers, and its
   official software path is well defined even though its direct computer-printing API is not.

2. **Do not design Spisordning's first implementation around direct P300BT printing yet.**
   Brother's documented path for the P300BT is the `P-touch Design&Print 2` app for iOS/Android.
   Community Python/raw-protocol paths exist, but they are unofficial and were not physically
   validated in this research pass. The first implementation should generate label content and a
   QR code in Spisordning, then let the household print through the official Brother app or another
    manually operated path. Direct printing can be added later, in the implementation change,
    after a physical test validates a concrete API surface.

3. **Use a QR code, not a 1D barcode, as the label's scannable element.** A QR code can carry a
   prefixed lot reference with enough density and error correction for small labels, and it is
   already supported by the scanning stack recommended in
   `docs/research/barcode-scanning-devices.md` (`BarcodeDetector` where available, ZXing
   fallback otherwise).

4. **Encode a self-distinguishing lot reference, not a GTIN.** Recommended payload:

   ```text
   spisordning:lot:<InventoryLot.ID>
   ```

   This is distinguishable from a manufacturer GTIN by both symbology (QR vs EAN/UPC in the
   normal case) and payload shape (prefixed, non-numeric string vs 8/12/13/14-digit numeric GTIN).

5. **Keep printing strictly optional.** A missing printer, failed Bluetooth connection, or failed
   print job must never block `RecordPurchase`, `RecordConsume`, `RecordDiscard`, or
   `RecordMarkEmpty`.

---

## 1. Printer hardware

### 1.1 Primary target: Brother P-touch P300BT

Confirmed model: **Brother P-touch P300BT** (`PT-P300BT`).

Relevant documented facts:

- Interface: **Bluetooth**.
- Label stock: Brother **TZe** laminated tape.
- Supported tape widths: **3.5 mm, 6 mm, 9 mm, 12 mm**.
- Official software: Brother **P-touch Design&Print 2** for iOS/Android.
- No public P300BT-specific desktop SDK was found in this research pass. General Brother
  QL/P-touch documentation should not be assumed to apply unmodified to the P300BT.

Sources:

- Brother PT-P300BT specification page:
  `https://support.brother.com/g/b/spec.aspx?c=us_ot&lang=en&prod=p300bteus`
- Brother PT-P300BT product page:
  `https://www.brother-usa.com/p/office-home-label-makers/PTP300BT`

#### Printing API surface

| Path | Status | Fit for Spisordning |
|---|---|---|
| Brother `P-touch Design&Print 2` app | Official, documented, iOS/Android. | Best near-term path. Spisordning can generate the label text/QR and the household can print it through the Brother app. Less seamless than direct printing, but reliable and supported. |
| Community Python tools for P300BT | Unofficial. Examples: `Ircama/PT-P300BT`, `labelprinterkit`, and a raw Bluetooth gist. | Useful future path for direct computer printing. Requires a host with Bluetooth access and validation against the physical printer. Not safe to assume before the implementation change performs the deferred physical test. |
| Raw Bluetooth SPP / raster protocol | Unofficial, reverse-engineered/community-documented. | Possible, but highest maintenance risk. Should be a later optimization, not the foundation of v1. |
| macOS/Windows system printer path | Not confirmed for P300BT. The research machine did not see a Brother printer. | Do not design against this until physically verified. |

Community sources:

- `Ircama/PT-P300BT` README:
  `https://raw.githubusercontent.com/Ircama/PT-P300BT/master/README.md`
  - Confirms the official app is `P-touch Design&Print 2` for Android/iOS.
  - Provides a pure Python command-line tool for printing from a computer.
  - Supports TrueType/OpenType fonts, automatic font sizing, multiline text, images, and
    PDF-to-image conversion.
- `labelprinterkit` PyPI page:
  `https://pypi.org/project/labelprinterkit/`
  - Python 3 library for creating and printing Brother P-Touch labels.
  - Provides a layout engine with `Label`, `Text`, `Box`, `Padding`, `Job`, `Page`, media
    constants, and backends such as `PyUSBBackend`.
  - Useful as an API-shape reference, but not proof that the household's P300BT will work
    unchanged over Bluetooth.
- Raw Bluetooth driver gist:
  `https://gist.github.com/vsigler/98eafaf8cdf2374669e590328164f5fc`

### 1.2 Alternative printers (fallback/comparison only)

The Brother unit is the primary target, so alternatives are context only. Several vendor pages
were inaccessible or bot-blocked during this research pass (Dymo 410, Niimbot 404, Zebra product
page 404, Brother tape page transport error), so this comparison is deliberately conservative and
should be re-verified before any purchase decision.

| Printer line | Connectivity | Strengths | Weaknesses for this use case |
|---|---|---|---|
| Brother P-touch P300BT | Bluetooth | Already owned; narrow laminated `TZe` tape; good for freezer/fridge bags; official mobile app. | No confirmed public desktop SDK; direct computer printing depends on unofficial community paths until physically tested. |
| Niimbot label printers | Usually Bluetooth/app-first | Cheap, small, phone-app workflow; good for simple text labels. | App-centric; less verified durability story for freezer use; narrower/less flexible label stock; not confirmed from accessible official pages in this pass. |
| Dymo label printers | USB/Bluetooth depending on model | Mature consumer/office label ecosystem; some models support computer printing. | Many models are aimed at office/cable labeling; freezer-grade durability varies by stock; official pages were not cleanly accessible in this pass. |
| Zebra desktop/mobile thermal printers | USB/Bluetooth/network depending on model | More durable hardware; stronger computer-printing and API story; good if Spisordning later needs a dedicated kitchen printer. | Overkill and more expensive for a household; often uses different media and a less consumer-friendly label workflow. |

**Recommendation:** stay with the Brother P300BT unless the deferred implementation-phase physical
test shows that neither the official app workflow nor a community direct-printing path is
acceptable. If direct
computer/server printing becomes a hard requirement and the P300BT cannot satisfy it, revisit a
Zebra-class device rather than a cheaper app-only consumer printer.

### 1.3 Hardware recommendation

**Primary recommendation: Brother P-touch P300BT.**

Rationale:

- It is already in the household, so it avoids a purchase decision.
- Its `TZe` laminated tape is the right stock class for frost/moisture/handling.
- Its 9 mm and 12 mm widths can fit both human-readable text and a QR code.
- The official app gives a supported printing path even before any Spisordning direct-printing
  code exists.
- The main risk (direct API integration) is deferred, not blocking, because printing is optional.

---

## 2. Label stock and freezer durability

The P300BT accepts Brother `TZe` tape in 3.5 mm, 6 mm, 9 mm, and 12 mm widths.

For this use case, **9 mm or 12 mm `TZe` is the practical minimum**; 6 mm is only suitable for
very short text labels and should not be assumed to fit a useful QR code plus human-readable
details.

Why `TZe` is the right stock class:

- It is **laminated thermal-transfer** tape, not plain direct-thermal paper.
- The laminated top layer is the relevant differentiator for freezer/fridge use: it is intended
  to resist water, moisture, abrasion, and fading better than ordinary thermal paper.
- Narrow widths fit bags, containers, Tupperware lids, and frozen portions without covering the
  whole container.

Research-only caveat:

- Brother's general `TZe` product positioning supports the durability conclusion, but the exact
  SKU availability, current price, and long-term freezer performance for the household's specific
  tape roll were not physically verified in this pass.
- Before implementation, the household should confirm which `TZe` width it has on hand and keep
  that width as the default label size.

**Recommended default label width: 12 mm** if available, because it gives the QR code the most
decode margin. **9 mm** is an acceptable fallback for shorter labels. **6 mm** should be treated
as text-only or experimental.

---

## 3. Bluetooth pairing reliability in a kitchen setting

This was evaluated from the P300BT's documented Bluetooth interface, the official app workflow,
and community tooling. It was **not** validated by a physical kitchen pairing test.

Likely reliability characteristics:

- The official `P-touch Design&Print 2` app is the lowest-risk path because Brother owns the
  pairing/printing experience.
- Bluetooth range is probably sufficient inside one kitchen, but walls, microwave interference,
  and distance to the paired phone can still matter.
- The printer will usually be paired to one phone/tablet at a time. Household use should assume
  sequential printing, not concurrent unattended printing from multiple devices.
- Battery state matters. A docked or regularly charged printer is more reliable than a forgotten
  mobile one.
- macOS/Windows system-level Bluetooth printing should not be assumed; the P300BT's documented
  consumer path is the Brother app.

Mitigations:

- Keep the printer on a fixed kitchen counter or dock, not stored away.
- Pair it to a dedicated household phone/tablet used for Spisordning.
- Treat "print label" as an optional action that can be retried later from the lot detail view.
- If direct computer printing is added later, run it from a host with stable Bluetooth access and
  make failures visible but non-blocking.

**Conclusion:** Bluetooth is acceptable for an optional labeling workflow, but the design must not
make inventory recording depend on the printer being paired, powered, or reachable.

---

## 4. Label content design

Label content is read from the existing `InventoryLot` shape:

```go
type InventoryLot struct {
    ID           int64
    IngredientID string
    ProductID    string
    LocationID   string
    Quantity     float64
    Unit         string
    Confidence   domain.Confidence
    BestBefore   *time.Time
    OpenedAt     *time.Time
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

Source: `internal/persistence/pantry.go:122`.

### 4.1 Required fields

| Label field | Source | Print rule |
|---|---|---|
| Contents name | `Product.Name` when `ProductID` is set; otherwise `Ingredient.Display` | Always print. This is the primary human-readable identifier. |
| Stored-on date | `InventoryLot.CreatedAt` | Always print as a date, not a timestamp. Freezer labels need "when did this go in?" |
| QR code | `spisordning:lot:<InventoryLot.ID>` | Always print. This is the machine-readable lot reference. |
| Human-readable lot hint | `lot:<InventoryLot.ID>` | Print below or beside the QR where space allows. Useful if the QR is damaged. |

### 4.2 Optional fields

| Label field | Source | Print rule |
|---|---|---|
| Quantity + unit | `InventoryLot.Quantity`, `InventoryLot.Unit` | Print when known and non-zero. Omit if the lot is ingredient-only and quantity is not meaningful. |
| Best-before / use-by | `InventoryLot.BestBefore` | Print when present. Use `use by` for opened/perishable items and `best before` for unopened shelf-stable items if that distinction is later available; v1 can simply print `best before`. |
| Location | `InventoryLocation.Name`, optionally with parent location names | Print when space allows. For nested locations, print the most specific name first, e.g. `Freezer / Basement`. |
| Storage hint | `InventoryLocation.LocationType` | If `FREEZER`, print `FROZEN` prominently. If `FRIDGE`, print `FRIDGE` when space allows. This is a hint, not identity, per pantry `design.md` D9. |
| Opened state | `InventoryLot.OpenedAt` | Print `opened` when set and space allows. |
| Confidence | `InventoryLot.Confidence` | Do not print in v1. It is useful in the app but clutters a small physical label. |

### 4.3 Layout mockups

These are mockups only, not implementation.

#### 12 mm preferred layout

```text
FROZEN
Lentil soup
2 portions
stored 2026-08-20
use by 2026-11-17
Freezer / Basement
[QR]
lot:12345
```

Design notes:

- `FROZEN` is the first line when the destination location type is `FREEZER`.
- The contents name should be the largest human-readable text.
- The QR code should occupy a contiguous square area with quiet space around it.
- `lot:12345` is a fallback for manual entry if the QR is frost-damaged.

#### 9 mm fallback layout

```text
FROZEN
Lentil soup
2026-08-20
2026-11-17
[QR]
lot:12345
```

Design notes:

- Drop quantity and full location name when they do not fit.
- Keep stored-on date and best-before date if present.
- If the QR becomes too small to decode reliably, prefer printing two smaller labels (one text,
  one QR) or using 12 mm tape.

#### 6 mm layout

Not recommended for the full label. If used:

```text
Lentil soup
2026-08-20
[QR]
```

This should be treated as experimental until physically tested.

---

## 5. Barcode vs QR code

| Criterion | 1D barcode (e.g. Code 128) | QR code |
|---|---|---|
| Payload capacity | Good for short tokens; poor for prefixed references or URLs. | Better for prefixed lot references and future deep links. |
| Small-size behavior | Can work with very short payloads, but error tolerance is lower. | 2D error correction recovers from frost, scratches, and partial obstruction better. |
| Curved/frosty surface | More sensitive to distortion and broken bars. | More robust when printed with sufficient module size and quiet zone. |
| Scanner support | Widely supported. | Supported by the recommended scanning stack: `BarcodeDetector` `qr_code` where available and ZXing fallback. |
| GTIN collision | Lower if using a non-retail symbology, but payload prefix still needed. | Lower in practice because package GTINs are usually EAN/UPC 1D, while Spisordning labels use QR. Payload prefix still required. |
| Future flexibility | Limited. | Can later carry a URL or app deep link without changing the label concept. |

**Recommendation: QR code.**

The QR payload should remain short in v1:

```text
spisordning:lot:<InventoryLot.ID>
```

A future version can move to a URL or app scheme without invalidating the core idea, but v1
should not depend on a public domain, reverse Proxy, or app deep-link registration.

### QR sizing guidance

- **12 mm tape:** preferred. Gives the best chance of a decodable QR plus human-readable text.
- **9 mm tape:** acceptable for short payloads; must be physically tested before being treated
  as reliable.
- **6 mm tape:** not recommended for QR in v1.
- The label renderer should preserve quiet space around the QR. A QR that fills the entire tape
  edge-to-edge is harder to decode.
- If a QR version/module count becomes too large for the selected tape width, the renderer should
  either increase tape width, shorten the payload, or fall back to a text-only label plus manual
  lot entry.

---

## 6. Lot-reference barcode namespace

### 6.1 Direct lot ID vs opaque token

| Option | Shape | Strengths | Weaknesses |
|---|---|---|---|
| Direct `InventoryLot.ID` | `spisordning:lot:12345` | Simple; no extra table; stable for the life of the database; easy to debug; remains valid after `RecordMarkEmpty`. | Exposes internal sequential ids on a physical label. |
| Short opaque token | `spisordning:lot:v1:<base32-token>` | More private; can be hashed or rotated; shorter if ids grow large. | Requires an extra lookup table or deterministic derivation; more moving parts; no household-scale benefit in v1. |

**Recommendation: direct `InventoryLot.ID` in v1.**

Household labels are not public internet artifacts, and the simplicity wins. If privacy or id
length ever becomes a problem, the payload parser can be extended to support an opaque token
format without changing the GTIN-distinguishability rule.

### 6.2 Recommended payload format

```text
spisordning:lot:<decimal InventoryLot.ID>
```

Examples:

```text
spisordning:lot:12345
spisordning:lot:98765
```

Rules:

- The prefix is mandatory.
- The lot id is decimal.
- The payload is UTF-8 text inside a QR code.
- The same value should be printed human-readably as `lot:<id>` where space allows.
- The parser should trim surrounding whitespace before matching.
- Unknown prefixes or malformed ids should route to manual entry, not guess.

### 6.3 Lifecycle

A lot-reference code should remain valid **for the life of the lot row**, including after the lot
is marked empty.

Reason:

- `RecordMarkEmpty` sets the lot quantity to zero but does not delete the lot row.
- A scanned empty lot should resolve to the lot and show "already empty" or history, not produce
  a mysterious unknown-code error.
- Auditability matters: a label on an empty container may be scanned after the fact.

Therefore:

- Do not expire lot-reference codes when quantity reaches zero.
- Do not reuse lot ids.
- Do not hard-delete `InventoryLot` rows while labels may still exist.

### 6.4 Distinguishability from GTIN at scan time

The code must be distinguishable from a manufacturer GTIN without relying on scan context.

Recommended mechanism: **symbology plus payload prefix**.

| Scan result | Interpretation | Route |
|---|---|---|
| QR code with payload starting `spisordning:lot:` | Spisordning lot reference | Resolve `InventoryLot.ID`, then show lot-specific check-out actions. |
| 1D EAN/UPC with 8/12/13/14 numeric digits | Manufacturer GTIN | Route to `LookupBarcode` / GTIN check-in or GTIN check-out flow. |
| QR code containing only 8/12/13/14 numeric digits | Likely package QR carrying a GTIN | Route to GTIN handling, not lot handling. |
| QR code with unknown prefix | Unknown | Show manual fallback; do not guess. |
| 1D non-EAN/UPC code | Unknown or non-retail code | Show manual fallback unless a future explicit namespace is added. |

This satisfies the requirement that the system determine from the code itself whether it is a
lot-reference code or a manufacturer GTIN.

It also coordinates cleanly with `research-barcode-scanning-devices`:

- That change's GTIN check-out flow expects EAN/UPC package barcodes and may need lot
  disambiguation.
- This change's label flow expects QR lot references and needs no lot disambiguation.
- Both can share the same camera/HID/manual scan input layer, then branch on symbology + payload.

### 6.5 Relationship to "a barcode SHALL NOT define product identity"

This does not violate `implement-pantry-inventory`'s invariant.

The pantry invariant says a **manufacturer barcode/GTIN** is a lookup key onto a `Product`, never
product identity. `InventoryLot` and `InventoryEvent` reference `product_id`, not a raw GTIN.

A printed Spisordning lot reference is a different kind of code:

- It identifies a specific **physical lot**, not a product type.
- It resolves to `InventoryLot.ID`, not to a `Product`.
- The lot itself still references `Ingredient` and optionally `Product` through the normal pantry
  model.
- No table treats the lot-reference payload as product identity.

In other words:

```text
GTIN        -> Product lookup key
lot label   -> InventoryLot pointer
```

Those are distinct namespaces with distinct semantics.

---

## 7. Print trigger points

### 7.1 Optional print-on-`RecordPurchase`

After a successful `RecordPurchase`, the UI may offer:

- **Print label**
- **Done**

Rules:

- The purchase is already committed before printing is offered.
- If no printer is configured, the offer can be hidden or disabled.
- If printing fails, the UI shows a non-blocking error and retains the **Print label** action on
  the lot.
- Batch check-in should not force a print decision after every item. It can offer a post-batch
  action: **Print labels for N new lots**.

This keeps printing strictly additive.

### 7.2 Explicit "print label for this lot"

The primary trigger is an explicit action available from any existing lot:

- lot detail view
- search result
- freezer/fridge location view
- home-prepared meal creation flow

Rules:

- Works for any `InventoryLot`, regardless of `source`.
- Works for ingredient-only lots (`ProductID == ""`).
- Works for lots created by `shopping_order`, `barcode_scan`, `manual_count`, `home_prepared`,
  or future sources.
- If the lot quantity is zero, the UI should warn that the lot is already empty but may still
  allow printing for audit/relabelling if the household wants it.

This is the first-class path for freezer meal labeling.

### 7.3 Home-cooked/frozen-meal path

The upstream pantry design already resolves how a home-cooked meal becomes inventory:

1. Household portions and freezes a home-cooked meal.
2. Spisordning calls `RecordPurchase` with:
   - `ingredientId` for the meal/ingredient
   - `productId` omitted
   - `locationId` for the freezer location
   - `quantity` and `unit`
   - `source: "home_prepared"`
3. A new `InventoryLot` is created at ingredient-only specificity.
4. The explicit **print label for this lot** action reads that lot and prints:
   - ingredient display name
   - stored-on date
   - quantity/unit if meaningful
   - freezer location hint
   - QR lot reference

No purchase-specific precondition blocks this path.

---

## 8. Check-out scan-to-consume loop

Recommended flow for scanning a Spisordning-printed label:

```text
[Start check-out]
  -> choose input method: Camera | Scanner | Manual
  -> scan QR label
  -> parse payload
  -> if payload starts "spisordning:lot:":
       extract InventoryLot.ID
       load InventoryLot
       show lot card:
         - contents name
         - location
         - quantity remaining
         - stored-on date
         - best-before date
       choose action:
         - Consume
         - Discard
         - Mark empty
       submit corresponding command
  -> if payload is GTIN-shaped:
       route to GTIN check-out flow from research-barcode-scanning-devices
  -> if payload is unknown:
       show manual fallback
```

### Command mapping

| User action | Command | Notes |
|---|---|---|
| Consume some | `RecordConsume(lotID, quantity, estimated, source)` | Use `source: "label_scan"` when the source vocabulary is extended. If not, `barcode_scan` is acceptable but less precise. |
| Discard some/all | `RecordDiscard(lotID, quantity, estimated, reason, source)` | Reason should be `expired`, `spoiled`, or `other`. |
| Mark empty | `RecordMarkEmpty(lotID)` | Fastest path for "this container is finished". Upstream D7 implements this as a `CONSUME` event with `source: "mark_empty"`. |

Recommended source value:

```text
label_scan
```

This distinguishes "scanned a Spisordning lot label" from "scanned a manufacturer GTIN" in the
event ledger. The pantry source vocabulary is explicitly extensible.

### No-disambiguation guarantee

A Spisordning lot label resolves directly to one `InventoryLot.ID`. Therefore this flow does not
need the GTIN lot-disambiguation picker described in
`docs/research/barcode-scanning-devices.md`.

Edge cases:

- **Lot not found:** show "Lot not found" and offer manual lot search. This should be rare if the
  database and label come from the same system.
- **Lot already empty:** show the lot as empty. Do not call `RecordMarkEmpty` again unless the
  user explicitly wants a zero-delta audit event; v1 should probably disable mark-empty when
  quantity is already zero.
- **QR damaged:** use the human-readable `lot:<id>` fallback and manual entry.
- **Wrong namespace:** route by payload rules in §6.4; never guess.

---

## 9. Future implementation phasing

This change does not implement printing. The recommended future shape is:

### Phase 1 — Label generation without direct printing

- Add a label-content builder that reads an `InventoryLot`, its `Ingredient`, optional
  `Product`, and `InventoryLocation`.
- Generate the QR payload `spisordning:lot:<id>`.
- Show a printable label preview in the future web/PWA surface.
- Allow the household to print via:
  - Brother `P-touch Design&Print 2` app, or
  - browser print to whatever printer the household has configured, or
  - manual re-entry into the Brother app.
- Add scan parsing for `spisordning:lot:` QR payloads in the check-out flow.

This phase delivers the household value (legible, scannable freezer labels) without depending on
an unvalidated direct-printing API.

### Phase 2 — Direct Brother printing

Only after the implementation change physically validates a concrete path:

- Choose between:
  - official app integration if Brother exposes a suitable mechanism,
  - community Python helper (`Ircama/PT-P300BT` or `labelprinterkit`),
  - raw Bluetooth protocol as a last resort.
- Keep the printer optional and failures non-blocking.
- Add printer configuration and status, but do not make inventory commands depend on it.

### Phase 3 — Optional dedicated printer

Only if the P300BT proves insufficient:

- Revisit Zebra-class hardware for direct computer/server printing.
- Keep the same label payload and scanning namespace; only the printing backend changes.

---

## 10. Open risks / things to verify before implementation

1. **No physical test print was performed.** The physical test is deferred to the future
   implementation change because the household printer is currently loaned out. The direct-printing
   API surface is therefore a research finding, not a validated implementation assumption.
2. **P300BT direct computer printing is unofficial.** Community tools are promising but may depend
   on OS Bluetooth permissions, Bluetooth SPP support, firmware behavior, or host-specific setup.
3. **macOS/Windows system printer support is unconfirmed.** Do not assume the P300BT appears as a
   normal system printer.
4. **QR decode on frosty/curved bags needs physical testing.** The design should use 12 mm tape,
   high contrast, quiet space, and a human-readable `lot:<id>` fallback.
5. **TZe SKU availability and price were not fully verified.** The durability conclusion is sound
   at the product-line level, but the household should confirm the exact roll it wants to use.
6. **Vendor pages for alternatives were partially inaccessible.** The Niimbot/Dymo/Zebra comparison
   should be re-verified before any purchase decision.
7. **Bluetooth household concurrency is unsupported.** Design for one active printing session at
   a time.
8. **Source vocabulary extension.** `label_scan` is recommended but not yet part of the upstream
   source list; it should be added when the scanning/label implementation change is scoped.

---

## Sources

- OpenSpec change:
  - `openspec/changes/research-inventory-label-printing/proposal.md`
  - `openspec/changes/research-inventory-label-printing/tasks.md`
  - `openspec/changes/research-inventory-label-printing/specs/label-printing/spec.md`
- Upstream pantry/inventory design:
  - `openspec/changes/implement-pantry-inventory/design.md` (D6, D7, D8, D9, Step 5 commands)
  - `internal/persistence/pantry.go`
  - `migrations/0009_pantry_inventory.sql`
- Sibling scanning research:
  - `docs/research/barcode-scanning-devices.md`
  - `openspec/changes/research-barcode-scanning-devices/proposal.md`
- Brother P300BT:
  - Specification:
    `https://support.brother.com/g/b/spec.aspx?c=us_ot&lang=en&prod=p300bteus`
  - Product page:
    `https://www.brother-usa.com/p/office-home-label-makers/PTP300BT`
- Community P300BT / P-Touch printing:
  - `Ircama/PT-P300BT` README:
    `https://raw.githubusercontent.com/Ircama/PT-P300BT/master/README.md`
  - `labelprinterkit`:
    `https://pypi.org/project/labelprinterkit/`
  - Raw Bluetooth driver gist:
    `https://gist.github.com/vsigler/98eafaf8cdf2374669e590328164f5fc`
- Scanning symbology/API background:
  - WICG Shape Detection API:
    `https://wicg.github.io/shape-detection-api/`
  - CanIUse `BarcodeDetector API`:
    `https://caniuse.com/mdn-api_barcodedetector`
