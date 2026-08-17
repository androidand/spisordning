# Tasks: research-and-integrate-ica

## 1. Seed repository investigation

- [x] 1.1 Clone/inspect `https://github.com/svendahlstrand/ica-api` (older client): record
      current README/docs claims, last commit date, and whether the maintainer or issues confirm
      breakage after ICA's April 2024 API changes. Done via `gh api`/`gh repo view` (no clone
      needed — read directly). Last push 2024-04-17; maintainer's own last commit is titled
      "Update README to inform about inaccurate documentation." See
      `docs/research/ica-current-api.md` §"Seed repo 1".
- [x] 1.2 Verify `PLAN.md`'s stated initial observation: "the older `ica-api` documentation
      became inaccurate after ICA API changes in April 2024" — confirm or refute with evidence
      (issues, commit history, changelog, or direct testing against the current ICA API if
      credentials are available). **Confirmed directly from the maintainer's own README banner**
      and issue #26 — not inferred. No live credential testing performed (none available/in
      scope this session); confirmed from primary-source commentary instead.
- [x] 1.3 Clone/inspect `https://github.com/LazyTarget/ha-ica-todo` (newer client): record
      current README/docs claims, last commit date, and license. Done. Last push 2026-04-14; no
      LICENSE file anywhere in the repo (confirmed via full tree listing) — recorded as a real
      constraint on code reuse in the findings doc.
- [x] 1.4 Verify `PLAN.md`'s stated initial observation: "`ha-ica-todo` has much newer work,
      including 2026 commits" — confirm commit dates and what changed in 2026 specifically.
      Confirmed: commits from March–April 2026 (PR #38 "Enhances shopping list upsert and
      sync", PR #37 label/release-drafting work).
- [x] 1.5 Verify `PLAN.md`'s stated initial observation: "it implements or investigates shopping
      lists, offers, article grouping, auth refresh and synchronization" — catalog which of these
      are actually implemented vs. merely investigated/discussed in issues or comments. All five
      confirmed **implemented** (not just discussed) by reading source directly
      (`authenticator.py`, `const.py`) — see the capability map in the findings doc.

## 2. `ICA+Grocy.md` inspection

- [x] 2.1 Locate the `ICA+Grocy.md` document (or closest equivalent) within `ha-ica-todo`. Found
      at the repo root.
- [x] 2.2 Read it in full; summarize the inventory lifecycle it describes (states, transitions,
      triggers). Done — see findings doc's "ICA+Grocy.md inspection" section (0–4 step
      walkthrough + Extras).
- [x] 2.3 Compare that lifecycle against Spisordning's own candidate inventory model
      (`PLAN.md`'s "Pantry"/"Inventory Events" sections: PURCHASE/CONSUME/DISCARD/ADJUST/
      TRANSFER/MARK_EMPTY/OPEN) — note overlaps and divergences explicitly. Done against
      Spisordning's **actual, shipped** event kinds (not just the candidate list) — comparison
      table in the findings doc.
- [x] 2.4 Extract specific, nameable ideas worth carrying into Spisordning's design (e.g. a
      particular event type, a particular reconciliation trick, a particular sync strategy).
      Four extracted: receipt-to-PURCHASE pipeline, offer-matching against favorites, Barcode
      Buddy's mapping concept (not its infrastructure), and ICA recipe import.
- [x] 2.5 For each extracted idea, explicitly note whether it depends on Home-Assistant-specific
      infrastructure (HA entities, HA services, HA storage) and, if so, how it would be
      re-expressed without that dependency. Do not adopt HA-specific design by default. Done
      per-idea in the findings doc — three of four are portable as-is; Barcode Buddy's
      federation-server infrastructure specifically flagged as not recommended for adoption.

## 3. Current ICA API access

- [x] 3.1 Determine the current ICA authentication flow (login mechanism, credential storage,
      token/session lifetime, refresh mechanism, MFA if present) as implemented by `ha-ica-todo`.
      Done by reading `authenticator.py` directly: OAuth2 Authorization Code + PKCE against a
      Curity-based identity server, HTML-form username/password login, `access_token`/
      `refresh_token` pair, standard refresh-token grant. BankID (for ICA Banken account
      holders) flagged as **unverified** — not found in the authenticator code read.
- [x] 3.2 Determine whether the current flow is officially supported, reverse-engineered, or a
      mix — and the practical risk of breakage (has ICA changed its API before? how did
      `ica-api` and `ha-ica-todo` each respond?). Reverse-engineered and explicitly disclaimed by
      ICA's own technical team (per a direct report in issue #26: "they do not have an API").
      Breakage history: yes, once, April 2024, which is exactly what broke `ica-api` and
      prompted `ha-ica-todo`'s reverse-engineering effort.
- [x] 3.3 Catalog what ICA data/actions are currently reachable per `ha-ica-todo`: shopping
      lists, offers/campaigns, article grouping, purchase/order history if any, store selection,
      product search/lookup. Done — full endpoint-level capability map in the findings doc,
      sourced from `const.py`. Purchase/order history: not found in the traced endpoint list —
      absent, not merely unverified.
- [x] 3.4 Produce an ICA capability map in the same shape as `docs/research/willys-capabilities.md`
      (capability | supported? | where) so a future ICA adapter proposal can be evaluated against
      real, verified capabilities rather than assumptions. Done — same three-column table shape.

## 4. Findings and recommendation

- [x] 4.1 Write `docs/research/ica-current-api.md` capturing all findings from sections 1-3,
      including explicit "unverified — requires live testing" markers for anything that could not
      be confirmed from source/docs alone. Done — two items explicitly marked unverified
      (BankID handling; live-account auth verification).
- [x] 4.2 Recommend whether ICA integration is currently viable (stable enough API access,
      acceptable auth risk) or should be deferred, with reasoning. Recommended: **viable to
      design toward, not yet to build** — gated on live auth verification and the BankID
      question, both named explicitly.
- [x] 4.3 If viable, sketch (do not implement) how a future ICA adapter would map onto the
      existing `retailer-adapter` capability's shape (search/resolve/pin/review/wishlist/cart,
      no checkout) so multiple retailers share one structural pattern. Done — standalone sibling
      repo wrapping an HTTP adapter, same shape as `willys-client`/`willys-adapter`, no
      automated checkout (also consistent with the fact that no checkout endpoint exists in
      ICA's traced surface at all).
- [x] 4.4 Explicitly flag this research as a prerequisite for any future `integrate-ica`
      implementation change — not itself that change. Done in the findings doc's closing
      paragraph.

## 5. Verification

- [x] 5.1 `docs/research/ica-current-api.md` exists, cites sources for every claim, and clearly
      distinguishes verified findings from open questions. Verified: every section cites its
      source (repo, file, issue comment with date); two explicit "Unverified" markers.
- [x] 5.2 No ICA adapter code is introduced by this change. Confirmed — this change touches only
      `docs/research/ica-current-api.md` and this `tasks.md`; no new packages, repos, or code.
