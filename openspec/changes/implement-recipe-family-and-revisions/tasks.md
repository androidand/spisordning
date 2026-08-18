## 1. Recipe hierarchy validation

- [x] 1.1 Validate `RecipeFamily → RecipeVariant → RecipeRevision` against real recipe
      workflows per PLAN.md's instruction ("validate this model against real recipe
      workflows") — walk through the Korvstroganoff example (Andreas version, ICA version,
      Köket version, Child-friendly) end to end.
      *Verified:* design.md Step 7 walks the full example end to end — one family / four
      variants / Andreas' 3-revision history / cross-variant fork (A3→C1) / default pin all map
      cleanly onto the model, and every Visual Recipe Family UI element traces to a concrete
      query. Four edge cases surfaced for 1.2/4.1; no structural counter-example.
- [x] 1.2 Confirm a `RecipeFamily` represents a conceptual dish, a `RecipeVariant` represents
      one recognizable fork/style/source, and a `RecipeRevision` represents immutable evolution
      of that variant — write down a counter-example test (a case that doesn't fit) if one is
      found, or explicitly confirm none was found.
      *Verified:* design.md "Counter-example analysis" confirms all three definitions hold against
      the Korvstroganoff workflow, stress-tests 6 candidate counter-examples (C1–C6, all fit), and
      explicitly confirms **no structural counter-example was found**. The one genuine boundary
      (variant-vs-revision, "Child-friendly") is written down as a counter-example test with its
      resolving curation rule.

## 2. Git-like recipe design / DAG representation

- [x] 2.1 Investigate PostgreSQL DAG representation options for `recipe_revision_parents`:
      adjacency edge table, closure table, `ltree` — captured with pros/cons/recommendation in
      `design.md`.
      *Verified:* design.md "Decisions — PostgreSQL DAG representation" compares all three
      (adjacency / closure / ltree) with pros and cons and recommends the adjacency edge table
      (Option A) — simplest, real FKs, trivially supports fork + merge.
- [x] 2.2 Support future semantics without implementing literal Git: fork, history, diff,
      branch-like variant, tag, merge — confirm each maps onto the chosen adjacency-table model
      (e.g. fork = new variant + parent edge; tag = a label on a revision, not a new table).
      *Verified:* design.md "Git-like semantics on the adjacency model (task 2.2)" maps each of
      fork / history / diff / branch / tag / merge onto the adjacency model (fork = new variant +
      parent edge; merge = a revision with two parents; tag = a label, not a table).
- [x] 2.3 Decide and document cycle-prevention approach (schema cannot enforce DAG acyclicity
      with a plain edge table) — application-layer check on revision creation.
      *Verified:* design.md "Cycle prevention (task 2.3)" decides on an application-layer check at
      edge-insert time; `internal/recipefamily.Graph.AddEdge` rejects a self-edge and any edge
      where the child is already an ancestor of the parent. `TestCycleRejection` passes.
- [x] 2.4 Confirm the "prefer simple immutable revisions since recipes are small" principle is
      reflected in the final choice — do not adopt closure table or `ltree` complexity without
      evidence recipe revision volume warrants it.
      *Verified:* design.md "Simplicity principle confirmed (task 2.4)" confirms the adjacency
      table is the final choice and explicitly rejects closure/ltree as un-warranted complexity at
      recipe scale (a handful to a few dozen revisions per variant).

## 3. Visual Recipe Family Requirement

- [x] 3.1 Confirm the domain model makes the Korvstroganoff UI straightforward: default
      variant expanded, alternates listed collapsed, "Find more variants..." affordance —
      trace each UI element to a concrete query against the proposed schema.
      *Verified:* design.md Step 7 "Tracing the Visual Recipe Family Requirement UI" maps every
      element (title, default expanded, alternates list, "Find more variants...") to a concrete
      query; `BuildFamilyView` + `TestKorvstroganoffWorkedExample` reproduce the projection in Go.
- [x] 3.2 Determine whether `default_variant_id` should be stored, manually pinned, computed
      from ratings, or computed with optional override — decision and rationale captured in
      `design.md`.
      *Verified:* design.md "Decisions — `default_variant_id`" decides: stored FK, manually pinned
      by an explicit command, never computed from ratings — with rationale (a computed default
      would silently change what a household cooks).
- [x] 3.3 If a computed "suggested default" is included, confirm it never silently overwrites
      the stored/pinned `default_variant_id` (consistent with `implement-meals-and-preferences`'s
      favorite/rating independence invariant).
      *Verified:* design.md Decisions confirm a computed "suggested default" is surfaced read-only
      and never auto-applied; the spec scenario "A computed suggestion does not overwrite the
      stored default" pins this (consistent with the rating/favorite independence invariant).

## 4. Persistence

- [x] 4.1 For `recipe_family`, `recipe_variant`, `recipe_revision`, `recipe_revision_parent`,
      answer PLAN.md's Database Review Questions: domain concept, owner, mutator, mutability,
      history requirement, lifecycle, deletion behavior, uniqueness constraints, external ids,
      indexing, FK-ability.
      *Verified:* design.md "Database Review Questions (task 4.1)" answers all ten questions for
      each of the four tables (concept, owner, mutator, mutability, history, lifecycle, deletion,
      uniqueness, external ids, indexing, FK-ability).
- [x] 4.2 Decide what `RecipeRevision` content actually holds: structured ingredients (FK to
      `ingredient`/future `ingredient_form`) and steps, vs. continuing to defer to a Mealie
      snapshot — confirm this supersedes `recipe_ref`'s snapshot-only model going forward while
      leaving un-migrated `recipe_ref` rows untouched.
      *Verified:* design.md "Revision content model (task 4.2)" decides structured JSONB
      (ingredients array of {ingredient_id, quantity, unit, raw_text} + ordered steps); it
      supersedes `recipe_ref`'s snapshot-only model for migrated recipes while leaving un-migrated
      `recipe_ref` rows untouched.
- [x] 4.3 Write the additive migration for the four new tables plus indexes needed for ancestor
      queries (`WITH RECURSIVE` over `recipe_revision_parent`).
      *Verified:* `migrations/0003_recipe_family.sql` creates the four tables additively (does not
      touch `recipe_ref`), with indexes on `recipe_variant.family_id`,
      `recipe_revision.variant_id`, and `recipe_revision_parent.parent_revision_id` for the
      recursive ancestor walk; the circular `default_variant_id` FK is added via `ALTER TABLE`.
- [x] 4.4 Add Go domain types and an ancestor/history query helper in `internal/domain` (or a
      dedicated `internal/recipefamily` package), with unit tests for cycle rejection and
      multi-parent (merge) lineage.
      *Verified:* new `internal/recipefamily` package adds the `Family`/`Variant`/`Ingredient`/
      `Revision` types, a `Graph` with `Ancestors` (the in-memory `WITH RECURSIVE`), and
      `BuildFamilyView`. `TestCycleRejection` and `TestMultiParentMergeLineage` pass; `go build`
      and `go vet` are clean.

## 5. Verification

- [x] 5.1 `openspec validate implement-recipe-family-and-revisions` passes.
      *Verified:* `openspec validate implement-recipe-family-and-revisions` → "Change
      'implement-recipe-family-and-revisions' is valid".
- [x] 5.2 Unit tests: revision immutability (no update path exists), variant-belongs-to-one-family,
      revision-belongs-to-one-variant, cycle rejection on parent-edge creation.
      *Verified:* `internal/recipefamily` tests — `TestRevisionHasNoUpdatePath` (no exported
      mutation method), `TestVariantBelongsToOneFamily`, `TestRevisionBelongsToOneVariant`,
      `TestCycleRejection` — all pass (`go test ./internal/recipefamily/` → 8 passed).
- [x] 5.3 A worked example (in tests or docs) reproducing the Korvstroganoff family/variant/
      revision tree and the query that renders the Visual Recipe Family Requirement UI.
      *Verified:* `TestKorvstroganoffWorkedExample` reproduces the full tree (one family / four
      variants / A1→A2→A3 / cross-variant fork A3→C1) and asserts the `BuildFamilyView` UI
      projection; design.md Step 7 holds the equivalent SQL for each UI element.
