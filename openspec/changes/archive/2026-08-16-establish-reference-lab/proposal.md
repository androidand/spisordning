# Establish reference lab

## Why

`PLAN.md`'s "First Principle" is explicit: do not mechanically port Mealie or Grocy — observe
mature implementation, understand behavior, understand data model, understand edge cases,
extract domain knowledge, *then* design Spisordning's own concept. Its Phase 1 ("Build the
Reference Laboratory") and the dedicated "Mealie Investigation" / "Grocy Investigation" sections
list, in detail, what must be studied before Phase 3 ("Design Spisordning's Domain Model") can
begin credibly. `docs/research/current-state.md` confirms this work has not happened yet:
Spisordning has a real, tested read-only Mealie sync client (`internal/mealie/client.go`), but
it only covers what the first vertical slice needed (recipe ids, ingredient lines, tags,
effort-time parsing) — not the deep behavioral/API/DB investigation `PLAN.md` calls for. Grocy
has zero code, config, or schema anywhere in this repo or its siblings; it is a pure research
target.

This change deploys nothing and writes no application code — it is the research vehicle for
`PLAN.md`'s "observe mature implementation" step, without which Phase 3's domain-model work
(RecipeFamily/Variant/Revision, Ingredient vs. Product, the inventory ledger, unit system, etc.)
would be guesswork rather than evidence-based.

This change **depends on Mealie and Grocy actually being deployed** to Proxmox through Tengil
with isolated persistence, per `PLAN.md`'s Phase 1. That deployment is happening in a parallel
effort (Epic H, in the `tengil` repo) and is out of scope here — this change assumes it exists
by the time its investigation tasks run, and records what it needs from that deployment (version,
image, license, deployment config, database/storage) as its own first task.

## What Changes

- Confirm and record the reference-lab deployment: Mealie and Grocy versions, commit/tag if
  applicable, container images, licenses, deployment config, and database/storage, per
  `PLAN.md`'s Phase 1.
- Create comparable representative data in both systems and exercise their important workflows,
  so the investigation below is grounded in observed behavior, not documentation alone.
- Investigate Mealie across the ~20 areas `PLAN.md`'s "Mealie Investigation" section lists,
  capturing for each: user behavior, API behavior, DB mutation, source implementation, tests,
  strengths, weaknesses, and the Spisordning lesson.
- Investigate Grocy across the ~20 areas `PLAN.md`'s "Grocy Investigation" section lists, with
  particular attention to edge cases accumulated through years of inventory-tracking use.
- Produce a Feature Overlap Matrix: for every capability both systems address, decide
  MEALIE / GROCY / MERGE / REDESIGN / DEFER / OMIT with a concrete stated reason — never "take
  the best parts."
- Perform Phase 2 "Database Archaeology": treat both systems' migrations as historical
  architecture documentation; determine tables, relationships, foreign keys, uniqueness
  constraints, nullable relationships, deletion behavior, quantity representations,
  audit/history structures, and schemas later migrated away from (and why). Produce ER diagrams
  for both.
- Produce the `docs/research/mealie-*.md` and `docs/research/grocy-*.md` document sets `PLAN.md`
  expects, plus ER diagrams, as this change's concrete deliverables.

## Capabilities

### New Capabilities

- `reference-lab-findings`: a meta-capability covering the project's obligation to document
  Mealie's and Grocy's real data models and behavior before Spisordning's own equivalent tables
  are designed. This is a documentation/process capability, not an application feature —
  Spisordning ships no new user-facing or API behavior as part of this change.

### Modified Capabilities

<!-- none — this is a pure investigation change; it produces docs, not code -->

## Impact

- New: `docs/research/mealie-*.md` (recipe model, editing, import/parsing, structured
  ingredients, foods/units/servings/scaling, images, tags/categories/cookbooks, search, meal
  plans, shopping, households/users, ratings, API, database, migrations, tests, provenance).
- New: `docs/research/grocy-*.md` (products, barcodes, locations, stock/stock journal, lots,
  expiry, purchase/consume/discard/transfer/adjust/mark-empty, units and conversions, shopping,
  recipes, meal planning, cost tracking, API, database, migrations, tests).
- New: a Feature Overlap Matrix document and ER diagrams for both systems (feeding directly into
  `PLAN.md`'s Phase 2 "Expected Database Documents": `docs/database/conceptual-er-model.md` and
  siblings, which are out of scope for this change but consume its output).
- No changes to `food-brain`'s Go code, schema, or runtime behavior.
- Depends on the parallel Epic H Tengil deployment of Mealie and Grocy; does not perform that
  deployment itself.
- Directly feeds `PLAN.md`'s Phase 3 ("Design Spisordning's Domain Model") and the later
  `establish-household-and-catalog` / `implement-recipe-family-and-revisions` /
  `implement-pantry-inventory` OpenSpec changes, none of which should proceed credibly without
  this change's findings.
