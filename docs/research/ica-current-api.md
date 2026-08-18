# ICA — current API access (research)

`PLAN.md`'s "External Research — ICA" section asks for this document before designing any
ICA-side retailer interface — the same discipline `willys-capabilities.md` applied to Willys.
This is a **research-only** record: it captures verified findings from inspecting the two seed
repositories (`svendahlstrand/ica-api`, `LazyTarget/ha-ica-todo`) and marks anything that could
not be confirmed from source/docs/issues as **unverified — requires live testing**. It does not
implement an ICA adapter.

Findings are recorded incrementally as `openspec/changes/research-and-integrate-ica/tasks.md`
sections are completed. Every claim cites its source; verified findings are stated as fact, and
open questions are explicitly flagged.

## 1. Older client — `svendahlstrand/ica-api`

Source: cloned `https://github.com/svendahlstrand/ica-api` (30 commits, HEAD `a39ab5e`);
`README.md` and `api-referens.md` read directly from the clone; GitHub issue #26 read via the web.

### Status: documentation self-declared inaccurate since April 2024

- **Last commit:** `a39ab5e`, **2024-04-17**, by Sven Dahlstrand — *"Update README to inform
  about inaccurate documentation."* (Verified: `git log`.) No commits since.
- **README warning banner** (introduced by that commit, `README.md` lines 3–4): *"Uppdatering
  17 april 2024: Tyvärr har ICA gjort ändringar i sitt API så dokumentationen här är inte längre
  korrekt. Mer information finns i detta ärende (issue #26)."* — "ICA has made changes to its API
  so the documentation here is no longer correct."
- **Issue #26** — *"API documentation is not accurate"*, opened **2024-04-06** by `@classek`,
  still **Open**, labeled `bug`, no assignee, no linked PR: *"Nice tutorial but it seems like
  they have closed access. Is there a workaround ??"* (Verified: GitHub issue page.)
- The maintainer's response to the breakage was to **mark the docs as inaccurate** (the README
  warning, 2024-04-17) rather than to restore or fix access. No fix has landed since.

**Conclusion (verified):** This **confirms** `PLAN.md`'s initial observation that the older
`ica-api` documentation became inaccurate after ICA's April 2024 API changes. The repository has
had no commits since 2024-04-17, and its only open issue is the breakage report itself.

### What the (now-inaccurate) docs claim

The README documents: base URL `handla.api.ica.se` (HTTPS); authentication via `GET /api/login`
using HTTP Basic auth (username = personnummer, password = the code mailed with the monthly
Buffé bonus statement) returning an `AuthenticationTicket` response header, sent on subsequent
calls; example `GET /api/user/cardaccounts`.

`api-referens.md` documents the following endpoint surface. **All of it is unverified against
the live API** — the docs are self-declared inaccurate, so this table is a record of what the
older client *claimed*, not a confirmed capability map:

| Area | Documented endpoints |
|---|---|
| Auth | `GET /api/login` |
| Cards | `GET /api/user/cardaccounts` |
| Bonus transactions | `GET /api/user/minbonustransaction` |
| Stores | `GET /api/user/stores`, `GET /api/stores/{id}`, `GET /api/stores/?LastSyncDate=`, `GET /api/stores/search` |
| Offers | `GET /api/offers?Stores=` |
| Shopping lists | `GET`/`POST /api/user/offlineshoppinglists`, `GET`/`DELETE .../{id}`, `POST .../{id}/sync` |
| Article groups | `GET /api/articles/articlegroups?lastsyncdate=` |
| Common articles | `GET /api/user/commonarticles/` |
| Recipes | `GET /api/user/recipes`, `GET /api/recipes/searchwithfilters`, `GET /api/recipes/search/filters`, `GET /api/recipes/recipe/{id}`, `GET /api/recipes/{id}/rating`, `GET /api/recipes/random`, `GET /api/recipes/categories/general...` |
| Barcode lookup | `GET /api/upclookup?upc=` |

**Unverified — requires live testing:** whether any of these endpoints still respond, and what
the post-April-2024 authentication flow actually looks like. The older client's docs cannot be
used as a capability map.

## 2. Newer client — `LazyTarget/ha-ica-todo`

_Pending (tasks 1.3–1.5)._

## 3. Current ICA API access

_Pending (tasks 3.1–3.3)._

## 4. Capability map

_Pending (task 3.4) — to be produced in the same shape as `willys-capabilities.md` once the
newer client's actually-implemented capabilities are confirmed._

## 5. Recommendation

_Pending (tasks 4.2–4.4)._
