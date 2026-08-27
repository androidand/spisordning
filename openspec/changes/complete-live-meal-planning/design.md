## Context

Two independent gaps, bundled because both close out "planning" before shopping/pricing work
starts:

1. `internal/planning/week.go`'s `WeekConfig` already exposes `EnergyFor(date) domain.Effort` and
   `SchoolTagsFor(date) []string`, and `internal/scoring/scoring.go` already scores both. The CLI
   path (`cmd/food-brain/plan.go`) populates them from `effort_profile` (per-weekday energy) and
   `internal/skolmaten`. The MCP path (`cmd/mcp-server/adapters.go`) does not — `PlanDinners` calls
   `planning.PlanWeek` with only `Candidates`, `People`, `Preferences` set. This is missing wiring,
   not missing capability.
2. `meal_plan_candidate` and `meal_plan_decision` are both keyed `(plan_id, slot_date)` — schema
   assumes exactly one decision per date. `internal/domain.Candidate`, `internal/scoring`, and
   `internal/mcptools` are dinner-only in naming and shape (`PlannedDinner`, `list_recipe_candidates`
   description says "dinner candidate"). Mealie recipes have no dedicated meal-type field — only
   free-text tags (`internal/mealie/client.go` lowercases `Tags []string` from Mealie's tag list).

## Goals / Non-Goals

**Goals:**
- `list_recipe_candidates` over MCP produces the same school-lunch-aware, energy-aware plan the CLI
  already produces.
- A week plan covers 7 dinners + 7 breakfasts + a small snack rotation for 1 adult + 2 kids, sourced
  from Mealie recipes tagged for the slot (e.g. a `frukost`/`breakfast` tag) plus a simple built-in
  fallback list for staple non-recipe items (yogurt, fruit, sandwiches) so breakfast/snacks don't
  block on every household having Mealie recipes tagged for them.
- Preference learning (`internal/ambient.RecordReaction`) works unchanged for breakfast/snack
  reactions — it already operates on tags, not a dinner-specific concept.

**Non-Goals:**
- No new energy-input UI/API beyond what already exists (`effort_profile`, per-weekday). A
  per-date one-off override ("today specifically I'm exhausted") is not in scope — flagged as an
  Open Question below, since `planning_constraint` could carry it later without a schema fight now.
- No change to dinner scoring sophistication (repetition penalty, campaign bias, etc.) for
  breakfast/snacks — those slots get a deliberately simpler rule set (see Decisions).
- No shopping/pricing/retailer/notes/deployment work — separate changes.

## Decisions

**D1 — Widen the plan schema's key from `(plan_id, slot_date)` to `(plan_id, slot_date,
slot_kind)`.** `slot_kind` is a `TEXT NOT NULL DEFAULT 'dinner' CHECK (slot_kind IN ('dinner',
'breakfast', 'snack'))` column added to `meal_plan_candidate` and `meal_plan_decision`, with the
primary key on `meal_plan_decision` becoming `(plan_id, slot_date, slot_kind)`. Default `'dinner'`
means every existing row and every existing query that doesn't know about slots keeps working
unchanged — this is additive, not a breaking migration. Alternative considered: a separate
`meal_plan_candidate_breakfast` / `..._snack` table pair — rejected, since it would duplicate the
scorer's I/O shape for no benefit and double the persistence surface to maintain.

**D2 — Source breakfast/snack candidates from Mealie tags, with a small built-in fallback list for
snacks.** Recipes tagged `frukost`/`breakfast` become breakfast candidates; recipes tagged
`mellanmål`/`snack` become snack candidates, using the same lowercased-tag mechanism
`internal/mealie/client.go` already applies for dinner tags. Because a household may have zero
Mealie recipes tagged for snacks, snacks additionally draw from a small hardcoded Swedish
staple list (yogurt + fruit, knäckebröd, morotsstavar, etc.) shaped as `domain.Candidate` with no
`MealieRecipeID`. **Persistence decision (task 2.1/2.2):** `internal/planning/staples.go` is about
shopping-list staples (ingredients assumed in the pantry), not meal candidates — it has no
persistence path. `meal_plan_candidate` has a hard `mealie_recipe_id TEXT NOT NULL REFERENCES
recipe_ref(mealie_recipe_id)` foreign key, so non-Mealie snack-fallback candidates cannot round-trip
through it. **Decision:** snack-fallback candidates are kept **transient/in-memory only** — returned
via MCP but not persisted to `meal_plan_candidate`. This avoids polluting `recipe_ref` with
synthetic rows for non-recipe items, avoids making `mealie_recipe_id` nullable (which would break
the FK contract for real recipes), and avoids inventing a new persistence mechanism for a small,
static fallback list. Breakfast candidates sourced from Mealie recipes (tagged `frukost`/`breakfast`)
**do** persist normally through `meal_plan_candidate` since they have real `mealie_recipe_id`s.

**D3 — Breakfast/snack scoring is a subset, not a new scorer.** `scoring.Rank` is reused for all
three slot kinds; `SchoolLunchTags` only applies when `slot_kind == 'dinner'` (breakfast/snack pass
an empty `SchoolLunchTags`), and the day's `KitchenEnergy` only gates dinner's `Effort` (breakfast
and staple snacks are assumed low-effort by construction — Effort filtering is skipped for those
slots rather than requiring every breakfast recipe to be tagged with an effort level). Alternative
considered: a fully separate lightweight scorer for non-dinner slots — rejected as needless
duplication per repo convention (`internal/scoring` is already a small, pure, well-tested unit).

**D4 — MCP tool shape: extend `list_recipe_candidates`'s output with a `Slot` field
(`"dinner"|"breakfast"|"snack"`) rather than adding three separate tools.** One tool call for a date
range returns all three slots per day; `record_meal_reaction` gains an optional `Slot` field
(defaulting to `"dinner"` for backward compatibility with existing callers/tests). Alternative
considered: `list_breakfast_candidates` / `list_snack_candidates` as separate tools — rejected,
since an AI chat planning "the week" wants one call per date range, not three.

**D5 — Breakfast needs a fallback too, and it should be a component spread, not discrete dishes.**
As implemented (task 4.4), `internal/planning/snacks.go`'s `BreakfastCandidates` has **no**
fallback — it returns an empty slice when no Mealie recipes carry a `frukost`/`breakfast` tag,
unlike `SnackCandidates`, which falls back to `FallbackSnacks`. Real household input (2026-08-27)
shows this will bite immediately: breakfast here is a flexible spread of components — levain
toast, cheese, ham, juice (orange/apple/other), Turkish yogurt, müsli, eggs, cucumber, "any
combination" — not a set of named dishes the way dinner is. The household's actual pattern:
**weekdays** stay simple (toast + one topping, ham or cheese); **weekends** add more components
(egg, cucumber, extra sides), and yogurt+müsli sometimes replaces toast entirely. `English
breakfast` (imported as a real Mealie recipe, tagged both `frukost` and `ovrigt`) is the one
actual discrete-dish exception — most days won't have a tagged recipe at all.

**Decision:** add `FallbackBreakfasts` to `internal/planning/snacks.go`, mirroring
`FallbackSnacks`'s shape (`[]domain.Candidate`, `Effort: domain.EffortLow`, `Slot:
domain.SlotBreakfast`), but split into two pools instead of one flat list:
- `FallbackBreakfastsWeekday` — 2-3 simple combos (e.g. "Levain med skinka", "Levain med ost",
  "Turkisk yoghurt med müsli"), each just bread+one topping or yogurt+müsli, `Ingredients` listing
  the real components (`levainbröd`, `skinka` / `ost` / `turkisk yoghurt`, `müsli`).
- `FallbackBreakfastsWeekend` — a fuller pool (e.g. "Ägg, levain, skinka, ost och gurka", "Turkisk
  yoghurt, müsli, ägg och juice") reflecting the household's actual weekend additions.

`BreakfastCandidates` needs a day-of-week input to pick the right pool — same shape as
`effort_profile`'s per-weekday concept (`internal/domain` already has a weekday-keyed pattern to
follow, not a new one to invent). Alternative considered: a true combinatorial generator
(bread × topping × side, scored/ranked like a real candidate set) — rejected as over-engineering
for what is, per the actual household description, "any combination" with no stated preference
ordering among components; a handful of representative combo candidates is enough for the scorer
and repetition-avoidance to work with, and it's trivial to add more combos later if a pattern
emerges from reactions.

## Risks / Trade-offs

- [Widening the plan-decision primary key touches two tables other changes (`implement-meal-
  planning`, `implement-recommendations`) were designed against with a single-slot assumption] →
  Mitigation: default `slot_kind='dinner'` keeps every existing row and query valid; grep both
  changes' Go code for `slot_date` before merging to confirm no raw SQL assumes single-row-per-date.
- [Snack "recipes" that aren't real Mealie recipes (the D2 fallback list) don't have a
  `mealie_recipe_id` to satisfy `meal_plan_candidate`'s `FOREIGN KEY ... REFERENCES
  recipe_ref(mealie_recipe_id)`] → **Resolved (task 2.1/2.2):** `internal/planning/staples.go` is
  about shopping-list staples, not meal candidates — it has no persistence path. Snack-fallback
  candidates are kept transient/in-memory only (returned via MCP, not persisted to
  `meal_plan_candidate`). Breakfast candidates from Mealie recipes persist normally.
- [Two Swedish kids may not eat what any generic "kid-friendly Swedish breakfast" recipe set
  assumes] → Mitigation: this is exactly what `internal/ambient.RecordReaction`'s preference
  learning already exists to correct over time; no special-casing needed, just make sure
  breakfast/snack reactions actually flow through it (task 3).

## Migration Plan

1. Migration `NNNN_meal_plan_slots.sql`: add `slot_kind` to `meal_plan_candidate` and
   `meal_plan_decision`, default `'dinner'`, widen `meal_plan_decision`'s primary key.
2. Land Go changes (domain/planning/scoring/mcptools) behind the new column; `slot_kind` defaults
   keep dinner-only callers working during rollout.
3. Wire `cmd/mcp-server/adapters.go` (task 1) independently — no schema dependency, can ship first.
4. No rollback complexity beyond a standard `DROP COLUMN` — no data migration of existing rows is
   needed since the default backfills them as `'dinner'`.

## Open Questions

- Should a per-date energy override (vs. today's per-weekday-only `effort_profile`) be added now or
  deferred to a later change? Leaning defer — `effort_profile` covers the common case ("Mondays I'm
  wiped") and a one-off override can reuse `planning_constraint`'s existing `kind`/`value` shape
  later without a schema change today.
- ~~Does `internal/planning/staples.go` already have a persistence path for non-Mealie candidates
  that the D2 snack fallback can reuse directly?~~ **Resolved:** No — `staples.go` is about
  shopping-list staples, not meal candidates. Snack-fallback candidates are kept transient/in-memory
  only (see D2 persistence decision).
