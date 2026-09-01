## 1. Inventory before touching anything

- [ ] 1.1 Enumerate the standalone willys-adapter's actual HTTP endpoint surface (pin
      list/add, alias list/add, review queue list/pick/dismiss, resolve, wishlist create) —
      the base surface lives in `food-brain-first-slice` until archived; confirm nothing has
      drifted from that.
- [ ] 1.2 Confirm whether `web/src/pages` (or `web/src/api`) calls the adapter directly today
      or only through food-brain already — resolves design.md's open question and determines
      whether task 4 below is additive or also touches `web/`.
- [ ] 1.3 Confirm whether `store-clients` publishes `willys-client`'s Go module with real
      version tags today, or only builds it in-repo — resolves design.md's other open question
      and scopes task 2.3's real effort.
- [ ] 1.4 Export the standalone adapter's current pin store and alias store content (whatever
      format it persists in on VMID 2335) to a file, for the one-shot import in task 3.

## 2. Embed the willys-client Go dependency

- [ ] 2.1 Add a `go.work` entry pointing food-brain at the local `store-clients` checkout for
      day-to-day development.
- [ ] 2.2 Wrap `willys-client`'s generated Go client behind a small food-brain-side interface
      (mirroring how other external integrations are isolated in this codebase) so the
      concrete client stays swappable/mockable in tests.
- [ ] 2.3 Once 1.3 is answered: pin a real versioned `require` in `go.mod` so a `katla stack
      deploy`-style repo-archive build (which packages only this repo's tracked/untracked
      files, not `store-clients`') doesn't silently depend on the local `go.work` file.

## 3. Move adapter-owned state into food-brain's schema

- [ ] 3.1 Design and add migration(s) under `db/migrations/` for pin store, alias store, and
      needs-review queue tables (term, product code, backup product code, source/confidence
      metadata, queued-at/picked-at timestamps as appropriate).
- [ ] 3.2 Add `internal/domain` types and `internal/persistence` repositories for the above,
      following the existing repository pattern (e.g. `internal/persistence/pantry.go`,
      `internal/persistence/price.go`).
- [ ] 3.3 Import the pin/alias data exported in task 1.4 into the new tables; verify row counts
      match the source.
- [ ] 3.4 Port the resolution logic (pinned-first, fallback to fuzzy search, size-hint parsing,
      confidence vs. quantityUncertain separation, needs-review queueing) from the adapter into
      a food-brain application-layer component that calls the wrapped client from task 2.2 and
      the repositories from 3.2. All `retailer-adapter` spec scenarios must still pass —
      recreate them as tests against this component.

## 4. Re-wire consumers

- [ ] 4.1 Add food-brain-native endpoints (HTTP and/or MCP) for whatever surface task 1.1 found
      real consumers using: resolve, pin list/add, alias list/add, review queue list/pick/dismiss.
- [ ] 4.2 Update the Apple Notes bridge (`expose-shopping-price-and-notes-bridge`) to call
      food-brain's new endpoints instead of the adapter's.
- [ ] 4.3 If task 1.2 found the web UI calling the adapter directly, re-point those calls at the
      new food-brain endpoints and regenerate the OpenAPI client (`web/src/generated`).

## 5. Cutover and retire the standalone adapter

- [ ] 5.1 Run a full plan → resolve → wishlist cycle end-to-end against the real Willys account
      through the embedded path, with the standalone adapter's credentials still the only ones
      logged in (per design.md's "no dual sessions" decision — develop/test 3.4/4.1 against a
      review path first, don't log the embedded client in with real creds until this step).
- [ ] 5.2 Swap the real Willys credentials over to the embedded path.
- [ ] 5.3 Stop (don't destroy yet) the standalone adapter instance (VMID 2335) via
      `katla app kill` — hold for one planning cycle as a rollback point per design.md's
      Migration Plan, then destroy it in a follow-up once confidence is high.
- [ ] 5.4 Remove `willys-adapter` from any remaining compose/deploy references (already dropped
      from `docker-compose.yml` in the Proxmox-deploy work that triggered this proposal —
      confirm nothing else still names it).

## 6. Docs and spec cleanup

- [ ] 6.1 Update `openspec/specs/retailer-adapter/spec.md`'s purpose statement to describe
      food-brain owning retailer session state directly (the delta in this change only updates
      the Apple Notes requirement; the purpose prose needs a manual edit at archive time).
- [ ] 6.2 Update `docs/infrastructure/deployment-and-access.md` to drop willys-adapter as a
      separately-deployed service.
- [ ] 6.3 Update `AGENTS.md`/`docs/research/current-state.md` wherever they describe the
      adapter as a standalone service, per this repo's convention of keeping those in sync
      with what's actually true (see how `deploy-food-brain-to-proxmox` tasks 4.2/4.3 handled
      the same kind of doc drift).
