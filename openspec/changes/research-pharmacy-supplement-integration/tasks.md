# Tasks: research-pharmacy-supplement-integration

## 1. Platform discovery

- [ ] 1.1 Check apotekhjartat.se for a discoverable REST/GraphQL API behind product
      search (browser devtools network tab on a real search, same technique used for
      the original ICA/Willys reverse-engineering)
- [ ] 1.2 Identify the underlying commerce platform (SAP Commerce / custom / other)
- [ ] 1.3 Repeat 1.1-1.2 for apoteket.se and kronans.se as comparison points, in case
      Apotek Hjärtat's own site is a dead end but a sibling pharmacy chain isn't

## 2. Auth and regulatory scoping

- [ ] 2.1 Determine whether OTC categories (supplements, vitamins) can be browsed
      and added to cart/wishlist without a BankID-verified account
- [ ] 2.2 Read the site's terms of service for automation/scraping restrictions
- [ ] 2.3 Confirm Omega-3, iron, and zinc products are unrestricted OTC — document
      the category/product IDs found, if any

## 3. Findings and go/no-go

- [ ] 3.1 Write up findings in this change's design.md (create if research proceeds
      past task 1-2): API shape (if any), auth model, ToS constraints, platform
      identification
- [ ] 3.2 Go/no-go recommendation: worth a full client build, or does the site make
      this impractical (no API, heavy bot-detection, ToS explicitly prohibits it)?
- [ ] 3.3 If go: sketch how a "supplement need" maps onto the shopping-requirement /
      ingredient-resolver shape already used for groceries — no implementation yet,
      just confirm the existing shape fits before scoping a follow-up change
