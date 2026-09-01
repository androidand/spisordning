## Why

`willys-adapter` is deployed as its own standalone service (separate LXC on Proxmox, its own
process, its own live Willys login session) that food-brain talks to over HTTP. But it isn't a
thin passthrough over the `store-clients` Go client it wraps — the adapter owns real domain
state and logic (a pin store, an alias store, a needs-review queue, size-hint parsing) that
belongs in spisordning's own layered architecture and Postgres schema, not in a second service
with its own file/process-local state. Splitting it out buys no real isolation today (it's
still one household, one Willys account, one team) and costs a second deploy target, a second
state store to keep consistent, and — surfaced while wiring up this repo's own Proxmox
deployment via `katla` — the operational risk of two processes independently holding a session
against the same Willys account. `store-clients`' `willys-client` is already OpenAPI-first with
a generated Go client (`willys-client/go`), so there is no technical reason food-brain can't
import it directly as a library.

## What Changes

- Add `store-clients`' `willys-client` Go module as a direct dependency of food-brain (via
  `go.work` or a pinned module dependency — whichever this repo's existing module setup makes
  idiomatic) and call it in-process instead of over HTTP.
- Move the pin store, alias store, and needs-review queue out of the adapter's own
  file/process-local state and into spisordning's own Postgres schema
  (`db/migrations/`) behind `internal/persistence`, per this repo's enforced layering
  (`internal/architecturetest`).
- Re-point the Apple Notes bridge (`expose-shopping-price-and-notes-bridge`) at food-brain's own
  resolve/wishlist logic instead of the adapter's HTTP endpoints — no behavior change from the
  note-taker's perspective, only what's on the other end of the call.
- Retire the standalone `willys-adapter` deployment (`stack:willys-adapter`, VMID 2335 on
  Proxmox) once the embedded path is verified against the real Willys account — **BREAKING** for
  anything still pointed at its HTTP endpoints directly.
- Update `openspec/specs/retailer-adapter/spec.md`'s purpose statement, which currently
  describes "wrapping the Willys client behind a stable HTTP interface so the Food Brain never
  handles retailer session state" — under this change food-brain *does* own retailer session
  state directly, in-process.

## Capabilities

### New Capabilities
(none — this reshapes where an existing capability's logic and state live, it doesn't add a new
domain capability)

### Modified Capabilities
- `retailer-adapter`: resolution (pins, aliases, needs-review queue, size-hint parsing) moves
  from a separately-deployed HTTP adapter's own state into food-brain's own persistence layer,
  called via an embedded Go client instead of HTTP. Requirements describing "the adapter"
  SHALL be read as "food-brain's retailer-resolution component" — behavior is preserved, the
  process boundary is not.

## Impact

- **Code**: `cmd/food-brain`, a new `internal/persistence` package (or extension of an existing
  one) for pins/aliases/review-queue, `internal/domain` types for the same, `go.mod`/`go.work`
  to depend on `store-clients`' `willys-client` Go module, `internal/httpapi` if any of the
  adapter's endpoints (pin list/add, review queue, picks) need a food-brain-native equivalent
  for the web UI.
- **Data**: new migration(s) under `db/migrations/` for pins/aliases/review-queue tables;
  one-time migration of whatever pin/alias data currently lives in the standalone adapter's own
  storage.
- **Deploy**: removes `willys-adapter` as a service from `docker-compose.yml`/the Tengil deploy
  target once cut over; retires the standalone Proxmox instance (VMID 2335).
- **Sibling repo**: no changes required in `store-clients` itself — this only changes how
  `willys-client` is consumed (embedded Go import vs. via the adapter's HTTP server built from
  `Dockerfile.adapter`). `Dockerfile.adapter`/the adapter HTTP server can stay in that repo for
  other consumers or be deprecated separately; out of scope here.
- **Docs**: `docs/infrastructure/deployment-and-access.md` and this repo's `docker-compose.yml`
  header comment, both of which currently describe `willys-adapter` as a deployed service.
