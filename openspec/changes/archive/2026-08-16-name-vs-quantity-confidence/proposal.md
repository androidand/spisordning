# Split resolution confidence: name match vs quantity certainty

## Why

The resolver conflates two unrelated uncertainties. A perfect name match ("mjölk" →
Mjölk 3%) gets its confidence capped at 0.65 — under the 0.7 review threshold — merely
because the requirement is in pieces and the product is sold by weight. Both live runs
proved the consequence: the weekly plan wishlist under-filled (6/26), and the Apple Notes
bridge resolved **0 of 9** items from a real shopping note, six of which were flawless
matches. Count-based inputs are the normal case for groceries, so the current model makes
the review queue useless (everything is in it) and the sync flows inert.

## What Changes

- `confidence` means **name-match confidence only**; the package-size/unit reconciliation
  no longer caps it.
- New `quantityUncertain: boolean` on resolutions: true when the package size could not be
  reconciled with the requirement's unit. `packages` keeps its safe default of 1 and
  `resolvedQuantity` stays null in that case — the uncertainty is *visible*, not punitive.
- `needsReview` is driven by name confidence (and broken pins, as before). A perfect name
  match with unknown pack size resolves; a dubious name match still goes to review.

## Capabilities

### New Capabilities

<!-- none -->

### Modified Capabilities

- `retailer-adapter`: requirement change — resolution confidence SHALL reflect name-match
  quality only; quantity uncertainty SHALL be reported separately and SHALL NOT by itself
  send a resolution to review.

## Impact

- `willys-client/apps/willys-adapter/core.ts` (+ pins.ts pass-through), jest tests
- Consumers (Go `internal/retailer`, notes bridge) unaffected structurally — unknown JSON
  fields are ignored; Go type optionally gains the new field
- Expected effect: typical note items (Ost, Mjölk, Pasta…) resolve; genuinely weak matches
  (e.g. "Coca cola zero 1,5 L" at 0.40) still go to review
