# Inspiration sites — `vadfanskajaglagatillmiddag.nu`

Research deliverable for `implement-recipe-discovery` Section 3 (Inspiration site
investigation). PLAN.md calls this site out as worth studying **even if it cannot be
integrated** — so this document deliberately separates two things: (a) the site's underlying
data/selection mechanism (can it be legally/technically reused?), and (b) its interaction UX
(what is worth copying into Spisordning's own discovery surface, independent of the data).

## What the site is

`https://vadfanskajaglagatillmiddag.nu/` ('what should I cook for dinner?') is a one-purpose
Swedish site: you land on it, it shows you **one** recipe, and a button lets you roll for
another. `/om` says the site was born in 2011 to solve 'beslutsångest' (decision anxiety) —
signed '/ Linus'. The entire product is a single 'surprise me' affordance.

## Investigation method

Fetched the live pages (home, `/vegetariskt`, `/om`) and inspected the returned HTML source
directly — the raw server response, not a browser devtools session. Two separate fetches of the
home page were made specifically to test whether the shown recipe changes per request. No
scraping was performed and none is recommended (see recommendation).

## Findings

### 3.1 / 3.2 — Candidate list and selection mechanism

The site is **server-rendered**. Each page load returns a fully-formed HTML page that already
contains exactly one chosen recipe, presented as a title linking out to the source site. The
'candidate list' is a curated set of `(title, source-URL)` pairs held server-side; the
selection is a random pick from that list performed at request time, and the result is baked
into the HTML before it is sent. There is no client-side list to inspect — the browser only
ever sees the one already-chosen recipe.

### 3.3 — Server-side vs. client-side randomness

**Server-side.** Two independent fetches of the home page returned two different recipes —
first 'Rågmacka med tonfiskröra' (linking to `coop.se`), then 'Ost- och skinkpaj' (linking to
`ica.se`). The returned HTML contains **no `<script>` tags at all**, so there is no
client-side randomness to be hiding; the choice is made on the server and the page is just the
result.

### 3.4 — Source sites and categories

The recipes link out to two Swedish publisher sites: **`ica.se`** and **`coop.se`** (the same
two publishers evaluated in Section 1). Two categories are exposed:
- `/` — 'VANLIG JÄVLA MAT' (regular food)
- `/vegetariskt` — 'VEGETARISKT?' (vegetarian)

### 3.5 — Hidden API

**None.** There is no hidden/JSON API. The page has no `<script>` tags; the 'roll again'
control is a plain link with `href=""` that simply reloads the current page (triggering a fresh
server-side pick). The only 'interface' is the HTML page itself.

### 3.6 — Legal / terms constraints

- **No `robots.txt`** (returns 404) and **no terms-of-service page** was found.
- The site is a **thin wrapper**: it does not host the recipe content, it only holds a curated
  list of titles linking out to `ica.se`/`coop.se`, where the actual recipes (and their
  copyright) live.
- The absence of a robots.txt or ToS does **not** make scraping appropriate. The recipe content
  belongs to the publishers, and the wrapper's value is its curation + the random-pick UX, not
  a data feed. **Conclusion: do not scrape or integrate the site's data.**

### 3.7 — UX study (the part worth keeping)

The interaction is the whole product, and it is worth copying as a *pattern*, independent of
the data:
- **One affordance, one tap, one result.** No search, no filters, no list to scroll — the user
  asks 'what should I cook?' and gets exactly one answer.
- **Zero decision fatigue.** The site exists to *remove* a decision, not add one. That is the
  opposite of a recipe browser and the exact job of a 'surprise me' / 'what should I make
  tonight?' button.
- **Re-roll is trivial.** Getting a different suggestion is a single reload — no navigation, no
  state.
- **A category is the only extra axis** (regular vs. vegetarian), and even that is a single
  toggle, not a filter panel.

For Spisordning this maps to a **'surprise me' affordance on its own discovery/recommendation
surface**: one tap returns one recommended recipe (drawn from the household's own cookbook /
plan / preferences), with a trivial re-roll and at most a single category constraint. That is a
UX lesson for the recommendation capability, **not** a reason to pull this site's data into
Spisordning.

## 3.8 — Recommendation

**OMIT the data mechanism; ADOPT the UX lesson.**

- **Data/mechanism: OMIT.** Do not scrape, mirror, or integrate the site's recipe list or its
  random-pick backend. It has no API, no license/ToS, no robots.txt, and its content is a
  curated set of links to `ica.se`/`coop.se` — the same publishers already covered (and
  integrable via JSON-LD) in Section 1. There is nothing here that Section 1's sources don't
  already provide, and pulling it in would add legal risk for zero data benefit.
- **UX: ADOPT (as a separate capability).** The single 'surprise me' affordance — one tap →
  one recipe → trivial re-roll → at most one category axis — is a strong, directly transferable
  interaction pattern for Spisordning's own discovery surface. It is recorded here as a design
  input; implementing it belongs to the recommendation/discovery capability, not to this
  change's import pipeline.
