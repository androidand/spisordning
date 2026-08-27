## ADDED Requirements

### Requirement: MCP dinner planning respects school lunch and kitchen energy
The `list_recipe_candidates` MCP tool SHALL populate the planner's school-lunch tags (from
Mariaskolan's published weekly menu) and the household's per-weekday kitchen energy (from
`effort_profile`) for every date it plans, so its output matches what `food-brain plan` (the CLI
path) already produces for the same date.

#### Scenario: Dinner suggestion avoids repeating the school lunch
- **WHEN** `list_recipe_candidates` is called for a date where Mariaskolan's published menu
  includes "fiskgratäng"
- **THEN** the returned dinner candidate's ranking penalizes recipes tagged with tags overlapping
  the school lunch's tokenized tags, the same way the CLI path already does

#### Scenario: Dinner suggestion respects a low-energy weekday
- **WHEN** `list_recipe_candidates` is called for a date whose weekday has `kitchen_energy = 1`
  (low) in `effort_profile`
- **THEN** the returned dinner candidate's `Effort` does not exceed low, unless no low/medium-effort
  candidate is feasible for that day

### Requirement: A week plan includes breakfast and snack slots alongside dinner
The planner SHALL be able to produce candidates for `breakfast` and `snack` slots, in addition to
`dinner`, for any date it plans, using the same repetition-avoidance and preference-learning
mechanisms as dinner.

#### Scenario: Requesting a week plan returns three slots per day
- **WHEN** an MCP client calls `list_recipe_candidates` for a 7-day range
- **THEN** the response includes a `dinner`, `breakfast`, and `snack` candidate for each of the 7
  dates

#### Scenario: Breakfast/snack candidates are not filtered against the school lunch
- **WHEN** the planner ranks breakfast or snack candidates for a date
- **THEN** the school lunch's tags do not penalize the ranking (only dinner is deduplicated against
  school lunch)

#### Scenario: A household reaction to a breakfast recipe updates its preference the same way a
dinner reaction does
- **WHEN** `record_meal_reaction` is called with `slot="breakfast"` for a person and a served
  breakfast recipe
- **THEN** the person's tag preferences update via the same confidence-weighted mechanism
  `internal/ambient.RecordReaction` already applies to dinner reactions

#### Scenario: A household with no Mealie recipes tagged for snacks still gets snack suggestions
- **WHEN** the planner has zero Mealie recipes tagged `mellanmål`/`snack`
- **THEN** the snack slot falls back to the built-in staple snack list rather than returning no
  candidate

#### Scenario: A household with no Mealie recipes tagged for breakfast still gets a breakfast
suggestion, matched to the day of week
- **WHEN** the planner has zero Mealie recipes tagged `frukost`/`breakfast` for a given date
- **THEN** the breakfast slot falls back to a simple weekday combo (e.g. toast + one topping) on a
  weekday, or a fuller weekend combo (e.g. egg + toast + extra sides) on a weekend, rather than
  returning no candidate

#### Scenario: A tagged breakfast recipe still wins over the fallback
- **WHEN** at least one Mealie recipe is tagged `frukost`/`breakfast` for a given date
- **THEN** the breakfast slot uses that recipe, not the weekday/weekend fallback combo
