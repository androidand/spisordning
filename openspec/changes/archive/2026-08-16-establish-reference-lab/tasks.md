# Tasks: establish-reference-lab

## 1. Confirm the reference-lab deployment (Phase 1)

- [x] 1.1 Confirm Mealie and Grocy are deployed to Proxmox through Tengil / Docker Compose with
      isolated persistence — done via Epic H (`androidand/tengil`, `add-reference-lab-packages`).
      Mealie VMID 2319 (`192.168.1.22:9000`), Grocy VMID 2320 (`192.168.1.183:80`).
- [x] 1.2 Record for Mealie: version, commit/tag if applicable, container image, license,
      deployment config, database/storage — see `docs/research/mealie-deployment.md` (v3.9.2,
      `ghcr.io/mealie-recipes/mealie:latest`).
- [x] 1.3 Record for Grocy: version, commit/tag if applicable, container image, license,
      deployment config, database/storage — see `docs/research/grocy-deployment.md` (4.6.0,
      `lscr.io/linuxserver/grocy:latest`).
- [x] 1.4 Create comparable representative data in both systems — done as part of the section 2/3
      investigations below (a household, recipes with structured ingredients across unit types,
      tags/categories/cookbook/meal plan/shopping list/rating in Mealie; locations, products
      spanning expiry/barcode/unit-conversion variations, exercised through purchase/consume/
      discard/transfer/adjust/open in Grocy).
- [x] 1.5 Exercise important workflows in both systems before deep investigation begins — done
      (URL recipe import via JSON-LD scrape in Mealie; live PURCHASE/CONSUME/DISCARD/TRANSFER/
      ADJUST/OPEN workflows in Grocy, each inspected via direct SQLite queries after the fact).

## 2. Mealie investigation

Use sub-agents. For each item below capture: user behavior, API behavior, DB mutation, source
implementation, tests, strengths, weaknesses, and the Spisordning lesson.

- [x] 2.1 Recipe model — `docs/research/mealie-recipe-model.md`
- [x] 2.2 Recipe editing — `docs/research/mealie-recipe-model.md` (incl. the live-reproduced
      PATCH/PUT data-loss and corruption bug)
- [x] 2.3 Recipe import — `docs/research/mealie-recipe-model.md`
- [x] 2.4 Recipe parsing — `docs/research/mealie-recipe-model.md`
- [x] 2.5 Structured ingredients — `docs/research/mealie-recipe-model.md`
- [x] 2.6 Foods — `docs/research/mealie-recipe-model.md`
- [x] 2.7 Units — `docs/research/mealie-recipe-model.md` (finding: no conversion system at all)
- [x] 2.8 Servings — `docs/research/mealie-recipe-model.md`
- [x] 2.9 Scaling — `docs/research/mealie-recipe-model.md`
- [x] 2.10 Images — `docs/research/mealie-recipe-model.md`
- [x] 2.11 Tags — `docs/research/mealie-recipe-model.md`
- [x] 2.12 Categories — `docs/research/mealie-recipe-model.md`
- [x] 2.13 Cookbooks — `docs/research/mealie-recipe-model.md` (finding: saved filter, no
      recipe-membership table)
- [x] 2.14 Search — `docs/research/mealie-planning-and-search.md`
- [x] 2.15 Meal plans — `docs/research/mealie-planning-and-search.md`
- [x] 2.16 Shopping — `docs/research/mealie-planning-and-search.md`
- [x] 2.17 Households — `docs/research/mealie-planning-and-search.md`
- [x] 2.18 Users — `docs/research/mealie-planning-and-search.md` (finding: conflates login
      identity with food-domain person)
- [x] 2.19 Ratings if any — `docs/research/mealie-planning-and-search.md` (finding: the
      group-wide-rating-retrofitted-to-per-user migration incident)
- [x] 2.20 API — `docs/research/mealie-api-and-database.md`
- [x] 2.21 Database — `docs/research/mealie-api-and-database.md`
- [x] 2.22 Migrations — `docs/research/mealie-api-and-database.md` (43 Alembic migrations read)
- [x] 2.23 Tests — `docs/research/mealie-api-and-database.md`
- [x] 2.24 Provenance — `docs/research/mealie-api-and-database.md`

## 3. Grocy investigation

Pay special attention to edge cases accumulated through years of inventory use.

- [x] 3.1 Products — `docs/research/grocy-inventory-and-stock.md`
- [x] 3.2 Barcodes — `docs/research/grocy-inventory-and-stock.md`
- [x] 3.3 Locations — `docs/research/grocy-inventory-and-stock.md`
- [x] 3.4 Stock — `docs/research/grocy-inventory-and-stock.md` (finding: rows deleted at zero
      quantity — only `stock_log` preserves history)
- [x] 3.5 Stock journal — `docs/research/grocy-inventory-and-stock.md` (finding: "undo" mutates
      a historical row in place)
- [x] 3.6 Lots — `docs/research/grocy-inventory-and-stock.md`
- [x] 3.7 Expiry — `docs/research/grocy-inventory-and-stock.md`
- [x] 3.8 Purchase — `docs/research/grocy-inventory-and-stock.md`
- [x] 3.9 Consume — `docs/research/grocy-inventory-and-stock.md`
- [x] 3.10 Discard — `docs/research/grocy-inventory-and-stock.md` (finding: not a distinct
      transaction type — `CONSUME` + `spoiled` boolean)
- [x] 3.11 Transfer — `docs/research/grocy-inventory-and-stock.md`
- [x] 3.12 Adjust — `docs/research/grocy-inventory-and-stock.md`
- [x] 3.13 Mark empty — `docs/research/grocy-inventory-and-stock.md` (finding: not a distinct
      transaction type — UI sugar over `CONSUME` with full quantity)
- [x] 3.14 Units — `docs/research/grocy-units-and-planning.md`
- [x] 3.15 Unit conversion — `docs/research/grocy-units-and-planning.md` (finding: live-reproduced
      auto-insert-wrong-default-then-collide bug)
- [x] 3.16 Product-specific conversion — `docs/research/grocy-units-and-planning.md`
- [x] 3.17 Shopping — `docs/research/grocy-units-and-planning.md`
- [x] 3.18 Recipes — `docs/research/grocy-units-and-planning.md` (finding: fulfillment logic as
      opaque SQL views)
- [x] 3.19 Meal planning — `docs/research/grocy-units-and-planning.md`
- [x] 3.20 Cost tracking — `docs/research/grocy-units-and-planning.md`
- [x] 3.21 API — `docs/research/grocy-api-and-database.md` (incl. the API-key-via-SQLite
      workaround needed because the LinuxServer image's `/login` route errored)
- [x] 3.22 Database — `docs/research/grocy-api-and-database.md` (finding: zero declared foreign
      keys anywhere)
- [x] 3.23 Migrations — `docs/research/grocy-api-and-database.md` (256 migration files read from
      source)
- [x] 3.24 Tests — `docs/research/grocy-api-and-database.md` (finding: no automated test suite
      at all)

## 4. Feature overlap matrix

- [x] 4.1 For every overlapping capability between Mealie and Grocy, decide one of: MEALIE,
      GROCY, MERGE, REDESIGN, DEFER, OMIT.
- [x] 4.2 State exactly what is useful and why for each decision — no vague "take the best
      parts."
- [x] 4.3 Publish the matrix as a standalone research document — `docs/research/
      feature-overlap-matrix.md`.

## 5. Phase 2 — Database archaeology

Treat database migrations as historical architecture documentation.

- [x] 5.1 For Mealie, determine: tables, relationships, foreign keys, uniqueness, nullable
      relationships, deletion behavior, quantity representations, audit/history structures,
      problematic schemas later migrated away from — `docs/research/mealie-api-and-database.md`.
- [x] 5.2 For Grocy, determine the same — `docs/research/grocy-api-and-database.md`.
- [x] 5.3 Produce an ER diagram for Mealie's schema — Mermaid `erDiagram` in
      `docs/research/mealie-api-and-database.md`.
- [x] 5.4 Produce an ER diagram for Grocy's schema — Mermaid `erDiagram` in
      `docs/research/grocy-api-and-database.md`.

## 6. Deliverables

- [x] 6.1 Write `docs/research/mealie-*.md` document set covering the areas investigated in
      section 2 — `mealie-recipe-model.md`, `mealie-planning-and-search.md`,
      `mealie-api-and-database.md`.
- [x] 6.2 Write `docs/research/grocy-*.md` document set covering the areas investigated in
      section 3 — `grocy-inventory-and-stock.md`, `grocy-units-and-planning.md`,
      `grocy-api-and-database.md`.
- [x] 6.3 Ensure the Feature Overlap Matrix (section 4) and both ER diagrams (section 5) are
      committed alongside the `docs/research/mealie-*.md` / `docs/research/grocy-*.md` sets.
- [x] 6.4 Cross-check findings against `PLAN.md`'s Phase 3 domain-model candidates — see
      `docs/research/feature-overlap-matrix.md`'s "Cross-check against PLAN.md's Phase 3
      domain-model candidates" section. Findings have also been propagated into the affected
      OpenSpec changes directly (`establish-household-and-catalog`, `implement-pantry-inventory`,
      `implement-recipe-family-and-revisions`, `implement-meals-and-preferences`,
      `implement-recipe-availability`, `implement-recipe-discovery`,
      `establish-enforced-go-architecture`) rather than left only in this document.
