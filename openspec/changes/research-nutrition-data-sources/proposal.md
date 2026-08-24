# Research nutrition data sources

## Why

Spisordning's meal planner currently scores candidates on preferences, effort, and
campaign pricing — but has no nutrition awareness. When a household member has dietary
restrictions (allergies, low-sodium, high-protein goals) or the family tracks macros,
the planner cannot filter or rank candidates on nutritional content. The gap exists
because no nutrition data source is wired into the system.

Three Swedish food data sources are available, each covering a different layer:

1. **Livsmedelsverket** (Swedish Food Agency) — authoritative reference database of
   ~2,600 generic food items with 50+ nutrients each, LanguaL™/FoodEx2 classifications,
   and raw commodity decomposition. Open API, CC BY 4.0. This is the canonical source
   for "what is the nutrient profile of hirs kokt?"
2. **Dabas** — branded product database of ~30,000 Swedish products with nutrition info,
   structured allergen data, full ingredient lists, and GTINs. No auth required,
   undocumented REST endpoint. This fills the gap where SLV has generic "korv" but the
   household buys "Chili/Cheddar Frankfurter Korv" from Danish Crown — Dabas has the
   branded product's actual nutrition.
3. **Matpriskollen** — Swedish price comparison service spanning ICA, Coop, Willys,
   Hemköp, Lidl, and City Gross. Search API works and returns products with GTINs,
   categories, and brands across all major Swedish retailers. Price data requires
   additional authenticated endpoints not yet reverse-engineered. This is the cross-store
   price comparison layer.

Additionally, **Primat.nu** was identified as a self-serve API for daily per-store
prices (ICA, Coop, Willys, Hemköp, Lidl, City Gross) with an OpenAPI contract, but
it requires an API key not yet obtained. It is tracked as a future opportunity.

## What Changes

- Create `internal/ingredients/` Go package with clients for Livsmedelsverket and
  Dabas, both tested against live endpoints.
- Create `internal/ingredients/matpriskollen.go` client for the Matpriskollen search
  endpoint (prices tracked as out-of-scope pending endpoint discovery).
- Add reverse-engineered OpenAPI spec for Dabas at
  `~/dev/store-clients/dabas-client/openapi.yaml` per the store-clients repo's
  design-first workflow.
- Document all three APIs in `docs/research/nutrition-data-sources.md` with live
  response samples, field inventories, and integration plans.
- Define the data model for a local nutrition cache table (`foods` + `nutrients`)
  that the sync job populates from SLV, and the cross-reference table
  (`product_mappings`) that links Dabas GTINs and SLV nummers to canonical ingredient
  IDs.
- Plan the sync job (`food-brain sync nutrition`) and the wiring path from recipe
  ingredients → canonical IDs → nutrition lookup → `Candidate` scoring.

## Out of Scope

- Price comparison implementation (Matpriskollen prices, Primat.nu) — these require
  authenticated endpoints or API keys not yet available. Tracked as follow-up.
- AI-based ingredient extraction from Dabas `Ingredient` text — useful but separate
  from the structured data this change provides.
- Real-time price fetching at planning time — prices are volatile; the existing
  retailer-adapter layer already handles that. This change is about stable nutrition
  reference data.

## Dependencies

- Depends on `implement-shopping-and-commerce` for the `ShoppingRequirement` →
  `Resolution` flow that will eventually consume nutrition-enriched product data.
- Depends on `establish-household-and-catalog` for the canonical ingredient ID system
  that nutrition lookups resolve against.
- Sibling of `research-and-integrate-ica` (both add data sources; this one is
  nutrition-focused, the other is retailer-integration-focused).
