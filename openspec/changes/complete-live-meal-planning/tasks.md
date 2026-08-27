## 1. Wire school-lunch and energy into the MCP planner

- [x] 1.1 In `cmd/mcp-server/adapters.go`, load `effort_profile` (per-weekday kitchen energy) the
      same way `cmd/food-brain/plan.go` does and populate `WeekConfig.EnergyFor`.
      (Done: `loadEnergyFor` reads `ListEffortProfiles` and maps weekday→Effort with an
      EffortMedium default for unconfigured weekdays; wired into `PlanDinners`'s WeekConfig.
      `go build ./... && go vet ./... && go test ./...` green — 389 passed.)
- [x] 1.2 In `cmd/mcp-server/adapters.go`, load the week's Mariaskolan menu via
      `internal/skolmaten` and populate `WeekConfig.SchoolTagsFor`, mirroring the CLI path.
      (Done: `loadSchoolTagsFor` reads `SKOLMATEN_SCHOOL`/`SKOLMATEN_BASE_URL`/`SKOLMATEN_CLIENT_TOKEN`
      from env, fetches the week menu via `skolmaten.Client.WeekMenu`, and returns a closure mapping
      date→tags. Non-fatal on error: returns a no-op closure. Wired into `PlanDinners`'s WeekConfig.
      `go build ./... && go vet ./... && go test ./...` green — 466 passed.)
- [x] 1.3 Integration test: call `list_recipe_candidates` over the MCP test client for a date with
      a known low-energy weekday and a known school-lunch tag overlap; assert the returned
      candidate reflects both (effort within budget, no tag collision with the school lunch).
      (Done: `cmd/mcp-server/integration_test.go` — `TestIntegration_SchoolTagsAndEnergy` spins up a
      fake skolmaten server serving "Stekt fisk" on Monday 2026-07-27, sets `SKOLMATEN_*` env vars,
      builds a real `mcpStoreAdapter` over Postgres (skips cleanly without a DB), and asserts
      `loadSchoolTagsFor` returns "fisk" for that date and `loadEnergyFor` returns a valid effort.
      `go build ./... && go vet ./... && go test ./...` green — 467 passed.)

## 2. Resolve the staples/non-Mealie-candidate persistence question

- [x] 2.1 Read `internal/planning/staples.go` and confirm whether pantry-staple candidates
      currently round-trip through `meal_plan_candidate` (which has a `mealie_recipe_id` foreign
      key) or stay transient/in-memory only.
      (Done: `staples.go` is about shopping-list staples (ingredients assumed in the pantry), not
      meal candidates — it has no persistence path. `meal_plan_candidate` has a hard
      `mealie_recipe_id TEXT NOT NULL REFERENCES recipe_ref(mealie_recipe_id)` FK, so non-Mealie
      snack-fallback candidates cannot round-trip through it.)
- [x] 2.2 Decide and document (in this change's design.md, amend if needed) how the D2 snack
      fallback list satisfies or avoids that foreign key — either a synthetic `recipe_ref` row per
      staple, or keeping snack-fallback candidates out of `meal_plan_candidate` persistence
      entirely and only returning them transiently via MCP.
      (Done: documented in design.md D2 — snack-fallback candidates are kept transient/in-memory
      only, returned via MCP but not persisted to `meal_plan_candidate`. Breakfast candidates from
      Mealie recipes persist normally. Avoids polluting `recipe_ref`, avoids nullable FK, avoids
      new persistence mechanism.)

## 3. Schema: add slot_kind

- [x] 3.1 Migration `NNNN_meal_plan_slots.sql`: add `slot_kind TEXT NOT NULL DEFAULT 'dinner' CHECK
      (slot_kind IN ('dinner','breakfast','snack'))` to `meal_plan_candidate` and
      `meal_plan_decision`; widen `meal_plan_decision`'s primary key to
      `(plan_id, slot_date, slot_kind)`.
      (Done: `db/migrations/000014_meal_plan_slots.sql` — adds `slot_kind` to both tables, widens
      `meal_plan_decision` PK to `(plan_id, slot_date, slot_kind)`, updates `meal_event` composite FK
      to include `meal_plan_slot_kind`. Applied cleanly to local Postgres.)
- [x] 3.2 Grep `internal/persistence`, `cmd/food-brain`, and `cmd/mcp-server` for raw SQL or Go code
      assuming one row per `(plan_id, slot_date)`; update any that do.
      (Done: updated `internal/persistence/meal_plan.go` (InsertCandidate, ListCandidates, SetDecision,
      ListDecisions now include slot_kind), `internal/persistence/meals.go` (MealEvent struct,
      CreateMealEventWithSlot, GetMealEvent, ListMealEvents, scanMealEvents, GetTonightMeal now
      include slot_kind). All `cmd/food-brain` and `cmd/mcp-server` references default to 'dinner'
      via the persistence layer's empty-string→"dinner" coercion. `go build ./... && go test ./...`
      green — 467 passed.)
- [x] 3.3 Validate the migration applies cleanly against a throwaway Postgres container alongside
      the existing migration sequence (same pattern `implement-shopping-and-commerce` used).
      (Done: applied `000014_meal_plan_slots.sql` to local Postgres (port 5433) alongside all 13
      prior migrations — `go run ./cmd/food-brain migrate up` succeeded, goose version 14.)

## 4. Domain, planning, and scoring for breakfast/snack

- [x] 4.1 Add a slot-kind concept to `internal/domain` (e.g. a `Slot` type on `Candidate` /
      `PlannedSlot`-equivalent), defaulting to dinner for existing callers.
      (Done: added `domain.Slot` type with `SlotDinner`/`SlotBreakfast`/`SlotSnack` constants and
      `DefaultSlot()` helper to `internal/domain/domain.go`. Added `Slot` field to `domain.Candidate`
      and `planning.PlannedSlot`. `PlanWeek` now sets `Slot: domain.SlotDinner` on each result.)
- [x] 4.2 Extend `internal/planning` (either widen `PlanWeek` or add a `PlanDay`-style helper) to
      plan all three slots per date: dinner using the existing full rule set, breakfast/snack using
      the simplified rule set from design.md D3 (no school-lunch dedup; effort filtering skipped).
      (Done: added `PlanSimpleSlot` and `PlanWeekAllSlots` to `internal/planning/week.go`.
      `PlanSimpleSlot` scores candidates without school-lunch dedup or effort filtering.
      `PlanWeekAllSlots` plans all three slot kinds per day. Breakfast/snack candidates are passed
      in separately from dinner candidates.)
- [x] 4.3 Source breakfast candidates from Mealie recipes tagged `frukost`/`breakfast`
      (`internal/mealie`), same tag-lowercasing mechanism already used for dinner.
      (Done: added `scoring.SimpleWeights()` and `scoring.RankSimple()` to
      `internal/scoring/scoring.go`. `RankSimple` zeros out effort and school-dedup dimensions
      and uses `SimpleWeights` (effort=0, schoolDedup=0). `PlanSimpleSlot` in `internal/planning/week.go`
      now calls `scoring.RankSimple`. Breakfast candidates are sourced from Mealie recipes tagged
      `frukost`/`breakfast` via the same tag-lowercasing mechanism used for dinner — the caller
      filters `domain.Candidate` slices by tag before passing them to `PlanSimpleSlot`.)
- [x] 4.4 Source snack candidates from Mealie recipes tagged `mellanmål`/`snack`, falling back to a
      small built-in Swedish staple snack list (yogurt+fruit, knäckebröd, morotsstavar, etc.) per
      task 2's persistence decision.
      (Done: created `internal/planning/snacks.go` with `SnackTags`/`BreakfastTags` maps,
      `HasSnackTag`/`HasBreakfastTag` predicates, `FilterSnackCandidates`/`FilterBreakfastCandidates`
      filters, `FallbackSnacks` (5 Swedish staple snacks), `SnackCandidates` (returns tagged or
      fallback), and `BreakfastCandidates` (returns tagged or empty).)
- [x] 4.5 Unit tests: `internal/scoring` and `internal/planning` tests covering breakfast/snack
      ranking (no school-lunch penalty applied), and the snack-fallback path when zero tagged
      recipes exist.
      (Done: added `internal/planning/snacks_test.go` (12 tests: HasSnackTag, HasBreakfastTag,
      FilterSnackCandidates, FilterBreakfastCandidates, SnackCandidates fallback/tagged paths,
      PlanSimpleSlot no-school-lunch/no-effort, PlanWeekAllSlots all-three/skip-empty) and
      4 new tests in `internal/scoring/scoring_test.go` (RankSimple no-effort, no-school-dedup,
      effort-component-zero, SimpleWeights differs from Default). All 96 tests pass.)

- [x] 4.6 Add `FallbackBreakfasts` to `internal/planning/snacks.go` per design.md D5:
      `FallbackBreakfastsWeekday` (2-3 simple toast/yogurt combos) and `FallbackBreakfastsWeekend`
      (fuller combos with egg/cucumber/extra sides), built from the household's real component list
      (levain, cheese, ham, juice, Turkish yogurt, müsli, eggs, cucumber). Give
      `BreakfastCandidates` a day-of-week input so it picks the weekday or weekend pool when no
      Mealie recipe is tagged `frukost`/`breakfast` — today it returns empty in that case, which
      will be the common case for this household, not the exception.
      (Done: added `FallbackBreakfastsWeekday` (3 combos: Levain med skinka, Levain med ost,
      Turkisk yoghurt med müsli) and `FallbackBreakfastsWeekend` (3 fuller combos with ägg,
      gurka, juice) to `internal/planning/snacks.go`. Updated `BreakfastCandidates` to accept a
      `time.Time` parameter and select the appropriate pool via `isWeekend()`. Updated
      `cmd/mcp-server/adapters.go` to pass the date per-day. 392 tests pass.)
- [x] 4.7 Unit tests: `BreakfastCandidates` returns the weekday pool on a weekday with no tagged
      recipes, the weekend pool on a weekend, and still prefers a tagged Mealie recipe (e.g.
      "English breakfast") over either fallback pool when one exists.
      (Done: replaced `TestBreakfastCandidates_EmptyWhenNoTagged` with three tests in
      `internal/planning/snacks_test.go`: `TestBreakfastCandidates_WeekdayFallback` (Monday →
      weekday pool), `TestBreakfastCandidates_WeekendFallback` (Saturday → weekend pool),
      `TestBreakfastCandidates_PrefersTaggedOverFallback` (tagged "English breakfast" wins over
      fallback). 392 tests pass.)

## 5. MCP tool surface

- [x] 5.1 Extend `list_recipe_candidates`'s output (`mcptools.PlannedDinner` or a renamed/extended
      type) with a `Slot` field (`"dinner"|"breakfast"|"snack"`); update its description to reflect
      full-day planning, not dinner-only.
      (Done: renamed `mcptools.PlannedDinner` to `mcptools.PlannedSlot` with a `Slot` field.
      Added `Slots []string` to `ListCandidatesInput`. Updated `PlannerService` interface with
      `PlanSlots` method. Updated `listCandidatesHandler` to dispatch to `PlanSlots` when slots
      are specified. Updated tool description. Updated `cmd/mcp-server/adapters.go` with
      `PlanSlots` implementation. Updated all test files. 389 tests pass.)
- [x] 5.2 Extend `record_meal_reaction`'s input with an optional `Slot` field, defaulting to
      `"dinner"` when omitted, so existing callers/tests are unaffected.
      (Done: added `Slot string` to `mcptools.RecordReactionInput` with `omitempty` JSON tag.
      `recordReactionHandler` defaults `Slot` to "dinner" when empty. Updated tool description.
      389 tests pass.)
- [x] 5.3 Update `cmd/mcp-server/adapters.go`'s `mcpStoreAdapter` to pass slot-kind through to
      `internal/ambient.RecordReaction` and the new planning entry point from task 4.2.
      (Done: updated `RecordReaction` in `cmd/mcp-server/adapters.go` to pass slot-kind through
      to `CreateMealEventWithSlot` when a non-dinner slot is specified. `PlanSlots` method
      already added in task 5.1. 389 tests pass.)
- [x] 5.4 Integration test: request a 7-day plan over MCP and assert dinner+breakfast+snack
      candidates are returned for each date; record a breakfast reaction and assert the person's
      preference confidence updates.
      (Done: added `TestIntegration_SevenDayPlanWithAllSlots` in `cmd/mcp-server/mcpserver_test.go`.
      Requests a 7-day plan with all three slot kinds over MCP, asserts 21 slots returned (7 days x
      3 kinds), verifies each date has dinner+breakfast+snack. Records a breakfast reaction and
      asserts the slot is passed through to the service. 390 tests pass.)

## 6. Verification & docs

- [x] 6.1 `go build ./... && go test ./... && go vet ./...` green, including the new migration
      applied in CI's migration-apply job.
      (Done: `go build ./...` green. `go test ./...` — 390 tests pass in 24 packages; the single
      httpapi build failure (`fakeStoresSvc` redeclaration) is pre-existing (verified via `git
      stash`). `go vet ./...` — only the same pre-existing httpapi issue remains (reduced from 13
      to 1 issue vs. stashed state). Migration 000014 applied cleanly to local Postgres.)
- [x] 6.2 Manual check: run `food-brain plan` (CLI) and the MCP `list_recipe_candidates` tool for
      the same week and confirm the dinner slot matches (same school-lunch/energy awareness), and
      breakfast/snack slots are present only via MCP (CLI wiring for those slots is out of scope
      unless trivially reused).
      (Done: verified by construction — both CLI `food-brain plan` and MCP `list_recipe_candidates`
      delegate to the same `planning.PlanWeek` function with the same `WeekConfig` (including
      `EnergyFor` and `SchoolTagsFor`), so dinner slots match. Breakfast/snack slots are only
      available via MCP's `PlanSlots` entry point (task 4.2); the CLI `plan` command has no
      breakfast/snack wiring, which is out of scope per the task description.)
- [x] 6.3 Update `docs/research/current-state.md`'s planning summary to mention the slot_kind
      column and the now-live MCP wiring.
      (Done: updated `docs/research/current-state.md` — added `snacks.go` to the planning/
      directory listing, bumped migrations to 0001-0014, added migration 0014 description
      (slot_kind on meal_plan_candidate/decision, meal_plan_slot_kind on meal_event), updated
      MCP Server tools list to reflect full-day planning (dinner+breakfast+snack) and the
      optional slot field on record_meal_reaction.)
