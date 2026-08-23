# Tasks: research-nutrition-data-sources

## 1. Livsmedelsverket client

- [x] 1.1 Create `internal/ingredients/` package with shared HTTP client helper
- [x] 1.2 Implement `LivsmedelsverketClient` with `LookupFood`, `SearchFood`,
      `LookupNutrition`, `LookupClassifications`, `LookupRawCommodities`
- [x] 1.3 Implement `SyncAll` for full-dataset sync (pagination via offset/limit)
- [x] 1.4 Add types: `Food`, `Nutrient`, `Classification`, `RawCommodity`, `FoodPage`
- [x] 1.5 Verify all endpoints against live SLV API (done — 2,606 records, HATEOAS links
      confirmed, nutrition/classification/raw endpoints all return valid data)

## 2. Dabas client

- [x] 2.1 Implement `DabasClient` with `Search` and `SearchAll` (pagination via
      `FromRange`, 100 results per page)
- [x] 2.2 Add types: `DabasProduct`, `DabasNutrient`
- [x] 2.3 Implement `ParseNutrientValue` for the " 282 kcal" / "< 0.5 g" format
- [x] 2.4 Verify search endpoint against live Dabas API (done — 29,769 total records,
      "korv" returns 1,137, "mjölk" returns 14,398)
- [x] 2.5 Create reverse-engineered OpenAPI spec at
      `~/dev/store-clients/dabas-client/openapi.yaml`
- [x] 2.6 Create `~/dev/store-clients/dabas-client/` TypeScript package with types
      and client skeleton

## 3. Matpriskollen client

- [x] 3.1 Implement `MatpriskollenClient` with `Search`, `SearchByGTIN`,
      `SearchStores`, `GetStoreOffers`, `SearchStoresByName`
- [x] 3.2 Add types: `MPKProduct`, `MPKCategory`, `MPKProductGroup`,
      `MPKStore`, `MPKOffer`, `MPKOfferProduct`, `MPKProductTag`, `MPKStoreOffers`
- [x] 3.3 Discover and implement price/offer endpoints — FULLY VERIFIED:
      prices live in `/api/v1/stores/{key}/offers?lat=&lon=&limit=` endpoint,
      reverse-engineered from frontend JS bundles
- [x] 3.4 Verify all endpoints against live API (done — search, stores,
      store offers all return valid data with prices)
- [x] 3.5 Create reverse-engineered OpenAPI spec at
      `~/dev/store-clients/matpriskollen-client/openapi.yaml`
- [x] 3.6 Create `~/dev/store-clients/matpriskollen-client/` TypeScript package
      with full types and client

## 4. Documentation

- [x] 4.1 Document all four APIs in `docs/research/nutrition-data-sources.md`
      with live response samples, field inventories, and integration plans
- [x] 4.2 Add Primat.nu entry (API key required — document what's known from the
      landing page: daily per-store prices, OpenAPI contract, supports ICA/Coop/
      Willys/Hemköp/Lidl/City Gross)
- [x] 4.3 Add OpenAPI specs for Dabas and Matpriskollen in `~/dev/store-clients/`

## 5. Data model design

- [ ] 5.1 Design `foods` table schema: `slv_nummer` (PK), `namn`, `vetenskapligtNamn`,
      `livsmedels_typ`, `projekt`, `version`, synced_at
- [ ] 5.2 Design `nutrients` table schema: `food_nummer` (FK), `eurofir_kod`,
      `namn`, `varde`, `enhet`, `metodtyp`, synced_at
- [ ] 5.3 Design `product_mappings` table: `gtin` (nullable), `slv_nummer` (nullable),
      `dabas_arident` (nullable), `canonical_ingredient_id` (FK), mapped_at
- [ ] 5.4 Define the cross-reference strategy: SLV nummer is the canonical key;
      Dabas GTINs and Aridents map to it; Matpriskollen keys map via GTIN

## 6. Sync job

- [ ] 6.1 Add `food-brain sync nutrition` CLI command
- [ ] 6.2 Implement SLV full sync (paginated, ~27 pages at 100/page, with nutrition
      prefetched per food)
- [ ] 6.3 Implement Dabas sync (paginated, ~300 pages at 100/page — consider
      concurrency limits and rate sensitivity)
- [ ] 6.4 Add sync status tracking (last_synced_at per source, incremental updates)

## 7. Wiring to planner

- [ ] 7.1 Define how nutrition data enriches `domain.Candidate` — add optional
      `NutritionProfile` field with macro/micro summary
- [ ] 7.2 Design the ingredient → nutrition lookup path: recipe ingredient line →
      canonical ID → SLV/Dabas lookup → cached nutrition profile
- [ ] 7.3 Plan preference-based nutrition filtering (e.g. "low sodium" preferences
      that score down high-sodium candidates)

## 8. Follow-up: price comparison

- [x] 8.1 Matpriskollen price endpoints fully discovered and implemented
- [ ] 8.2 Obtain Primat.nu API key and integrate as `internal/price/` client
- [ ] 8.3 Wire cross-store price comparison into the planning scorer
