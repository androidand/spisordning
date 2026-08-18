// Package ambient is the family-facing "ambient surface" for Food Brain: it
// projects the week's dinners for display and folds one-tap meal reactions back
// into the family's preference model. Home Assistant (via the homeops MCP) is
// the presentation layer; this package owns the data shape and the learning
// step so the surface stays a thin projection of the durable model rather than
// a second planner.
package ambient

import (
	"math"

	"github.com/androidand/spisordning/internal/domain"
)

// observationWeight is how much a single reaction moves a preference's
// confidence. A fresh observation is worth a quarter of a fully-observed
// preference, so one tap nudges a belief instead of overwriting it.
const observationWeight = 0.25

// Slot is one planned dinner in the week projection written by
// `food-brain plan --write-tonight`. It is the in-memory-slice stand-in for the
// meal_event row until Postgres persistence lands.
type Slot struct {
	Date   string   `json:"date"` // YYYY-MM-DD
	Title  string   `json:"title"`
	Reason string   `json:"reason"`
	Tags   []string `json:"tags"` // preference vocabulary for the meal
}

// PlanFile is the week projection persisted for the ambient surface.
type PlanFile struct {
	Week  string `json:"week"`
	Slots []Slot `json:"slots"`
}

// Tonight returns the slot for date. If date is empty or not present in the
// projection, it returns the first slot (the week's anchor dinner).
func (p PlanFile) Tonight(date string) (Slot, bool) {
	if date != "" {
		for _, s := range p.Slots {
			if s.Date == date {
				return s, true
			}
		}
	}
	if len(p.Slots) == 0 {
		return Slot{}, false
	}
	return p.Slots[0], true
}

// Render formats a slot for an ambient display (e.g. an HA dashboard card).
// It is deliberately plain text so the homeops MCP can push it verbatim.
func Render(s Slot) string {
	if s.Reason == "" {
		return s.Title
	}
	return s.Title + " — " + s.Reason
}

// RecordReaction folds a single reaction into the person's preferences for the
// meal's tags. For each tag the preference's sentiment becomes the
// confidence-weighted mean of prior observations plus this one, and its
// confidence grows (capped at 1). A missing preference is seeded from the
// reaction. This is the durable-learning step from the meal-planning spec: a
// meal reaction updates the affected person's preference confidence.
//
// It returns a new slice and never mutates its input, so the caller can decide
// whether to persist the result.
func RecordReaction(prefs []domain.Preference, personID string, tags []string, sentiment domain.Sentiment) []domain.Preference {
	out := make([]domain.Preference, len(prefs))
	copy(out, prefs)

	seen := make(map[string]bool, len(tags))
	for _, tag := range tags {
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true

		idx := -1
		for i, p := range out {
			if p.PersonID == personID && p.Tag == tag {
				idx = i
				break
			}
		}
		if idx == -1 {
			out = append(out, domain.Preference{
				PersonID:   personID,
				Tag:        tag,
				Sentiment:  sentiment,
				Confidence: observationWeight,
			})
			continue
		}

		old := out[idx]
		total := old.Confidence + observationWeight
		weighted := old.Confidence*float64(old.Sentiment) + observationWeight*float64(sentiment)
		newConf := total
		if newConf > 1 {
			newConf = 1
		}
		out[idx] = domain.Preference{
			PersonID:   personID,
			Tag:        tag,
			Sentiment:  domain.Sentiment(math.Round(weighted / total)),
			Confidence: newConf,
		}
	}
	return out
}
