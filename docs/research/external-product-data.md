# External Product Data Sources Research

Research date: 2026-08-22

---

## 1. Livsmedelsverket (Sweden's Food Agency)

- **Name:** Livsmedelsverket — Swedish Food Agency
- **API available:** **Partially.** The agency maintains a public food composition database at `lvudata.livsmedelsverket.se`. The domain resolves but the API endpoint (`/api/v1/`) returned a connection error during this research — the service may be temporarily unavailable or behind a firewall. The database is known to contain food composition data (nutrients, ingredients, classifications) and has been used by researchers and developers.
- **License/terms:** The Swedish government data portal (`data.gov.se`) hosts Livsmedelsverket data under the **CC BY 4.0** license (Creative Commons Attribution 4.0 International), which permits commercial use with attribution. However, the specific terms for the food composition database should be verified directly.
- **Rate limits:** Not documented publicly. Government data portals typically have reasonable rate limits for research use.
- **Retailers covered:** **N/A** — this is a food composition database, not a price or product catalog. It contains canonical ingredient/vocabulary data, nutrition information, and food classification — not retailer-specific products.
- **Store-level granularity:** **N/A** — not applicable. This is a national food database, not retail-specific.
- **EAN identity:** **Partial.** The database uses its own internal food item IDs. Some items may reference EAN/GTIN codes, but the primary identifier is Livsmedelsverket's own food item ID system.
- **Campaign coverage:** **N/A** — not applicable.
- **Member-price coverage:** **N/A** — not applicable.
- **Price history depth:** **N/A** — this is a composition database, not a price database.
- **Update interval:** **Periodic.** The database is updated periodically as new food composition data becomes available. Specific update frequency is not publicly documented.
- **Commercial use:** **Likely permitted** under CC BY 4.0, but should be verified. Government data in Sweden is generally open for commercial use with attribution.
- **Veracity:** **Unverified** — the API endpoint was not reachable during research. The database is known to exist and has been used historically, but current availability and exact terms need verification.

**Assessment:** Livsmedelsverket's food composition database is a valuable reference for canonical ingredient vocabulary, nutrition data, and food classification. It is **not a price data source** — it would complement the price intelligence system by providing authoritative food composition data, but it does not feed the price observation pipeline. The ontology should be cross-checked against spisordning's `ingredient` model before any adoption. The CC BY 4.0 license (if confirmed) permits commercial use with attribution, making it suitable for spisordning's use case.

**Recommendation:** Verify the API availability and license terms directly. Cross-check the food classification ontology against spisordning's `ingredient` model. If compatible, use as a reference for canonical ingredient naming and nutrition data — not as a price source.

---

## 2. Open Prices (Product-Data Angle)

- **Name:** Open Prices (open-prices GitHub org)
- **API available:** **No.** The GitHub org (`github.com/open-prices`) has three repositories: `database-models` (MIT license, last updated Jul 2022), `services` (last updated Feb 2019), and `openprices-app` (last updated Feb 2019). No documentation site, no API, no data.
- **License/terms:** The `database-models` repo uses the MIT license for code, but this covers the code, not the data. No data license is specified.
- **Rate limits:** N/A.
- **Retailers covered:** **Unknown.** No documentation describes coverage. The project appears abandoned (last commits 2019-2022).
- **Store-level granularity:** **Unknown.**
- **EAN identity:** **Unknown.** The database models use Sequelize ORM but the actual schema is not visible without cloning the repo.
- **Campaign coverage:** **Unknown.**
- **Member-price coverage:** **Unknown.**
- **Price history depth:** **Unknown.**
- **Update interval:** **Unknown.**
- **Commercial use:** **Unknown.** Code is MIT-licensed, but data licensing is not addressed.
- **Veracity:** **Not found** — the GitHub org appears inactive. No documentation, no website, no clear description of scope or coverage.

**Assessment:** Open Prices appears to be an abandoned project. The GitHub org has no documentation, no active development, and no clear description of what data it was intended to hold. The Swedish coverage cannot be determined. **This source is not usable for spisordning's needs.**

**Scope overlap with §3.6:** The price-focused evaluation in `swedish-price-data.md` already covers Open Prices and reaches the same conclusion — the project appears abandoned with no actionable data or API. No separate product-data research is needed.

---

## 3. Open Food Facts (Out of Scope)

- **Note:** Open Food Facts barcode lookup is **out of scope** for this change. It belongs to `implement-pantry-inventory` (Epic D), which already implements `LookupBarcode` against `ProductIdentifier`. This change (Epic F: Price Intelligence) does not need to re-implement or re-evaluate Open Food Facts.

---

## Summary

| Source | Price Data | Product Data | API Available | Commercial Use | Actionable |
|--------|-----------|--------------|---------------|----------------|------------|
| Livsmedelsverket | No | Yes (composition) | Partial (unverified) | Likely (CC BY 4.0) | Reference only |
| Open Prices | No (abandoned) | No | No | Unknown | No |
| Open Food Facts | No (out of scope) | Yes | Yes | Open (ODbL) | Out of scope |

**Recommendation:** Livsmedelsverket is the only actionable source in this research slice, and only as a reference for canonical ingredient vocabulary and nutrition data — not as a price source. Price intelligence should focus on the Tier 1 and Tier 2 sources identified in `swedish-price-data.md` (Matmoms, Matpriskollen).
