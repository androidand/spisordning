# Tasks: research-and-integrate-ica

## 1. Seed repository investigation

- [x] 1.1 Clone/inspect `https://github.com/svendahlstrand/ica-api` (older client): record
      current README/docs claims, last commit date, and whether the maintainer or issues confirm
      breakage after ICA's April 2024 API changes.
      → Docs-only (`README.md` + `api-referens.md`, no code); last commit `a39ab5e` 2024-04-17;
      README warning (17 Apr 2024) + open issue #26 confirm breakage. A third repo,
      `mellamomax/ica_shopping` (MIT licensed), was found via the same issue thread and recorded
      as an unexplored, more-permissive option. Doc §1.1.
- [x] 1.2 Verify `PLAN.md`'s stated initial observation: "the older `ica-api` documentation
      became inaccurate after ICA API changes in April 2024" — confirm or refute with evidence
      (issues, commit history, changelog, or direct testing against the current ICA API if
      credentials are available).
      → **Confirmed**: README warning, issue #26 ("closed access"), commit history ends 2024, and
      the old `handla.api.ica.se`/HTTP-Basic surface ≠ the current `apimgw-pub.ica.se`/OAuth2
      surface. The issue thread's own timeline (gateway rebuild 2024-04-18, ICA's own team
      denying a public API 2024-09-28, reverse-engineering resuming 2025-05, `ha-ica-todo`
      confirmed as successor 2025-06-25) is preserved in Doc §1.1 for context. No live credential
      testing performed (none available/in scope). Doc §1.1.
- [x] 1.3 Clone/inspect `https://github.com/LazyTarget/ha-ica-todo` (newer client): record
      current README/docs claims, last commit date, and license.
      → HA component (`custom_components/ica/`, manifest v0.8.4, `cloud_polling`); last commit
      `69100f3` 2026-04-14; **no LICENSE file**. Doc §1.2.
- [x] 1.4 Verify `PLAN.md`'s stated initial observation: "`ha-ica-todo` has much newer work,
      including 2026 commits" — confirm commit dates and what changed in 2026 specifically.
      → **Confirmed**: 25 commits in 2025 + 25 in 2026, last 2026-04-14. 2026 work = shopping-list
      MERGE-sync (`createdRows`/`changedRows`/`deletedRows`), conflict modes (APPEND/MERGE/IGNORE),
      unit conversion/normalization, product-name normalization + casefolding, `upsert_shopping_list`.
      Doc §1.2.
- [x] 1.5 Verify `PLAN.md`'s stated initial observation: "it implements or investigates shopping
      lists, offers, article grouping, auth refresh and synchronization" — catalog which of these
      are actually implemented vs. merely investigated/discussed in issues or comments.
      → **Implemented**: shopping lists (read + write MERGE-sync), offers, article grouping
      (`get_articles`), auth refresh (30-day token + refresh + re-login fallback), synchronization
      (delta-based list sync). **Investigated/Todo in-repo**: ICA barcode lookup handler, Kivra
      receipt → scanner, ICA recipe import. Doc §1.2, §3.2, §3.3.

## 2. `ICA+Grocy.md` inspection

- [x] 2.1 Locate the `ICA+Grocy.md` document (or closest equivalent) within `ha-ica-todo`.
      → Found at `ha-ica-todo/ICA+Grocy.md`. Doc §2.
- [x] 2.2 Read it in full; summarize the inventory lifecycle it describes (states, transitions,
      triggers).
      → 5-stage scenario (0 review → 1 meal prep → 2a in-store scan → 2b after purchase → 3
      organize → 4 cook) + Barcode Buddy + ICA offers extras; named events. Doc §2.
- [x] 2.3 Compare that lifecycle against Spisordning's own candidate inventory model
      (`PLAN.md`'s "Pantry"/"Inventory Events" sections: PURCHASE/CONSUME/DISCARD/ADJUST/
      TRANSFER/MARK_EMPTY/OPEN) — note overlaps and divergences explicitly.
      → Compared against Spisordning's **actual, shipped** six-value event enum (not just the
      candidate list): `TRANSFER`+`ADJUST` split is an improvement over the document's single
      `OrganizingProducts` event; `OPEN` has no equivalent in `ICA+Grocy.md` at all — a gap in the
      reference document, not in Spisordning. Doc §2 table.
- [x] 2.4 Extract specific, nameable ideas worth carrying into Spisordning's design (e.g. a
      particular event type, a particular reconciliation trick, a particular sync strategy).
      → Six extracted: two-way MERGE list sync; receipt→purchase derived fields (best-before =
      receipt date + default due date; amount = qty × count); due-score meal prep; offer-driven
      shopping; barcode-as-intake-key; recipe import from ICA's `recipeservice` (kept explicitly
      distinct from the already-built JSON-LD `internal/recipeimport` — different data source,
      same target domain). Doc §2 "Useful ideas".
- [x] 2.5 For each extracted idea, explicitly note whether it depends on Home-Assistant-specific
      infrastructure (HA entities, HA services, HA storage) and, if so, how it would be
      re-expressed without that dependency. Do not adopt HA-specific design by default.
      → HA-specific parts (event_bus, coordinator, blueprints, Grocy backend, Barcode Buddy app)
      are explicitly rejected; each kept idea is re-expressed backend-free. Doc §2 "Rejected".

## 3. Current ICA API access

- [x] 3.1 Determine the current ICA authentication flow (login mechanism, credential storage,
      token/session lifetime, refresh mechanism, MFA if present) as implemented by `ha-ica-todo`.
      → DCR + OAuth2 auth-code + PKCE (S256) + HTML-form login (personal ID + PIN) against
      `ims.icagruppen.se`; access token declared 30 days in code, but an independent 2025 issue
      comment reports bearer-token expiry "on the order of minutes" — flagged as an open,
      unresolved discrepancy rather than silently picked one way (Doc §3.1). MFA not observed
      (unverified live). BankID-gated ICA Banken accounts flagged as a separate, unverified case.
      Doc §3.1.
- [x] 3.2 Determine whether the current flow is officially supported, reverse-engineered, or a
      mix — and the practical risk of breakage (has ICA changed its API before? how did
      `ica-api` and `ha-ica-todo` each respond?).
      → **Reverse-engineered mobile API** (`sverige/digx/mobile/...`), explicitly disclaimed by
      ICA's own technical team per a direct issue-thread report ("they do not have an API"). ICA
      changed it once already (April 2024, broke `ica-api`); `ha-ica-todo` was rewritten to track
      it and is maintained through 2026. No contract/versioning. Doc §3.1, §4.1.
- [x] 3.3 Catalog what ICA data/actions are currently reachable per `ha-ica-todo`: shopping
      lists, offers/campaigns, article grouping, purchase/order history if any, store selection,
      product search/lookup.
      → Shopping lists (read+write), base items, articles/groups, offers, barcode lookup, stores,
      bonus, recipes. **No** cart, checkout, order/purchase history, or free-text product search —
      purchase/order history absent, not merely unverified. Doc §3.2, §3.3.
- [x] 3.4 Produce an ICA capability map in the same shape as `docs/research/willys-capabilities.md`
      (capability | supported? | where) so a future ICA adapter proposal can be evaluated against
      real, verified capabilities rather than assumptions.
      → Capability map table (capability | supported? | where) in Doc §3.3, mirroring
      `willys-capabilities.md`; §3.4 lists everything still requiring live verification.

## 4. Findings and recommendation

- [x] 4.1 Write `docs/research/ica-current-api.md` capturing all findings from sections 1-3,
      including explicit "unverified — requires live testing" markers for anything that could not
      be confirmed from source/docs alone.
      → Written; §3.4 lists live-testing-only items as unverified, including the token-lifetime
      discrepancy and the BankID question.
- [x] 4.2 Recommend whether ICA integration is currently viable (stable enough API access,
      acceptable auth risk) or should be deferred, with reasoning.
      → **Viable only as an unofficial, best-effort, opt-in adapter** (fragile, unlicensed seed
      code, ToS unverified, PII-sensitive personal ID + PIN). Doc §4.1.
- [x] 4.3 If viable, sketch (do not implement) how a future ICA adapter would map onto the
      existing `retailer-adapter` capability's shape (search/resolve/pin/review/wishlist/cart,
      no checkout) so multiple retailers share one structural pattern.
      → Reuse the Willys adapter shape (not HA's); DCR/PKCE/HTML-form auth; carry over
      pinning/review/confidence; MERGE list sync; offers → price intelligence; no checkout.
      Doc §4.2.
- [x] 4.4 Explicitly flag this research as a prerequisite for any future `integrate-ica`
      implementation change — not itself that change.
      → Flagged in Doc "Bottom line" (prerequisite, not implementation; future change must cite,
      re-verify the live API, resolve ToS/licensing, and read §5 before picking a surface).

## 5. Verification

- [x] 5.1 `docs/research/ica-current-api.md` exists, cites sources for every claim, and clearly
      distinguishes verified findings from open questions.
      → Doc exists; every claim cites a source file/commit/issue; §3.4 separates verified from
      live-testing-only, including the token-lifetime discrepancy.
- [x] 5.2 No ICA adapter code is introduced by this change.
      → Only documentation files touched by this OpenSpec change; no Go/adapter code added here.
      (Existing "ICA" refs in Go are ICA.se *recipe* import, a separate concern — see Doc's scope
      note.) A sibling repo outside this change, `~/dev/willys/ica-client`, has independently
      begun implementing an adapter-shaped client — documented in Doc §5 as context, not
      introduced by this change.

## 6. Reconciliation (2026-08-18)

- [x] 6.1 Two independent research passes for this same change ended up on separate branches
      (`research-and-integrate-ica` here, and a parallel `feat/ica-current-api-research` branch)
      without either being aware of the other. Compared both versions of
      `docs/research/ica-current-api.md` line by line and merged them into one document: kept
      the more detailed/actionable auth-flow and capability-map writing from one pass, the
      issue-thread timeline and the `mellamomax/ica_shopping` third-repo finding from the other,
      and flagged the one substantive disagreement (token lifetime, §3.1) explicitly rather than
      silently resolving it. `proposal.md`, `tasks.md`, and `specs/ica-integration/spec.md` were
      also compared — the spec was already identical between branches; `proposal.md`'s addendum
      and this file were reconciled the same way.
