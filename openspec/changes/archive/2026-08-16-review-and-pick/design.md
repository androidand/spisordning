## Context

Pins exist (file store + GET/POST /pins) and /resolve flags needs-review, but nothing
connects them: a new term stays unresolvable until someone finds a product code by hand.
The adapter is a long-running LAN service with search access — the natural place for a
small review surface. The watcher skips unchanged notes via a content hash, so a pin pick
alone would not re-sync today.

## Goals / Non-Goals

**Goals:** zero-friction pick-from-hits pinning; picks take effect within a watch cycle;
queue reflects reality (confident resolution clears it).
**Non-Goals:** auth on the LAN page (homelab scope); persistence of the queue across
adapter restarts (it repopulates on the next sync pass); a full SPA.

## Decisions

- D1: Queue is in-memory in the adapter (Map keyed by normalized term, storing quantity/unit/
  lastSeen), populated/cleared inside /resolve. Restart loss is acceptable: the 30s watcher
  repopulates it.
- D2: /review is server-rendered HTML + vanilla JS (no build step): per term, live search
  (top 6 hits with name/size/price/image); tap = POST /pins (primary), long-form allows
  backup; dismiss = DELETE /review/:term.
- D3: /review/queue returns JSON for programmatic use (future HA card).
- D4: Bridge watch hash = sha256(note.body + note.html + pinsETag) where pinsETag is fetched
  from GET /pins each cycle (local, cheap). Pin change → hash change → re-sync.
- D5: Queue module is pure (reviewQueue.ts) and unit-tested; the page handler stays thin.

## Risks / Trade-offs

- Unauthenticated LAN page can modify pins: consistent with the adapter's existing
  unauthenticated endpoints on the homelab; revisit if ever exposed beyond LAN.
- Re-sync on pin change re-adds ALL confident items additively? No: additive increment only
  applies quantities from the note; a re-sync of an unchanged note would double quantities.
  Mitigation: bridge tracks per-mapping last-synced item set; on pins-only change it syncs
  only terms that were previously unresolved (new confident items), not the whole list.
