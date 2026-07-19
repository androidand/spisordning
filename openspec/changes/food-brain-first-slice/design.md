## Context

A homelab recon (2026-07) established that the generic layer largely exists: Mealie is
deployed (`hlab-mealie`), Olla provides a local OpenAI-compatible LLM, Skolmaten exposes
school-lunch menus, Home Assistant + homeops MCP cover the ambient surface, and the
`willys-client` TypeScript repo already speaks the Willys v1 API (including store-scoped
campaigns and durable wishlists). A working-but-unlearning n8n workflow ties a subset of these
together today. The gap is a durable, tested model of *the family* — preferences, effort,
reactions, decisions — and a reproducible way to turn that into a weekly plan and a shopping
list. This slice builds exactly that gap and nothing more, wiring the existing pieces together
end to end.

## Goals / Non-Goals

**Goals:**
- Prove the full pipe: Mealie recipe → Food Brain plan → canonical shopping requirements →
  Willys wishlist, with family preference/effort data actually influencing the plan.
- Establish the durable schema and the one-owner-per-domain boundaries so later slices deepen
  rather than reshape.
- Make the scorer deterministic and unit-tested; make the LLM's role additive and non-load-bearing.

**Non-Goals:**
- No automated checkout, payment, or slot booking — ever (BankID/Klarna is human).
- No pantry/Grocy integration in this slice (added once a workflow proves itself).
- No Open Food Facts enrichment yet (the Willys product detail endpoint covers first-slice needs).
- No new recipe UI — Mealie is the recipe-facing app.
- No tengil packaging here — that's a separate change in the tengil repo.

## Decisions

### D1: One owner per domain (the load-bearing rule)
Recipes/ingredients → Mealie. Preferences, effort, reactions, plan decisions, canonical
shopping requirements → Food Brain. Retailer product mappings + wishlist → willys-adapter.
Final cart & payment → Willys (human). Food Brain stores Mealie references
(`mealie_recipe_id`, `mealie_food_id`, `last_synced_at`, `raw_mealie_snapshot`), never
authoritative copies.

### D2: LLM-assisted, Go-scored suggestion engine
The Go scorer computes feasibility and a numeric score from hard/soft signals: per-person
preferences with confidence, effort profile vs. the day's kitchen energy, a repetition penalty
over recent `meal_events`, Skolmaten dedup (don't echo today's school lunch), and a Willys
campaign bias (cheaper-this-week meals rank up). Olla receives the top-N feasible candidates
and (a) proposes variations within constraints and (b) writes the "why this meal" explanation.
The LLM never gates feasibility — a Go test can assert the plan without Olla present.

### D3: The Willys handoff is a wishlist, not a basket
The Willys API has no standalone basket — only the session cart and durable wishlists, and
checkout needs BankID. So the adapter's primary output is `CreateShoppingList` → a per-week
Willys **wishlist** (durable, reviewable in the phone app, one call from becoming a cart).
Cart-filling (`POST /v1/wishlist/{id}/cart`) is an optional, explicitly-triggered second step;
payment and slot booking are always manual. This also degrades gracefully to an in-store list.

### D4: Retailer-independent requirements
Food Brain emits `{ ingredientId, quantity, unit, acceptableForms[], preferredForm }`; the
adapter resolves to `{ retailerProductId, packages, resolvedQuantity, matchType, confidence }`.
A Mealie recipe never carries a Willys article number — that mapping lives in the adapter and
`ingredient_mappings`, and it can change per store and per week.

### D5: willys-adapter wraps, doesn't port
The TypeScript `willys-client` stays TypeScript, run as a small HTTP service owning session
caching, CSRF, retry, rate-limiting, and `ensureHomeStore()` (store-scoping matters for
campaigns/prices). Go calls it over HTTP and never sees a cookie. Language boundary = process
boundary.

### D6: Swedish ingredient mapping is first-class
`ingredient_mappings` (Mealie food → canonical ingredient → typical form/quantity) is a
core table with a review surface from day one. dl/msk/tsk/förp → grams → package sizes is the
grindiest, highest-garbage-risk step; every downstream quantity depends on it.

### D7: Deployment baseline is docker-compose
Postgres + food-brain + willys-adapter in a compose file for the first slice. Tengil
`package` deployment is the strategic target, planned separately in the tengil repo.

## Risks / Trade-offs

- **Ingredient-mapping quality** gates shopping-list correctness; mitigated by making the
  mapping table + review first-class (D6) and starting with a small curated recipe set.
- **Absorbing the n8n v1** risks a gap where neither system is trusted; mitigated by keeping
  n8n running until the Go pipe is verified end to end, then demoting it.
- **No confirmed Postgres** on the homelab — this slice provisions its own; revisit if a
  shared instance appears.
- **Store-scoped campaign data** requires the adapter to pin the home store before querying;
  handled by `ensureHomeStore()` already in the client.
- **LLM variability** could leak into decisions if D2 is violated; the unit tests assert the
  scorer's output independently of Olla to prevent that drift.
