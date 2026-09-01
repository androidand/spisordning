## Context

`willys-adapter` is a standalone Go HTTP service (built from `Dockerfile.adapter` in the
sibling `~/dev/store-clients` repo, specifically `willys-client/`) that wraps
`willys-client`'s generated Go client (itself generated from `willys-client/openapi.yaml`,
OpenAPI-first, same pattern as the sibling `skolmaten` project). It is deployed as its own
Proxmox LXC via Tengil (`stack:willys-adapter`, VMID 2335, `192.168.1.171:8402`) and holds a
live, logged-in Willys session plus its own state: a pin store (term → product code), an alias
store, and a needs-review queue with a pick-to-pin flow, all documented in
`openspec/specs/retailer-adapter/spec.md`. `docker-compose.yml` currently references it as a
peer service to `food-brain`.

food-brain itself is a layered Go+Postgres app with import-boundary enforcement
(`internal/architecturetest`, from the archived `establish-enforced-go-architecture` change):
`internal/domain` (pure), an application layer, `internal/persistence` (the only package
allowed to import `pgx`), `internal/httpapi`, `internal/mcptools`. 21 migrations under
`db/migrations/` already give food-brain its own relational schema for every other domain
concern (households, recipes, pantry, shopping, price intelligence, meal plans, nutrition).
Retailer resolution is the one remaining piece of durable state living outside that schema, in
a second process.

This surfaced while wiring up this repo's own Proxmox deployment via `katla` (Tengil's CLI):
`willys-adapter` was found already running standalone with a live session, and redeploying a
second instance from spisordning's own compose stack risked two processes independently
holding a session against the same Willys account.

## Goals / Non-Goals

**Goals:**
- food-brain calls `willys-client`'s Go client in-process; no HTTP hop, no second deployed
  service, no second live session against the Willys account.
- Pin store, alias store, and needs-review queue become food-brain's own Postgres tables behind
  `internal/persistence`, following the same repository pattern as every other domain object.
- The Apple Notes bridge (`expose-shopping-price-and-notes-bridge`) keeps working, now calling
  food-brain instead of the adapter directly.
- All `retailer-adapter` spec scenarios (pin resolution, alias rewriting, size-hint parsing,
  needs-review queueing, pick-to-pin) keep passing — this is a relocation of logic and state,
  not a behavior change to the resolution algorithm itself.

**Non-Goals:**
- No changes to REST/MCP/SSE/the React SPA beyond whatever new endpoints food-brain needs to
  expose for pin/review-queue management (previously served by the adapter's own HTTP API) —
  scoping those endpoints is a `tasks.md` item, not a redesign of the presentation layer.
- No changes to `store-clients`/`willys-client` itself. `Dockerfile.adapter` and the adapter's
  HTTP server can keep existing in that repo for other consumers; this change only stops
  spisordning from being one of them.
- Not adding retailers beyond Willys — ICA integration is tracked separately
  (`integrate-ica`/`research-and-integrate-ica`).

## Decisions

- **Consume `willys-client` via `go.work`, not a pinned module dependency.** `store-clients` is
  a sibling checkout on the same machine today; a `go.work` entry lets food-brain build against
  the local checkout without a publish step, matching how this machine already works day to day.
  Before any real (non-local) deploy, food-brain's `go.mod` needs a proper versioned
  `require` on the published module path so `katla stack deploy`'s repo-archive build doesn't
  depend on a workspace file that only exists on this Mac — tracked as a task, not a design risk,
  since `store-clients` already publishes Go modules conventionally (no new publish work needed,
  only wiring food-brain's `go.mod` to depend on the right version).
- **Pin/alias/review-queue tables live in food-brain's own schema, not a shared/imported
  schema.** These are food-brain's data now; no reason to keep them separate from
  `internal/persistence`'s existing repository pattern. Rejected: keeping the adapter's on-disk
  pin file format and just changing who reads it — that keeps a second, invisible source of
  truth outside Postgres, which is exactly the problem being fixed.
- **One-time data migration, not dual-write.** Since this is a single-household deployment, a
  one-shot import of the adapter's current pin/alias data into the new tables (part of
  `tasks.md`) is simpler and safer than running both stores in parallel during a transition
  window.
- **Retire the standalone adapter only after the embedded path is verified against the real
  Willys account**, not on the same PR that lands the embedding. Cutting over pin resolution
  and session handling is the risky part; keeping the old instance as a fallback until a real
  planning run has gone through the new path costs nothing extra.

## Risks / Trade-offs

- [Two live Willys sessions during the transition window (old adapter + new embedded client)]
  → Keep the standalone adapter's Willys credentials as the only ones in use until cutover;
  develop/test the embedded client against a review/staging path first, then swap credentials
  over in one step rather than running both live simultaneously.
- [Losing the adapter's pin/alias data in migration] → One-shot export from the running
  adapter instance (VMID 2335) before cutover; verify row counts against the new tables before
  decommissioning it.
- [`go.work`-based local dependency breaks the `katla stack deploy` repo-archive build, which
  packages this repo's own tracked/untracked files, not `store-clients`'] → Resolve before
  cutover by pinning a real versioned `require` in `go.mod` (see Decisions above); flagged
  explicitly so it isn't discovered only at deploy time.
- [Notes bridge or web UI depends on adapter HTTP endpoints not yet replicated in food-brain]
  → Inventory the adapter's actual endpoint surface (pin list/add, review queue, picks) as the
  first `tasks.md` item, before writing any embedding code.

## Migration Plan

1. Add `willys-client` as a food-brain dependency (`go.work` for local dev now; a versioned
   `go.mod` `require` before any real deploy relies on the archived repo build).
2. Add pin/alias/review-queue tables + `internal/persistence` repositories; import the current
   adapter's data into them.
3. Add food-brain-native resolve/pin/review endpoints (HTTP and/or MCP as needed); point the
   Apple Notes bridge at food-brain instead of the adapter.
4. Verify a full plan → resolve → wishlist run end-to-end against the real Willys account
   through the embedded path.
5. Swap Willys credentials to the embedded path, stop the standalone adapter (VMID 2335), remove
   it from any compose/deploy files that still reference it.
6. Update `openspec/specs/retailer-adapter/spec.md`'s purpose statement and
   `docs/infrastructure/deployment-and-access.md` to drop the adapter-as-a-service description.

Rollback: keep the standalone adapter instance stopped-but-not-destroyed for one planning cycle
after cutover, so reverting `ADAPTER_URL`-style HTTP calls is a redeploy, not a rebuild.

## Open Questions

- Does the web UI's pin/review-queue pages (if any exist under `web/src/pages`) call the
  adapter directly today, or only through food-brain already? Determines whether step 3 above
  is additive or also touches `web/`.
- Is there a real published version of `willys-client`'s Go module today, or does `store-clients`
  only tag/publish on request? Affects how much work the `go.mod` pinning step in Decisions
  actually is.
