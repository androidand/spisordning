package planning

import (
	"context"
	"time"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/androidand/spisordning/internal/scoring"
)

// PlannedSlot is one chosen dinner for a date.
type PlannedSlot struct {
	Date   time.Time
	Winner scoring.ScoredCandidate
}

// WeekConfig carries everything PlanWeek needs beyond the dates themselves.
type WeekConfig struct {
	// Candidates are the recipes considered for every slot.
	Candidates []domain.Candidate
	// People and Preferences are the household the plan is for.
	People      []domain.Person
	Preferences []domain.Preference
	// EnergyFor reports the cook's energy budget for a date (nil = unspecified).
	EnergyFor func(date time.Time) domain.Effort
	// SchoolTagsFor reports the day's school-lunch tags (nil = none).
	SchoolTagsFor func(date time.Time) []string
	// Reorder, when non-nil, may reorder a day's ranking before the winner is
	// picked. It is the additive LLM layer and never changes feasibility.
	Reorder func(ctx context.Context, ranked []scoring.ScoredCandidate) []scoring.ScoredCandidate
}

// PlanWeek plans days dinners starting at monday (inclusive): for each date it
// builds the PlanContext from the household config, scores every candidate, and
// picks the first feasible winner. Each chosen meal feeds the next day's
// repetition penalty, so the plan avoids repeats within the week. Days with no
// feasible candidate yield no slot (the caller reports the take-away night).
// The result is deterministic for fixed inputs.
func PlanWeek(ctx context.Context, cfg WeekConfig, monday time.Time, days int) []PlannedSlot {
	var slots []PlannedSlot
	var recent []domain.RecentMeal // chosen days feed the next day's repetition penalty

	for i := 0; i < days; i++ {
		date := monday.AddDate(0, 0, i)
		pctx := domain.PlanContext{
			Day:           date,
			People:        cfg.People,
			Preferences:   cfg.Preferences,
			RecentMealIDs: recent,
		}
		if cfg.EnergyFor != nil {
			pctx.KitchenEnergy = cfg.EnergyFor(date)
		}
		if cfg.SchoolTagsFor != nil {
			pctx.SchoolLunchTags = cfg.SchoolTagsFor(date)
		}

		ranked := scoring.Rank(cfg.Candidates, pctx, scoring.DefaultWeights())
		if cfg.Reorder != nil {
			ranked = cfg.Reorder(ctx, ranked)
		}

		for _, sc := range ranked {
			if sc.Feasible {
				slots = append(slots, PlannedSlot{Date: date, Winner: sc})
				recent = append(recent, domain.RecentMeal{MealieRecipeID: sc.Candidate.MealieRecipeID, Served: date})
				break
			}
		}
	}
	return slots
}
