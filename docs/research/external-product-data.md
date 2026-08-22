# External Product Data Sources Research

Research date: 2026-08-22

---

## 1. Livsmedelsverket (Sweden's Food Agency)

- **Name:** Livsmedelsverket — Swedish Food Agency
- **Website:** `www.livsmedelsverket.se/livsmedel-och-innehall/naringsamne/livsmedelsdatabasen/`
- **API available:** **No public API.** Livsmedelsverket maintains a public food composition database ("Livsmedelsdatabasen") with ~2,100 food items and over 50 nutrients per item. The database is accessible only through a web search interface — no REST or GraphQL API is publicly documented or reachable. Attempts to reach `lvudata.livsmedelsverket.se/api/v1/`, `api.livsmedelsverket.se/`, and `data.gov.se/api/action/package_list?organization=livsmedelsverket` all returned connection errors or no response. The database has been used historically by researchers and developers, but there is no programmatic access point.
- **License/terms:** **Not explicitly stated on the website.** Swedish government data is generally published under open licenses (commonly CC BY or the Swedish Public Access to Information and Secrecy Act framework), but no specific license is published on the Livsmedelsverket pages. The data portal `data.gov.se` hosts some government datasets under CC BY 4.0, but Livsmedelsverket's food composition database does not appear to be listed there. Commercial use should be verified directly with the agency (`livsmedelsverket@slv.se`, +46 18 175 500).
- **Rate limits:** N/A — no API exists.
- **Data available:**
  - **Ingredient vocabulary:** ~2,100 Swedish food items and dishes with Swedish names (e.g., "Kålrabbi", "Norrbottnisk gröt"). This is a Swedish-centric canonical vocabulary, highly relevant to spisordning.
  - **Nutrition data:** Over 50 nutrient values per item (energy, protein, fat, carbs, fiber, vitamins, minerals, etc.). Values are per 100g. This is authoritative, government-grade nutritional data.
  - **Classification:** Items are categorized by food group (grains, dairy, meat, vegetables, etc.) through the website's navigation structure, but there is no formal ontology or machine-readable classification exported.
- **Ontology compatibility with spisordning:** The existing `ingredient` table (`id TEXT PRIMARY KEY, display TEXT NOT NULL`) is minimal — no nutrition fields, no classification, no source tracking. Livsmedelsverket's data would **not fit** into the current schema without migration. If adopted as a reference, spisordning would need to extend `ingredient` with fields like `nutrition JSONB`, `category TEXT`, `source TEXT`, and `source_id TEXT`. The Swedish names and food items in Livsmedelsverket's database are a strong match for spisordning's target market, but the ontology should be **cross-checked and adapted**, not adopted wholesale (per the instruction to not adopt their ontology wholesale).
- **Commercial use:** **Likely permissible** under Swedish government open data principles, but not confirmed. Should be verified directly.
- **Update frequency:** **Periodic.** The database is updated as new food composition research becomes available. Specific update frequency is not publicly documented. The "Uppdateringar" page lists historical updates but no schedule.
- **Retailer coverage:** **N/A** — this is a food composition database, not a product catalog or price source.
- **EAN/GTIN support:** **No.** The database uses its own internal food item IDs, not barcodes.

**Assessment:** Livsmedelsverket's food composition database is the most authoritative Swedish source for canonical ingredient vocabulary and nutrition data. It is **not a price source** and has no API. The value is in using it as a **reference ontology** for ingredient naming and nutrition — not for programmatic integration. The current `ingredient` schema is too simple to absorb this data directly. If spisordning wants to use this data, it would require: (1) manual or scripted download of the database, (2) schema extension to support nutrition/classification, (3) license verification, and (4) mapping Swedish names to canonical IDs.

**Recommendation:** Do not attempt programmatic integration (no API). Consider a one-time manual import of the database as a reference for canonical Swedish ingredient names and nutrition values, if the license permits. Cross-check the food classification against spisordning's ingredient model before any adoption. For the current scope (Epic F: Price Intelligence), this source is **low priority** — it is a reference for ingredient ontology, not for price or product data.

---

## 2. Open Prices (Product-Data Angle)

- **Name:** Open Prices (open-prices GitHub org)
- **GitHub org:** `github.com/open-prices`
- **API available:** **No.** The org has three repositories:
  - `database-models` (MIT license, last updated Jul 2022) — Sequelize ORM models for PostgreSQL
  - `services` (last updated Feb 2019) — Seneca.js microservices
  - `openprices-app` (last updated Feb 2019) — Create React App frontend
  The website `openprices.org` does not resolve. No API endpoints are reachable (`api.openprices.org`, `us-central1-open-prices.cloudfunctions.net`). The app proxies to `localhost:3001`, indicating a local-only backend that is no longer running.
- **Database schema (from `database-models`):**
  - `Product` — `barcode` (STRING, unique, NOT NULL), `name` (STRING, nullable)
  - `ProductName` — `name` (STRING, NOT NULL), FK to `Product` and `User`, unique on `(ProductId, UserId)`
  - `Price` — `price` (DOUBLE, NOT NULL), `date` (DATE, NOT NULL), FK to `Product`, `User`, `Vendor`, unique on `(ProductId, UserId, VendorId, date)`
  - `Vendor` — `code` (STRING, unique, NOT NULL), `name` (STRING), `address` (STRING)
  - `User` — `username` (EMAIL, unique), `nickname` (STRING, unique), `password`, `password_salt`
  - Relations: `Product` ↔ `Vendor` (many-to-many via `VendorProduct` join table), `Product` has many `ProductName`s and `Price`s
- **License/terms:** The `database-models` repo uses the **MIT license** for code. However, this covers the code, not the data. No data license is specified anywhere in the org.
- **Rate limits:** N/A — no running API.
- **Retailers covered:** **Unknown.** No documentation describes coverage. The `Vendor` model has a `code` field, suggesting retail chains could be represented, but there is no evidence of any actual data or Swedish retailer coverage.
- **Store-level granularity:** **Unknown.** The schema supports vendor-level pricing but no store-level granularity.
- **EAN/GTIN identity:** **Yes, barcode-based.** The `Product` model uses `barcode` as the primary identifier. This aligns with standard retail product identification.
- **Campaign coverage:** **Unknown.** No documentation.
- **Member-price coverage:** **Unknown.** No documentation.
- **Price history depth:** **Unknown.** The `Price` model has a `date` field, suggesting temporal tracking, but no data exists.
- **Update interval:** **Unknown.** The project appears abandoned.
- **Commercial use:** **Unknown.** Code is MIT-licensed, but data licensing is not addressed.
- **Veracity:** **Not found.** The GitHub org appears inactive (last commits 2019–2022). No documentation, no website, no clear description of scope or coverage. The Swedish coverage cannot be determined.

**Assessment:** Open Prices appears to be an abandoned project. The GitHub org has no documentation, no active development, and no clear description of what data it was intended to hold. The database schema suggests a price-tracking system (products, prices, vendors, users), but there is no evidence of actual data, a running API, or Swedish coverage. **This source is not usable for spisordning's needs.**

**Scope overlap with §3.6:** The price-focused evaluation in `swedish-price-data.md` already covers Open Prices and reaches the same conclusion — the project appears abandoned with no actionable data or API. No separate product-data research is needed.

---

## 3. Open Food Facts (Out of Scope)

- **Note:** Open Food Facts barcode lookup is **out of scope** for this change. It belongs to `implement-pantry-inventory` (Epic D), which already implements `LookupBarcode` against `ProductIdentifier`. This change (Epic F: Price Intelligence) does not need to re-implement or re-evaluate Open Food Facts.

---

## Summary

| Source | Price Data | Product Data | API Available | Commercial Use | Actionable |
|--------|-----------|--------------|---------------|----------------|------------|
| Livsmedelsverket | No | Yes (composition) | No | Likely (gov open data, unconfirmed) | Reference only |
| Open Prices | No (abandoned) | No | No | Unknown | No |
| Open Food Facts | No (out of scope) | Yes | Yes | Open (ODbL) | Out of scope |

**Recommendation:** Livsmedelsverket is the only potentially actionable source in this research slice, and only as a reference for canonical Swedish ingredient vocabulary and nutrition data — not as a price or product source. Price intelligence should focus on the Tier 1 and Tier 2 sources identified in `swedish-price-data.md` (Matmoms, Matpriskollen).

**Schema note:** spisordning's current `ingredient` table (`id TEXT, display TEXT`) is too minimal to absorb Livsmedelsverket data directly. Any future integration would require extending the schema with nutrition, classification, and source-tracking fields.
