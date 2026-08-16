## Context

Today's `recipe_ref` table is a flat reference to a Mealie recipe id plus a cached snapshot —
there is no concept of variants ("Andreas' version" vs. "ICA's version" of the same dish) or
of revision history within a variant. PLAN.md proposes `RecipeFamily → RecipeVariant →
RecipeRevision` as the replacement hierarchy, git-inspired but explicitly **not** literal Git:
"prefer simple immutable revisions since recipes are small." This design.md validates that
hierarchy against a real workflow (the Korvstroganoff example), and — the part PLAN.md flags
as needing investigation rather than a snap decision — researches how to represent the
revision parentage DAG in PostgreSQL.

This change does not decide whether recipe content itself keeps living in Mealie
(`recipe_ref`'s current model) or moves into Spisordning's own `recipe_revision` rows. That is
a prerequisite question answered in Step 2 below, because it changes what a "revision" even
contains.

## Step 1 — Vocabulary

| Term | Definition |
|---|---|
| **RecipeFamily** | A conceptual dish ("Korvstroganoff"). Groups all variants that are recognizably the same dish. |
| **RecipeVariant** | One recognizable fork/style/source of a family ("Andreas version", "ICA version", "Child-friendly"). The unit a household actually cooks and rates. |
| **RecipeRevision** | One immutable snapshot of a variant's ingredients/steps/metadata at a point in time. A variant's history is its sequence of revisions. |
| **RecipeRevisionParent** | An edge recording that one revision was derived from one or more prior revisions (fork/merge lineage). |
| **default_variant_id** | Which variant of a family is shown expanded by default in the family view. |

## Step 2 — Aggregates

- **RecipeFamily aggregate** (root: `RecipeFamily`). Owns the *set membership* of variants
  (which variants belong to this family) and the `default_variant_id` pointer, but does not
  own variant content.
- **RecipeVariant aggregate** (root: `RecipeVariant`). Owns its `RecipeRevision` history. A
  variant is the meaningful unit for rating/favoriting (this is the recipe identity
  `implement-meals-and-preferences`'s `Favorite`/`MealReview` should eventually reference,
  once this change lands — noted as follow-up there, not solved here).
- **RecipeRevision** is content-immutable once created; it is owned by its `RecipeVariant` but
  is itself the aggregate root for lineage purposes — `RecipeRevisionParent` edges connect two
  `RecipeRevision` aggregates and are best modeled as their own thin relationship, not owned by
  either endpoint (symmetric ownership problem, same shape as `IngredientSubstitution` in
  `establish-household-and-catalog`).

## Step 3 — Conceptual relationships

```text
RecipeFamily (1) ──── (N) RecipeVariant
     │  default_variant_id (nullable FK → RecipeVariant)
     │
     ▼
RecipeVariant (1) ──── (N) RecipeRevision
                              │
                              │ (N) RecipeRevisionParent (N)
                              ▼
                        RecipeRevision (earlier)
```

- A `RecipeVariant` always belongs to exactly one `RecipeFamily` (no shared variants across
  families — if the same recipe fits two families, that's a sign the families should merge, a
  curation decision, not a many-to-many schema).
- A `RecipeRevision` belongs to exactly one `RecipeVariant` (revisions do not move between
  variants; forking to a new variant creates a new `RecipeVariant` whose first revision has a
  `RecipeRevisionParent` edge into the source variant's revision — this is how "fork" works).
- `RecipeRevisionParent` supports multiple parents per revision (a future merge), and a
  revision may have zero parents (the variant's first revision).

## Step 4 — Lifecycle

| Entity | Lifecycle |
|---|---|
| `RecipeFamily` | Mutable (name, `default_variant_id`); soft-archived, not deleted once it has variants. |
| `RecipeVariant` | Mutable metadata (title, source attribution); its revision list only grows. Archiving a variant hides it without deleting revision history. |
| `RecipeRevision` | **Immutable** once created — content (ingredients, steps, servings) never changes in place. A correction is a new revision with a `RecipeRevisionParent` edge to the one it corrects. |
| `RecipeRevisionParent` | Append-only edge; never deleted (would corrupt history), though a revision can be superseded. |

## Step 5 — Commands

- `CreateRecipeFamily(name)`
- `CreateRecipeVariant(familyId, title, sourceAttribution?)`
- `CreateRecipeRevision(variantId, content, parentRevisionIds[])` — the only way content ever
  enters the system; there is no `UpdateRecipeRevision`.
- `ForkVariant(sourceRevisionId, newTitle)` → creates a new `RecipeVariant` (in the same or a
  different family) whose first `RecipeRevision` has `sourceRevisionId` as parent.
- `SetDefaultVariant(familyId, variantId)` (manual pin) — see Step 6/Decisions for whether this
  coexists with a computed default.
- `TagRevision(revisionId, label)` — optional, supports "tag" semantics (e.g. "current
  household standard") without a new table if modeled as a label on a revision pointer.

## Step 6 — Invariants

1. **A RecipeRevision is immutable once created.** No update path may change a revision's
   stored content; corrections create a new revision.
2. **A RecipeVariant belongs to exactly one RecipeFamily.**
3. **A RecipeRevision belongs to exactly one RecipeVariant**, regardless of how many parent
   revisions (possibly in other variants) it has.
4. **Revision parentage never cycles.** A `RecipeRevisionParent` edge SHALL NOT create a cycle
   (enforced at the application layer since a plain FK table cannot express acyclicity; see
   Decisions below).
5. **`default_variant_id` is nullable and always resolves within its own family** — a family's
   default variant, if set, SHALL belong to that family.

## Decisions — PostgreSQL DAG representation

Three realistic options for `recipe_revision_parents`-style lineage in Postgres:

### Option A: Adjacency edge table (PLAN.md's candidate)

```sql
CREATE TABLE recipe_revision_parent (
    revision_id        BIGINT REFERENCES recipe_revision(id),
    parent_revision_id BIGINT REFERENCES recipe_revision(id),
    PRIMARY KEY (revision_id, parent_revision_id)
);
```
- **Pros**: simplest, real FKs, trivially supports multiple parents (merge) and multiple
  children (fork), cheap writes, matches how Git itself stores commit parents.
- **Cons**: "give me the full history of this revision" or "is A an ancestor of B" needs a
  recursive CTE (`WITH RECURSIVE`) — fine at recipe scale (a handful to a few dozen revisions
  per variant) but not O(1).
- **Cycle prevention**: not enforced by the schema; must be checked at write time (walk
  ancestors of the proposed parent, reject if it includes the child) — acceptable given writes
  are infrequent and human-triggered (creating a revision), not a hot path.

### Option B: Closure table

Store every ancestor→descendant pair (transitive closure), not just direct edges.
- **Pros**: O(1) ancestor/descendant queries, no recursive CTE needed for "show full history."
- **Cons**: write amplification — every new revision requires inserting a row for every
  ancestor, not just the direct parent; more complex to keep consistent on multi-parent merges;
  meaningful overhead for a benefit (fast deep-ancestor queries) this domain doesn't need.

### Option C: `ltree` (ancestry path as label path)

Postgres's `ltree` extension encodes a path like `1.4.9` for fast subtree queries.
- **Pros**: excellent for pure trees with fast subtree/depth queries via GiST/GIN index.
- **Cons**: `ltree` is fundamentally a tree structure — a genuine DAG (a revision with two
  parents, i.e. a merge) does not fit one label path per node without workarounds (multiple
  paths per node, defeating the simplicity). Requires enabling a Postgres extension for a
  domain that, per PLAN.md, explicitly wants to avoid over-engineering ("recipes are small").

### Recommendation

**Option A (adjacency edge table)**, matching PLAN.md's original candidate. Rationale:
recipe revision graphs are shallow and narrow (a variant accumulates revisions over months to
years, not the commit volume of a software repo); `WITH RECURSIVE` over a few dozen rows is
not a performance concern; it keeps real FK integrity (PLAN.md's stated preference over
polymorphic/denormalized shortcuts); and it's the smallest schema that still supports fork,
merge, and history queries. Revisit only if a variant's revision count grows into the hundreds
and ancestor queries measurably matter — not expected for a household cookbook.

Cycle prevention and "compute full history" both become small, well-tested application-layer
functions (`internal/domain` or a query package) rather than schema-enforced constraints —
consistent with accepting a documented tradeoff rather than reaching for a heavier structure
PLAN.md is explicitly wary of ("Do not implement literal Git unless compelling evidence
appears").

## Decisions — `default_variant_id`

PLAN.md poses four options: stored, manually pinned, computed from ratings, or computed with
override. Recommendation: **stored, manually pinned, with an optional computed suggestion
surfaced but not auto-applied.** Rationale: the Visual Recipe Family Requirement UI
("★★★★★ Default household variant") implies a household actively curates which version is
"the" one to cook — that's a deliberate, low-frequency decision, not something that should
silently flip when someone leaves a bad review of the currently-pinned variant.
`default_variant_id` is therefore a plain nullable FK on `RecipeFamily`, set by
`SetDefaultVariant`; a *separate*, read-only computation ("highest-rated variant" from
`implement-meals-and-preferences`'s aggregate) can be surfaced in the UI as a suggestion
("ICA version now rates higher — set as default?") without ever writing to
`default_variant_id` automatically. This keeps favorite/rating independence (established in
`implement-meals-and-preferences`) consistent here too: a computed signal informs, never
silently overrides, an explicit household choice.

## Risks / Trade-offs

- Recursive CTE ancestor queries are the main scaling risk of Option A; mitigated by the
  expected shallow/narrow shape of real recipe histories and revisitable later without a
  breaking schema change (a closure table could be added alongside the edge table if ever
  needed, without removing it).
- Cycle prevention living in application code (not the schema) means a direct SQL write
  bypassing the domain layer could corrupt lineage; mitigated by this being an internal-only
  write path (no public SQL access; PLAN.md's "AI SHALL call application-layer tools, never
  unrestricted SQL" principle applies equally to any future admin tooling).
- Deferring the question of whether revision content lives in Spisordning or stays
  Mealie-referenced: this change assumes `RecipeRevision` becomes the authoritative content
  store (structured ingredients + steps), superseding `recipe_ref`'s snapshot-only model for
  recipes migrated into the family/variant/revision structure — `recipe_ref` remains as-is for
  any recipe not yet migrated. This is a deliberate scope boundary, not an oversight.
