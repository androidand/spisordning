# Tasks: research-barcode-scanning-devices

## 1. Upstream dependencies

- [ ] 1.1 Confirm `implement-pantry-inventory`'s `LookupBarcode`, `RecordPurchase`,
      `RecordConsume`, `RecordDiscard`, `RecordMarkEmpty` command shapes are stable enough to
      design against (D6/D8, Step 5) before finalizing this change's flow designs
- [ ] 1.2 Read `implement-pantry-inventory` `design.md` D6 in full before starting — do not
      re-derive or duplicate GTIN normalization/resolution logic here

## 2. Mobile / camera scanning research

- [ ] 2.1 Survey the `BarcodeDetector` Web API: current browser support (which browsers/OS ship
      it natively, which need a polyfill), symbology coverage (EAN-13/UPC-A at minimum)
- [ ] 2.2 Survey JS decode-library fallbacks for unsupported browsers (evaluate at least one
      concrete option; check license and maintenance status)
- [ ] 2.3 Investigate camera-permission UX on the platforms Spisordning actually targets
      (household phones — confirm which OS/browsers are in actual use before assuming iOS
      Safari/Android Chrome coverage is sufficient)
- [ ] 2.4 Investigate real-world scan reliability concerns: poor lighting, reflective/curved
      packaging, damaged barcodes — determine whether a manual-entry fallback (typing the GTIN)
      is table stakes for v1 (it likely already exists via `implement-pantry-inventory` D6's
      manual fallback — confirm it's reachable from a failed scan, not just a missing lookup)

## 3. Dedicated scanner hardware research

- [ ] 3.1 Confirm the keyboard-wedge (HID) assumption: a USB or Bluetooth barcode scanner types
      the decoded value followed by Enter/Tab into whatever field has focus, needing no custom
      driver — verify against actual product documentation, not just general knowledge
- [ ] 3.2 Survey 2-3 concrete affordable models (USB and Bluetooth) suitable for household use;
      note symbology support, pairing stability (Bluetooth), and price
- [ ] 3.3 Identify integration gotchas: scan-then-terminator framing (does the UI need a
      dedicated "scan mode" input that captures rapid keystrokes-then-Enter distinctly from
      normal typing, to avoid a scan being misread as slow manual entry), multi-scanner
      household use (more than one person scanning concurrently)

## 4. Check-in flow design

- [ ] 4.1 Design the scan → `LookupBarcode` → pre-filled `RecordPurchase` UI/workflow shape
      (screens/states, not code)
- [ ] 4.2 Design the manual-fallback handoff when a scan doesn't resolve
      (`implement-pantry-inventory` D6's chain exhausted)
- [ ] 4.3 Design batch check-in (scanning an entire grocery haul in sequence) — confirm whether
      this is a v1 requirement or deferred

## 5. Check-out flow design

- [ ] 5.1 Design lot disambiguation when a scanned GTIN matches multiple existing
      `InventoryLot` rows (e.g. two milk cartons) — candidate approaches to evaluate: prompt the
      household member to pick (by location/recency), default to the lot expiring soonest
      (FIFO, consistent with `implement-pantry-inventory` task 9.3's Grocy-matched FIFO
      consumption default), or require location context (scanning while standing at the fridge)
- [ ] 5.2 Design the scan → resolved-lot → `RecordConsume`/`RecordDiscard`/`RecordMarkEmpty`
      UI/workflow shape, including the "just finished it, into recycling" one-tap `MarkEmpty`
      case the household specifically described
- [ ] 5.3 Explicitly confirm out of scope: resolving a label-printed lot-reference barcode
      (`research-inventory-label-printing`'s concern, not a GTIN-ambiguity problem)

## 6. Recommendation & docs

- [ ] 6.1 Produce `docs/research/barcode-scanning-devices.md` recording findings, the concrete
      hardware/library recommendations, and the check-in/check-out flow designs
- [ ] 6.2 Recommend, but do not implement, which approach(es) to build first (mobile-only,
      dedicated-scanner-only, or both) based on cost/effort vs. household benefit
- [ ] 6.3 `openspec validate research-barcode-scanning-devices`
