# Tasks: research-barcode-scanning-devices

## 1. Upstream dependencies

- [x] 1.1 Confirm `implement-pantry-inventory`'s `LookupBarcode`, `RecordPurchase`,
      `RecordConsume`, `RecordDiscard`, `RecordMarkEmpty` command shapes are stable enough to
      design against (D6/D8, Step 5) before finalizing this change's flow designs — confirmed
      2026-08-19 in `implement-pantry-inventory/design.md` Step 5: `LookupBarcode(gtin)`,
      `RecordPurchase(ingredientId, productId?, locationId, quantity, unit, bestBefore?,
      source)`, `RecordConsume(lotId, quantity, source)`, `RecordDiscard(lotId, quantity,
      reason, source)`, `RecordMarkEmpty(lotId, source)`; `barcode_scan` is already a `source`
      value and D7/D8/D9 constrain the flow designs without redefining the commands
- [x] 1.2 Read `implement-pantry-inventory` `design.md` D6 in full before starting — do not
      re-derive or duplicate GTIN normalization/resolution logic here — read 2026-08-19; D6's
      chain is normalize GTIN → `ProductIdentifier` → Open Food Facts → retailer lookup → manual
      fallback, and this change consumes that chain rather than re-deriving it

## 2. Mobile / camera scanning research

- [x] 2.1 Survey the `BarcodeDetector` Web API: current browser support (which browsers/OS ship
      it natively, which need a polyfill), symbology coverage (EAN-13/UPC-A at minimum) —
      completed 2026-08-19 in `docs/research/barcode-scanning-devices.md` §2.1 using CanIUse
      `mdn-api_barcodedetector` and the WICG Shape Detection API: Chrome/Edge/Samsung/Chrome
      Android show support or partial support, Safari/iOS is disabled by default, Firefox is not
      supported, and the implementation must runtime-probe `getSupportedFormats()` for `ean_13`
      and `upc_a`
- [x] 2.2 Survey JS decode-library fallbacks for unsupported browsers (evaluate at least one
      concrete option; check license and maintenance status) — completed 2026-08-19 in
      `docs/research/barcode-scanning-devices.md` §2.2: evaluated `@zxing/library` 0.23.0
      (`Apache-2.0`, maintenance-mode signal), `@zxing/browser` 0.2.1 (`MIT`), and
      `html5-qrcode` 2.3.8 (`Apache-2.0`, older publish timestamp); recommended
      `@zxing/browser` + `@zxing/library`
- [x] 2.3 Investigate camera-permission UX on the platforms Spisordning actually targets
      (household phones — confirm which OS/browsers are in actual use before assuming iOS
      Safari/Android Chrome coverage is sufficient) — completed 2026-08-19 against the
      user-confirmed target matrix (`iOS Safari`, `Android Chrome`, `iOS other browsers`,
      `Android other browsers`) in `docs/research/barcode-scanning-devices.md` §2.3: HTTPS +
      `getUserMedia` required, native permission prompts, denied-camera recovery must expose
      manual GTIN entry, and iOS Safari cannot be assumed to have native `BarcodeDetector`
- [x] 2.4 Investigate real-world scan reliability concerns: poor lighting, reflective/curved
      packaging, damaged barcodes — determine whether a manual-entry fallback (typing the GTIN)
      is table stakes for v1 (it likely already exists via `implement-pantry-inventory` D6's
      manual fallback — confirm it's reachable from a failed scan, not just a missing lookup) —
      completed 2026-08-19 in `docs/research/barcode-scanning-devices.md` §2.4: manual GTIN
      entry is table stakes, and the design distinguishes "scan failed" manual entry from D6's
      "lookup failed" manual resolution

## 3. Dedicated scanner hardware research

- [x] 3.1 Confirm the keyboard-wedge (HID) assumption: a USB or Bluetooth barcode scanner types
      the decoded value followed by Enter/Tab into whatever field has focus, needing no custom
      driver — verified against actual product documentation, not just general knowledge —
      completed 2026-08-19 in `docs/research/barcode-scanning-devices.md` §3.1 using the Zebra
      DS2200 Series Specification Sheet: supported host interfaces include `Keyboard Wedge`,
      keyboard support covers more than 90 international keyboards, and DS2278 pairs via
      Bluetooth with Scan-to-Connect; exact default suffix/terminator remains a purchase-time
      verification item
- [x] 3.2 Survey 2-3 concrete affordable models (USB and Bluetooth) suitable for household use;
      note symbology support, pairing stability (Bluetooth), and price — completed 2026-08-19 in
      `docs/research/barcode-scanning-devices.md` §3.2: Zebra DS2208 (corded USB, 1D/2D,
      UPC/EAN, IP52, 60-month warranty, observed guide price €96/£84/$178) and Zebra DS2278
      (cordless Bluetooth 4.0, 1D/2D, UPC/EAN, Scan-to-Connect, 2400 mAh battery, IP52,
      36-month warranty, observed guide price €126/£187/$289)
- [x] 3.3 Identify integration gotchas: scan-then-terminator framing (does the UI need a
      dedicated "scan mode" input that captures rapid keystrokes-then-Enter distinctly from
      normal typing, to avoid a scan being misread as slow manual entry), multi-scanner
      household use (more than one person scanning concurrently) — completed 2026-08-19 in
      `docs/research/barcode-scanning-devices.md` §3.3: use a dedicated focused scan-target
      field submitted on Enter, do not rely on keystroke-speed heuristics, and assume one active
      scanning session per browser tab because HID scanner identity is not reliably exposed to
      web pages

## 4. Check-in flow design

- [x] 4.1 Design the scan → `LookupBarcode` → pre-filled `RecordPurchase` UI/workflow shape
      (screens/states, not code) — completed 2026-08-19 in
      `docs/research/barcode-scanning-devices.md` §4.1: states are start/capture/resolving/
      pre-filled purchase/success, and the pre-filled command uses `ingredientId`, optional
      `productId`, user-selected `locationId`, editable quantity/unit, optional `bestBefore`,
      and `source: barcode_scan` for scan-initiated check-in
- [x] 4.2 Design the manual-fallback handoff when a scan doesn't resolve
      (`implement-pantry-inventory` D6's chain exhausted) — completed 2026-08-19 in
      `docs/research/barcode-scanning-devices.md` §4.2: manual fallback is reachable both when
      the scan fails to produce a GTIN and when `LookupBarcode` exhausts its resolution chain,
      and both paths return to the same `RecordPurchase` pre-fill form
- [x] 4.3 Design batch check-in (scanning an entire grocery haul in sequence) — confirm whether
      this is a v1 requirement or deferred — completed 2026-08-19 in
      `docs/research/barcode-scanning-devices.md` §4.3: basic batch check-in is a v1
      requirement (scan sequence → pending list → review → individual `RecordPurchase`
      commands); receipt/price matching, multipack inference, offline queueing, and multi-user
      concurrent batch merging are deferred

## 5. Check-out flow design

- [x] 5.1 Design lot disambiguation when a scanned GTIN matches multiple existing
      `InventoryLot` rows (e.g. two milk cartons) — candidate approaches to evaluate: prompt the
      household member to pick (by location/recency), default to the lot expiring soonest
      (FIFO, consistent with `implement-pantry-inventory` task 9.3's Grocy-matched FIFO
      consumption default), or require location context (scanning while standing at the fridge)
      — completed 2026-08-19 in `docs/research/barcode-scanning-devices.md` §5.1: recommended
      hybrid is optional location filter + default to soonest `bestBefore` + recency tie-break +
      user can pick another lot before confirming
- [x] 5.2 Design the scan → resolved-lot → `RecordConsume`/`RecordDiscard`/`RecordMarkEmpty`
      UI/workflow shape, including the "just finished it, into recycling" one-tap `MarkEmpty`
      case the household specifically described — completed 2026-08-19 in
      `docs/research/barcode-scanning-devices.md` §5.2: check-out has its own state machine,
      presents lot candidates when ambiguous, supports `RecordConsume` and `RecordDiscard` with
      `source: barcode_scan`, and supports one-tap `RecordMarkEmpty` using upstream D7's
      `source: mark_empty` semantics
- [x] 5.3 Explicitly confirm out of scope: resolving a label-printed lot-reference barcode
      (`research-inventory-label-printing`'s concern, not a GTIN-ambiguity problem) — completed
      2026-08-19 in `docs/research/barcode-scanning-devices.md` §5.3: label-printed
      lot-reference barcodes are explicitly out of scope and belong to
      `research-inventory-label-printing`

## 6. Recommendation & docs

- [x] 6.1 Produce `docs/research/barcode-scanning-devices.md` recording findings, the concrete
      hardware/library recommendations, and the check-in/check-out flow designs — completed
      2026-08-19: document created at `docs/research/barcode-scanning-devices.md`
- [x] 6.2 Recommend, but do not implement, which approach(es) to build first (mobile-only,
      dedicated-scanner-only, or both) based on cost/effort vs. household benefit — completed
      2026-08-19 in `docs/research/barcode-scanning-devices.md` §6: build both input methods
      behind one scan-result abstraction, ship mobile camera scanning first, add dedicated HID
      scanner support second, and do not implement either before the web/PWA surface exists
- [x] 6.3 `openspec validate research-barcode-scanning-devices` — passed 2026-08-19:
      `Change 'research-barcode-scanning-devices' is valid`
