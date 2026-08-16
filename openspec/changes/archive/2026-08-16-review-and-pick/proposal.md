# Review & pick: choose the default product for new terms from live search hits

## Why

Writing a new keyword on a list ("majskolvar") currently dead-ends in the review pile: the
term won't sync until someone hand-crafts a pin with a product code found elsewhere. The
household should instead be shown what Willys' search returns for the term and simply pick
the product they mean — that pick becomes the pin (with optional backup), and the next sync
pass resolves it. This closes the pins learning loop with an actual picking experience.

## What Changes

- The adapter keeps a **review queue**: every `/resolve` that flags a term needs-review adds
  it; a later confident resolution (e.g. once pinned) removes it.
- **`GET /review`**: a small server-rendered page (LAN, phone-friendly) listing queued terms,
  each with live Willys search hits (name, size, price, image). Tapping a hit pins it as the
  term's default; a second tap can set the backup. Terms can also be dismissed.
- `GET /review/queue` exposes the queue as JSON; `DELETE /review/:term` dismisses.
- The notes watcher's change-hash incorporates the **pin store version**, so a pick takes
  effect on the next watch cycle (≤30 s) without editing the note.

## Capabilities

### Modified Capabilities

- `retailer-adapter`: new requirement — needs-review terms SHALL be queued and pickable from
  live search hits, with the pick persisted as a pin; pin changes SHALL trigger re-sync of
  watched notes.

## Impact

- `willys-client/apps/willys-adapter`: review queue module + `/review` endpoints + page
- `apps/notes-sync/bridge.ts`: watch hash includes pins state
- jest tests for the queue and hash logic
