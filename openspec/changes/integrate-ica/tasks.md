# Tasks: integrate-ica

## 1. Stabilization gate (blocks everything below)

- [ ] 1.1 Re-check `~/dev/willys/ica-client` against `docs/research/ica-current-api.md` §5's
      snapshot (2026-08-18) — confirm whether `tsc --noEmit` is clean, the duplicate
      singular/plural service directories (`offer/`+`offers/`, `product/`+`products/`,
      `recipe/`+`recipes/`, `store/`+`stores/`) have been resolved, and `tests/`/`test.ts` pass.
      Do not proceed past this task on stale information — re-verify live, don't trust this
      document's snapshot once time has passed.
- [ ] 1.2 Confirm live auth works against a real ICA account (both the mobile OAuth2/PKCE flow
      and, if `ica-adapter` will use it, the web-storefront cookie-session flow) — the specific
      gate `research-and-integrate-ica`'s `ica-integration` capability requires before any
      capability claim can justify adapter design.
- [ ] 1.3 Confirm the credential-hygiene issue flagged in `ica-current-api.md` §5 (real-looking
      PII in `.env.example`) has been resolved by the client repo's own maintainer — not this
      change's job to fix, but this change SHALL NOT proceed while real credentials sit in a
      conventionally-safe-to-commit filename in a repo this change is about to depend on.
- [ ] 1.4 If any of 1.1–1.3 fail, stop here and update this task list with current status —
      do not begin adapter design against an unstable dependency.

## 2. Adapter design (only after task 1 passes)

- [ ] 2.1 Decide which ICA auth surface(s) `ica-adapter` uses for which capability — mobile
      OAuth2/PKCE API (shopping lists, recipes, bonus, barcode, offers) vs. web-storefront
      cookie-session API (cart, free-text product search) — per `ica-current-api.md` §5's
      finding that these are two separate auth models, not one. Record the decision and why,
      the same way `willys-adapter` documents its own session model.
- [ ] 2.2 Design `ica-adapter`'s HTTP surface mirroring `willys-adapter`'s exact shape
      (`/search`, `/products/:code`, `/offers`, `/shopping-lists`, `/resolve`, `/pins`,
      `/review/queue`), adding ICA-specific routes only where ICA's capability genuinely differs
      (barcode lookup, bonus balance) — do not invent new shapes where the existing pattern
      already fits.
- [ ] 2.3 Design the shopping-list sync strategy using ICA's MERGE semantics
      (`createdRows`/`changedRows`/`deletedRows`) per `ica-current-api.md` §4.2 — decide and
      document the conflict-resolution mode explicitly, don't leave it implicit.
- [ ] 2.4 Confirm the resolution `Resolution` shape (`matchType`, `confidence`, `needsReview`,
      `quantityUncertain`) carries over unchanged from `willys-adapter`'s — no ICA-specific
      weakening of the review-queue invariant.
- [ ] 2.5 Explicitly re-affirm in the design: no checkout/payment/order endpoint is implemented,
      regardless of what `ica-client`'s cart service technically permits (it currently exposes
      cart item CRUD only, no order placement — keep it that way on the Spisordning side too).

## 3. Adapter implementation (only after task 2's design is written down)

- [ ] 3.1 Stand up the `ica-adapter` HTTP service (new directory in `~/dev/willys`, alongside
      `willys-client`'s own `apps/willys-adapter`, or as a new top-level sibling — decide based
      on how `ica-client`'s own repo is laid out once stable) wrapping `ica-client`.
- [ ] 3.2 Add the Go-side client (`internal/retailer` extended, or a new package — task 2's
      design work decides which) so `food-brain` can call `ica-adapter` the same way it already
      calls `willys-adapter`.
- [ ] 3.3 Wire ICA barcode lookup and offers into the same catalog/price-intelligence paths
      Willys already feeds, not a parallel, divergent path.
- [ ] 3.4 Integration tests against `ica-adapter` (real or recorded), mirroring
      `internal/retailer`'s existing test coverage for Willys.

## 4. Verification

- [ ] 4.1 `openspec validate integrate-ica` passes.
- [ ] 4.2 No task in section 1 was skipped or assumed — each has a dated, evidenced check, not
      an inherited assumption from this proposal's own initial (necessarily stale-by-then) text.
- [ ] 4.3 Every invariant `research-and-integrate-ica/specs/ica-integration/spec.md` establishes
      is satisfied by the shipped adapter: no automated checkout, HA-specific design not
      inherited, `retailer-adapter`'s invariants followed.
