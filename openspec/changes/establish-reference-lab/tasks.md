# Tasks: establish-reference-lab

## 1. Confirm the reference-lab deployment (Phase 1)

- [ ] 1.1 Confirm Mealie and Grocy are deployed to Proxmox through Tengil / Docker Compose with
      isolated persistence (Epic H, `tengil` repo — precondition for this change, not performed
      here).
- [ ] 1.2 Record for Mealie: version, commit/tag if applicable, container image, license,
      deployment config, database/storage.
- [ ] 1.3 Record for Grocy: version, commit/tag if applicable, container image, license,
      deployment config, database/storage.
- [ ] 1.4 Create comparable representative data in both systems.
- [ ] 1.5 Exercise important workflows in both systems before deep investigation begins.

## 2. Mealie investigation

Use sub-agents. For each item below capture: user behavior, API behavior, DB mutation, source
implementation, tests, strengths, weaknesses, and the Spisordning lesson.

- [ ] 2.1 Recipe model
- [ ] 2.2 Recipe editing
- [ ] 2.3 Recipe import
- [ ] 2.4 Recipe parsing
- [ ] 2.5 Structured ingredients
- [ ] 2.6 Foods
- [ ] 2.7 Units
- [ ] 2.8 Servings
- [ ] 2.9 Scaling
- [ ] 2.10 Images
- [ ] 2.11 Tags
- [ ] 2.12 Categories
- [ ] 2.13 Cookbooks
- [ ] 2.14 Search
- [ ] 2.15 Meal plans
- [ ] 2.16 Shopping
- [ ] 2.17 Households
- [ ] 2.18 Users
- [ ] 2.19 Ratings if any
- [ ] 2.20 API
- [ ] 2.21 Database
- [ ] 2.22 Migrations
- [ ] 2.23 Tests
- [ ] 2.24 Provenance

## 3. Grocy investigation

Pay special attention to edge cases accumulated through years of inventory use.

- [ ] 3.1 Products
- [ ] 3.2 Barcodes
- [ ] 3.3 Locations
- [ ] 3.4 Stock
- [ ] 3.5 Stock journal
- [ ] 3.6 Lots
- [ ] 3.7 Expiry
- [ ] 3.8 Purchase
- [ ] 3.9 Consume
- [ ] 3.10 Discard
- [ ] 3.11 Transfer
- [ ] 3.12 Adjust
- [ ] 3.13 Mark empty
- [ ] 3.14 Units
- [ ] 3.15 Unit conversion
- [ ] 3.16 Product-specific conversion
- [ ] 3.17 Shopping
- [ ] 3.18 Recipes
- [ ] 3.19 Meal planning
- [ ] 3.20 Cost tracking
- [ ] 3.21 API
- [ ] 3.22 Database
- [ ] 3.23 Migrations
- [ ] 3.24 Tests

## 4. Feature overlap matrix

- [ ] 4.1 For every overlapping capability between Mealie and Grocy, decide one of: MEALIE,
      GROCY, MERGE, REDESIGN, DEFER, OMIT.
- [ ] 4.2 State exactly what is useful and why for each decision — no vague "take the best
      parts."
- [ ] 4.3 Publish the matrix as a standalone research document.

## 5. Phase 2 — Database archaeology

Treat database migrations as historical architecture documentation.

- [ ] 5.1 For Mealie, determine: tables, relationships, foreign keys, uniqueness, nullable
      relationships, deletion behavior, quantity representations, audit/history structures,
      problematic schemas later migrated away from.
- [ ] 5.2 For Grocy, determine the same: tables, relationships, foreign keys, uniqueness,
      nullable relationships, deletion behavior, quantity representations, audit/history
      structures, problematic schemas later migrated away from.
- [ ] 5.3 Produce an ER diagram for Mealie's schema.
- [ ] 5.4 Produce an ER diagram for Grocy's schema.

## 6. Deliverables

- [ ] 6.1 Write `docs/research/mealie-*.md` document set covering the areas investigated in
      section 2.
- [ ] 6.2 Write `docs/research/grocy-*.md` document set covering the areas investigated in
      section 3.
- [ ] 6.3 Ensure the Feature Overlap Matrix (section 4) and both ER diagrams (section 5) are
      committed alongside the `docs/research/mealie-*.md` / `docs/research/grocy-*.md` sets.
- [ ] 6.4 Cross-check findings against `PLAN.md`'s Phase 3 domain-model candidates (Household,
      Preferences, Recipe Hierarchy, Meals, Favorites, Ratings, Ingredient Model, Ingredient
      Forms, Substitution, Unit System, Product, Retailer Identity, Pantry, Inventory
      Uncertainty, Inventory Events) and flag which candidates this research supports,
      contradicts, or leaves open.
