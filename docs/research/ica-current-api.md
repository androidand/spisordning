# ICA — current API access and capability map

`PLAN.md`'s "External Research — ICA" section asks for exactly this document before designing
any ICA-side retailer interface — the same discipline `willys-capabilities.md` already applied
to Willys. Sources: GitHub repo metadata, commit history, README/source content, and one
maintainer issue thread, all read directly via `gh api`/`gh repo view` — nothing here is
inferred or fabricated. Every claim below is tagged with where it came from; anything not
directly verifiable is marked **unverified**.

## Seed repo 1: `svendahlstrand/ica-api` — confirmed stale (task 1.1/1.2)

- Created 2013-04-07, last pushed **2024-04-17** — no activity in over two years as of this
  research (2026-08-17).
- No declared license (`licenseInfo: null` via `gh repo view`).
- **The maintainer's own last commit is titled "Update README to inform about inaccurate
  documentation"** (2024-04-17), adding this banner to the README:

  > **Uppdatering 17 april 2024:** Tyvärr har ICA gjort ändringar i sitt API så dokumentationen
  > här är inte längre korrekt. ("Unfortunately ICA has made changes to its API so the
  > documentation here is no longer correct.")

  This **directly confirms**, from the primary source, `PLAN.md`'s stated initial observation —
  not merely "possibly stale," the maintainer says so explicitly and points to their own issue
  #26.
- **Issue #26** ("API documentation is not accurate", opened 2024-04-06, still open) is the
  single most valuable source in this research — an 11-comment thread spanning April
  2024–June 2025 that documents the entire transition in near-real-time:
  - 2024-04-18: ICA rebuilt the API behind a new gateway, `https://apimgw-pub.ica.se`.
  - 2024-09-28: a commenter who contacted ICA's technical team directly was told **ICA claims
    they do not have a public API**, and wouldn't if they did — i.e. whatever surface exists now
    is unofficial/undocumented from ICA's own stated position, not merely "changed."
  - 2025-05-25 to 2025-05-27: reverse-engineering resumes — a new shopping-list endpoint shape
    is found (`apimgw-pub.ica.se/sverige/digx/shopping-list/v1/api/list/{listId}`), and a bearer
    token is confirmed to expire on the order of minutes. One commenter also flags that **ICA
    Banken account holders must authenticate via BankID** (Swedish e-ID), materially raising the
    complexity for that subset of users beyond username/password.
  - **2025-06-25: `LazyTarget` (author of the second seed repo) posts directly in this thread**,
    stating they "successfully reverse engineered their API and authentication flow" and linking
    `ha-ica-todo` — establishing that repo as the direct successor to this one, not an unrelated
    parallel effort.
- **Verdict on `ica-api`**: correctly assessed as stale by `PLAN.md`. Retains historical/context
  value only (it documents the *pre-2024* API shape, useful for understanding what changed) —
  not a usable foundation for a new client today.

## Seed repo 2: `LazyTarget/ha-ica-todo` — confirmed active, real reverse-engineering (task 1.3/1.4/1.5)

- Created 2025-02-05 (itself a fork of `dennisgranasen/ha-ica-todo`, "very much rewritten
  since," per its own README — a third lineage point worth knowing).
- Last pushed **2026-04-14** — real 2026 commits confirmed directly (`gh api .../commits`):
  PR #38 "Enhances shopping list upsert and sync", PR #37 label/release-drafting work, and
  several shopping-list-sync commits from March 2026. This is 4 months stale relative to this
  research's date (2026-08-17), not currently-being-pushed-today, but genuinely active within
  the last several months — confirms `PLAN.md`'s "including 2026 commits" claim.
- No declared license (no `LICENSE` file anywhere in the repo tree; `licenseInfo: null`).
  **This matters directly for "inherit or borrow"**: no explicit permissive license means
  default copyright applies — reading it for ideas/API shape (what this document does) is fine,
  but copying code verbatim would need the maintainer's permission first.
- README confirms, matching `PLAN.md`'s claim almost line for line: shopping list picker,
  automatic article-group categorization, offer-tracking automation blueprint, token refresh on
  expiry and on 401, re-login on refresh failure. Explicitly labeled "Currently under
  development and might be unstable!"
- **Auth flow, read directly from `custom_components/ica/authenticator.py`** (task 3.1): a real
  OAuth2 Authorization Code + PKCE flow against `https://ims.icagruppen.se`, which is a
  **Curity Identity Server** deployment (`acr: urn:se:curity:authentication:html-form:IcaCustomers`
  in the request — Curity is a commercial OIDC/OAuth2 identity platform, not a bespoke ICA
  auth system). Flow: dynamic client registration → PKCE code_challenge → HTML-form login
  (username/password) → authorization code → token exchange → `access_token` +
  `refresh_token` pair. Refresh uses a standard `grant_type=refresh_token` call with HTTP Basic
  client auth. **This is a properly reverse-engineered, standards-shaped flow**, not a hack —
  the complexity is in discovering the Curity-specific parameters (`acr`, `redirect_uri:
  icacurity://app` mimicking the native app's URL scheme), not in the flow's own structure.
- **Auth stability assessment (task 3.2)**: reverse-engineered, unofficial (ICA's own technical
  team told a user they don't consider this a public API — see issue #26 above), but
  *structurally* a normal OAuth2/PKCE flow rather than scraped HTML or an ad-hoc token scheme —
  more likely to keep working across minor ICA changes than the 2013-era `ica-api`'s Basic-auth
  scheme was, precisely because it follows a real protocol. Risk is real (ICA could rotate
  Curity client registration parameters or add MFA at any time, as they evidently did once
  already in April 2024) but not obviously higher than Willys's own reverse-engineered flow that
  `willys-client` already runs against successfully.
- **BankID note**: the OAuth flow above is username+password. Issue #26 flags that ICA Banken
  account holders specifically are forced through BankID instead — **unverified** whether
  `ha-ica-todo` handles this case at all (not found in the authenticator code read); likely a
  gap or an explicit non-goal. A household without an ICA Banken account (i.e. plain ICA
  loyalty membership) is unaffected.

## ICA API capability map (task 3.3/3.4)

Read directly from `custom_components/ica/const.py`'s endpoint constants — the fullest, most
current inventory of what's reachable through this reverse-engineered surface. Base URLs:
auth at `https://ims.icagruppen.se`, data API at `https://apimgw-pub.ica.se` (all data
endpoints under `sverige/digx/mobile/...`).

| Capability | Supported? | Where |
|---|---|---|
| Authentication (OAuth2 + PKCE, username/password) | Yes | `authenticator.py`, `ims.icagruppen.se/oauth/v2/*` |
| Token refresh | Yes | `authenticator.py get_refresh_token` |
| BankID-gated accounts | **Unverified** | Not found in `authenticator.py`; likely unhandled |
| Shopping lists — list/get/sync | Yes | `shoppinglistservice/v1/shoppinglists`, `.../{id}`, `.../{id}/sync` |
| Shopping lists — base items ("common" recurring items) | Yes | `shoppinglistservice/v1/baseitems` |
| Shopping lists — article groups (categorization) | Yes | `shoppinglistservice/v1/articles/articlegroups` |
| Shopping lists — common articles | Yes | `shoppinglistservice/v1/shoppinglists/common` |
| Offers/campaigns search | Yes | `offerservice/v1/offers/search` |
| Store-specific offers/discounts | Yes | `offerservice/v1/offersdiscounts/{storeId}` |
| Barcode → product lookup | Yes | `productservice/v1/product/{barcode}` |
| Loyalty card / account | Yes | `cardservice/v1/card/cardaccounts` |
| Bonus balance | Yes | `bonusservice/v1/bonus/current` |
| Favorite stores | Yes | `storeservice/v1/favorites` |
| Store detail / search | Yes | `storeservice/v1/stores/{id}`, `stores/search` |
| Recipes — favorites, by id, random | Yes | `recipeservice/v1/recipes/*` — **not anticipated by `PLAN.md`'s ICA section**, but real; relevant to `implement-recipe-discovery` later, not just retailer/shopping |
| Checkout / cart / payment | **Not present** | No endpoint found anywhere in the traced surface — consistent with `willys-client`'s own boundary (wishlist/list only, never automated checkout) |
| Push/webhook updates | No | `manifest.json` declares `"iot_class": "cloud_polling"` — polling only |

## `ICA+Grocy.md` inspection (task 2)

Read in full from the repo root. It describes a **household grocery lifecycle**, not an API
spec — a scenario walkthrough, not code:

0. **Review current inventory** — Grocy's due-dates/min-stock features surface what's low or
   spoiling.
1. **Meal prep** — plan recipes against Grocy inventory; missing items go to the Grocy shopping
   list; a `GrocyShoppingListUpdated` event is proposed to sync that list to ICA (marked as an
   unimplemented "Todo" — not built, just designed).
2. **In the store** — scan via Grocy, the ICA app, HA's to-do list, or in-store scanners
   (explicitly left as user's choice, not prescribed).
2b. **After purchase** — a receipt lands in Kivra (Sweden's digital-mail service ICA receipts
   route through); a proposed (unbuilt) integration would parse the receipt PDF, match products
   by barcode/name, compute quantity from the receipt row, and set best-before from receipt
   date + the product's default shelf life — triggering a `NewReciept` event that becomes a
   Grocy PURCHASE.
3. **Once home** — sorting items into Pantry/Fridge/Freezer, with a chance to correct due dates
   (`OrganizingProducts` event).
4. **Cooking** — look up the recipe in Grocy, adjust servings, consume the ingredients, closing
   the loop back to step 0.
- **Extras**: Barcode Buddy integration for barcode→product/quantity mapping (with a
  crowd-sourced barcode federation server); enriching Grocy products with price (for a cost
  graph) and calorie data; auto-adding ICA offers matching favorited products to the shopping
  list; an unbuilt "import ICA recipes into Grocy" idea.

### Comparison against Spisordning's own inventory model (task 2.3)

Spisordning's actual `inventory_event.kind` (from `establish-household-and-catalog`'s pantry
schema, `migrations/0005_pantry_inventory.sql`) is a six-value enum: `PURCHASE`, `CONSUME`,
`DISCARD`, `ADJUST`, `TRANSFER`, `OPEN` (each with a defined field matrix and a
`corrects_event_id` self-reference for undo — the ledger-plus-projection design in that
migration's own D2/D10 decisions).

| `ICA+Grocy.md` step | Spisordning event | Overlap / divergence |
|---|---|---|
| 2b. After purchase (receipt → Grocy) | `PURCHASE` | Direct match — the ICA-Grocy design's whole "receipt becomes a purchase event" idea maps exactly onto Spisordning's `PURCHASE` kind. The receipt-parsing mechanics (Kivra PDF export, barcode/name matching, quantity-from-receipt-row) are a genuinely reusable idea, not HA-specific — see extraction below. |
| 3. Once home (correct due date/location) | `TRANSFER` + `ADJUST` | Spisordning already separates "moved to a different location" (`TRANSFER`) from "quantity/date correction" (`ADJUST`) more precisely than the document's single `OrganizingProducts` event — an improvement, not a gap. |
| 4. Cooking (consume ingredients) | `CONSUME` | Direct match. |
| 0. Review inventory (spoiled → toss) | `DISCARD` | Direct match; the document treats this as a manual review step rather than an event trigger, same as Spisordning's model (no auto-discard). |
| (none) | `OPEN` | Spisordning tracks "opened" as its own event (relevant to shelf-life-after-opening); `ICA+Grocy.md` has no equivalent concept at all — a genuine gap in the reference document, not something to import. |
| Barcode Buddy's barcode↔product↔quantity mapping | (none directly) | Conceptually close to `establish-household-and-catalog`'s `product_identifier` (barcode is optional, never identity) — the crowd-sourced "federation server" idea is HA/Barcode-Buddy-specific infrastructure, explicitly not something to inherit. |

### Extracted, nameable ideas (task 2.4/2.5)

1. **Receipt-to-PURCHASE-event pipeline** (Kivra PDF export → parse → match product by
   barcode/name → PURCHASE event with quantity from the receipt row and best-before computed
   from receipt date + product default shelf life). **Not HA-specific** — this is a genuinely
   portable design worth carrying into a future receipt-import feature (`PLAN.md`'s "Receipts"
   section already names this as low-priority future work). The Kivra-specific delivery
   mechanism is Sweden-specific but not HA-specific, and directly relevant here since Spisordning
   is itself a Swedish household tool.
2. **Offer-matching against favorited products** (`ICA_NEW_OFFERS` event → check against
   favorites → auto-add to shopping list). Portable in concept; Spisordning's own campaign-bias
   scoring (`internal/scoring`'s `campaignBonus`) already does something adjacent for meal
   planning — this idea is more about shopping-list automation than meal scoring, a distinct
   future feature, not a redesign of what exists.
3. **Barcode Buddy's mapping model** (barcode → product → quantity/unit) — the *concept* (a
   scan resolves to a known quantity/unit, not just a product id) is portable; the
   implementation (a separate self-hosted service with a crowd-sourced federation server) is
   HA-community-specific infrastructure, explicitly not recommended for adoption as-is.
4. **Recipe import from ICA → local cookbook** — listed as an unbuilt idea in `ICA+Grocy.md`;
   directly relevant to `implement-recipe-discovery` and PLAN.md's "Automatic Cookbook Growth"
   section, and newly *technically viable* per this research's capability map (ICA's
   `recipeservice` endpoints exist and were not previously known to be available) —
   worth flagging to that change explicitly.

## A third repo, discovered but not investigated (per this session's scope decision)

`mellamomax/ica_shopping`, referenced directly in `ica-api` issue #26 by its own author
(2025-06-09: "I have created an integration which talks with ICA... this is quiet useless
unless there's a way to retrieve the access token automatically"). Notably **MIT licensed**
(confirmed via `gh repo view`) — unlike either seed repo, meaning it's the one candidate here
with an explicit permissive license. Last pushed 2025-07-29 (a year stale as of this research).
Not cloned or read in this session — the user explicitly scoped this research to the two
`PLAN.md`-named repos. Recorded here as a pointer for whoever next touches ICA integration,
since its license status alone makes it worth a look before assuming `ha-ica-todo` (no license)
is the only option for anything beyond "read for ideas."

## Recommendation (task 4.2/4.3/4.4)

**ICA integration is currently viable to design toward, not yet to build.** The reverse-engineered
OAuth2/PKCE flow is real, working (per `ha-ica-todo`'s 2026 commit activity), and standards-shaped
enough to have a reasonable chance of surviving minor ICA-side changes — a materially better
starting position than this research initially expected given `ica-api`'s total breakage in 2024.
The capability surface (shopping lists, offers, barcode lookup, and — newly discovered —
recipes) covers everything `PLAN.md`'s ICA section anticipated and one thing it didn't
(recipes). The specific things that should gate moving from "research" to "build":

1. **Verify the auth flow still works today**, live, against a real ICA account — everything
   above is sourced from reading `ha-ica-todo`'s code and commit history, not from executing it.
   This is the single most important unverified item.
2. **Decide the BankID question** before committing: does the target household have an ICA
   Banken account? If yes, the username/password flow this research verified does not apply and
   a separate BankID flow (unverified whether `ha-ica-todo` or any seed repo handles it) would
   need its own investigation.
3. **License posture**: neither seed repo is licensed. A future ICA client should be written
   independently, informed by (not copied from) `ha-ica-todo`'s discovered endpoint shapes and
   auth flow — exactly the "inherit or borrow ideas, not code" approach already used for this
   document. `mellamomax/ica_shopping`'s MIT license makes it worth a closer look if actual code
   reuse is desired.

If a future `integrate-ica` change proceeds, it should mirror `retailer-adapter`'s existing
structural shape exactly, the same way this document's capability table mirrors
`willys-capabilities.md`'s: a standalone sibling repo (own git history, own package, matching
`~/dev/willys/willys-client`'s pattern — not code embedded in this repo) wrapped as an HTTP
adapter service that Spisordning's `internal/retailer`-equivalent calls, implementing
search/resolve/pin/review/wishlist with **no automated checkout**, consistent with both Willys's
existing boundary and the fact that no checkout/payment endpoint exists in ICA's traced API
surface anyway.

**This research is a prerequisite for, not itself, that future `integrate-ica` change** — no
adapter code, no new repo, and no client scaffolding were created in this session; this document
is the sole deliverable, per `research-and-integrate-ica`'s own proposal scope.

Sources: `gh repo view`/`gh api` against `svendahlstrand/ica-api` and `LazyTarget/ha-ica-todo`
(repo metadata, commit history, README, `ICA+Grocy.md`, `authenticator.py`, `const.py`,
`manifest.json`), and `svendahlstrand/ica-api#26` (issue thread, 2024-04-06 to 2025-06-25).

## Update — a sibling `ica-client` scaffold has started (2026-08-18)

The line above ("no adapter code, no new repo, and no client scaffolding were created in this
session") is now stale. `~/dev/willys/ica-client` exists as an untracked, in-progress
TypeScript package — read directly from source on disk, not from a commit message or README, so
treat the specifics below as a snapshot, not a stable interface:

- **Structurally, it is exactly the sibling-repo shape this document already recommended**: its
  own `package.json`, a layered `IcaClient` composing `HttpClient` + `GraphQLClient` +
  per-concern `*Service` classes (`auth`, `config`, `customer`, `favorites`, `search`, `cart`),
  built by close analogy to `~/dev/willys/willys-client`'s own architecture (same
  `fetch-cookie`/`tough-cookie` cookie-jar pattern, same `ecom-request-source` header, same
  `x-csrf-token`-stripping helper, same per-concern service-class layout). This validates the
  "mirror `willys-client`'s pattern" structural call made above.
- **It targets a different ICA surface than the one this document verified.** Everything above
  (auth flow, capability map) is sourced from `ha-ica-todo`, which reverse-engineers the
  **mobile-app API**: OAuth2 Authorization Code + PKCE against a Curity identity server at
  `ims.icagruppen.se`, data calls at `apimgw-pub.ica.se/sverige/digx/...`. The `ica-client`
  scaffold instead targets `https://handlaprivatkund.ica.se` — ICA's **web storefront** — with
  cookie/session-based auth (no OAuth2/PKCE in sight) and a GraphQL endpoint at
  `/stores/{storeId}/graphql` alongside REST under `/stores/{storeId}/api/...` and top-level
  `/api/...` (e.g. `/api/customer/v1/customer`, `/api/config/v1/config`). **These are two
  distinct integration paths with two different auth models** — a future `integrate-ica` change
  must not conflate "ICA API access" as one thing; it should decide explicitly whether it targets
  the mobile surface (documented above, auth flow verified-on-paper via `ha-ica-todo`), the web
  storefront surface (being scaffolded now, auth flow not yet solved), or both.
- **Auth is an explicit unresolved stub on the web-storefront path**: `AuthService.login()`
  always returns `{ success: false }`, with its own comment marking it "a placeholder until the
  login endpoint is reverse-engineered." So gate #1 from this document's recommendation ("verify
  the auth flow live") is **not yet satisfied** by this scaffold — if anything it opens a second,
  still-unverified auth question alongside the mobile-API one. No BankID handling is attempted
  either.
- **Scaffolded-but-unverified endpoints** (shapes appear inferred from `willys-client`'s own
  discovered Axfood API conventions, not yet confirmed against live ICA traffic): active cart +
  cart-status (`/stores/{storeId}/api/cart/v1/...`), product search
  (`/stores/{storeId}/api/search/v1/products`), customer profile, config, and a GraphQL
  favorites/adverts query (`GetAdvertsAndModulesForFavoritesPage`) whose shape —
  `retailerProductId`/`productId` pairing, `__typename`-driven fragments, advert
  modules/styles — closely resembles patterns already seen in Willys's own storefront. That
  resemblance is suggestive of a shared commerce-platform vendor pattern across Nordic grocery
  e-commerce sites, not confirmed proof of one; worth a specific verification pass if/when this
  path is pursued.
- **Earlier-stage than `willys-client`'s own first commit**: no tests (the `tests/` directory is
  empty), no `openapi.yaml`, no `docs/api-discovery.md`-equivalent findings doc yet.
- **Net effect on this document's recommendation**: still **viable to design toward, not yet to
  build**, but the "which surface" question is now a precondition alongside the three gates
  already listed, and the web-storefront path's own auth question needs the same live-verification
  treatment given to the mobile path before either can be considered a solved prerequisite.
