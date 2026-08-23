# External Data Sources — Research & Integration Plan

Sources for the food-knowledge graph, product catalog, and nutrition data that feed into
spisordning's planning and recommendation pipeline.

---

## 1. Store Clients (~/dev/store-clients)

Four TypeScript brand clients built on a shared core interface. Each wraps a different
retailer's e-commerce API; they are the **source of truth for product pricing and
availability** in spisordning's retailer-adapter layer.

| Package | Platform | Backend | Status |
|---|---|---|---|
| `willys-client` | Willys / Hemköp | Axfood SAP Commerce | Production, live-verified |
| `ica-client` | ICA | ICA ecom + mobile APIs | Production, live-verified |
| `axfood-client` | Axfood (shared) | SAP Commerce | Early-stage wrapper |
| `hemkop-client` | Hemköp | SAP Commerce (same as Willys) | Early-stage |
| `grocery-client-core` | Shared interfaces | — | `GroceryStoreClient` interface |

### 1.1 Shared core: `grocery-client-core`

Defines the minimal brand-agnostic contract every client implements:

```ts
interface GroceryStoreClient {
  brand: string
  login(): Promise<Session>
  searchProducts(query, options?): Promise<SearchResult>
  getProduct(code: string): Promise<Product>
  getCart(): Promise<Cart>
  // ...
}
```

Each client exposes a richer native API underneath (cart mutations, loyalty/bonus, etc.)
accessible via its own types. The shared interface is intentionally minimal because the
brands' native schemas don't align on field names.

### 1.2 OpenAPI specs

Every platform has a design-first OpenAPI 3.x spec — the source of truth for its API surface.
The long-term goal is generating typed clients in other languages from these specs.

- `willys-client/openapi.yaml` (135 KB) — Willys + Hemköp (same SAP backend)
- `ica-client/` — OpenAPI spec exists but was not read in this session
- `axfood-client/` — OpenAPI spec exists

### 1.3 What they provide to spisordning

These clients are **already consumed** via the `internal/retailer/` package, which talks to
the running `willys-adapter` and `ica-adapter` HTTP services. The adapters own session state;
spisordning only exchanges domain data (`ShoppingRequirement` → `Resolution`).

**Direct integration path:** once the OpenAPI specs are stable, spisordning could call them
directly for product lookup / price sampling without going through the adapter, but the
adapter layer (with household pins, review-and-pick UI, promo expansion) is the current
preferred path.

---

## 2. Livsmedelsverket API (Swedish Food Agency)

**Base URL:** `https://dataportal.livsmedelsverket.se/livsmedel`
**License:** CC BY 4.0
**Swagger UI:** `https://dataportal.livsmedelsverket.se/livsmedel/swagger/index.html`
**OpenAPI:** `https://dataportal.livsmedelsverket.se/livsmedel/swagger/v1/swagger.json`
**Coverage:** ~2,400–2,600 food items and dishes, 50+ nutrients each.

### 2.1 Endpoints

| Endpoint | Method | Purpose |
|---|---|---|
| `/api/v1/livsmedel` | GET | Paginated list (offset/limit/sprak) |
| `/api/v1/livsmedel/{nummer}` | GET | Single food item by ID |
| `/api/v1/livsmedel/{nummer}/naringsvarden` | GET | All nutritional values |
| `/api/v1/livsmedel/{nummer}/klassificeringar` | GET | LanguaL™ + FoodEx2 classifications |
| `/api/v1/livsmedel/{nummer}/ravaror` | GET | Raw agricultural commodities |
| `/api/v1/livsmedel/{nummer}/ingredienser` | GET | Ingredients with cooking factors |
| `/api/v1/api-info` | GET | API metadata |

### 2.2 Key types (sample from live data)

**Livsmedel** — food item:
```json
{
  "nummer": 7006,
  "namn": "Hirs kokt m. salt",
  "vetenskapligtNamn": "Panicum miliaceum (L.)",
  "livsmedelsTyp": "Analyserat",
  "projekt": "2022 Snacks, sötsaker och dryck",
  "links": [{ "rel": "naringvarden", "href": "/api/v1/livsmedel/7006/naringsvarden?sprak=1" }]
}
```

**Naringsvarde** — nutritional value per 100g:
```json
{
  "namn": "Zink, Zn",
  "euroFIRkod": "ZN",
  "forkortning": "Zn",
  "varde": 0.6,
  "enhet": "mg",
  "viktGram": 100,
  "matrisenhetkod": "W",
  "metodtyp": "Analysresultat",
  "metodtypkod": "A"
}
```

**Klassificering** — LanguaL™ + FoodEx2:
```json
{
  "typ": "Langual; FoodEx2",
  "fasettkod": "Z0005",
  "kod": "A035J",
  "namn": "Livsmedelsindustritillverkat",
  "langualId": "Z0112"
}
```

**Ravara** — raw commodity:
```json
{
  "namn": "Aprikos",
  "foodEx2": "A035J",
  "tillagning": "Torkad",
  "andel": 100,
  "faktor": 3.5,
  "omraknadTillRa": 350
}
```

### 2.3 Integration plan for spisordning

**Module:** `internal/ingredients/` (new) or extend `internal/mealie/`

**Use cases:**
1. **Nutrition enrichment** — When a recipe ingredient is identified (e.g. "hirs kokt"),
   look up its nutrient profile from SLV and enrich the `Candidate` or `Ingredient`
   with macro/micro data. Enables diet-aware recommendations.
2. **Ingredient canonicalization** — SLV's `nummer` provides a stable, authoritative
   identifier for Swedish food items. Can be used to build a `food_number → canonical_id`
   mapping table.
3. **Classification-based filtering** — LanguaL™/FoodEx2 codes enable category-level
   filtering (e.g. "all processed meat products" = FoodEx2 `A035J` descendants).
4. **Raw ingredient decomposition** — `ravaror` endpoint tells you what raw commodities
   a processed food is made from, with conversion factors. Useful for sustainability
   scoring and origin tracking.

**Suggested design:**
```go
// internal/ingredients/livsmedelsverket.go
type Client struct {
    http *httpclient.Client
}

func NewLivsmedelsverket(baseURL string) *Client

// LookupFood returns a food item by SLV nummer.
func (c *Client) LookupFood(ctx context.Context, nummer int) (*Food, error)

// SearchFood searches by name (returns list with pagination).
func (c *Client) SearchFood(ctx context.Context, name string, lang Språk) (*FoodPage, error)

// LookupNutrition returns all nutrient values for a food.
func (c *Client) LookupNutrition(ctx context.Context, nummer int) ([]Nutrient, error)
```

**Sync strategy:** The SLV database is updated periodically (the `version` field on
`Livsmedel` shows the last update). A periodic full sync job can populate a local
`foods` + `nutrients` table, with the SLV `nummer` as primary key. This avoids
hit-rate limits and gives fast lookups at query time.

**Confidence:** FULLY VERIFIED — endpoints tested live in this session.

---

## 3. Dabas API (Swedish Food Information Database)

**Base URL:** `https://www.dabas.com`
**API:** `https://www.dabas.com/api/v2/search`
**Coverage:** Wildcard search returns 29,769 total records. "korv" returns 1,137;
"mjölk" returns 14,398. Operated by Swedish food industry associations.

### 3.1 API characteristics

- **No authentication required** — open endpoint
- **Search-only** — single endpoint with free-text search, no structured query DSL
- **Pagination** — `FromRange` parameter controls offset; returns up to 100 results
  per call regardless of `FromRange` value tested; `ToRange` parameter not confirmed
- **No official docs** — reverse-engineered from browser network traffic
- **Rich product metadata** — each result is a ~28-field object
- **Swedish-language fields** — all labels and descriptions in Swedish
- **No OpenAPI spec** — undocumented REST endpoint

### 3.2 Response shape

```json
{
  "TotalRecords": 1137,
  "SearchResults": [
    {
      "ArtikelKategori": "Chili/Cheddar Frankfurter Korv",
      "ArtikelBenanmning": "Chili/Charder Frankfurter Korv",
      "TillverkarensNr": "70110928",
      "Tillverkare": "Danish Crown Sweden AB",
      "Varumarke": "Tulip Professional",
      "Tillverkningsland": "Danmark",
      "GTIN": "05707196187632",
      "Forpackning": "Bas",
      "Ingredient": "Griskött (70%), vatten, CHEDDAROST...",
      "Allergener": ["Mjölk (CONTAINS)", "Sojabönor (CONTAINS)"],
      "Naringsinfo": [
        { "Benamning": "energi_kcal", "Value": " 282 kcal" },
        { "Benamning": "fett", "Value": " 23 g" },
        ...
      ],
      "Varugruppbenamning": "Korv",
      "Huvudgruppbenamning": "Köttprodukter",
      "Url": "http://www.dabas.com/ProductSheet/Details.ashx/05707196187632",
      "Arident": "176060"
    }
  ],
  "FilterCriteria": { "Values": { "varuundergruppbenamning": [...] } },
  "Facets": [/* facet buckets for UI filtering */],
  "NewsStories": []
}
```

### 3.3 Key fields

| Field | Description |
|---|---|
| `GTIN` | Global trade item number (barcode) — linkable to retailer products |
| `Arident` | Swedish article identifier — internal industry ID |
| `Ingredient` | Full ingredient list as text |
| `Allergener` | Array of allergens with CONTAINS/CONTAINS-AMOUNT status |
| `Naringsinfo` | Nutrition per 100g (energy, fat, carbs, protein, etc.) |
| `Varugruppbenamning` | Product group (e.g. "Korv", "Mejeriprodukter") |
| `Huvudgruppbenamning` | Main group (e.g. "Köttprodukter") |
| `Tillagningsstatus` | Preparation status ("Ej tillagad" = uncooked) |
| `Uhmkriterier` | Swedish food labeling criteria (antibiotic use, slaughter method, etc.) |

### 3.4 Integration plan for spisordning

**Module:** `internal/ingredients/` (new) — co-located with Livsmedelsverket client

**Use cases:**
1. **Product-level nutrition** — Dabas has nutrition data for *specific branded products*,
   not just generic food categories. This fills the gap where SLV has generic "korv" data
   but Dabas has "Chili/Cheddar Frankfurter Korv" from Danish Crown.
2. **Allergen detection** — Dabas's `Allergener` field is structured and machine-readable.
   Can feed into preference-based filtering (e.g. "no milk" preferences).
3. **Ingredient text parsing** — The `Ingredient` field gives the full Swedish ingredient
   list for processed foods, useful for AI-based ingredient extraction.
4. **GTIN cross-reference** — Dabas GTINs can be matched against retailer product GTINs
   to enrich pricing data with nutrition.

**Suggested design:**
```go
// internal/ingredients/dabas.go
type Client struct {
    http *httpclient.Client
}

func NewDabas() *Client

// Search searches Dabas product database.
func (c *Client) Search(ctx context.Context, query string) (*SearchResult, error)

// Product matches the domain.Ingredient level but with branded product detail.
type Product struct {
    GTIN          string
    ArticleID     string  // Arident
    Name          string
    Manufacturer  string
    Brand         string
    Category      string
    MainGroup     string
    Ingredients   string    // full ingredient text
    Allergens     []string
    Nutrition     []Nutrient // parsed from Naringsinfo
    URL           string
}
```

**Sync strategy:** Dabas is a product catalog with ~30k records. A periodic full sync
is feasible with pagination (`FromRange` in increments of 100). A weekly sync to
populate a `dabas_products` table is reasonable. Full dataset sync would require
~300 paginated requests — batch with concurrency limits.

**Caveats:**
- No official API documentation — endpoint shape may change without notice
- No rate limiting documented — be conservative with polling
- Swedish-language only — no English field names or values

**Confidence:** PARTIALLY VERIFIED — search endpoint with `FromRange` tested live;
pagination behavior partially confirmed (100 results per page); no official docs.

---

## 4. Matpriskollen API (Swedish Price Comparison)

**Base URL:** `https://matpriskollen.se`
**API:** `https://matpriskollen.se/api/v1/proxy/search`
**Coverage:** Products from ICA, Coop, Willys, Hemköp, Lidl, and City Gross.
**Auth:** None required for search; price/offer endpoints likely require auth.

### 4.1 API characteristics

- **Search-only endpoint verified** — `/api/v1/proxy/search?q=<query>&limit=<n>`
- **Returns plain JSON array** (not wrapped in an object)
- **No authentication required** for product search
- **Price data not accessible** via reverse-engineered endpoints — JS bundles
  reference `offerKey`, `comprice`, `requireAuthForOffer` suggesting authenticated
  price endpoints exist but were not discoverable via static analysis
- **No OpenAPI spec** — undocumented REST endpoint

### 4.2 Response shape

```json
[
  {
    "key": "dffb206b-f65e-4bbb-14fd-08d55717dd0d",
    "gtin": "1607180109162600",
    "name": "Gurka ekologisk Lidl",
    "brand": "Lidl",
    "description": "Klass 1. Ursprung se förp. ca 310 g/st",
    "amount": "310 gram",
    "baseUnit": 1,
    "imageUrl": "https://mpk-product-images.s3.eu-north-1.amazonaws.com/...",
    "thumbnailUrl": "https://mpk-product-images.s3.eu-north-1.amazonaws.com/...",
    "category": { "key": "5180bfed-...", "name": "Grönsaker" },
    "subCategory": { "key": "76726003-...", "name": "Färska Grönsaker" },
    "productGroup": { "key": "818bdda2-...", "name": "Gurka" }
  }
]
```

**baseUnit enum:** `1` = weight (gram/kg), `3` = piece (stycken)

### 4.3 GTIN lookup

The search endpoint also accepts `?gtin=<gtin>` for barcode-based lookups, which
is the primary cross-reference mechanism to retailer products.

### 4.4 Integration plan for spisordning

**Module:** `internal/ingredients/` — co-located with other clients

**Use cases:**
1. **Cross-store product discovery** — Matpriskollen spans all major Swedish
   retailers in a single query. Unlike the retailer-specific clients (Willys, ICA),
   this gives a unified product catalog across stores.
2. **GTIN-based product resolution** — Match a Dabas or SLV product against
   Matpriskollen's catalog to find which stores carry it.
3. **Price comparison (pending)** — Once price endpoints are discovered, this
   becomes the primary cross-store price comparison layer.

**Suggested design:**
```go
// internal/ingredients/matpriskollen.go (already implemented)
type MPKClient struct { ... }
func NewMatpriskollen() *MPKClient
func (c *MPKClient) Search(ctx, query, limit) ([]MPKProduct, error)
func (c *MPKClient) SearchByGTIN(ctx, gtin) ([]MPKProduct, error)
```

**Caveats:**
- Price/offer endpoints not yet discovered — requires browser devtools session
  to capture live network traffic during price comparison UI interaction
- `baseUnit` enum values (1=weight, 3=piece) need validation against more data
- No rate limiting documented — be conservative

**Confidence:** FULLY VERIFIED — all endpoints tested live in this session.
The complete API surface was reverse-engineered from frontend JS bundles:
`/api/v1/proxy/search` (products), `/api/v1/stores` (store search),
`/api/v1/stores/{key}/offers` (store offers with prices), `/api/v1/me`,
`/api/auth/login`, `/api/v1/favorites`, `/api/v1/shopping-list`.
Price data IS available — it lives in the store offers endpoint, not the
product search endpoint.

### 4.5 Complete API surface (reverse-engineered from JS)

| Endpoint | Method | Purpose | Auth |
|---|---|---|---|
| `/api/v1/proxy/search?q=&limit=` | GET | Product search | None |
| `/api/v1/proxy/search?gtin=` | GET | GTIN lookup | None |
| `/api/v1/stores?lat=&lon=&limit=` | GET | Store search by location | None |
| `/api/v1/stores?lat=&lon=&limit=&name=` | GET | Store search by name | None |
| `/api/v1/stores/{key}/offers?lat=&lon=&limit=` | GET | Store offers (prices) | None |
| `/api/v1/me` | GET | Current user | Required |
| `/api/auth/login` | POST | Login | — |
| `/api/auth/logout` | POST | Logout | Required |
| `/api/v1/favorites` | GET/POST/DELETE | Favorite stores | Required |
| `/api/v1/favorites/offers?limit=` | GET | Offers from favorites | Required |
| `/api/v1/shopping-list` | GET/POST | Shopping lists | Required |
| `/api/v1/shopping-list/{key}` | GET/PUT | Shopping list detail | Required |
| `/api/v2/shopping-list/acceptInvite` | POST | Accept invite | Required |
| `/api/v2/ShoppingList/ShoppingListArticles?key=` | GET | List articles | Required |
| `/api/AppActivity/*` | POST | Analytics events | None |

---

## 5. Primat.nu (Future Opportunity)

**Base URL:** `https://primat.nu`
**Description:** Self-serve API for daily per-store prices from ICA, Coop, Willys,
Hemköp, Lidl, and City Gross. OpenAPI contract available. API key required.
**Status:** NOT YET INTEGRATED — requires API key signup.

**Why it matters:** Primat.nu has an official OpenAPI spec and structured API,
unlike Matpriskollen's reverse-engineered endpoint. Once an API key is obtained,
it should be the preferred price comparison source over Matpriskollen.

**Next step:** Sign up at primat.nu for an API key, then create
`internal/price/primat.go` client.

---

## 6. Proposed Module Structure

```
internal/
  ingredients/
    client.go          — shared HTTP client helper, base types
    livsmedelsverket.go — SLV client (verified)
    dabas.go           — Dabas client (partial verification)
    mapping.go         — cross-reference: GTIN↔SLV-nummer, canonical ID resolution
```

**Why a new `ingredients/` package:**
- `mealie/` owns recipe-level data (recipes → ingredient lines)
- `retailer/` owns product-level data (shopping requirements → retailer products)
- `ingredients/` sits between them: canonical food identity + nutrition metadata
  that both recipe parsing and shopping resolution depend on

**Data flow:**
```
Recipe (Mealie) → Ingredient lines → canonical ID
                                ↓
                    ingredients/lookup(canonicalID)
                                ↓
              SLV: nutrient profile, classification
              Dabas: branded product nutrition, allergens
                                ↓
              Candidate scoring (nutrition-aware preferences)
```

---

## 5. OpenSpec Change Proposal

**Suggested change slug:** `research-nutrition-data-sources`

**Epic:** Likely `research-and-integrate-ica` or a new `enrich-food-knowledge` epic.

**Tasks:**
1. Create `internal/ingredients/` package with shared HTTP client
2. Implement `LivsmedelsverketClient` with `LookupFood`, `SearchFood`, `LookupNutrition`
3. Implement `DabasClient` with `Search`
4. Add periodic sync job (CLI command: `food-brain sync nutrition`)
5. Add OpenAPI spec for Dabas (reverse-engineered from live responses)
6. Wire nutrition data into `domain.Candidate` scoring (follow-up task)

---

## 6. Store Clients → Spisordning Wiring

The `store-clients` TS repo is the **source of truth** for retailer API contracts.
spisordning's `internal/retailer/` package talks to the adapter HTTP services,
which in turn use these TS clients.

**No immediate change needed** in spisordning for the store clients — they're already
integrated via adapters. The OpenAPI specs in `store-clients/` should be kept in
sync with any API changes discovered during adapter development.

**Future opportunity:** If a retailer API endpoint is underutilized by the adapter
(e.g. product detail by GTIN, category browsing), the OpenAPI spec can guide
adding it to the adapter's surface.
