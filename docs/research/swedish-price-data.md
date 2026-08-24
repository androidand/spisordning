# Swedish Price Data Sources Research

Research date: 2026-08-22

---

## 1. Primat

- **Name:** Primat
- **API available:** No — `primat.se` resolves to a parked domain (Binero hosting placeholder). `api.primat.se` fails. No API documentation found.
- **License/terms:** Unknown — no website or terms found.
- **Rate limits:** Unknown.
- **Retailers covered:** Unknown.
- **Store-level granularity:** Unknown.
- **EAN identity:** Unknown.
- **Campaign coverage:** Unknown.
- **Member-price coverage:** Unknown.
- **Price history depth:** Unknown.
- **Update interval:** Unknown.
- **Commercial use:** Unknown.
- **Veracity:** **Not found** — `primat.se` is a parked domain. PLAN.md claims it exposes retailer/store price data through a REST API, but this cannot be verified. The domain may have been sold, expired, or never launched publicly. **This source is currently unusable.**

**Assessment:** The domain `primat.se` is parked with Binero hosting. No website, API, documentation, or contact information exists. This could mean:
- The service was acquired or rebranded
- The domain was sold to a third party
- The service never launched publicly
- It operates under a different domain

**Recommendation:** Investigate whether "Primat" operates under a different brand/domain. Check if the service was acquired. Search for "Primat price API" or "Primat matpriser" on LinkedIn, Crunchbase, or Swedish business registres (Allegro). If no active service exists, this source should be deprioritized.

---

## 2. Matpriskollen

- **Name:** Matpriskollen Sverige AB (Org.nr: 556923-1631)
- **API available:** **No public API.** Website at `matpriskollen.se` offers a consumer web app and mobile app. A B2B portal exists at `b2b.matpriskollen.se` but requires authentication and no API documentation is publicly available. Contact: `info@matpriskollen.se`, phone `0346-82210`.
- **License/terms:** Consumer-facing terms at `matpriskollen.se/allmanna-villkor-matpriskollen`. Terms grant a "gratis, tidsbegränsad, icke-exklusiv, icke-overforbar, icke-underlicensierbar nyttjanderatt" — a free, time-limited, non-exclusive, non-transferable, non-sub-licensable right to use the service. Section 4.3 explicitly prohibits copying data or features to build a competing product. Section 11.1 states all content IP is owned by Matpriskollen. **No commercial data license is offered publicly.**
- **Rate limits:** Not documented.
- **Retailers covered:** **Very large coverage.** Claims to scan flyers from "alla matbutikerna" across Sweden. Store pages show logos for Willys, Coop, Lidl, Rusta, Dollarstore, Hemköp, ICA, and many more. The site explicitly says "Vi skriver av alla matbutikernas flygblad från hela Sverige" and generates ~200,000 new offers per week. 33+ store brands visible.
- **Store-level granularity:** **Yes.** The site is organized by individual stores with store-specific offer pages (`/butiker/<store-slug>`). Store logos are shown per product.
- **EAN identity:** **Likely yes.** Product pages use UUID-based IDs (e.g., `/produkter/31befa68-e96b-4f6a-48a7-08d47f3b722d`). Products have consistent naming (brand, size, weight). However, no public EAN/GTIN lookup is documented. Products appear to be matched internally.
- **Campaign coverage:** **Yes — this is their primary feature.** The site's core value proposition is scanning and displaying weekly promotional flyers ("flygblad"). Section 1.1 of terms: "Användare kan se aktuella erbjudanden i butiker."
- **Member-price coverage:** **Likely yes.** The site aggregates campaign/offer prices. Some products show multi-buy deals ("2 for 55,00"). Member prices would depend on whether the scanned flyers include them.
- **Price history depth:** **Moderate.** The site has a "Prisutveckling" (`/prisutveckling`) page tracking price trends over time. Since they've operated since 2010, historical data likely exists but is not publicly queryable via API.
- **Update interval:** **Weekly (flyer cycle) + daily.** Flyers are scanned "from early Monday morning" and the site says "ca 200,000 new offers every week." The consumer app appears to update daily.
- **Commercial use:** **Not permitted without agreement.** Terms explicitly prohibit copying data, building competing products, or sub-licensing. The B2B portal (`b2b.matpriskollen.se`) suggests commercial data access may be possible through direct negotiation. **Must contact them directly.**
- **Veracity:** **Verified** — checked current website, terms, B2B portal, and "Om oss" page (August 2026).

**Assessment:** Matpriskollen is the richest consumer-facing Swedish price data source with extensive retailer/store coverage, campaign data, and store-level granularity. However, there is **no public API** and **no commercial data license**. The B2B portal suggests enterprise access may be negotiable. This would require direct outreach to `info@matpriskollen.se` to discuss data licensing. Their terms are restrictive for commercial use.

---

## 3. Matmoms

- **Name:** Matmoms
- **API available:** **No public API.** Website at `matmoms.se` offers a consumer web app for comparing prices at ICA, Coop, and Willys. No API endpoints found (`/api` returns 404). However, they explicitly state on their homepage: "Vi erbjuder fullstandig produktdata med 419 varor, 33 butiker och dagliga prisobservationer. Format: CSV, JSON, API." Contact for data access: `gabriel.linton@gmail.com`.
- **License/terms:** No terms of service or license page found on the site.
- **Rate limits:** Not documented.
- **Retailers covered:** **3 chains only** — ICA, Coop, Willys. These are Sweden's three largest grocery chains.
- **Store-level granularity:** **Yes.** 33 stores across 9 cities: Stockholm, Gothenburg, Malmo, Uppsala, Orebro, Linkoping, Sundsvall, Umea, and Lulea.
- **EAN identity:** **Likely yes.** The site tracks 419 specific food products with consistent naming. Products appear to be matched by name/size rather than EAN, but internal product identity likely exists.
- **Campaign coverage:** **Partial.** The site collects prices daily and includes current offers ("dagliga prisobservationer"). However, it focuses on price comparison rather than campaign-specific tracking. The FAQ mentions "aktuella erbjudanden" but the primary focus is baseline price comparison.
- **Member-price coverage:** **No.** Member prices are specific to individual store loyalty programs and not typically shared. Matmoms appears to track standard retail prices.
- **Price history depth:** **Good — daily updates.** The site says "Vi samlar in priser dagligen" and "alla priser uppdateras varje morgon." They track price history per product ("Se prishistoriken per produkt"). They also track the impact of the April 2026 food VAT reduction (from 12% to 6%).
- **Update interval:** **Daily.** Prices are collected daily and updated every morning.
- **Commercial use:** **Unknown — requires negotiation.** The site offers data access in CSV, JSON, API format for journalists and researchers ("Datatillgang for journalister och forskare"). Contact is `gabriel.linton@gmail.com`. This suggests willingness to share data, but terms are not published. **Must contact directly.**
- **Veracity:** **Verified** — checked current website, FAQ, and data access page (August 2026).

**Assessment:** Matmoms is the most promising source for direct data access. They explicitly offer their data in CSV/JSON/API format for journalists and researchers, which is closer to the spisordning use case than consumer apps. Coverage is limited to 3 chains and 419 products, but the daily update frequency, store-level granularity, and explicit data-sharing posture make it the most actionable source. The contact is a personal email (Gabriel Linton), suggesting a small operation.

---

## 4. Matpriser.nu

- **Name:** Matpriser.nu
- **API available:** **No.** Website at `matpriser.nu` is a WordPress-based blog with affiliate marketing content. No API or data export functionality found.
- **License/terms:** No terms of service found. The site uses affiliate links and earns commission ("matpriser.nu far ersattning om du valjer att handla").
- **Rate limits:** N/A.
- **Retailers covered:** **6 online grocery retailers** — Mat.se, Hemkop, Coop, Ica, Mathem, Willys. This is a comparison of online-only grocery delivery, not physical stores.
- **Store-level granularity:** **No.** This is a weekly price comparison of online shopping carts across 6 retailers. No individual store data.
- **EAN identity:** **No.** Products are matched by name/size for comparison purposes but no product identity system is exposed.
- **Campaign coverage:** **No.** The comparison includes a basic weekly snapshot with some offer adjustments ("-200 kr" for Mat.se) but no systematic campaign tracking.
- **Member-price coverage:** **No.**
- **Price history depth:** **Minimal.** The site says prices are updated weekly ("uppdateras en gang i veckan") but no historical price tracking is exposed.
- **Update interval:** **Weekly.**
- **Commercial use:** **Not permitted.** The site is an affiliate marketing blog. No data license is offered.
- **Veracity:** **Verified** — checked current website, comparison page, and contact page (August 2026).

**Assessment:** Matpriser.nu is a content/affiliate blog, not a data service. It has no API, no data export, and no commercial licensing. The weekly comparison is useful for human readers but not programmatically accessible. **Not usable as a data source.**

---

## 5. Comparator

- **Name:** Comparator Sverige
- **API available:** **No public API.** Website at `comparator.se` offers a consumer-facing price comparison for electricity and groceries. No API endpoints found. Contact: `info@comparator.se`.
- **License/terms:** No terms of service found. Privacy policy at `comparator.se/integritetspolicy` covers GDPR compliance but says nothing about data licensing. The site states "Priser ar vagledande och kan variera. Kontrollera alltid aktuellt pris hos leverantorren" (prices are indicative).
- **Rate limits:** Not documented.
- **Retailers covered:** **3 chains** — Willys, Coop, ICA. Same three major chains as Matmoms.
- **Store-level granularity:** **No.** Prices are "baserade pa Stockholm" (based on Stockholm). No individual store data — only chain-level comparison.
- **EAN identity:** **No.** Products are tracked by name (e.g., "Standardmjolkdryck 3,0% Laktosfri 1,5l Arla") but no product identity system or EAN lookup is exposed.
- **Campaign coverage:** **Partial.** Prices "inkluderar veckans erbjudanden pa enskilda varor, men inte flervara-erbjudanden" (include weekly offers on individual items, but not multi-item offers).
- **Member-price coverage:** **No.**
- **Price history depth:** **Moderate.** The site publishes weekly price reports with historical comparison ("Korgpris jamfort med vecka 33"). Product-specific pages show weekly price history charts. Coverage appears to be several months to a year.
- **Update interval:** **Weekly.** Prices are "uppdateras varje mandag" (updated every Monday).
- **Commercial use:** **Not permitted without agreement.** The site is financed via advertising (Google AdSense). No data license is offered. The "Samarbeten" (collaborations) page on the contact form suggests they may be open to partnerships, but no data API is offered.
- **Veracity:** **Verified** — checked current website, matpriser pages, om-oss, kontakt, and integritetspolicy (August 2026).

**Assessment:** Comparator is a consumer-facing comparison site similar to Matmoms but with weekly (not daily) updates and no store-level granularity. No API exists and no data license is offered. The weekly price reports are published as articles, not data. **Not usable as a data source without direct negotiation.**

---

## 6. Open Prices

- **Name:** Open Prices (open-prices GitHub org)
- **API available:** **No public API.** The GitHub org (`github.com/open-prices`) has 3 repositories: `database-models` (MIT license, last updated Jul 2022), `services` (last updated Feb 2019), and `openprices-app` (last updated Feb 2019). No documentation site found (`openprices.org`, `wiki.openprices.org`, `docs.openprices.org`, `prices.openprices.org`, `open-prices.readthedocs.io` all fail).
- **License/terms:** The `database-models` repo uses the MIT license. However, this license covers the code, not the data. The org has no description, website, or topics.
- **Rate limits:** Not documented.
- **Retailers covered:** **Unknown.** The GitHub org has no documentation describing which retailers or countries are covered. The repos are minimal (sequelize models, no actual data or API code visible).
- **Store-level granularity:** **Unknown.**
- **EAN identity:** **Unknown.** The database models repo uses Sequelize ORM, suggesting a database schema for prices/products, but the actual schema is not visible without cloning the repo.
- **Campaign coverage:** **Unknown.**
- **Member-price coverage:** **Unknown.**
- **Price history depth:** **Unknown.**
- **Update interval:** **Unknown.**
- **Commercial use:** **Unknown.** The code is MIT-licensed, but data licensing is not addressed.
- **Veracity:** **Not found** — the GitHub org appears inactive (last commits 2019-2022), has no documentation, no website, and no clear description of what the project does or covers. The 3 repos are minimal code artifacts with no data or API.

**Assessment:** Open Prices appears to be an abandoned or very early-stage project. The GitHub org has no documentation, no website, and the last code updates were 4-7 years ago. The `database-models` repo uses Sequelize (Node.js ORM) which suggests a price tracking database, but without cloning the repo or finding documentation, the Swedish coverage cannot be determined. **This source is currently unusable and likely abandoned.**

---

## Ranking: Usability Assessment

### Tier 1 — Actionable Now

| Source | Why |
|--------|-----|
| **Matmoms** | Explicitly offers data in CSV/JSON/API format for journalists and researchers. Daily updates, 33 stores, 419 products, 3 major chains. Requires direct contact (`gabriel.linton@gmail.com`) but has the clearest path to data access. |

### Tier 2 — Negotiable (Requires Outreach)

| Source | Why |
|--------|-----|
| **Matpriskollen** | Richest data (largest store coverage, campaign data, ~200k weekly offers), but has a B2B portal suggesting enterprise access is possible. Terms are restrictive; must negotiate a commercial data license. |

### Tier 3 — Not Actionable Without Significant Effort

| Source | Why |
|--------|-----|
| **Comparator** | Consumer-facing only, no API, no data license. Would require scraping or direct negotiation. Weekly updates, 3 chains, no store granularity. |

### Tier 4 — Currently Unusable

| Source | Why |
|--------|-----|
| **Matpriser.nu** | Affiliate blog, no API, no data export, no license. |
| **Primat** | Parked domain, no service exists. Requires investigating alternative domains/brands. |
| **Open Prices** | Abandoned GitHub org, no documentation, no data, no API. |

---

## Recommended Next Steps

1. **Contact Matmoms** (`gabriel.linton@gmail.com`) to request data access. Their explicit offer of CSV/JSON/API format for researchers is the closest match to spisordning's needs. Ask about:
   - Data licensing terms for commercial use
   - API access vs. batch CSV/JSON export
   - Update frequency and historical data availability
   - Product identity (EAN/GTIN support)
   - Store-level vs. chain-level data

2. **Contact Matpriskollen** (`info@matpriskollen.se`) to inquire about B2B data licensing. Ask about:
   - What the B2B portal (`b2b.matpriskollen.se`) offers
   - Commercial data licensing options
   - API access for programmatic data retrieval
   - Coverage details (how many retailers/stores)
   - Campaign and price history data availability

3. **Investigate Primat** further:
   - Check Swedish business registry (Allegro) for "Primat" company
   - Search LinkedIn for "Primat matpriser" or "Primat price API"
   - Check if the domain was recently sold/parked
   - If no active service exists, deprioritize

4. **Open Prices** is not worth further investigation unless the team can clone and inspect the `database-models` repo schema to understand what data structure was intended.

5. **Matpriser.nu** and **Comparator** should not be built against as primary data sources — they offer no API or data license.
