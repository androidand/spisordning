## Context

Resolution today is one-shot fuzzy: `/resolve` searches Willys per requirement and picks the
best lexical match (`apps/willys-adapter/core.ts`). The notes-sync PoC (live on the Mac as a
launchd service) already proved term→code pinning works for the household, but without a
backup product, availability awareness, or reuse outside notes-sync. The live first-slice
run showed fuzzy confidence alone under-fills the wishlist.

## Goals / Non-Goals

**Goals:**
- Deterministic, household-specific resolution for known terms; fuzzy only for the unknown.
- Backup product per pin; broken pins surface for review instead of degrading silently.
- Pins grow over time (promote reviewed matches) and survive restarts.

**Non-Goals:**
- Migrating notes-sync onto the adapter (future consolidation).
- DB-backed pins — file-based until spisordning's Postgres wiring lands; the file format is
  the future table's shape.
- Auto-learning pins from purchase history (later; this store is the substrate for it).

## Decisions

- **D1: Pin store shape.** One JSON file:
  `{"pins": {"<term>": {"primary": "<code>", "backup": "<code>?", "note": "<label>?"}}, "aliases": {"<term>": "<searchTerm>"}}`.
  Terms are normalized (lowercase, trimmed) on lookup. Lookup tries the requirement's
  `searchTerm`, then `ingredientId`.
- **D2: Availability check via product detail.** A pinned code is "available" when
  `GET /axfood/rest/v1/p/{code}` succeeds and the product is not out of stock / not
  unsalable online. Detail also supplies `displayVolume` so package counts keep working for
  pinned items.
- **D3: Confidence semantics.** `pinned` → confidence 1.0; `pinned-backup` → 0.95; both
  `needsReview: false`. Both-dead → fuzzy result with `needsReview: true` and the pin
  failure noted, so the review queue says "your pin is broken", not "low confidence".
- **D4: Growable via endpoints.** `GET /pins` and `POST /pins {term, primary, backup?, note?}`
  write the live file (gitignored; `product-pins.example.json` committed). File writes are
  atomic (temp + rename).
- **D5: Pure core, thin IO.** Pin lookup/normalization and the resolve-order decision live in
  a pure module (like `core.ts`) tested without HTTP; the server supplies availability.

## Risks / Trade-offs

- Product codes rot (assortment changes): mitigated by the availability check + backup +
  review flagging; the pin file's `note` keeps entries human-auditable.
- Two mapping stores exist during the transition (notes-sync's `preferences.json` and the
  adapter's pins): acceptable short-term; consolidation is an explicit follow-up.
