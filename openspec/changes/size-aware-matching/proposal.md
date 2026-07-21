# Size-aware matching: honour the size written in a term

## Why

A term like "Coca cola zero 1,5 L" fails twice today: Willys search for the full string returns
50cl/33cl products (it ignores the size), and the size tokens ("1,5 l") pollute the name match,
scoring it 0.40 → review, on the wrong 50cl product. Shopping terms routinely carry a size
("mjölk 1,5l", "ägg 15-pack"), and the household clearly means that size.

## What Changes

- A term is split into a **name part** and an optional **size hint** (e.g. "Coca cola zero
  1,5 L" → name "coca cola zero", hint 1500 ml).
- The Willys **search query** uses the name part only, which returns better hits (the 1,5l
  product surfaces instead of only small cans).
- **Name-match confidence** is scored on the name part, so the size no longer drags it down.
- Among candidates, one whose package size matches the hint is **preferred**; a hint with no
  size match still resolves on name (quantityUncertain), never worse than today.
- The review picker orders hits so size-matching products come first.

## Capabilities

### Modified Capabilities

- `retailer-adapter`: requirement change — a size written in a term SHALL be parsed out and
  used to (a) search by name, (b) score confidence on the name, and (c) prefer the
  size-matching product.

## Impact

- `willys-client/apps/willys-adapter/core.ts` (splitSizeHint + size-aware scoring/search
  query), `server.ts` (search + picker use the name query and size ordering), jest tests
- Expected: "Coca cola zero 1,5 L" resolves to the 1,5l Pet, not a 50cl can
