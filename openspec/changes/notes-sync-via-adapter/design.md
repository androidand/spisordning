## Context

`apps/notes-sync/sync.ts` (1529 lines) does everything: osascript note reading, checklist
parsing (delegated to the pure `core.ts`), Willys login + session cache, `resolveItems`
(its own `client.search.searchProducts` + alias/preferredProduct maps), and wishlist
creation. The willys-adapter now owns session, pins (term→primary+backup, availability-
checked), `/resolve`, and `/shopping-lists`. The launchd service
`com.andreas.willys-notes-sync` runs `sync.ts --mapping 1 --apply --watch` for
"Köp: Testlista" → wishlist "Testlista". No live `preferences.json` exists.

## Goals / Non-Goals

**Goals:**
- One retailer stack (the adapter). Notes-sync keeps only what's unique: reading + parsing an
  Apple Note.
- Preserve PoC behaviours: mapping config, dry-run default, `--apply`, watch, additive
  wishlist, review items never auto-added.
- Notes items automatically benefit from adapter pins.

**Non-Goals:**
- Pull-wishlist-back-into-note (needs an adapter wishlist-read endpoint) — follow-up.
- Deleting `sync.ts` outright — retained (its osascript writers/pull feature) but no longer
  the sync entrypoint; the bridge supersedes its resolution/wishlist path.
- Porting `preferences.json` (none exists; pins are the new home, documented).

## Decisions

- **D1: Extract note readers.** Move `readAppleNotePlainText` / `readAppleNoteBodyHtml` into
  `apps/notes-sync/notes.ts` as a reusable `loadNote(title) → {body, html}`. `sync.ts` and
  `bridge.ts` both use it; no logic change.
- **D2: bridge.ts is thin and HTTP-only to Willys via the adapter.** Flow: load mapping from
  `config.json` → `loadNote` → `core.parseNoteText` → map each `ParsedNoteItem` to a
  `{ ingredientId, searchTerm: term, quantity, unit: "st" }` requirement → `POST /resolve` →
  partition confident vs needs-review → `POST /shopping-lists` with the confident items
  (name = mapping's `wishlistName`). No `WillysClient` import at all.
- **D3: Behaviour parity.** Dry-run is default; `--apply` creates the wishlist. `--watch
  --watch-interval-sec N` re-runs on a timer (reuse the note-hash snapshot idea to skip
  unchanged notes). Review items printed, never added — same contract as the adapter's
  `food-brain plan`.
- **D4: Additive semantics via the adapter.** `/shopping-lists` already increments quantities
  on an existing wishlist (the wishlist add path uses `increment: true`), so additive
  behaviour is preserved without notes-sync reading the wishlist.
- **D5: Repoint launchd.** Plist runs the new `sync:notes:bridge` script; a comment/log line
  states the adapter must be running. Old plist backed up before edit.

## Risks / Trade-offs

- **New runtime dependency**: the bridge needs the adapter up. On the homelab the adapter is
  the always-on service; on the Mac both run. The bridge fails loudly (clear error) if the
  adapter is unreachable, rather than silently doing nothing.
- **Unit defaulting**: note items are counts ("2 mjölk"), mapped to `unit: "st"`; the adapter
  handles piece/weight reconciliation and review-flagging exactly as for plan requirements.
- Two entrypoints (`sync.ts`, `bridge.ts`) briefly coexist; the change repoints the only
  consumer (launchd) to the bridge, and marks `sync.ts`'s sync path superseded in its header.
