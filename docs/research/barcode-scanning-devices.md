# Barcode scanning devices research (tasks.md 2.1–6.3)

Researched 2026-08-19 for `openspec/changes/research-barcode-scanning-devices`.

This document answers only the question this change owns: **how does a barcode get into
Spisordning in the first place?** It does not redesign GTIN normalization/resolution, which
belongs to `implement-pantry-inventory` D6 (`LookupBarcode`), and it does not redesign the
inventory event commands, which are consumed as-is:

- `LookupBarcode(gtin)`
- `RecordPurchase(ingredientId, productId?, locationId, quantity, unit, bestBefore?, source)`
- `RecordConsume(lotId, quantity, source)`
- `RecordDiscard(lotId, quantity, reason, source)`
- `RecordMarkEmpty(lotId, source)`

The confirmed target scanning platforms are:

- `iOS Safari`
- `Android Chrome`
- `iOS other browsers`
- `Android other browsers`

---

## Executive recommendation

1. **Treat scanning as a small input layer that produces one normalized value: a GTIN string
   plus an input-method tag** (`camera` or `hid_scanner`). Everything downstream is the same
   check-in/check-out workflow regardless of how the digits arrived.

2. **Mobile camera scanning:**
   - Use the native `BarcodeDetector` Web API when it is present and reports the needed
     formats.
   - Fall back to a JS decode library when `BarcodeDetector` is absent or incomplete. The
     concrete recommended fallback is **`@zxing/browser` + `@zxing/library`**.
   - Keep **manual GTIN entry** permanently reachable. It is table stakes for v1, not an
     error-recovery afterthought.

3. **Dedicated scanner hardware:**
   - The keyboard-wedge/HID assumption is confirmed by Zebra DS2200 documentation: the scanner
     can act as a keyboard and needs no Spisordning-specific driver.
   - Concrete affordable models to buy if hardware is wanted:
     - **Zebra DS2208** — corded USB handheld, fixed check-in station.
     - **Zebra DS2278** — cordless Bluetooth handheld, mobile/shared check-in and check-out.
   - Both support the symbologies a household pantry needs, including `UPC/EAN` and 2D codes.

4. **Build order:**
   - Design both input methods behind the same scan-result abstraction.
   - Ship **mobile camera scanning first**: no hardware purchase, works on devices the household
     already has, and covers both check-in and check-out.
   - Add **dedicated HID scanner support second**: it is mostly UI input handling once the
     scan-target field and check-in/check-out flows exist.
   - Do not build either before the future web/PWA surface exists; Spisordning is CLI-only
     today.

---

## 2. Mobile / camera scanning

### 2.1 `BarcodeDetector` Web API

The native API is the preferred path where available because it avoids bundling a decode
library and is likely to be the fastest supported path on browsers that ship it.

API shape (research only, not implementation):

```js
if ('BarcodeDetector' in window) {
  const formats = await BarcodeDetector.getSupportedFormats();
  // probe for the formats Spisordning needs, e.g. `ean_13`, `upc_a`
  const detector = new BarcodeDetector({ formats });
  const results = await detector.detect(videoElement);
}
```

Important consequence: **the UI must probe support at runtime**. Both constructor `formats`
and `getSupportedFormats()` matter. A browser can expose `BarcodeDetector` but still not
support every format Spisordning cares about.

#### Browser support observed on CanIUse (2026-08-19)

Source: `https://caniuse.com/mdn-api_barcodedetector`.

| Platform / browser | Observed support | Implication |
|---|---|---|
| Global usage | `76.34%` | Not universal; fallback required. |
| Chrome desktop | Partial in recent versions | Use native only after runtime format probe. |
| Edge desktop | Partial in recent versions | Use native only after runtime format probe. |
| Opera desktop | Partial in recent versions | Not a confirmed target; same probe rule. |
| Chrome for Android | Supported in recent versions | Good native path for `Android Chrome`. |
| Samsung Internet | Supported in recent versions | Useful for some Android devices, but still probe. |
| Safari / iOS Safari | Disabled by default in recent versions | **No reliable native path on the primary iOS target.** |
| Firefox | Not supported | JS fallback required. |
| Android Browser | Supported in recent observed entries | Treat as non-primary; still probe. |

The practical reading for the confirmed target matrix:

- `Android Chrome`: likely native `BarcodeDetector`, but still feature-detect.
- `iOS Safari`: assume no native `BarcodeDetector`; use the JS fallback.
- `Android other browsers`: may or may not have it; feature-detect.
- `iOS other browsers`: many are WebKit-based and should be assumed to need the fallback
  unless runtime detection says otherwise.

#### Symbology coverage

The WICG Shape Detection API spec defines `BarcodeFormat` values including retail 1D formats
such as `ean_13`, `ean_8`, `upc_a`, and `upc_e`, as well as 2D formats such as `qr_code`.
Source: `https://wicg.github.io/shape-detection-api/`.

For Spisordning, the minimum is:

- `EAN-13`
- `UPC-A`

Those are the common manufacturer barcode formats for packaged food. The implementation must
not assume they are available just because `BarcodeDetector` exists; it should call
`getSupportedFormats()` and fall back if either required format is missing.

### 2.2 JS decode-library fallback

Concrete options checked on npm (2026-08-19):

| Library | Latest version observed | License | Approx. unpacked size | Notes |
|---|---:|---|---:|---|
| `@zxing/library` | `0.23.0` | `Apache-2.0` | ~11.8 MB | Core ZXing decode library. npm page indicated maintenance mode, but it is the substantive decode engine. |
| `@zxing/browser` | `0.2.1` | `MIT` | ~5.8 MB | Browser-oriented wrapper around ZXing for camera/video decoding. |
| `html5-qrcode` | `2.3.8` | `Apache-2.0` | ~2.6 MB | Higher-level, batteries-included camera QR/barcode UI. Older publish timestamp (around 2023). |

Recommendation: **`@zxing/browser` + `@zxing/library`**.

Reasons:

- `@zxing/browser` is the right shape for a browser camera fallback: it handles the
  browser/video side, while `@zxing/library` provides the decode engine.
- Licenses are permissive and compatible: `MIT` + `Apache-2.0`.
- `html5-qrcode` is a reasonable alternative if the future frontend wants a simpler
  drop-in scanning UI, but it is less ideal as the primary research recommendation because it
  bundles more UI behavior and has a weaker maintenance signal in the observed metadata.

The future implementation should still wrap the decoder behind an internal interface so the
library can be swapped if maintenance or bundle size becomes a problem.

### 2.3 Camera-permission UX on the target platforms

Camera scanning requires `getUserMedia`, which means HTTPS and a browser permission prompt.

Common UX shape:

1. User taps "Scan with camera".
2. App requests the rear camera (`facingMode: environment` where supported).
3. Browser shows its native permission prompt.
4. If granted, show live viewfinder and decode loop.
5. If denied/unavailable, show a clear recovery state with:
   - "Try again"
   - "Open browser/site settings" guidance
   - **Manual GTIN entry**

Platform notes:

| Platform | Permission UX notes |
|---|---|
| `iOS Safari` | Camera permission is controlled by Safari/site settings. If denied, the web app cannot programmatically open the exact settings screen; it can only explain where to look. Manual entry is mandatory. |
| `Android Chrome` | Camera permission prompt is native to Android/Chrome. If denied, the user can change it in site/app settings. Manual entry is mandatory. |
| `iOS other browsers` | Usually WebKit-based; expect the same class of permission behavior and the same need for fallback. |
| `Android other browsers` | Chromium-based browsers generally follow the same permission model, but non-Chromium Android browsers should not be assumed to behave identically. |

Design consequence: the scan UI must have three stable input tabs or equivalents:

- **Camera**
- **Scanner** (for HID scanner input, if present/focused)
- **Manual**

The tabs should be peers. Manual entry is not an error state.

### 2.4 Real-world scan reliability and manual fallback

Camera scanning will fail or degrade in real pantry/store conditions:

- poor lighting
- reflective packaging
- curved or cylindrical packaging
- damaged, bent, or partially obscured barcodes
- small barcodes far from the camera
- camera blur from hand movement
- browser/OS camera contention

Therefore:

- **Manual GTIN entry is table stakes for v1.**
- `implement-pantry-inventory` D6 already has a manual fallback for the case where a GTIN
  resolves through no lookup source. That is not enough by itself.
- This change requires a second manual path: **the scan itself failed**, so there is no GTIN
  to resolve. The UI must let the user type the GTIN (or create/select the ingredient manually)
  without leaving the check-in/check-out flow.

In other words, there are two distinct manual fallbacks:

| Failure | Meaning | UI handoff |
|---|---|---|
| Scan failed | Camera/HID did not produce a usable GTIN. | "Enter GTIN manually" or "choose ingredient manually". |
| Lookup failed | GTIN was captured but `LookupBarcode` exhausted its chain. | D6 manual product/ingredient resolution, then continue to the same check-in/check-out form. |

---

## 3. Dedicated scanner hardware

### 3.1 Keyboard-wedge / HID assumption

Confirmed against Zebra DS2200 documentation.

Source: Zebra DS2200 Series Specification Sheet,
`https://www.zebra.com/us/en/products/spec-sheets/scanners/general-purpose-scanners/handheld/ds2200-series.html`.

Relevant documented facts:

- Supported host interfaces include **`USB`, `RS232`, `Keyboard Wedge`, `TGCS (IBM)`, and
  `46XX over RS485`**.
- Keyboard support: **more than 90 international keyboards**.
- The cordless DS2278 pairs via Bluetooth and uses Zebra **Scan-to-Connect** for one-step
  pairing with Bluetooth-enabled hosts.

For Spisordning, the relevant mode is **Keyboard Wedge**: the scanner behaves like a fast
human typing the decoded barcode value into whatever text field has focus, usually followed by
a terminator such as Enter.

No Spisordning-specific driver is needed. The web UI only needs to:

1. have a known scan-target field focused, and
2. accept rapid text input ending with the configured terminator.

One gap remains: the accessible Zebra pages/spec sheet did not clearly state the exact
default suffix/terminator for the specific SKUs. The UI should therefore **not depend on a
magic terminator alone**. It should treat "rapid input into the scan field + Enter" as the
scan event, and it should allow manual entry if the terminator is missing or misconfigured.

### 3.2 Concrete affordable models

Zebra's DS2200 line is positioned as an affordable general-purpose 1D/2D handheld imager line.
The two models below cover the two household hardware shapes: fixed USB and cordless
Bluetooth.

#### Zebra DS2208 — corded USB

Source: `https://www.zebra.com/us/en/products/scanners/general-purpose-handheld-scanners/ds2200-series/ds2208.html`

- Type: corded handheld scanner.
- Use case: fixed check-in station (kitchen table, pantry shelf, desktop).
- Connectivity: corded; no battery required for normal use.
- Scan type: 1D/2D imager.
- Symbologies: DS2200 series supports `UPC/EAN`, `Code 128`, `Code 39`, `DataMatrix`,
  `QR Code`, PDF417, and other common 1D/2D formats.
- Durability: IP52, 5 ft drops to concrete, 250 tumbles.
- Warranty observed in spec sheet: 60 months for DS2208.
- Price observed on Zebra product page (2026-08-19, "starting at" guide price):
  - EU: **€96**
  - UK: **£84**
  - North America: **$178 USD**
  - Other regional guide prices were present in the page's price list, but EU/UK/NA are the
    relevant reference points here.

DS2208 is the cheapest and simplest hardware path if the household wants a dedicated scanner
for one fixed location.

#### Zebra DS2278 — cordless Bluetooth

Source: `https://www.zebra.com/us/en/products/scanners/general-purpose-handheld-scanners/ds2200-series/ds2278.html`

- Type: cordless handheld scanner.
- Use case: shared/mobile scanning while putting groceries away or clearing the pantry.
- Connectivity: Bluetooth 4.0.
- Pairing: Zebra Scan-to-Connect, documented as one-step pairing with Bluetooth-enabled PC,
  tablet, or smartphone.
- Battery: 2400 mAh Li-Ion; spec sheet lists 84 hours operating time per full charge.
- Charging: Micro USB cable or Presentation Cradle; cradle allows hands-free scanning and
  charges during use.
- Scan type: 1D/2D imager.
- Symbologies: same DS2200 series 1D/2D coverage, including `UPC/EAN` and 2D formats.
- Durability: IP52, 5 ft drops to concrete, 250 tumbles.
- Warranty observed in spec sheet: 36 months for DS2278; 36 months for CR2278 Presentation
  Cradle; 12 months for battery.
- Price observed on Zebra product page (2026-08-19, "starting at" guide price):
  - EU: **€126**
  - UK: **£187**
  - North America: **$289 USD**

DS2278 is the better fit if the household wants one shared scanner that can move between
fridge, pantry, table, and recycling bin.

#### Model comparison

| Property | Zebra DS2208 | Zebra DS2278 |
|---|---|---|
| Form factor | Corded handheld | Cordless handheld |
| Primary connection | USB/cabled | Bluetooth 4.0 |
| Battery | Not needed | 2400 mAh Li-Ion, ~84 h |
| Best household role | Fixed check-in station | Shared mobile check-in/check-out |
| Symbology | 1D/2D, including UPC/EAN | 1D/2D, including UPC/EAN |
| Durability | IP52, 5 ft drop, 250 tumbles | IP52, 5 ft drop, 250 tumbles |
| Warranty | 60 months | 36 months (battery 12 months) |
| Observed guide price | €96 / £84 / $178 | €126 / £187 / $289 |

No third model was promoted to the recommendation. Several cheaper/alternative scanner sources
were attempted but did not yield clean, verifiable product/price data from this environment
(XJack 404, Honeywell 500, several price aggregators 404 or bot-blocked). The two Zebra models
are sufficient for the research decision because they directly cover the two required
connectivity shapes and have accessible official documentation.

### 3.3 Integration gotchas

#### Scan-then-terminator framing

A HID scanner does not send "a barcode object" to the browser. It sends keystrokes very
quickly, usually followed by Enter/Tab/CR.

The UI needs a dedicated scan-target behavior:

- A visible or explicitly focusable "Scan here" field.
- The field accepts rapid input.
- Submission is triggered by the terminator (default: Enter).
- If no terminator arrives, the UI should not silently treat the buffer as a scan. It can
  either wait, show a "press Enter to submit" hint, or fall back to manual entry.

Do not rely on keystroke speed alone to distinguish scanner from human. Speed heuristics are
fragile and unnecessary if the workflow explicitly focuses a scan field and submits on Enter.

#### Multi-scanner / multi-person household use

Browsers treat HID barcode scanners as ordinary keyboards. They generally do not expose a
stable "scanner device identity" to web pages the way WebHID might for special devices, and
Spisordning should not depend on that for v1.

Consequences:

- Two people scanning into the same browser tab at the same time is ambiguous. The UI cannot
  reliably say which human or which scanner produced which keystrokes.
- A shared Bluetooth scanner is fine for sequential use, but not for concurrent unattended
  scanning.
- If concurrent household use matters, the UI should require an active user/session context
  before starting a batch, or use separate devices/tabs per person.

Recommended v1 rule: **one active scanning session at a time per browser tab.** A shared
scanner is acceptable if the household member starts their own session or is clearly the active
user.

#### Bluetooth-specific gotchas

For DS2278:

- Pairing is documented as easy (Scan-to-Connect), but the scanner still needs to be paired to
  the host that has the focused browser field.
- If the scanner is paired to a phone, it types into the phone's focused field; if paired to a
  laptop/tablet, it types there instead.
- Battery charging should be part of the household routine. The Presentation Cradle is useful
  because it provides a fixed hands-free spot and charges during use.
- Bluetooth range/obstructions are unlikely to be a pantry problem, but they are a real
  difference from a USB cable.

---

## 4. Check-in flow design

Check-in means: a product enters the household inventory. The scan result is handed to
`LookupBarcode`, and the resolved product/ingredient pre-fills a `RecordPurchase`.

This flow is **not** the same as check-out. Check-in creates new inventory; it does not need
lot disambiguation.

### 4.1 Scan → `LookupBarcode` → pre-filled `RecordPurchase`

High-level state machine:

```text
[Start check-in]
   -> choose location (default: last used or household default)
   -> choose input method: Camera | Scanner | Manual
   -> capture GTIN
   -> call LookupBarcode(gtin)
   -> if resolved:
        show pre-filled RecordPurchase form
        user confirms/edits
        submit RecordPurchase(source: barcode_scan)
   -> if lookup exhausted:
        show manual resolution fallback
        user selects/creates ingredient and optional product
        continue to same RecordPurchase form
```

Pre-filled `RecordPurchase` fields:

| Field | Source |
|---|---|
| `ingredientId` | Resolved by `LookupBarcode` or manual fallback. Required. |
| `productId` | Resolved product if available. Optional except for upstream sources that require it, e.g. `shopping_order`. |
| `locationId` | User-selected location for this check-in. Default to last used or a sensible household default. |
| `quantity` | Default `1`, editable. |
| `unit` | Default from resolved product/ingredient if available, otherwise user-selected. |
| `bestBefore` | Optional; pre-fill if lookup/product data provides it, otherwise user-entered or blank. |
| `source` | `barcode_scan` for scan-initiated check-in. Use the appropriate upstream source for manual/non-scan flows. |

UI screens/states:

1. **Check-in start**
   - Location picker.
   - Input method selector: Camera / Scanner / Manual.
   - Optional "batch mode" toggle (see §4.3).

2. **Capture state**
   - Camera viewfinder, or focused scanner field, or manual GTIN input.
   - Cancel/back affordance.

3. **Resolving state**
   - Spinner or progress text.
   - Cancel affordance.
   - If resolution takes too long, allow manual continuation.

4. **Pre-filled purchase card**
   - Product/ingredient name.
   - Resolved GTIN.
   - Location.
   - Quantity/unit.
   - Best-before.
   - Confirm button.
   - "Not right?" button leading to manual correction.

5. **Success state**
   - Single mode: "Added" and prompt for next scan or finish.
   - Batch mode: add to pending list and immediately return to capture state.

### 4.2 Manual-fallback handoff when a scan doesn't resolve

There are two entry points into manual fallback:

#### A. Scan failed

The camera or scanner did not produce a usable GTIN.

UI:

- "Couldn't read a barcode."
- Buttons:
  - "Try scanning again"
  - "Enter GTIN manually"
  - "Choose ingredient manually"

If the user enters a GTIN manually, the flow continues to `LookupBarcode` exactly as if the
GTIN had been scanned. The only difference is the input method tag, not the downstream
resolution.

#### B. Lookup failed

The GTIN was captured, but `LookupBarcode` exhausted its chain:

1. normalize GTIN
2. existing `ProductIdentifier`
3. Open Food Facts
4. retailer lookup
5. manual fallback

This change does not redesign that chain. It only specifies the UI handoff:

- "We couldn't identify this barcode automatically."
- Show the captured GTIN.
- Ask the user to:
  - select an existing ingredient, or
  - create/search for the appropriate ingredient, and optionally
  - attach a specific product if one exists or should be created.
- Continue to the same pre-filled `RecordPurchase` card.

This satisfies the requirement that manual fallback be reachable from a failed scan, not only
from a missing lookup.

### 4.3 Batch check-in

**Recommendation: basic batch check-in is a v1 requirement.**

The household use case explicitly includes checking in a whole grocery haul. A scan flow that
forces full review after every single item would make dedicated scanner hardware much less
valuable and would make camera scanning slower than it needs to be.

Recommended v1 batch shape:

1. User starts "Batch check-in".
2. User chooses default location for the batch.
3. User scans items one by one.
4. Each resolved item is added to a **pending list** with minimal confirmation:
   - If lookup is confident and product/ingredient resolves cleanly, auto-add with default
     quantity `1`.
   - If lookup is ambiguous or fails, pause and resolve that item before continuing.
5. After the haul is scanned, user reviews the pending list:
   - edit quantity/unit/location/best-before
   - remove mistakes
   - submit all as individual `RecordPurchase` commands.

Deferred for v1:

- receipt/price matching
- automatic multipack quantity inference
- offline queueing
- multi-user concurrent batch merging
- camera auto-advance without user confirmation (could be added later as a preference)

Batch check-in is still a sequence of normal `RecordPurchase` commands. It does not invent a
new "batch purchase" inventory event.

---

## 5. Check-out flow design

Check-out means: inventory leaves the household. The scan result identifies a product/GTIN, but
the system must then choose a specific `InventoryLot` before calling
`RecordConsume`, `RecordDiscard`, or `RecordMarkEmpty`.

This is the genuinely new design question in this change.

### 5.1 Lot disambiguation when a GTIN matches multiple lots

A scanned manufacturer GTIN is not a lot ID. Two milk cartons can have the same GTIN but be
different lots because they were purchased at different times, have different best-before
dates, or are in different locations.

Candidate approaches:

| Approach | Behavior | Strengths | Weaknesses |
|---|---|---|---|
| Prompt user to pick | Show all matching lots; user selects one. | Always correct if user understands the list. | Slower; annoying if lots are indistinguishable. |
| Default to soonest expiry (FIFO) | Automatically choose the lot with earliest `bestBefore`. | Matches pantry intuition and upstream FIFO consumption reference behavior. | Can be wrong if user physically took a different carton. |
| Require location context | User selects "I am at Fridge/Pantry/Freezer" before scanning; filter lots by location. | Reduces ambiguity in common cases. | Adds a step; fails if lots share location. |
| Hybrid: default + confirm | Choose a default lot (soonest expiry, then location/recency), show it for one-tap confirmation, allow "pick another". | Fast for common case, safe for ambiguous case. | Slightly more UI. |

**Recommendation: hybrid default + confirm.**

Default lot selection order:

1. If the user selected a location context, filter to lots in that location first.
2. Among remaining candidates, choose the lot with the **soonest `bestBefore`**.
3. If `bestBefore` is missing/tied, prefer the lot with the most recent purchase/open event.
4. If still tied, present the user with a picker.

UI:

- Single candidate: show one card with lot details and action buttons.
- Multiple candidates: show a compact list:
  - location
  - best-before
  - quantity remaining
  - last updated/purchased
  - radio selection or tap-to-select
- The default candidate is pre-selected, but the user can change it before confirming.

This keeps the common case fast (scan → default lot → tap action) while preserving correctness
when two lots are genuinely ambiguous.

### 5.2 Scan → resolved lot → `RecordConsume` / `RecordDiscard` / `RecordMarkEmpty`

High-level state machine:

```text
[Start check-out]
   -> optional location context ("Fridge", "Pantry", "Freezer", etc.)
   -> choose input method: Camera | Scanner | Manual
   -> capture GTIN
   -> find matching InventoryLots
   -> if no lots:
        show "No inventory lot found for this barcode"
        offer: check-in instead, manual lot search, or cancel
   -> if one or more lots:
        select/default lot (see §5.1)
        choose action:
          - Consume
          - Discard
          - Mark empty
        submit corresponding command
```

Action details:

| Action | Command | Extra input | Notes |
|---|---|---|---|
| Consume | `RecordConsume(lotId, quantity, source)` | quantity | Source is `barcode_scan` for scan-initiated check-out. |
| Discard | `RecordDiscard(lotId, quantity, reason, source)` | quantity, reason (`expired` / `spoiled` / `other`) | Source is `barcode_scan` for scan-initiated check-out. |
| Mark empty | `RecordMarkEmpty(lotId, source)` | none | One-tap "finished it, into recycling" case. Upstream D7 treats this as a command that writes a `CONSUME` event with `source: mark_empty`, not a separate event kind. |

The "just finished it, into recycling" case should be the fastest path:

1. User starts check-out in **Mark empty** mode, or selects "Mark empty" after scanning.
2. User scans the empty package.
3. If exactly one plausible lot exists, show a single confirmation:
   - "Mark Milk (Fridge, expires 2026-08-21) empty?"
4. User taps confirm.
5. App calls `RecordMarkEmpty(lotId, source: mark_empty)`.

If multiple lots exist, the same lot-disambiguation UI appears first, then the one-tap
confirmation.

Check-out is deliberately not designed as a special case of check-in:

- Check-in resolves a barcode to a product/ingredient and creates a new lot.
- Check-out resolves a barcode to one or more existing lots and mutates a chosen lot.
- The UI may reuse the same scan capture component, but the post-scan state machines are
  separate.

### 5.3 Out of scope: label-printed lot-reference barcodes

This change explicitly does **not** cover scanning a label-printed barcode that directly
encodes an `InventoryLot` reference.

That is a different namespace and a different disambiguation problem:

- A manufacturer GTIN identifies a product type and may match many lots.
- A label-printed lot reference identifies a specific lot and is disambiguation-free by
  construction.

Label-printed barcodes belong to `research-inventory-label-printing`. The physical scanner or
camera can later be reused for that workflow, but this change's check-out design is only for
package barcodes/GTINs.

---

## 6. Recommendation: what to build first

Recommended approach: **both input methods, staged behind one scan-result abstraction.**

Do not choose "mobile-only forever" or "dedicated-scanner-only". The household has distinct use
cases for each:

- phone camera: always available, no hardware, good for occasional scans and check-out while
  moving around the kitchen;
- dedicated HID scanner: faster and more reliable for repeated scanning, especially batch
  grocery check-in.

Recommended staging:

### Stage 1 — Mobile camera scanning

Build first because:

- no hardware purchase is required;
- it works on the confirmed target platforms;
- it validates the shared scan-result → check-in/check-out workflow;
- it provides immediate household value even if no scanner is ever bought.

Stage 1 should include:

- `BarcodeDetector` feature detection;
- ZXing fallback;
- camera permission UX;
- manual GTIN entry;
- check-in flow with single and basic batch mode;
- check-out flow with lot disambiguation and `RecordConsume` / `RecordDiscard` /
  `RecordMarkEmpty`.

### Stage 2 — Dedicated HID scanner support

Build second because:

- it reuses the same check-in/check-out workflows;
- the main new work is a focused scan-target field and terminator handling;
- it makes batch check-in materially faster;
- it avoids buying or designing hardware-specific integrations before the web surface exists.

Stage 2 should include:

- "Scanner" input tab;
- focused scan-target field;
- Enter-terminator submission;
- guidance for pairing/placing a Bluetooth scanner;
- one-active-session rule for shared scanners.

### What not to do

- Do not implement scanning code before the web/PWA surface exists.
- Do not build a native mobile app decision into this change.
- Do not build label-printed lot barcode scanning here.
- Do not duplicate `LookupBarcode`'s GTIN resolution logic.
- Do not treat check-out as a mode of check-in; keep the post-scan flows separate.

---

## Open risks / things to verify before implementation

1. **Scanner suffix/terminator default.** Zebra docs confirm Keyboard Wedge mode, but the
   accessible pages did not clearly state the exact default terminator for the specific SKUs.
   Verify with the purchased unit and configure it to send Enter if possible.
2. **`BarcodeDetector` format coverage.** Even where the API exists, `getSupportedFormats()`
   must be checked for `ean_13` and `upc_a`.
3. **iOS camera permission recovery.** The web app cannot deeply integrate with iOS settings;
   manual entry must be excellent.
4. **Camera reliability on real packaging.** The implementation should be tested on reflective,
   curved, and poorly lit pantry packaging before considering camera scanning "done".
5. **Multi-scanner concurrency.** Browsers do not give the UI a reliable scanner identity for
   HID keyboard-wedge input. v1 should assume sequential use per tab/session.
6. **Price drift.** Zebra guide prices are "starting at" and location-based; verify actual
   retail price at purchase time.

---

## Sources

- OpenSpec change:
  - `openspec/changes/research-barcode-scanning-devices/proposal.md`
  - `openspec/changes/research-barcode-scanning-devices/tasks.md`
  - `openspec/changes/research-barcode-scanning-devices/specs/barcode-scanning/spec.md`
- Upstream pantry/inventory design:
  - `openspec/changes/implement-pantry-inventory/design.md` (D6, D7, D8, D9, Step 5 commands)
- Browser API support:
  - CanIUse `BarcodeDetector API`: `https://caniuse.com/mdn-api_barcodedetector`
  - WICG Shape Detection API: `https://wicg.github.io/shape-detection-api/`
  - MDN `BarcodeDetector`: `https://developer.mozilla.org/en-US/docs/Web/API/BarcodeDetector`
  - MDN `BarcodeDetector.getSupportedFormats()`:
    `https://developer.mozilla.org/en-US/docs/Web/API/BarcodeDetector/getSupportedFormats`
  - MDN `BarcodeDetector.detect()`:
    `https://developer.mozilla.org/en-US/docs/Web/API/BarcodeDetector/detect`
  - MDN `MediaDevices.getUserMedia()`:
    `https://developer.mozilla.org/en-US/docs/Web/API/MediaDevices/getUserMedia`
- JS decode libraries:
  - `@zxing/library`: `https://registry.npmjs.org/@zxing/library/latest`
  - `@zxing/browser`: `https://registry.npmjs.org/@zxing/browser/latest`
  - `html5-qrcode`: `https://registry.npmjs.org/html5-qrcode/latest`
- Hardware:
  - Zebra DS2208 product page:
    `https://www.zebra.com/us/en/products/scanners/general-purpose-handheld-scanners/ds2200-series/ds2208.html`
  - Zebra DS2278 product page:
    `https://www.zebra.com/us/en/products/scanners/general-purpose-handheld-scanners/ds2200-series/ds2278.html`
  - Zebra DS2200 Series Specification Sheet:
    `https://www.zebra.com/us/en/products/spec-sheets/scanners/general-purpose-scanners/handheld/ds2200-series.html`
