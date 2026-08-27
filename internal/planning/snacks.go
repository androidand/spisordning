package planning

import (
	"strings"
	"time"

	"github.com/androidand/spisordning/internal/domain"
)

// SnackTags are the Mealie tags (lowercased) that identify a recipe as a snack.
var SnackTags = map[string]bool{
	"mellanmål": true,
	"mellanmal": true,
	"snack":     true,
}

// BreakfastTags are the Mealie tags (lowercased) that identify a recipe as a breakfast.
var BreakfastTags = map[string]bool{
	"frukost":   true,
	"breakfast": true,
}

// HasSnackTag reports whether any of the candidate's tags (lowercased) is a
// known snack tag.
func HasSnackTag(tags []string) bool {
	for _, t := range tags {
		if SnackTags[strings.ToLower(strings.TrimSpace(t))] {
			return true
		}
	}
	return false
}

// HasBreakfastTag reports whether any of the candidate's tags (lowercased) is a
// known breakfast tag.
func HasBreakfastTag(tags []string) bool {
	for _, t := range tags {
		if BreakfastTags[strings.ToLower(strings.TrimSpace(t))] {
			return true
		}
	}
	return false
}

// FilterSnackCandidates returns only the candidates that have a snack tag.
func FilterSnackCandidates(candidates []domain.Candidate) []domain.Candidate {
	var out []domain.Candidate
	for _, c := range candidates {
		if HasSnackTag(c.Tags) {
			out = append(out, c)
		}
	}
	return out
}

// FilterBreakfastCandidates returns only the candidates that have a breakfast tag.
func FilterBreakfastCandidates(candidates []domain.Candidate) []domain.Candidate {
	var out []domain.Candidate
	for _, c := range candidates {
		if HasBreakfastTag(c.Tags) {
			out = append(out, c)
		}
	}
	return out
}

// FallbackSnacks is a small built-in Swedish staple snack list used when no
// Mealie recipes are tagged as snacks. These are simple, no-cook options that
// are commonly available in a Swedish household.
var FallbackSnacks = []domain.Candidate{
	{
		Title:       "Yoghurt med frukt",
		Tags:        []string{"mellanmål", "snack"},
		Ingredients: []string{"yoghurt", "frukt"},
		Effort:      domain.EffortLow,
		Slot:        domain.SlotSnack,
	},
	{
		Title:       "Knäckebröd med ost",
		Tags:        []string{"mellanmål", "snack"},
		Ingredients: []string{"knäckebröd", "ost"},
		Effort:      domain.EffortLow,
		Slot:        domain.SlotSnack,
	},
	{
		Title:       "Morotsstavar med hummus",
		Tags:        []string{"mellanmål", "snack"},
		Ingredients: []string{"morot", "hummus"},
		Effort:      domain.EffortLow,
		Slot:        domain.SlotSnack,
	},
	{
		Title:       "Äpple med mandelmus",
		Tags:        []string{"mellanmål", "snack"},
		Ingredients: []string{"äpple", "mandelmus"},
		Effort:      domain.EffortLow,
		Slot:        domain.SlotSnack,
	},
	{
		Title:       "Kex med smör och ost",
		Tags:        []string{"mellanmål", "snack"},
		Ingredients: []string{"kex", "smör", "ost"},
		Effort:      domain.EffortLow,
		Slot:        domain.SlotSnack,
	},
}

// SnackCandidates returns snack candidates from the given Mealie candidates.
// If any candidates have a snack tag, those are returned. Otherwise the
// built-in FallbackSnacks list is returned.
func SnackCandidates(candidates []domain.Candidate) []domain.Candidate {
	tagged := FilterSnackCandidates(candidates)
	if len(tagged) > 0 {
		return tagged
	}
	return FallbackSnacks
}

// FallbackBreakfastsWeekday is a small built-in list of simple weekday
// breakfast combos for a Swedish household. Used when no Mealie recipes are
// tagged as breakfast on a weekday.
var FallbackBreakfastsWeekday = []domain.Candidate{
	{
		Title:       "Levain med skinka",
		Tags:        []string{"frukost", "breakfast"},
		Ingredients: []string{"levainbröd", "skinka"},
		Effort:      domain.EffortLow,
		Slot:        domain.SlotBreakfast,
	},
	{
		Title:       "Levain med ost",
		Tags:        []string{"frukost", "breakfast"},
		Ingredients: []string{"levainbröd", "ost"},
		Effort:      domain.EffortLow,
		Slot:        domain.SlotBreakfast,
	},
	{
		Title:       "Turkisk yoghurt med müsli",
		Tags:        []string{"frukost", "breakfast"},
		Ingredients: []string{"turkisk yoghurt", "müsli"},
		Effort:      domain.EffortLow,
		Slot:        domain.SlotBreakfast,
	},
}

// FallbackBreakfastsWeekend is a fuller pool of weekend breakfast combos
// reflecting the household's actual weekend additions (egg, cucumber, extra
// sides). Used when no Mealie recipes are tagged as breakfast on a weekend.
var FallbackBreakfastsWeekend = []domain.Candidate{
	{
		Title:       "Ägg, levain, skinka, ost och gurka",
		Tags:        []string{"frukost", "breakfast"},
		Ingredients: []string{"ägg", "levainbröd", "skinka", "ost", "gurka"},
		Effort:      domain.EffortLow,
		Slot:        domain.SlotBreakfast,
	},
	{
		Title:       "Turkisk yoghurt, müsli, ägg och juice",
		Tags:        []string{"frukost", "breakfast"},
		Ingredients: []string{"turkisk yoghurt", "müsli", "ägg", "juice"},
		Effort:      domain.EffortLow,
		Slot:        domain.SlotBreakfast,
	},
	{
		Title:       "Levain, ägg, skinka och gurka",
		Tags:        []string{"frukost", "breakfast"},
		Ingredients: []string{"levainbröd", "ägg", "skinka", "gurka"},
		Effort:      domain.EffortLow,
		Slot:        domain.SlotBreakfast,
	},
}

// isWeekend reports whether the given date falls on a Saturday or Sunday.
func isWeekend(date time.Time) bool {
	dow := date.Weekday()
	return dow == time.Saturday || dow == time.Sunday
}

// BreakfastCandidates returns breakfast candidates from the given Mealie
// candidates. If any candidates have a breakfast tag, those are returned.
// Otherwise the appropriate fallback pool (weekday or weekend) is selected
// based on the given date.
func BreakfastCandidates(candidates []domain.Candidate, date time.Time) []domain.Candidate {
	tagged := FilterBreakfastCandidates(candidates)
	if len(tagged) > 0 {
		return tagged
	}
	if isWeekend(date) {
		return FallbackBreakfastsWeekend
	}
	return FallbackBreakfastsWeekday
}
