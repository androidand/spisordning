## Why

`recipe_ref` in `migrations/0001_init.sql` is a flat pointer to a Mealie recipe id with a
cached snapshot — it has no concept of "this household has three versions of Korvstroganoff"
or "this is a corrected revision of last month's version." PLAN.md's Epic C vision is a
git-like hierarchy — `RecipeFamily → RecipeVariant → RecipeRevision` — explicitly without
implementing literal Git, because household recipes are small in number and revision depth.
This is the part of PLAN.md flagged as needing real investigation before committing to a
schema (PostgreSQL DAG representation for revision parentage), so `design.md` researches
adjacency-table, closure-table, and `ltree` options with a recommendation before any table is
proposed here.

## What Changes

- Introduce `RecipeFamily` (a conceptual dish), `RecipeVariant` (one recognizable
  fork/style/source), and `RecipeRevision` (an immutable snapshot of a variant's content) as
  new tables, validated against the Korvstroganoff example from PLAN.md's Visual Recipe Family
  Requirement (default variant expanded, alternates listed, "Find more variants...").
- Introduce `RecipeRevisionParent` as an adjacency edge table (see `design.md` for the DAG
  representation research and recommendation) to support fork/history/diff/branch/tag/merge
  semantics without literal Git.
- Introduce `default_variant_id` on `RecipeFamily` as a manually-pinned, stored FK — with a
  read-only computed "suggested default" surfaced separately, never auto-applied (see
  `design.md` Decisions).
- Leave `recipe_ref` (Mealie-referenced recipes) untouched for any recipe not yet migrated into
  the family/variant/revision structure; this change does not force a big-bang migration of
  existing synced recipes.

## Capabilities

### New Capabilities

- `recipe-family`: the RecipeFamily/RecipeVariant/RecipeRevision hierarchy, revision
  parentage/lineage, and the default-variant selection model.

### Modified Capabilities

<!-- none — recipe-family is additive alongside recipe_ref; the meal-planning capability's
     consumption of recipe_ref is unaffected by this change -->

## Impact

- Affected code: `migrations/` (new tables, additive), `internal/domain` (new
  RecipeFamily/RecipeVariant/RecipeRevision/RecipeRevisionParent types and ancestor-query
  helpers).
- Depends conceptually on `implement-meals-and-preferences`'s rating aggregation for the
  "suggested default variant" read-side feature, but does not require it to land first — the
  stored `default_variant_id` and manual pin work standalone.
- Out of scope: migrating existing `recipe_ref` rows into the new hierarchy (a follow-up data
  migration once this schema and a review UI exist); recipe web import / external recipe
  sourcing (`implement-recipe-discovery`'s concern).
