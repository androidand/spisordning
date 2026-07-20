package planning

import "strings"

// Assumed staples (README inventory class ASSUMED_STAPLE): pantry basics every
// kitchen is assumed to have, so they are never put on the shopping list. Kept
// deliberately conservative — only things that are almost always on hand — so
// real ingredients are never dropped. Swedish, matched per-word against the
// canonical ingredient id.
var assumedStaples = map[string]bool{
	"salt":        true,
	"peppar":      true,
	"vitpeppar":   true,
	"svartpeppar": true,
	"socker":      true,
	"strösocker":  true,
	"olja":        true,
	"matolja":     true,
	"rapsolja":    true,
	"smör":        true,
	"vatten":      true,
	"mjöl":        true,
	"vetemjöl":    true,
	"bakpulver":   true,
	"jäst":        true,
}

// IsAssumedStaple reports whether a canonical ingredient id is a pantry staple
// that should not appear on the shopping list. It matches if any whitespace-
// separated word of the id is a known staple, so "salt och peppar" or a parser
// artifact like "salt " still resolves.
func IsAssumedStaple(ingredientID string) bool {
	for word := range strings.FieldsSeq(strings.ToLower(ingredientID)) {
		word = strings.Trim(word, ".,()")
		if assumedStaples[word] {
			return true
		}
	}
	return false
}

// PartitionStaples splits requirements into those to actually buy and the
// assumed-staples that are dropped. Returning the dropped set (rather than
// silently filtering) lets the caller report what it skipped — silent
// truncation would read as "nothing was assumed" when things were.
func PartitionStaples(reqs []ShoppingRequirement) (buy, dropped []ShoppingRequirement) {
	for _, r := range reqs {
		if IsAssumedStaple(r.IngredientID) {
			dropped = append(dropped, r)
		} else {
			buy = append(buy, r)
		}
	}
	return buy, dropped
}
