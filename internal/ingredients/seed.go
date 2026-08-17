// Package ingredients holds the small curated set of Swedish-unit → grams →
// package-size mappings for task 2.3. It is the in-memory mirror of the SQL
// seed at migrations/seed/ingredient_mappings.sql, so the CLI review surface
// (food-brain ingredients) can show the curated knowledge before Postgres
// persistence lands (that arrives with the food-brain HTTP server, see
// establish-enforced-go-architecture).
package ingredients

// Mapping is one curated Swedish-unit → grams → package-size conversion for a
// recipe ingredient. It mirrors a row of the ingredient_mapping table.
type Mapping struct {
	// MealieFoodID is the canonical id the plan keys on. Until persistence
	// lands that's the lowercase Mealie food name; the seed's PLACEHOLDER-*
	// UUIDs stand in for the real per-food UUID a live sync will assign.
	MealieFoodID string
	// IngredientID is the canonical ingredient id (ingredient.id).
	IngredientID string
	// Display is the human-readable name.
	Display string
	// Unit is the Swedish recipe unit (dl, msk, tsk, förp, st).
	Unit string
	// GramsPerUnit is grams per one recipe unit.
	GramsPerUnit float64
	// PackageSizeGrams is a typical retail package size in grams.
	PackageSizeGrams float64
	// DefaultForm is 'fresh' | 'frozen' | 'dry' | 'solid' | ...
	DefaultForm string
	// NeedsReview marks rows that must be re-pointed at a real mealie_food_id
	// once a live sync assigns one.
	NeedsReview bool
}

// Seed is the small curated set for task 2.3. Keep it in sync with
// migrations/seed/ingredient_mappings.sql.
var Seed = []Mapping{
	{
		MealieFoodID:     "PLACEHOLDER-vetemjol-dl",
		IngredientID:     "vetemjol",
		Display:          "Vetemjöl",
		Unit:             "dl",
		GramsPerUnit:     60,
		PackageSizeGrams: 1000,
		DefaultForm:      "dry",
		NeedsReview:      true,
	},
	{
		MealieFoodID:     "PLACEHOLDER-smor-msk",
		IngredientID:     "smor",
		Display:          "Smör",
		Unit:             "msk",
		GramsPerUnit:     14,
		PackageSizeGrams: 250,
		DefaultForm:      "solid",
		NeedsReview:      true,
	},
	{
		MealieFoodID:     "PLACEHOLDER-salt-tsk",
		IngredientID:     "salt",
		Display:          "Salt",
		Unit:             "tsk",
		GramsPerUnit:     6,
		PackageSizeGrams: 500,
		DefaultForm:      "dry",
		NeedsReview:      true,
	},
	{
		MealieFoodID:     "PLACEHOLDER-falukorv-forp",
		IngredientID:     "falukorv",
		Display:          "Falukorv",
		Unit:             "förp",
		GramsPerUnit:     400,
		PackageSizeGrams: 400,
		DefaultForm:      "fresh",
		NeedsReview:      true,
	},
}

// ByIngredientID returns the curated mapping for a canonical ingredient id, or
// ok=false if the ingredient isn't in the curated set.
func ByIngredientID(id string) (Mapping, bool) {
	for _, m := range Seed {
		if m.IngredientID == id {
			return m, true
		}
	}
	return Mapping{}, false
}
