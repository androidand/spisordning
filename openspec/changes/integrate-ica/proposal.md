## Why

`research-and-integrate-ica` concluded ICA integration is "viable to design toward, not yet to
build," gated on live auth verification and a licensing/ToS question — pure research, no
adapter code, per its own explicit scope. Since that research landed, a **real implementation
attempt has independently started**: a standalone TypeScript package at
`~/dev/willys/ica-client`, structurally mirroring the existing, live `~/dev/willys/willys-client`
(layered `IcaClient` composing `HttpClient` + `GraphQLClient` + per-concern `*Service` classes).
It is early-stage and actively being stabilized by another agent as of this proposal — not yet
building cleanly, no passing tests, a mid-refactor duplicate-directory state, and one real
credential-hygiene issue flagged directly to the repo owner (not this change's job to fix,
tracked here only as a known blocker). `docs/research/ica-current-api.md` §5 documents this
scaffold's current state in full and should be treated as the up-to-date source of truth over
this proposal's own summary, which will drift.

This change exists so that once `ica-client` stabilizes, the Spisordning-side integration work
(an `ica-adapter` HTTP service, wired the same way `willys-adapter` already is) has a
tracked, backlog-discoverable home — mirroring exactly how `food-brain-first-slice` and the
`retailer-adapter` capability track the Willys side. Without this change, that work has nowhere
to land except ad-hoc, undiscoverable effort.

## What Changes

- Track `~/dev/willys/ica-client`'s stabilization (build clean, tests green, live auth
  confirmed working against a real account) as an explicit prerequisite gate — this change's
  early tasks are verification, not new code, because the client isn't ready yet.
- Once stabilized, wrap it behind a standalone `ica-adapter` HTTP service — same shape as
  `willys-adapter` (`apps/willys-adapter` in the sibling repo): owns ICA session/token state,
  exposes `/search`, `/products/:code`, `/offers`, `/shopping-lists`, `/resolve`, `/pins`,
  `/review/queue`, plus ICA-specific routes for barcode lookup and bonus balance.
- Add a Go-side client in `internal/retailer` (or a new `internal/icaretailer` if the existing
  package turns out too Willys-coupled to generalize cleanly — a call this change's design work
  makes explicitly, not silently) so `food-brain` can resolve shopping requirements against ICA
  the same way it already does against Willys.
- Inherit, not re-derive, every invariant `research-and-integrate-ica`'s `ica-integration`
  capability already established: no automated checkout/payment/slot-booking, retailer product
  identity kept distinct from canonical `ingredient`, low-confidence resolutions routed to
  human review, and no Home-Assistant-specific design inherited from `ha-ica-todo`.
- Two ICA auth surfaces exist per `ica-current-api.md` §5 (mobile OAuth2/PKCE API for shopping
  lists/recipes/bonus, and a separate cookie-session web-storefront API for cart/product
  search) — this change's design work explicitly decides which surface(s) `ica-adapter` uses
  for which capability, rather than assuming one auth model covers everything.

## Non-Goals

- No work on `~/dev/willys/ica-client` itself — that repo's stabilization is explicitly another
  agent's active work; this change only tracks its readiness as a gate.
- No automated checkout, payment, or order placement — inherited invariant, not re-litigated.
- No changes to the existing `retailer-adapter`/Willys capability.
- No BankID-gated (ICA Banken) account support unless a future update to this proposal
  explicitly scopes it in — `ica-current-api.md` flags this as unverified/likely unhandled.

## Capabilities

### New Capabilities

- `ica-adapter`: the Spisordning-side HTTP wrapper around `ica-client`, structurally parallel
  to `retailer-adapter` (Willys). Currently records only the gating/inheritance requirements
  (this change's early-stage scope); grows real resolve/search/wishlist requirements once
  `ica-client` stabilizes and adapter implementation actually starts.

### Modified Capabilities

<!-- none — retailer-adapter (Willys) and ica-integration (research invariants) are both
     referenced, not modified -->

## Impact

- Affected code: none yet in this repo (gated on `ica-client` stabilization — see Non-Goals).
  Eventually: a new `internal/retailer`-adjacent Go client, no changes to existing Willys code
  paths.
- Depends on: `~/dev/willys/ica-client` (sibling repo, not owned by Spisordning) reaching a
  stable, tested, live-auth-verified state — tracked as this change's task 1, re-checked against
  `docs/research/ica-current-api.md` §5 before any adapter work begins.
- Cross-references `openspec/specs/retailer-adapter/spec.md` (structural template) and
  `research-and-integrate-ica/specs/ica-integration/spec.md` (inherited invariants) rather than
  duplicating either.
- Part of Epic F: Retailer, Pricing & Commerce (tracking issue #6).
