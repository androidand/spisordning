# Integrate notes-sync into the adapter (retire the standalone stack)

## Why

The Apple Notes → Willys wishlist PoC (`apps/notes-sync`, running as launchd service
`com.andreas.willys-notes-sync`) has its own login, session cache, product search, alias /
preferred-product mapping, and wishlist creation — all of which the willys-adapter now owns,
and does better (pins with backup + availability, one shared session). Two copies of the
same retailer logic drift apart; the adapter's pins are the canonical place for "what this
household means by a term". Notes-sync should become a thin **Apple Notes source** that feeds
the adapter, not a parallel stack.

## What Changes

- Extract the Apple Notes readers (osascript) into a small reusable module; keep `core.ts`'s
  pure checklist parser as-is.
- New thin entrypoint `bridge.ts`: read the mapped note → parse items → POST the parsed
  terms to the adapter's `/resolve` → create/extend the wishlist via `/shopping-lists`.
  Preserves the PoC's behaviours: mapping config, dry-run default, `--apply`, watch mode,
  additive wishlist semantics, and needs-review items are reported and never silently added.
- **Delete the duplication**: the standalone login/session-cache/auth, `resolveItems`
  (own search), and `createWishlist` paths are no longer the sync path — resolution and
  wishlist writes go through the adapter (so notes-sync automatically benefits from pins).
- Repoint the launchd service to the bridge; document that it now depends on the adapter
  running (the always-on service).
- Aliases / preferred-products migrate conceptually to adapter pins (no live
  `preferences.json` exists today, so nothing to port — documented for future users).

## Capabilities

### New Capabilities

<!-- none -->

### Modified Capabilities

- `retailer-adapter`: new requirement — an Apple Notes checklist can drive the adapter's
  resolve + wishlist flow via a thin bridge, reusing pins; no independent retailer session.

## Impact

- `willys-client/apps/notes-sync`: new `notes.ts` (extracted readers) + `bridge.ts`;
  `sync.ts` retained but no longer the sync entrypoint (its resolution/wishlist path is
  superseded); new `sync:notes:bridge` npm script
- `~/Library/LaunchAgents/com.andreas.willys-notes-sync.plist`: repointed to the bridge
- Out of scope: pull-wishlist-back-into-note (needs a wishlist read endpoint; follow-up)
