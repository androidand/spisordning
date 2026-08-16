## 1. Recipe hierarchy validation

- [ ] 1.1 Validate `RecipeFamily → RecipeVariant → RecipeRevision` against real recipe
      workflows per PLAN.md's instruction ("validate this model against real recipe
      workflows") — walk through the Korvstroganoff example (Andreas version, ICA version,
      Köket version, Child-friendly) end to end.
- [ ] 1.2 Confirm a `RecipeFamily` represents a conceptual dish, a `RecipeVariant` represents
      one recognizable fork/style/source, and a `RecipeRevision` represents immutable evolution
      of that variant — write down a counter-example test (a case that doesn't fit) if one is
      found, or explicitly confirm none was found.

## 2. Git-like recipe design / DAG representation

- [ ] 2.1 Investigate PostgreSQL DAG representation options for `recipe_revision_parents`:
      adjacency edge table, closure table, `ltree` — captured with pros/cons/recommendation in
      `design.md`.
- [ ] 2.2 Support future semantics without implementing literal Git: fork, history, diff,
      branch-like variant, tag, merge — confirm each maps onto the chosen adjacency-table model
      (e.g. fork = new variant + parent edge; tag = a label on a revision, not a new table).
- [ ] 2.3 Decide and document cycle-prevention approach (schema cannot enforce DAG acyclicity
      with a plain edge table) — application-layer check on revision creation.
- [ ] 2.4 Confirm the "prefer simple immutable revisions since recipes are small" principle is
      reflected in the final choice — do not adopt closure table or `ltree` complexity without
      evidence recipe revision volume warrants it.

## 3. Visual Recipe Family Requirement

- [ ] 3.1 Confirm the domain model makes the Korvstroganoff UI straightforward: default
      variant expanded, alternates listed collapsed, "Find more variants..." affordance —
      trace each UI element to a concrete query against the proposed schema.
- [ ] 3.2 Determine whether `default_variant_id` should be stored, manually pinned, computed
      from ratings, or computed with optional override — decision and rationale captured in
      `design.md`.
- [ ] 3.3 If a computed "suggested default" is included, confirm it never silently overwrites
      the stored/pinned `default_variant_id` (consistent with `implement-meals-and-preferences`'s
      favorite/rating independence invariant).

## 4. Persistence

- [ ] 4.1 For `recipe_family`, `recipe_variant`, `recipe_revision`, `recipe_revision_parent`,
      answer PLAN.md's Database Review Questions: domain concept, owner, mutator, mutability,
      history requirement, lifecycle, deletion behavior, uniqueness constraints, external ids,
      indexing, FK-ability.
- [ ] 4.2 Decide what `RecipeRevision` content actually holds: structured ingredients (FK to
      `ingredient`/future `ingredient_form`) and steps, vs. continuing to defer to a Mealie
      snapshot — confirm this supersedes `recipe_ref`'s snapshot-only model going forward while
      leaving un-migrated `recipe_ref` rows untouched.
- [ ] 4.3 Write the additive migration for the four new tables plus indexes needed for ancestor
      queries (`WITH RECURSIVE` over `recipe_revision_parent`).
- [ ] 4.4 Add Go domain types and an ancestor/history query helper in `internal/domain` (or a
      dedicated `internal/recipefamily` package), with unit tests for cycle rejection and
      multi-parent (merge) lineage.

## 5. Verification

- [ ] 5.1 `openspec validate implement-recipe-family-and-revisions` passes.
- [ ] 5.2 Unit tests: revision immutability (no update path exists), variant-belongs-to-one-family,
      revision-belongs-to-one-variant, cycle rejection on parent-edge creation.
- [ ] 5.3 A worked example (in tests or docs) reproducing the Korvstroganoff family/variant/
      revision tree and the query that renders the Visual Recipe Family Requirement UI.
