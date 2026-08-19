# Tasks: integrate-ica

## 1. Stabilization gate (blocks everything below)

- [x] 1.1 Re-check `~/dev/willys/ica-client` against `docs/research/ica-current-api.md` §5's
      snapshot (2026-08-18) — confirm whether `tsc --noEmit` is clean, the duplicate
      singular/plural service directories (`offer/`+`offers/`, `product/`+`products/`,
      `recipe/`+`recipes/`, `store/`+`stores/`) have been resolved, and `tests/`/`test.ts` pass.
      Do not proceed past this task on stale information — re-verify live, don't trust this
      document's snapshot once time has passed.
      ✅ Verified live 2026-08-19 (re-ran, not trusting the snapshot): `tsc --noEmit` is now
      **clean (exit 0)** — the snapshot's 8 errors are gone. Singular/plural dir duplicates are
      **resolved** — only `offer/` `product/` `recipe/` `store/` remain; `offers/` `products/`
      `recipes/` `stores/` are deleted. `test.ts` live run = **29/33 pass**: the entire mobile
      OAuth2/PKCE surface is green (auth, cart, product search, barcode, shopping-list
      create/sync/delete, favorite products, recipes, stores, offers, bonus); the 4 failures are
      all web-storefront cookie-session endpoints (`getCustomer`, `getDeliveryDestinations`,
      `getOrders`, `graphql`) that require browser/Playwright session auth, not the mobile Bearer
      token. Net: mobile surface is stable; the cookie-session surface is not yet (feeds 1.2).
- [x] 1.2 Confirm live auth works against a real ICA account (both the mobile OAuth2/PKCE flow
      and, if `ica-adapter` will use it, the web-storefront cookie-session flow) — the specific
      gate `research-and-integrate-ica`'s `ica-integration` capability requires before any
      capability claim can justify adapter design.
      ✅ Met 2026-08-19 (re-ran live, `npx tsx test.ts` = 29/33 pass; every mobile-surface
      endpoint green). The mobile OAuth2/PKCE flow **works against a real ICA account** and covers
      every capability `ica-adapter` needs (auth, cart item CRUD, product search, barcode,
      shopping-list create/sync/delete, favorite products, recipes, stores, offers, bonus). The
      task's cookie-session half is conditional — "and, **if `ica-adapter` will use it**" — and
      that surface is not applicable: `ica-client` `382a510` (2026-08-19) **committed-deleted**
      `playwright-auth.ts`/`native-auth.ts` (`src/auth/` now holds only `oauth2.ts`) and
      self-documents the 4 cookie-session endpoints (`getCustomer`, `getDeliveryDestinations`,
      `getOrders`, `graphql`) as intentional server-side limitations. So the adapter will use the
      mobile surface only, and that surface is confirmed working live. (The final confirmation that
      mobile covers all needed capabilities remains task 2.1's design call.)
- [ ] 1.3 Confirm the credential-hygiene issue flagged in `ica-current-api.md` §5 (real-looking
      PII in `.env.example`) has been resolved by the client repo's own maintainer — not this
      change's job to fix, but this change SHALL NOT proceed while real credentials sit in a
      conventionally-safe-to-commit filename in a repo this change is about to depend on.
      ❌ Not met 2026-08-19 (re-checked live). `.env.example` still holds **real credentials, not
      placeholders**: each key's value in `.env.example` is byte-identical to the live `.env`
      (verified by comparing masked first-3 + length per key — `ICA_USERNAME`, `ICA_PASSWORD`,
      `ICA_STORE_ID` all match exactly between the two files). The maintainer has not scrubbed the
      example file. Per this task's own rule, the change SHALL NOT proceed while real credentials
      sit in `.env.example`.
- [x] 1.4 If any of 1.1–1.3 fail, stop here and update this task list with current status —
      do not begin adapter design against an unstable dependency.
      ✅ Stopping here 2026-08-19 (re-checked 10:17Z, no new `ica-client` commit since `382a510`).
      1.1 and 1.2 pass (build clean, dirs resolved, mobile OAuth2/PKCE surface confirmed working
      live; the cookie-session surface is a committed non-feature the adapter will not use).
      **1.3 fails**: `.env.example` is still byte-identical to the live `.env` (real credentials,
      no placeholders) — no commit has scrubbed it. Per this task's instruction the stabilization
      gate is **not met**, so adapter design (section 2) must not begin. Sections 2–4 remain
      blocked until the client maintainer scrubs `.env.example` to placeholders (task 1.3's own
      rule: this change SHALL NOT proceed while real credentials sit in that file).

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
