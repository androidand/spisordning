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

**Validated by `establish-reference-lab`'s Mealie findings (2026-08-16):** `docs/research/
mealie-recipe-model.md` confirms Mealie has **zero revision/version history anywhere in its
62-table schema** — every edit overwrites in place, with nothing recording what a recipe looked
like before. This change's entire premise (immutable revisions, variant lineage) is filling a
real, confirmed gap, not solving a problem Mealie already handles adequately. The same
investigation live-reproduced a concrete failure mode of mutate-in-place editing worth citing
directly: a `PATCH` request on a recipe silently dropped nested `unit`/`food` objects (only
`PUT` preserved them), and a separate malformed nested payload committed a partial database
write *before* the API's own response validation failed — leaving the recipe unreadable via
both `GET` and `DELETE` until it was fixed with a direct SQL repair. An architecture built on
mutate-in-place editing produced an unrecoverable-via-API corrupted record; immutable revisions
(this change's model) make that class of failure structurally impossible — a bad edit is a new,
inspectable, discardable revision, never a destructive overwrite of the only copy.

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

## Step 7 — Korvstroganoff worked example (end-to-end validation)

PLAN.md's Visual Recipe Family Requirement is the concrete acceptance target for this hierarchy.
This section walks the Korvstroganoff example end to end through the model above (vocabulary,
relationships, lifecycle, commands, invariants) to validate that `RecipeFamily →
RecipeVariant → RecipeRevision` actually produces that UI and handles the real workflow — not
just the happy path. Task 1.2's counter-example search builds on the edge cases surfaced at the
end of this section.

### The family and its four variants

Korvstroganoff is a Swedish home dish (sliced sausage in a creamy sauce, over rice or mash). A
household accumulates several recognizably-same-dish versions over time:

| Variant | Source | How it enters the system |
|---|---|---|
| **Andreas version** | household (Andreas) | authored in-house; the one actually cooked |
| **ICA version** | ICA Kök (supermarket recipe) | imported from a published source |
| **Köket version** | Köket (recipe site) | imported from a published source |
| **Child-friendly** | household fork of Andreas | forked from Andreas' latest, milder |

All four are recognizably "Korvstroganoff" — same dish, different fork/style/source — so they
are **one** `RecipeFamily` ("Korvstroganoff") holding four `RecipeVariant`s. This is exactly the
conceptual-dish-vs-recognizable-fork split: the family is the dish; each variant is a version a
household points at and names ("the ICA one", "the mild one for the kids").

### The end-to-end command sequence

A realistic sequence of the Step 5 commands, in the order a household would actually perform it:

```text
CreateRecipeFamily("Korvstroganoff")                          → family F1
CreateRecipeVariant(F1, "Andreas version", src=household)     → variant V1
CreateRecipeRevision(V1, content, parents=[])                 → rev A1  Andreas' original
CreateRecipeRevision(V1, content, parents=[A1])               → rev A2  cream amount corrected
CreateRecipeRevision(V1, content, parents=[A2])               → rev A3  sausage swap + mushrooms
SetDefaultVariant(F1, V1)                                     → F1.default_variant_id = V1
CreateRecipeVariant(F1, "ICA version", src="ICA Kök")         → variant V2
CreateRecipeRevision(V2, content, parents=[])                 → rev I1  imported, single revision
CreateRecipeVariant(F1, "Köket version", src="Köket")         → variant V3
CreateRecipeRevision(V3, content, parents=[])                 → rev K1  imported, single revision
ForkVariant(sourceRevisionId=A3, newTitle="Child-friendly")   → variant V4 + rev C1 (parent A3)
```

### Resulting lineage graph

```text
Family: Korvstroganoff   (default_variant_id → V1)
│
├─ V1  Andreas version
│      A1 ──► A2 ──► A3
│
├─ V2  ICA version
│      I1
│
├─ V3  Köket version
│      K1
│
└─ V4  Child-friendly
       C1          (C1's parent edge is A3 → C1: a cross-variant fork)
```

The fork edge `A3 → C1` crosses variant boundaries (A3 ∈ V1, C1 ∈ V4). This is the load-bearing
property of the model: **the DAG is over revisions, not variants.** A `RecipeRevision` belongs to
exactly one variant (invariant 3) yet may have parents in other variants, so "Child-friendly was
forked from Andreas' latest" is a single parent edge — no many-to-many variant table, no
special-casing.

### Tracing the Visual Recipe Family Requirement UI

PLAN.md's target renders as:

```text
Korvstroganoff
★★★★★ Default household variant
       ▼ expand
       ├── Andreas version
       ├── ICA version
       ├── Köket version
       ├── Child-friendly
       └── Find more variants...
```

Each element maps to a concrete query against the proposed schema:

| UI element | Query (against the proposed schema) |
|---|---|
| `Korvstroganoff` (title) | `SELECT name FROM recipe_family WHERE id = F1` |
| `★★★★★ Default household variant` | `SELECT v.title FROM recipe_variant v WHERE v.id = (SELECT default_variant_id FROM recipe_family WHERE id = F1)` → "Andreas version" (the ★ rating is `implement-meals-and-preferences`'s aggregate on that variant, surfaced read-only) |
| `▼ expand` (which variant is expanded) | the same `default_variant_id` — the default variant's latest revision is shown expanded |
| `├── Andreas version` … `├── Child-friendly` (the listed alternates) | `SELECT id, title, source_attribution FROM recipe_variant WHERE family_id = F1 AND id <> (SELECT default_variant_id FROM recipe_family WHERE id = F1) ORDER BY title` |
| `└── Find more variants...` | an affordance, not a row — it triggers `implement-recipe-discovery`'s external sourcing to propose new `RecipeVariant`s for F1 |

The "latest revision" a variant expands to is the tip of its lineage: `WITH RECURSIVE` over
`recipe_revision_parent` to find the revision in that variant with no children (a single
`CreateRecipeRevision` per variant makes this unambiguous; see edge case 2 below).

### What the walk-through validates

- **One family, many variants.** Four recognizably-same-dish versions sit under one family — the
  "conceptual dish" level earns its place.
- **Immutable revision history.** Andreas' variant shows three revisions (original → cream fix →
  sausage swap); each correction is a new row, the prior one untouched (invariant 1).
- **Cross-variant fork.** Child-friendly derives from Andreas' latest via a single parent edge
  that crosses variants — the revision-level DAG handles it with no variant-level join table.
- **Imported variants.** ICA and Köket enter as variants with a single imported revision; if the
  source later republishes, that's a new revision on the same variant (history for sourced
  recipes, not just home ones).
- **Default pin.** `default_variant_id` selects the expanded variant independently of the
  alternates list; it is a stored FK, set by `SetDefaultVariant`, never computed (Step 6
  invariant 5; Decisions below).
- **UI is a projection, not a source of truth.** Every element of the required UI is a read over
  the four tables — no UI-specific denormalization.

### Edge cases surfaced (input to task 1.2 and later tasks)

1. **A variant with zero revisions.** The schema allows `CreateRecipeVariant` before any
   `CreateRecipeRevision`; a variant is a "recognizable fork" only once it has content. Decision
   needed (task 4.1): is a zero-revision variant legal (create-then-populate) or must the first
   revision be atomic with variant creation? The walk-through assumes create-then-populate is
   fine but the UI should not list an empty variant as cookable.
2. **"Latest revision" is ambiguous once a variant forks or merges.** A single linear variant has
   an obvious tip, but after a fork (A3 → C1) or a future merge (two parents) "the current
   revision" needs a rule. The model resolves this by making the variant's current revision the
   latest on its own main line, and using `TagRevision` (Step 5) for explicit labels like "current
   household standard" — not by adding an `is_current` column a merge would have to break.
3. **A recipe that fits two families** (e.g. is the ICA version really "Stroganoff", not
   "Korvstroganoff"?). The model says a variant belongs to exactly one family; resolving this is a
   family-merge curation decision, not a schema change (Step 3). Not a counter-example to the
   hierarchy — a curation rule.
4. **Fork target family.** `ForkVariant` can land the new variant in the same or a different
   family (Step 5). The Korvstroganoff example keeps Child-friendly in the same family; the
   cross-family case is a capability the model supports but the example doesn't exercise.

**Conclusion:** the Korvstroganoff walk-through fits the `RecipeFamily → RecipeVariant →
RecipeRevision` model end to end with no structural counter-example. The four edge cases above
are decisions to be resolved in tasks 1.2/4.1, not rejections of the hierarchy.

## Counter-example analysis — do the three definitions hold? (task 1.2)

Task 1.1 walked the Korvstroganoff example; this section confirms the three definitions actually
hold and deliberately searches for a case that does **not** fit. The definitions under test
(Step 1):

- **RecipeFamily** = a conceptual dish.
- **RecipeVariant** = one recognizable fork/style/source.
- **RecipeRevision** = immutable evolution of that variant.

### Do the definitions hold?

| Definition | Holds because (Korvstroganoff evidence) |
|---|---|
| Family = conceptual dish | "Korvstroganoff" is one dish; the four variants are all recognizably that dish. The family is the thing a household means by "we make Korvstroganoff," independent of whose version. |
| Variant = recognizable fork/style/source | Each variant is independently nameable and cookable: "Andreas'", "ICA's", "Köket's", "the mild one for the kids." A household points at one and rates it — the unit of cooking and (future) rating. |
| Revision = immutable evolution of a variant | Andreas' variant shows three revisions (original → cream fix → sausage swap); each is a new immutable snapshot, the prior untouched. A correction never edits A2 — it adds A3. |

All three hold against the real workflow.

### Candidate counter-examples, stress-tested

A counter-example would be a real case the three tiers **cannot** represent. I constructed the
most likely candidates and evaluated each:

| # | Candidate case | Does it fit? | Why |
|---|---|---|---|
| C1 | A recipe that is arguably two dishes (Beef Lasagna vs. Veggie Lasagna) | **Yes** | One family with two variants, or two families — a **curation judgment**, not a structural gap. The model represents both outcomes. |
| C2 | A one-off tweak that isn't a "recognizable fork" (Andreas tried a different cream once) | **Yes** | Modeled as a **revision** of Andreas' variant, not a new variant. The variant/revision line is drawn by intent (see boundary rule below). |
| C3 | A hybrid combining two dishes (Korvstroganoff × Beef Stroganoff) | **Yes** | A new variant (or family) whose first revision has **two parents** (one per source). Multi-parent revisions are the model's merge mechanism. |
| C4 | Content with no variant yet (a freshly imported, unassigned recipe) | **Yes (out of scope)** | It is a `implement-recipe-discovery` **candidate**, not a `RecipeRevision`. Within the family/variant/revision model every revision has exactly one variant (invariant 3). |
| C5 | A side dish shared by two meals (mash in Korvstroganoff *and* Sunday Roast) | **Yes** | The mash is its **own family** ("Mashed Potatoes"); the *meal* grouping is the meal-planning capability's concern, not a recipe family. The model correctly does not conflate meals with dish-families. |
| C6 | A variant whose source is itself another variant (Andreas adapted ICA's recipe) | **Yes** | Variant `source_attribution` is a label; the actual **lineage** is a revision parent edge (Andreas' first revision → ICA's revision). |

None of these is a structural counter-example: in every case the three tiers represent the
situation, and the only judgment involved is a **curation** decision (which family, variant, or
revision a given real-world thing maps to), not a failure of the model to express it.

### The one boundary that must be pinned down (closest thing to a counter-example)

The single place a *naive* application of the model mis-classifies is the **variant-vs-revision
line**: both are "a version of a recipe," and the same content change can be read either way. The
Korvstroganoff "Child-friendly" case is the concrete test:

> **Counter-example test (variant vs. revision).** Andreas' latest Korvstroganoff (A3) is made
> milder for the kids — less cream, smaller sausage pieces.
>
> - **Naive reading A (it's a revision):** add A4 under variant V1 (Andreas). Then the family
>   view lists only Andreas / ICA / Köket, and "Child-friendly" is buried in Andreas' history —
>   it does **not** appear as its own line, contradicting PLAN.md's required UI, which lists
>   "Child-friendly" as a distinct variant.
> - **Naive reading B (it's a variant):** fork V4 (Child-friendly) from A3. The family view lists
>   all four, matching the required UI.
>
> **Resolving rule (curation, not schema):** a change becomes a **new variant** when the
> household treats it as a *distinct, independently cookable and rateable thing with its own
> name* ("the mild one for the kids"); it stays a **revision** when it is *an update to the same
> named thing* (Andreas tweaks his own recipe). The required UI is the tie-breaker: anything the
> household lists as a separate, selectable version is a variant; anything that is "the same
> recipe, newer" is a revision.

This rule is a **curation policy** the domain layer must apply consistently; it is not a schema
constraint (the schema permits both readings — which is exactly why the policy must be explicit).
It is recorded here as the counter-example test because it is the one case where an *incorrect*
application of the model produces a result that contradicts the required UI.

### Conclusion

**No structural counter-example was found.** The three definitions — Family = conceptual dish,
Variant = recognizable fork/style/source, Revision = immutable evolution of a variant — hold
against the Korvstroganoff workflow and every candidate case stress-tested above. The one genuine
boundary (variant vs. revision, the "Child-friendly" test) is a **curation rule** to be applied by
the domain layer, not a case the model cannot represent.

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

## Git-like semantics on the adjacency model (task 2.2)

PLAN.md asks for fork / history / diff / branch / tag / merge semantics *without* literal Git.
Each maps onto the chosen adjacency edge table (Option A) as follows — **no new table is needed
for any of them**:

| Git-like concept | Mapping onto the adjacency model |
|---|---|
| **fork** | `ForkVariant` (Step 5): a new `RecipeVariant` whose first `RecipeRevision` carries a `RecipeRevisionParent` edge into the source revision. The Korvstroganoff `A3 → C1` edge is a fork. |
| **history** | `WITH RECURSIVE` over `recipe_revision_parent` from a revision up to all its ancestors — the `Ancestors` helper in `internal/recipefamily`. |
| **diff** | A read over two `RecipeRevision` rows (both immutable, fully stored): compare their ingredients/steps. No schema support needed because a revision stores its full content, not a delta. |
| **branch-like variant** | A `RecipeVariant` *is* the branch: a named, long-lived line of revisions. Forking a variant is forking a branch. |
| **tag** | `TagRevision` (Step 5): a label on a revision (e.g. "current household standard"), modeled as a label — not a new table, not a new revision. |
| **merge** | A `RecipeRevision` with **two** `RecipeRevisionParent` edges (two parents). The edge table's composite PK `(revision_id, parent_revision_id)` already allows a child to have many parents. |

The load-bearing point: every one of these is a *read or a single edge insert* over the same four
tables. There is no Git object store, no reflog, no index — the "branch" is a variant row and the
"commit graph" is the edge table, exactly the "git-inspired but not literal Git" line PLAN.md
draws.

## Cycle prevention (task 2.3)

A plain edge table cannot express acyclicity as a constraint (Postgres has no "this FK graph must
be a DAG" DDL). Decision: **enforce it at the application layer, at revision-creation time.**

- The domain layer (`internal/recipefamily.Graph.AddEdge`) checks, before inserting a
  `RecipeRevisionParent` edge `(child → parent)`, whether `child` is already an ancestor of
  `parent` (or `child == parent`). If so, the edge is rejected and no row is written.
- This is a small, pure, unit-tested function (see `internal/recipefamily` tests), not a schema
  constraint. It runs on the same infrequent, human-triggered path as creating a revision, so the
  cost of an ancestor walk is irrelevant.
- The tradeoff (a direct SQL write bypassing the domain could create a cycle) is accepted and
  documented in Risks: this is an internal-only write path, and PLAN.md's "AI SHALL call
  application-layer tools, never unrestricted SQL" principle applies to any future admin tooling.

## Simplicity principle confirmed (task 2.4)

PLAN.md's instruction — "prefer simple immutable revisions since recipes are small" — is the
deciding factor in the Option A recommendation above, and it is reflected in the final choice:

- **Adjacency edge table chosen; closure table and `ltree` rejected** specifically because their
  extra machinery (write-amplified transitive closure; a tree-only path label that cannot
  represent a merge) buys a performance property (O(1) deep-ancestor queries) that a household
  cookbook — a handful to a few dozen revisions per variant — will never need.
- **Revisions are immutable and fully stored** (not deltas), so "diff" and "history" are trivial
  reads; this is the "simple immutable revisions" principle made concrete.
- The escape hatch is explicit and non-breaking: if a variant's revision count ever grows into the
  hundreds and ancestor queries measurably matter, a closure table can be added *alongside* the
  edge table without removing it. No evidence of that scale exists today, so the simpler structure
  stands.

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

## Database Review Questions (task 4.1)

PLAN.md's Database Review Questions, answered for each of the four new tables.

### `recipe_family`

| Question | Answer |
|---|---|
| Domain concept | A conceptual dish ("Korvstroganoff") — the grouping level that holds all recognizably-same-dish variants. |
| Owner | The household (curated). Created by `CreateRecipeFamily`. |
| Mutator | `CreateRecipeFamily`, `SetDefaultVariant`, rename/archive commands. |
| Mutable? | Yes — `name`, `description`, `default_variant_id`, `archived`. |
| Requires history? | No — the *content* history lives on revisions; the family row is a lightweight, mutable pointer. |
| Lifecycle | Created → active → soft-archived (never hard-deleted once it has variants). |
| Deletion behavior | `ON DELETE RESTRICT` from variants (a family with variants cannot be deleted); archiving hides it. |
| Uniqueness | `id` (slug) PK; `name` UNIQUE. |
| External IDs | None (a family is a household concept, not sourced externally). |
| Indexed | PK on `id`; unique on `name`. |
| Real FK? | `default_variant_id → recipe_variant(id)` (added after `recipe_variant` exists, to break the circular reference). |
| JSON used? | No JSON. |

### `recipe_variant`

| Question | Answer |
|---|---|
| Domain concept | One recognizable fork/style/source of a family ("Andreas version", "ICA version") — the unit a household cooks and rates. |
| Owner | The household. Created by `CreateRecipeVariant` / `ForkVariant`. |
| Mutator | `CreateRecipeVariant`, `ForkVariant`, rename/archive commands. |
| Mutable? | Metadata yes (`title`, `source_attribution`, `archived`); its revision list only grows. |
| Requires history? | Its history *is* its revisions (`recipe_revision`); the variant row needs no separate history. |
| Lifecycle | Created → active → soft-archived (archiving hides it without deleting revision history). |
| Deletion behavior | `ON DELETE CASCADE` to its revisions (deleting a variant removes its revision subtree); `ON DELETE RESTRICT` while it is a family's `default_variant_id`. |
| Uniqueness | `id` (slug) PK, globally unique. |
| External IDs | None on the variant row; provenance for imported variants is `source_attribution` (a label) plus, post-promotion, the `recipe_import_candidate.promoted_variant_id` back-reference. |
| Indexed | PK on `id`; index on `family_id` (list a family's variants). |
| Real FK? | `family_id → recipe_family(id)` (one family per variant — invariant 2). |
| JSON used? | No JSON. |

### `recipe_revision`

| Question | Answer |
|---|---|
| Domain concept | One immutable snapshot of a variant's content (ingredients, steps, servings, times) at a point in time. |
| Owner | Its `RecipeVariant`. Created only by `CreateRecipeRevision` (the sole content-entry command). |
| Mutator | None after creation — **immutable** (invariant 1). A correction is a new revision. |
| Mutable? | **No.** No UPDATE path exists for a revision's content. |
| Requires history? | It *is* a history record; the sequence of a variant's revisions is the history. |
| Lifecycle | Created → (superseded by a later revision, but never altered or deleted). |
| Deletion behavior | `ON DELETE CASCADE` from its variant; `ON DELETE RESTRICT` while referenced by a `recipe_revision_parent` edge (a revision with children cannot be deleted — it is someone's parent). |
| Uniqueness | `id` (BIGSERIAL) PK. |
| External IDs | None (content is authored/imported into Spisordning, not referenced by external id). |
| Indexed | PK on `id`; index on `variant_id` (list a variant's revisions / find its tip). |
| Real FK? | `variant_id → recipe_variant(id)` (one variant per revision — invariant 3). |
| JSON used? | Yes — `ingredients` and `steps` are **structured** JSONB (see Revision content model below). This is correct, not a modeling shortcut: a revision's content is always read and written as a whole, so per-line child tables would add write amplification for content never queried line-by-line at household scale. |

### `recipe_revision_parent`

| Question | Answer |
|---|---|
| Domain concept | A lineage edge: "revision `revision_id` was derived from revision `parent_revision_id`" (fork/merge parentage). |
| Owner | Neither endpoint (symmetric relationship, like `IngredientSubstitution`); owned by the lineage graph as a whole. |
| Mutator | Only `CreateRecipeRevision` (which inserts the new revision's parent edges atomically with the revision). |
| Mutable? | **No** — append-only; an edge is never updated or deleted (deleting one would corrupt history). |
| Requires history? | It *is* the history structure (the DAG). |
| Lifecycle | Created with its child revision → permanent. |
| Deletion behavior | Child-side (`revision_id`) `ON DELETE CASCADE` — deleting a revision drops its own parent-edges. Parent-side (`parent_revision_id`) `ON DELETE RESTRICT` — a revision that is someone's parent cannot be deleted, which also protects cross-variant fork edges (e.g. `A3 → C1`). |
| Uniqueness | Composite PK `(revision_id, parent_revision_id)` — a child may list a given parent at most once, but may have many parents (merge). |
| External IDs | None. |
| Indexed | PK `(revision_id, parent_revision_id)`; secondary index on `parent_revision_id` to support "who derived from this revision" (descendant) walks. |
| Real FK? | Both `revision_id` and `parent_revision_id` are real FKs to `recipe_revision(id)`. Acyclicity is enforced in the application layer (Cycle prevention above), not the schema. |
| JSON used? | No JSON. |

## Revision content model (task 4.2)

Decision: **`RecipeRevision` is the authoritative content store** for recipes migrated into the
hierarchy, holding **structured** ingredients and steps — superseding `recipe_ref`'s opaque
`raw_snapshot` model going forward.

- **Structured, not an opaque blob.** A revision stores `ingredients` as a JSONB array of
  structured lines — each `{ingredient_id, quantity, unit, raw_text}` — and `steps` as an ordered
  JSONB array of strings. `ingredient_id` references the canonical `ingredient.id` (validated at
  the application layer when a revision is created). This is the key improvement over
  `recipe_ref.raw_snapshot`: each ingredient line is individually addressable and carries a
  canonical ingredient reference, so a revision can be turned into shopping requirements without
  re-parsing free text.
- **Why JSONB and not child tables.** A revision's content is always read and written as a whole
  (you render or correct the entire recipe, never a single ingredient row in isolation), and it is
  immutable. Storing it as structured JSONB keeps the hierarchy to the four core tables, avoids
  the write amplification of a `recipe_revision_ingredient` child table, and matches how an
  immutable snapshot is actually used. A relational child table with a hard FK is a documented
  future refinement if line-level ingredient queries ever become a real need — not warranted at
  household scale today (consistent with the simplicity principle, task 2.4).
- **Supersedes `recipe_ref` for migrated recipes; leaves it untouched otherwise.** Once a recipe
  is promoted into a `RecipeVariant`/`RecipeRevision`, the revision is the source of truth for its
  content. `recipe_ref` (and its `recipe_ingredient`/`raw_snapshot`) remains exactly as-is for any
  recipe not yet migrated — this change does not force a big-bang migration (proposal.md scope
  boundary). The two models coexist until the follow-up data migration lands.

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
