# Pinned product resolution: target + backup before fuzzy search

## Why

Fuzzy search can match any plausible product: "handdiskmedel" should mean *Yes Original
1,25l* in this household, not whichever Eldorado bottle ranks first that week. The live
first-slice run confirmed the shape of the problem — correct-but-uncertain matches pile into
the review queue, and free-text terms have no memory of what the household actually buys.
The notes-sync PoC proved the concept (`preferredProducts` term→code mapping) but has no
backup product and is trapped inside one app.

## What Changes

- The willys-adapter consults a **pin store** before fuzzy search: term → primary product
  code + optional backup code (+ optional alias rewrites). Pinned hits resolve with full
  confidence and `matchType: "pinned"`.
- **Availability-aware fallback**: if the pinned primary is unavailable, the backup is used
  (`"pinned-backup"`); if both fail, resolution falls back to fuzzy search with
  `needsReview` forced true — a broken pin must be surfaced, never silently fuzzy-matched.
- Pins are a household-curated file (committed example, gitignored live copy), editable by
  hand and via adapter endpoints (`GET /pins`, `POST /pins`), so a reviewed fuzzy match can
  be promoted into a pin — that's how the mapping grows.
- Aliases (e.g. "gurka" → "slanggurka") move into the same store and apply to fuzzy search
  for unpinned terms.

## Capabilities

### New Capabilities

<!-- none -->

### Modified Capabilities

- `retailer-adapter`: new requirement — resolution SHALL prefer household pins
  (primary, then backup, availability-checked) over fuzzy search, and broken pins SHALL be
  flagged for review.

## Impact

- `willys-client/apps/willys-adapter`: pin store module + resolution wiring + endpoints;
  `product-pins.example.json`; jest tests
- spisordning Go side unchanged (`Resolution.matchType` is already a string)
- notes-sync keeps its own mapping for now; future consolidation onto the adapter noted
