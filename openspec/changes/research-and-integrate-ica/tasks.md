# Tasks: research-and-integrate-ica

## 1. Seed repository investigation

- [ ] 1.1 Clone/inspect `https://github.com/svendahlstrand/ica-api` (older client): record
      current README/docs claims, last commit date, and whether the maintainer or issues confirm
      breakage after ICA's April 2024 API changes.
- [ ] 1.2 Verify `PLAN.md`'s stated initial observation: "the older `ica-api` documentation
      became inaccurate after ICA API changes in April 2024" — confirm or refute with evidence
      (issues, commit history, changelog, or direct testing against the current ICA API if
      credentials are available).
- [ ] 1.3 Clone/inspect `https://github.com/LazyTarget/ha-ica-todo` (newer client): record
      current README/docs claims, last commit date, and license.
- [ ] 1.4 Verify `PLAN.md`'s stated initial observation: "`ha-ica-todo` has much newer work,
      including 2026 commits" — confirm commit dates and what changed in 2026 specifically.
- [ ] 1.5 Verify `PLAN.md`'s stated initial observation: "it implements or investigates shopping
      lists, offers, article grouping, auth refresh and synchronization" — catalog which of these
      are actually implemented vs. merely investigated/discussed in issues or comments.

## 2. `ICA+Grocy.md` inspection

- [ ] 2.1 Locate the `ICA+Grocy.md` document (or closest equivalent) within `ha-ica-todo`.
- [ ] 2.2 Read it in full; summarize the inventory lifecycle it describes (states, transitions,
      triggers).
- [ ] 2.3 Compare that lifecycle against Spisordning's own candidate inventory model
      (`PLAN.md`'s "Pantry"/"Inventory Events" sections: PURCHASE/CONSUME/DISCARD/ADJUST/
      TRANSFER/MARK_EMPTY/OPEN) — note overlaps and divergences explicitly.
- [ ] 2.4 Extract specific, nameable ideas worth carrying into Spisordning's design (e.g. a
      particular event type, a particular reconciliation trick, a particular sync strategy).
- [ ] 2.5 For each extracted idea, explicitly note whether it depends on Home-Assistant-specific
      infrastructure (HA entities, HA services, HA storage) and, if so, how it would be
      re-expressed without that dependency. Do not adopt HA-specific design by default.

## 3. Current ICA API access

- [ ] 3.1 Determine the current ICA authentication flow (login mechanism, credential storage,
      token/session lifetime, refresh mechanism, MFA if present) as implemented by `ha-ica-todo`.
- [ ] 3.2 Determine whether the current flow is officially supported, reverse-engineered, or a
      mix — and the practical risk of breakage (has ICA changed its API before? how did
      `ica-api` and `ha-ica-todo` each respond?).
- [ ] 3.3 Catalog what ICA data/actions are currently reachable per `ha-ica-todo`: shopping
      lists, offers/campaigns, article grouping, purchase/order history if any, store selection,
      product search/lookup.
- [ ] 3.4 Produce an ICA capability map in the same shape as `docs/research/willys-capabilities.md`
      (capability | supported? | where) so a future ICA adapter proposal can be evaluated against
      real, verified capabilities rather than assumptions.

## 4. Findings and recommendation

- [ ] 4.1 Write `docs/research/ica-current-api.md` capturing all findings from sections 1-3,
      including explicit "unverified — requires live testing" markers for anything that could not
      be confirmed from source/docs alone.
- [ ] 4.2 Recommend whether ICA integration is currently viable (stable enough API access,
      acceptable auth risk) or should be deferred, with reasoning.
- [ ] 4.3 If viable, sketch (do not implement) how a future ICA adapter would map onto the
      existing `retailer-adapter` capability's shape (search/resolve/pin/review/wishlist/cart,
      no checkout) so multiple retailers share one structural pattern.
- [ ] 4.4 Explicitly flag this research as a prerequisite for any future `integrate-ica`
      implementation change — not itself that change.

## 5. Verification

- [ ] 5.1 `docs/research/ica-current-api.md` exists, cites sources for every claim, and clearly
      distinguishes verified findings from open questions.
- [ ] 5.2 No ICA adapter code is introduced by this change.
