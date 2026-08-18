# ICA — current API access and capability map

`PLAN.md`'s "External Research — ICA" section asks for exactly this before designing any
ICA-side retailer interface — the same discipline `willys-capabilities.md` applied to Willys.
Both seed repositories were read directly (not inferred), plus one issue thread and one sibling
implementation:

- **`ica-api`** — `https://github.com/svendahlstrand/ica-api` — the *older* reference.
- **`ha-ica-todo`** — `https://github.com/LazyTarget/ha-ica-todo` — the *newer*, 2026-active client.

> **Provenance note.** This document merges two independent research passes that ended up on
> separate branches (`research-and-integrate-ica` and `feat/ica-current-api-research`) without
> either knowing about the other — a side effect of running multiple agents on this repo at
> once. Nothing here is fabricated to reconcile them; where the two passes genuinely disagreed
> (see the token-lifetime note in §3.1), both readings are kept and flagged rather than one being
> silently dropped.

> **Scope note.** This document is about the ICA **shopping/account** API (shopping lists,
> offers, stores, bonus, recipes-as-shopping-data) — a private, authenticated,
> reverse-engineered mobile API. It is *not* about ICA.se as a **recipe** source, which is
> already handled separately by the read-only JSON-LD import in `internal/recipeimport`
> (`docs/research/recipe-web-import.md`). The two are distinct: one is a public recipe page, the
> other is a private customer-account API. §5 below covers a third, more literal overlap: a
> sibling client now implementing pieces of *both*.

## 1. The two seed repos

### 1.1 `ica-api` (older) — confirmed stale / broken

`PLAN.md`'s initial observation is **confirmed**: the documentation became inaccurate after
ICA's April 2024 API changes.

- The repo is **docs-only** — `README.md` + `api-referens.md`, no client code. Created
  2013-04-07, no declared license (`licenseInfo: null`).
- Last commit `a39ab5e` on **2024-04-17**, "Update README to inform about inaccurate
  documentation." Commit activity ends at 2024; the repo has been dormant ~2 years as of this
  research (2026-08-17).
- `README.md` carries an explicit maintainer warning (Swedish):
  > "Uppdatering 17 april 2024: Tyvärr har ICA gjort ändringar i sitt API så dokumentationen här
  > är inte längre korrekt" — "Update 17 April 2024: Unfortunately ICA has made changes to its
  > API so the documentation here is no longer correct."
- The documented API in `api-referens.md` is the **old** surface: base `handla.api.ica.se`, HTTP
  Basic auth → `AuthenticationTicket`, flat endpoints (login, card accounts, bonus, stores,
  offers, offline shopping lists, article groups, recipes, UPC lookup). None of this matches the
  current `apimgw-pub.ica.se` / `ims.icagruppen.se` surface used by `ha-ica-todo`.
- **Issue #26** ("API documentation is not accurate", opened 2024-04-06 by `@classek`: "seems
  like they have closed access. Is there a workaround??", labeled `bug`, still **open**) is the
  single most valuable source in this research — an 11-comment thread spanning April
  2024–June 2025 that documents the entire transition in near-real-time:
  - 2024-04-18: ICA rebuilt the API behind a new gateway, `https://apimgw-pub.ica.se`.
  - 2024-09-28: a commenter who contacted ICA's technical team directly was told **ICA claims
    they do not have a public API**, and wouldn't if they did — i.e. whatever surface exists now
    is unofficial/undocumented from ICA's own stated position, not merely "changed."
  - 2025-05-25 to 2025-05-27: reverse-engineering resumes — a new shopping-list endpoint shape
    is found (`apimgw-pub.ica.se/sverige/digx/shopping-list/v1/api/list/{listId}`), and a bearer
    token is reported to expire on the order of minutes (see the §3.1 discrepancy note — this
    conflicts with what `ha-ica-todo`'s own source says). One commenter also flags that **ICA
    Banken account holders must authenticate via BankID** (Swedish e-ID), materially raising the
    complexity for that subset of users.
  - **2025-06-25: `LazyTarget` (author of the second seed repo) posts directly in this thread**,
    stating they "successfully reverse engineered their API and authentication flow" and linking
    `ha-ica-todo` — establishing that repo as the direct successor to this one.
  - **2025-06-09: a third author, `mellamomax`, posts** that they built their own integration
    (`mellamomax/ica_shopping`) but call it "quite useless unless there's a way to retrieve the
    access token automatically." Notably **MIT licensed** (confirmed via `gh repo view`) — the
    one candidate among all three known repos with an explicit permissive license. Last pushed
    2025-07-29 (a year stale as of this research), not cloned or read in depth in either research
    pass — recorded here as a pointer, since its license alone makes it worth a look before
    assuming `ha-ica-todo` (unlicensed) is the only option for anything beyond reading for ideas.

**Conclusion:** `ica-api` is a historical artifact. It documents the *pre*-April-2024 API and is
not usable as a reference for current access — its only value is as evidence that ICA has
changed this API before (a breakage-risk data point).

### 1.2 `ha-ica-todo` (newer) — active, but Home-Assistant-specific

`PLAN.md`'s initial observation is **confirmed**: this repo has much newer work, including 2026
commits.

- A Home Assistant custom component (`custom_components/ica/`); `manifest.json` domain `ica`,
  version `v0.8.4`, `iot_class: cloud_polling`, codeowner `@lazytarget`. Created 2025-02-05,
  itself a fork of `dennisgranasen/ha-ica-todo`, "very much rewritten since" per its own README.
- Commit history: **25 commits in 2025** and **25 commits in 2026**; last commit `69100f3` on
  **2026-04-14** ("Minor stuff") — 4 months stale relative to this research's date, but genuinely
  active within the last several months.
- **No `LICENSE` file** anywhere in the repo tree (`licenseInfo: null`). This matters directly
  for "inherit or borrow": reading it for ideas/API shape (what this document does) is fine, but
  copying code verbatim would need the maintainer's permission first.
- The 2026 work is concentrated on **shopping-list synchronization**: MERGE-sync
  (`createdRows`/`changedRows`/`deletedRows`), conflict modes (`APPEND`/`MERGE`/`IGNORE`), unit
  conversion/normalization, product-name normalization + casefolding, and the
  `upsert_shopping_list` service. The 2025 work added the product registry, OpenFoodFacts
  enrichment, caching, and optional shopping-list tracking.
- README confirms, matching `PLAN.md`'s claim almost line for line: shopping list picker,
  automatic article-group categorization, offer-tracking automation blueprint, token refresh on
  expiry and on 401, re-login on refresh failure. Explicitly labeled "Currently under
  development and might be unstable!"
- Tests are minimal: only two pure-utility test files (`test_utils_merge.py`,
  `test_utils_product_names.py`). The API client itself is **not** unit-tested (it requires live
  credentials), so "the API works today" is supported by code + active maintenance, not by a
  passing test suite.

**Conclusion:** `ha-ica-todo` is the authoritative reference for *current* ICA access. But it is
an HA component — its auth state, coordinator, config-flow, and service plumbing are
HA-specific and must not be inherited (see §2, §4.2).

## 2. `ICA+Grocy.md` — inventory lifecycle

Located at `ha-ica-todo/ICA+Grocy.md`, read in full. It describes a **Grocy-backed** household
inventory lifecycle that a Home Assistant user drives from the ICA shopping list, firing named
events at key stages — a scenario walkthrough, not an API spec:

| Stage | What happens | Named event | Spisordning analogue |
|---|---|---|---|
| 0. Review inventory | Check what's on hand and what's spoiled; toss + re-list. Grocy tracks via due dates + minimum stock. | — | `DISCARD`; pantry min-stock |
| 1. Meal prep | Plan the week's recipes; see in-stock items sorted by "due score"; add missing items to the Grocy list. | `GrocyShoppingListUpdated` — sync the list into ICA | shopping-list → `retailer-adapter` projection |
| 2a. In-store scan | Scan items via the Grocy list, the ICA app, the HA to-do list, or (simplest) in-store scanners bound to the ICA list. | — | (checkout stays a human action; no analogue needed) |
| 2b. After purchase | Receipt lands in **Kivra**; a folder watcher processes the PDF into a Grocy "Purchase": look up product by barcode/name, parse the store, amount = line qty × count, best-before = receipt date + product default due date. | `NewReciept` | `PURCHASE` (receipt → stock); see `receipt-import-sources.md` (Kivra) |
| 3. Once home | Sort items into locations (pantry / fridge / freezer); correct due dates + storage location. | `OrganizingProducts` | `TRANSFER` + `ADJUST` |
| 4. Cooking | Look up the recipe, scale servings; consume the recipe's products. Circle complete. | — | `CONSUME` |

Two "extras" round it out: **Barcode Buddy** (barcode-driven Grocy updates with a
product↔barcode↔quantity mapping and a "federation server" that could crowd-source barcodes) and
**ICA offers** (`ICA_NEW_OFFERS` — auto-add new favorite-store offers to the ICA list; Grocy only
learns those products once they're scanned in 2a/2b).

**Comparison against Spisordning's actual (shipped) model.** Spisordning's real
`inventory_event.kind` (from `establish-household-and-catalog`'s pantry schema,
`migrations/0005_pantry_inventory.sql`) is a six-value enum — `PURCHASE`, `CONSUME`, `DISCARD`,
`ADJUST`, `TRANSFER`, `OPEN` — each with a defined field matrix and a `corrects_event_id`
self-reference for undo (the ledger-plus-projection design in that migration's own decisions).
Against the table above: `TRANSFER`+`ADJUST` is a genuine improvement over the document's single
`OrganizingProducts` event (Spisordning already separates "moved location" from "quantity/date
correction"); `OPEN` has **no equivalent at all** in `ICA+Grocy.md` — a real gap in the reference
document, not something to import, not something Spisordning is missing.

**Useful ideas (keep, HA-agnostic):**

1. **Treat the retailer shopping list as a durable, two-way-synced object** with explicit
   create/change/delete deltas (MERGE), not a fire-and-forget push. Maps cleanly onto
   Spisordning's `retailer-adapter` wishlist projection and the `implement-shopping-and-commerce`
   shopping-list model.
2. **Receipt-to-PURCHASE-event pipeline**: Kivra PDF export → parse → match product by
   barcode/name → `PURCHASE` event with quantity from the receipt row and best-before = receipt
   date + product default shelf life. Not HA-specific — genuinely portable, and Kivra is already
   a candidate source in `docs/research/receipt-import-sources.md`.
3. **Due-score / minimum-stock for meal prep** — ranking in-stock items by proximity to expiry to
   drive "what should we cook" — a useful domain idea independent of any backend.
4. **Offer-matching against favorited products** (`ICA_NEW_OFFERS` → check favorites → auto-add
   to list). Portable in concept; Spisordning's own `internal/scoring` `campaignBonus` already
   does something adjacent for meal *scoring* — this idea is shopping-list automation instead, a
   distinct feature, not a redesign of what exists.
5. **Barcode as the primary intake key** (Barcode Buddy's *concept*, not its infrastructure): a
   scan resolving to a known product+quantity/unit is portable and close to
   `establish-household-and-catalog`'s `product_identifier` (barcode optional, never identity).
   The crowd-sourced "federation server" is HA-community-specific and not recommended for
   adoption as-is.
6. **Recipe import from ICA → local cookbook** — an unbuilt idea in `ICA+Grocy.md`, and newly
   *technically viable* per this document's own capability map (§3.3): the authenticated
   `recipeservice` (favorites/by-id/random) is real. This is a **different** pathway than the
   already-built `internal/recipeimport` JSON-LD scraper (see the scope note above) — one reads
   public recipe pages, the other would read a logged-in user's ICA account data — worth keeping
   distinct if both are ever pursued, not merged into one feature.

**Rejected (HA-specific, do not inherit):** firing HA `event_bus` events and relying on HA
automations/blueprints to react; the HA coordinator/config-entry/cache-invalidation machinery;
Grocy as the inventory backend (Spisordning has its own relational pantry model); Barcode Buddy
as a separate app (fold barcode intake into Spisordning's own flow instead).

## 3. Current ICA API access (as implemented by `ha-ica-todo`)

### 3.1 Authentication (verified from `authenticator.py`)

The current flow is a **reverse-engineered mobile-app OAuth2 flow** against ICA's identity
provider (`ims.icagruppen.se`, a Curity Identity Server deployment — a commercial OIDC/OAuth2
platform, not a bespoke ICA auth system). It is *not* BankID and *not* a public API.

Sequence:

1. **Dynamic Client Registration (DCR).**
   - `POST oauth/v2/token` with a **hardcoded** bootstrap client
     (`client_id=ica-app-dcr-registration`, `client_secret=<public in the repo>`,
     `grant_type=client_credentials`, `scope=dcr`) → DCR access token.
   - `POST register` with `software_id=dcr-ica-app-template` + that token → a fresh per-session
     `OAuthClient` (client_id + client_secret).
2. **Authorization Code + PKCE.**
   - Generate `code_verifier`/`code_challenge` (S256).
   - `GET oauth/v2/authorize` with `response_type=code`, `code_challenge_method=S256`,
     `redirect_uri=icacurity://app`, `prompt=login`, and
     `acr=urn:se:curity:authentication:html-form:IcaCustomers` → 302 with `state`.
3. **HTML-form customer login.**
   - `POST authn/authenticate/IcaCustomers` with `userName` = **personal ID** and
     `password` = **PIN code** → parse hidden `state` + `token` from the returned HTML. The
     credential is the ICA customer's **personal ID + PIN** (the "ICA-kund" login), not a
     national eID.
4. **Token exchange.**
   - `POST oauth/v2/authorize` (with `forceAuthN=true`, `token`, `state`) → 302 with `code`.
   - `POST oauth/v2/token` with `code` + `code_verifier` → `access_token`, `refresh_token`,
     `id_token` (JWT), `expires_in` (default **2592000 s = 30 days**, per `authenticator.py`).
5. **Refresh + fallback.**
   - `POST oauth/v2/token` with `grant_type=refresh_token` (Basic client auth) on expiry.
   - If refresh returns 400, retry ≤2 then fall back to a **full re-login** (re-register app).

> **Open question — token lifetime discrepancy.** `authenticator.py`'s token response declares
> `expires_in: 2592000` (30 days), but a 2025-05 comment on `ica-api`#26 reports a bearer token
> expiring "on the order of minutes" during independent reverse-engineering (§1.1). These may
> describe different tokens (e.g. a short-lived intermediate/web token vs. the mobile app's
> long-lived OAuth `access_token`) or reflect a since-changed ICA-side configuration — unresolved
> without live testing (§3.4). Don't design a refresh strategy around either number without
> verifying live.

**BankID note**: the flow above is personal-ID + PIN. Issue #26 flags that ICA Banken account
holders specifically are forced through BankID instead — **unverified** whether `ha-ica-todo`
handles this case at all (not found in the authenticator code read); likely a gap or an explicit
non-goal. A household without an ICA Banken account is unaffected.

**Stability / reverse-engineered-ness:** This is the ICA **mobile app's** private API (note the
`sverige/digx/mobile/...` path prefix on every data endpoint). It has changed at least once
(April 2024, which broke `ica-api`). `ha-ica-todo` tracks it and has been updated through 2026,
but there is **no contract, no versioning guarantee, and no ToS-cleared public surface** — the
same breakage risk that stranded `ica-api` applies, though not obviously higher than Willys's
own reverse-engineered flow that `willys-client` already runs against successfully.

### 3.2 API surface (verified from `const.py` + `icaapi_async.py`)

Base: data on `https://apimgw-pub.ica.se`, auth on `https://ims.icagruppen.se`. All data calls
are under `sverige/digx/mobile/...`:

| Service | Endpoints | Client method(s) |
|---|---|---|
| Shopping lists | `shoppinglistservice/v1/shoppinglists`, `/{id}`, `/{id}/sync`, `/common` | `get_shopping_lists`, `get_shopping_list`, `create_shopping_list`, `sync_shopping_list`, `delete_shopping_list` |
| Base items (favorites) | `shoppinglistservice/v1/baseitems` | `get_baseitems`, `sync_baseitems` |
| Articles / groups | `shoppinglistservice/v1/articles`, `/articles/articlegroups?lastsyncdate=` | `get_articles` |
| Offers | `offerservice/v1/offers/search`, `/offersdiscounts/{id}`, `offers?Stores=` | `get_offers`, `search_offers` |
| Product by barcode | `productservice/v1/product/{barcode}` | `lookup_barcode` (+ OpenFoodFacts enrichment) |
| Stores | `storeservice/v1/favorites`, `/stores/{id}`, `stores/search?Filters&Phrase=` | `get_favorite_stores`, `get_store` |
| Bonus / card | `bonusservice/v1/bonus/current`, `cardservice/v1/card/cardaccounts?api-version=2` | `get_current_bonus` |
| Recipes | `recipeservice/v1/favorites`, `/recipes/{id}?api-version=2.0`, `/recipes/random?numberofrecipes=` | `get_recipe`, `get_random_recipes` |

**Write operations** (the interesting part for an adapter): `create_shopping_list`,
`sync_shopping_list` (MERGE — `createdRows`/`changedRows`/`deletedRows`), `delete_shopping_list`,
`sync_baseitems`. The sync protocol is delta-based, well-suited to two-way list projection.

### 3.3 Capability map

Shape mirrors `willys-capabilities.md` (capability | supported? | where).

| Capability | Supported? | Where |
|---|---|---|
| Authentication | Yes | `authenticator.py` — DCR + OAuth2 auth-code + PKCE + HTML-form (personal ID + PIN) against `ims.icagruppen.se`. |
| Token/session refresh | Yes | 30-day access tokens (see §3.1 discrepancy note), refresh-token grant, full re-login fallback on 400. |
| BankID-gated accounts | **Unverified** | Not found in `authenticator.py`; likely unhandled — see BankID note in §3.1. |
| Store selection / favorites | Yes | `storeservice/v1/favorites`, `/stores/{id}`, `stores/search`; `get_favorite_stores()`, `get_store()`. |
| Product search (free text) | **No** | No free-text product-search endpoint; offers are queried per store, not by query. |
| GTIN/EAN/barcode lookup | Yes | `productservice/v1/product/{barcode}`; `lookup_barcode()` → `ProductLookup`, enriched via OpenFoodFacts. |
| Product detail by code | Yes | Same barcode endpoint; `IcaProduct` model. |
| Prices | Partial | On offer/article objects (`IcaArticleOffer`, `IcaOfferDetails`); no standalone price-history service. |
| Campaigns / promotions / offers | Yes | `offerservice/v1/offers/search`, `/offersdiscounts/{id}`; `get_offers(store_ids)`, `search_offers()`. |
| Shopping list (read) | Yes | `shoppinglistservice/v1/shoppinglists[/{id}]`; `get_shopping_lists()`, `get_shopping_list()`. |
| Shopping list (write / sync) | Yes, thoroughly | `shoppinglists/{id}/sync` with MERGE deltas; conflict modes `APPEND`/`MERGE`/`IGNORE`. |
| Favorite products (base items) | Yes | `shoppinglistservice/v1/baseitems`; `get_baseitems()`, `sync_baseitems()`. |
| Cart | **No** | No cart/basket object in either seed repo. |
| Checkout | **No, deliberately** | No checkout/payment code anywhere in either seed repo. |
| Orders / purchase history | **No** | No order-history endpoint in either seed repo. |
| Bonus / voucher | Yes | `bonusservice/v1/bonus/current`; `get_current_bonus()` → balance, vouchers, discount summary. |
| Recipes | Yes | `recipeservice/v1/favorites`, `/recipes/{id}`, `/recipes/random`; not anticipated by `PLAN.md`'s ICA section but real — see idea 6 in §2. |
| Customer profile | Partial | `get_authenticated_user()` from the decoded `id_token` JWT; no full profile management. |
| Push/webhook updates | No | `manifest.json` declares `"iot_class": "cloud_polling"` — polling only. |
| Inventory lifecycle | **Idea only** (not an API) | `ICA+Grocy.md` describes a Grocy-backed lifecycle (§2); not an ICA API capability. |

### 3.4 Unverified — requires live testing

- Live operation of the DCR + PKCE + HTML-form auth flow against the current ICA API.
- Live operation of the data endpoints (shopping lists, offers, barcode, bonus, recipes).
- The token-lifetime discrepancy in §3.1 (30 days per code vs. minutes per a 2025 issue comment).
- Whether MFA / step-up is ever required beyond personal ID + PIN.
- Whether ICA Banken account holders (BankID-gated, §3.1) can use this flow at all.
- Rate limits and any ToS compliance of automated access.

## 4. Viability assessment

### 4.1 Is ICA integration currently viable?

**Technically reachable, but high-maintenance and legally/ToS-uncertain.**

- **Reachable:** `ha-ica-todo` demonstrates a client for the current API — read + write
  shopping-list MERGE-sync, offers, barcode lookup, bonus, and recipes — actively maintained
  through 2026-04. The capability surface fits Spisordning's `retailer-adapter` (list
  projection, offers → price intelligence, barcode → catalog) well, and covers everything
  `PLAN.md`'s ICA section anticipated plus one thing it didn't (recipes).
- **Fragile:** it is a reverse-engineered mobile API with a documented breakage history (April
  2024 stranded `ica-api`). Any ICA change can break it with no notice; the only mitigation is a
  maintainer actively tracking it, as `@lazytarget` does for HA.
- **Legally/ToS-uncertain:** none of the three known repos (`ica-api`, `ha-ica-todo`,
  `mellamomax/ica_shopping`) except the last is licensed, so a future client should be written
  independently, informed by (not copied from) the discovered endpoint shapes and auth flow —
  `mellamomax/ica_shopping`'s MIT license (§1.1) is worth a closer look first if actual code
  reuse, not just ideas, is desired. Whether automated access complies with ICA's ToS is
  **unverified** and should be resolved before any production adapter — ICA's own technical team
  told a user (§1.1) they don't consider this a public API at all.
- **Credential sensitivity:** the login is **personal ID + PIN** (customer account), not BankID.
  Storing/handling these must follow the same care as any PII (encrypted at rest, never logged).

**Verdict:** viable as a **reference-informed, best-effort, clearly-labeled "unofficial"
adapter** — not as a stable, contract-backed integration. Gate behind explicit user opt-in and
treat as breakable.

### 4.2 Future adapter sketch (NOT implemented by this research — see §5 for what already is)

If a future `integrate-ica` change proceeds, the adapter should map onto the existing
`retailer-adapter` invariants (`openspec/specs/retailer-adapter/spec.md`):

- **Reuse the Willys adapter shape**, not HA's: a standalone HTTP adapter (like
  `willys-adapter`) exposing `/search`, `/products/:code`, `/offers`, `/shopping-lists`,
  `/resolve`, `/pins`, `/review/queue` — with ICA-specific routes for barcode lookup and bonus.
  Do **not** port `ha-ica-todo`'s coordinator/config-flow/event plumbing.
- **Auth:** implement the DCR + PKCE + HTML-form flow; store the personal ID + PIN encrypted;
  cache the access token + refresh; full re-login fallback on refresh failure.
- **Pinning / review queue / confidence:** carry over Willys' `Resolution` shape (`matchType`,
  `confidence`, `needsReview`, `quantityUncertain`) — ICA's barcode lookup and offer data feed
  the same resolution pipeline.
- **Shopping list:** project Spisordning's list onto ICA via the **MERGE sync**
  (`createdRows`/`changedRows`/`deletedRows`) with an explicit conflict mode — the strongest
  ICA-specific capability, mapping directly onto `implement-shopping-and-commerce`'s list model.
- **Offers → price intelligence:** `get_offers`/`search_offers` feed the price-intelligence
  store (retailer/store/SKU/offer readings).
- **No checkout, ever:** the API has no checkout and Spisordning's invariant forbids it — keep
  payment a human action regardless of any future ICA capability.

## 5. A sibling implementation has already started: `~/dev/willys/ica-client`

Unlike the "no adapter code, no scaffolding" state this research originally described, a
standalone TypeScript package now exists at `~/dev/willys/ica-client`, structurally mirroring
`~/dev/willys/willys-client`'s architecture (layered `IcaClient` composing `HttpClient` +
`GraphQLClient` + per-concern `*Service` classes). As of 2026-08-18 it is **actively being
edited** — treat everything below as a snapshot, not a stable interface.

**It independently confirms §3.1.** `src/auth/oauth2.ts` hardcodes the exact same DCR bootstrap
client (`client_id=ica-app-dcr-registration`), the same `acr` value
(`urn:se:curity:authentication:html-form:IcaCustomers`), and the same `redirect_uri`
(`icacurity://app`) against `ims.icagruppen.se` that this document derived from
`ha-ica-todo/authenticator.py`. This is strong independent evidence for §3.1 — two unrelated
efforts (this document's research and that scaffold's implementation) arrived at the identical
flow, not two conflicting theories.

**It targets both ICA surfaces at once**, per its own code comments: the **mobile OAuth2 API**
(Bearer-token, `ims.icagruppen.se` / `apimgw-pub.ica.se`) for shopping lists, recipes, and bonus
— i.e. exactly what §3 documents — *and* ICA's **web storefront**
(`https://handlaprivatkund.ica.se`, cookie/session auth, GraphQL at `/stores/{storeId}/graphql`
plus REST under `/stores/{storeId}/api/...`) for cart and product search, which this document's
two seed repos never covered (neither has a cart or free-text product search — see §3.3). A
future `integrate-ica` change should treat these as two auth models feeding one client, not
assume "ICA API access" is a single question.

**Current state (2026-08-18, will drift):**

- Not yet building cleanly — `tsc --noEmit` reports 8 errors, mostly `string | undefined`
  narrowing gaps in `oauth2.ts`/`playwright-auth.ts` plus a `node-fetch`/`undici` `Headers` type
  mismatch in `lib/http.ts`.
- Mid-refactor: duplicate singular/plural service directories exist side by side
  (`offer/`+`offers/`, `product/`+`products/`, `recipe/`+`recipes/`, `store/`+`stores/`).
  `src/auth/service.ts` (the old cookie-only stub) has already been deleted in favor of
  `oauth2.ts` + a Playwright-driven `playwright-auth.ts` (browser automation, presumably because
  the web-storefront login isn't a plain form POST) — real progress on the item §3.4 and §4.1
  flag as the top gate (live auth), but not yet proven live.
- No passing automated tests: `tests/` is empty; a new ad-hoc `test.ts` script exists
  ("Comprehensive test script... uses credentials from .env") but hasn't been confirmed green in
  this research.
- ⚠️ **Credential-handling flag**: `.env.example` in that scaffold currently holds what looks
  like a real Swedish personal-ID and PIN rather than obviously-fake placeholders. Not committed
  as part of this reconciliation; worth the repo owner's direct attention before this goes near
  version control.

**Net effect on §4.1/§4.2:** this scaffold is, in effect, an early attempt at exactly the
adapter §4.2 sketches — DCR+PKCE auth outside Home Assistant, informed by (not copied from)
`ha-ica-todo`'s discovered shapes. The verdict in §4.1 stands unchanged: **viable to design
toward, not yet to build** — the scaffold has not yet cleared this document's own gates (§3.4:
live auth verification; the token-lifetime and BankID open questions).

## Bottom line

- `ica-api` is **confirmed stale** (broke April 2024, dormant since) — use only as breakage
  evidence; `mellamomax/ica_shopping` is a third, MIT-licensed but unexplored option worth a
  look before assuming no seed repo is reusable.
- `ha-ica-todo` is the **current reference**: a reverse-engineered mobile API (DCR + PKCE +
  HTML-form personal-ID/PIN auth) with read+write shopping-list MERGE-sync, offers, barcode
  lookup, bonus, and recipes — actively maintained through 2026-04.
- The strongest ICA-specific capabilities are **delta-based shopping-list sync** plus **offer
  data for price intelligence**; both fit the existing `retailer-adapter` shape.
- ICA integration is **viable only as an unofficial, best-effort, opt-in adapter**: fragile
  (breakage history), unlicensed reference code, ToS unverified, PII-sensitive credentials, and
  an unresolved token-lifetime discrepancy. No checkout, no HA plumbing inherited.
- **A real implementation attempt has already started** (§5), independently confirming this
  document's auth findings — but it hasn't yet cleared this document's own live-verification
  gate, so the recommendation is unchanged.
- **This is a prerequisite, not an implementation.** Any future `integrate-ica` change must cite
  this document, re-verify the live API (§3.4), resolve the ToS/licensing questions, and read §5
  before assuming which ICA surface (mobile API, web storefront, or both) it's building against.

Sources: `gh repo view`/`gh api` against `svendahlstrand/ica-api` and `LazyTarget/ha-ica-todo`
(repo metadata, commit history, README, `ICA+Grocy.md`, `authenticator.py`, `const.py`,
`icaapi_async.py`, `manifest.json`), `svendahlstrand/ica-api#26` (issue thread, 2024-04-06 to
2025-06-25), and direct inspection of `~/dev/willys/ica-client` source (§5).
