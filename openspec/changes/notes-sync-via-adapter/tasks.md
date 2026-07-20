# Tasks: notes-sync-via-adapter

## 1. Extract + bridge

- [x] 1.1 `apps/notes-sync/notes.ts`: extract the osascript note readers into a reusable
      `loadNote(title) → {body, html}`; `sync.ts` re-uses it (no behaviour change)
- [x] 1.2 `apps/notes-sync/bridge.ts`: config mapping → loadNote → `core.parseNoteText` →
      requirements → adapter `/resolve` → `/shopping-lists`; no `WillysClient` import
- [x] 1.3 Dry-run default + `--apply`; `--adapter-url` (default `http://localhost:8402`);
      `--mapping <id>`; review items reported, never added

## 2. Watch + parity

- [x] 2.1 `--watch --watch-interval-sec N`: re-run on a timer, skip unchanged notes via a
      content hash
- [x] 2.2 Fail loudly with a clear message if the adapter is unreachable

## 3. Cutover

- [x] 3.1 `sync:notes:bridge` npm script; mark `sync.ts`'s resolution/wishlist path superseded
      in its header comment
- [x] 3.2 Repointed `com.andreas.willys-notes-sync.plist` to the bridge (old plist backed up `.bak-20260719`); added `com.andreas.willys-adapter` launchd service so the bridge has its always-on dependency
- [x] 3.3 README: notes-sync is now an adapter source; pins live in the adapter

## 4. Tests & verification

- [x] 4.1 Jest: bridge maps parsed items → requirements, posts to a fake adapter, adds only
      confident items, honours dry-run
- [~] 4.2 Bridge logic verified (169 tests) + adapter service live-healthy; live Apple Notes READ blocked from headless shell (osascript needs the GUI session) — owner verifies in-session
- [x] 4.3 Reloaded launchd on the bridge (service com.andreas.willys-notes-sync now runs notes:bridge:watch; adapter service healthy on :8402)
