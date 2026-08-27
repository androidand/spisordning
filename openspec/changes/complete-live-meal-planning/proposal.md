## Why

`internal/planning/week.go`'s `PlanWeek` and `internal/scoring/scoring.go` already know how to avoid
repeating the school lunch (`SchoolLunchTags`) and how to respect the cook's daily energy budget
(`KitchenEnergy` vs. a candidate's `Effort`) — `cmd/food-brain/plan.go` (the CLI path) already wires
both from `effort_profile` (per-weekday energy, `db/migrations/000001_init.sql`) and
`internal/skolmaten` (Mariaskolan's published menu). But `cmd/mcp-server/adapters.go` — the
composition root behind the MCP tool an AI chat actually calls (`list_recipe_candidates`) — never
populates `WeekConfig.EnergyFor` or `WeekConfig.SchoolTagsFor`. So today, planning a week over MCP
silently ignores both inputs even though the household already asked the planner to use them via the
CLI. Separately, the entire planning/scoring/MCP stack only knows about dinner — there is no
breakfast or snack candidate list, slot, or tool — even though a full week for this household means
dinner + breakfast + snacks for one adult and two kids.

## What Changes

- Wire `EnergyFor` (from `effort_profile`) and `SchoolTagsFor` (from `internal/skolmaten`, keyed to
  Mariaskolan) into `cmd/mcp-server/adapters.go`'s `mcpStoreAdapter.PlanDinners`, mirroring the
  existing `cmd/food-brain/plan.go` wiring so `list_recipe_candidates` actually gets both inputs.
- Add breakfast and snack as additional meal slots alongside dinner: domain types, a
  `PlanWeek`-equivalent (or `PlanWeek` extension) that plans all three per day, and Mealie/manual
  candidate sourcing for breakfast and snack recipes. Reuse the existing scorer, repetition
  avoidance, and `internal/ambient.RecordReaction` preference-learning machinery rather than
  building parallel systems — breakfast/snack scoring can be a simplified subset (no school-lunch
  dedup, effort weighting optional) rather than the full dinner rule set.
- Extend the MCP tool surface (`internal/mcptools`) so `list_recipe_candidates` (or a sibling tool)
  returns breakfast/snack candidates alongside dinner for a requested date range, and
  `record_meal_reaction` accepts reactions to non-dinner slots.

## Capabilities

### New Capabilities
- `meal-slot-planning`: extends planning beyond dinner to breakfast and snack slots — domain
  modeling, scoring reuse, and MCP tool exposure for a full-day (not just full-dinner) plan.

### Modified Capabilities
- None in `openspec/specs/` today reference dinner-only planning explicitly as a constraint (the
  merged specs directory has no `meal-planning` capability yet — `implement-meal-planning` has not
  been archived/synced), so this is additive, not a behavior change to an existing merged spec.

## Impact

- `cmd/mcp-server/adapters.go`: wire `EnergyFor`/`SchoolTagsFor` into `PlanDinners`.
- `internal/planning`: extend `WeekConfig`/`PlanWeek` (or add a parallel `PlanDay` helper) to cover
  breakfast/snack slots.
- `internal/domain`: add a meal-slot concept (dinner/breakfast/snack) to `Candidate`/`PlannedSlot`-
  adjacent types.
- `internal/scoring`: confirm/adjust which scoring inputs apply per slot kind.
- `internal/mcptools`, `cmd/mcp-server/adapters.go`: extend tool input/output schemas and wiring.
- No changes to retailer, pricing, shopping-list, Apple Notes, or deployment code — those are
  separate changes (`expose-shopping-price-and-notes-bridge`, `deploy-food-brain-to-proxmox`).
