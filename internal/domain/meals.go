// Package domain holds the core value types for the Food Brain: the family,
// their preferences, and the meal candidates the planner scores. These types
// are deliberately free of persistence and transport concerns so the scorer
// can be exercised as pure functions.
package domain

import "time"

// MealParticipant records that a person was actually present at (ate) a
// meal_event. Distinct from MealReaction: a person can attend without
// leaving a reaction, and a reaction can exist without an explicit
// attendance row (the reaction itself is evidence of presence).
//
// See migrations/0012_meal_history.sql meal_participant.
type MealParticipant struct {
	ID         int64
	MealEventID int64
	PersonID   string
	CreatedAt  time.Time
}

// MealReview records one person's considered review of one specific
// meal_event instance. The rating is on a 1–5 scale, separate from the
// quick-directional meal_reaction.sentiment (-2..2). A person may leave
// both a reaction and a review for the same meal; they answer different
// questions ("how did I feel about it?" vs. "how was it, on reflection?").
//
// Recipe-level rating is an aggregate derived from MealReview rows across
// all meal_event instances that share the same recipe — it is not stored
// on the recipe itself (computed on read). See migrations/0012_meal_history.sql.
type MealReview struct {
	ID          int64
	MealEventID int64
	PersonID    string
	Rating      int // 1..5
	Note        string
	CreatedAt   time.Time
}

// RecipeRating is the read-side aggregate of MealReview rows for all
// meal_events that used a given recipe. Computed on read; never cached
// in the database. The aggregation is a simple mean across all reviews.
//
// Once implement-recipe-family-and-revisions lands, this aggregation
// should be re-evaluated at the variant or family level — flagged as a
// known follow-up, not solved here.
type RecipeRating struct {
	MealieRecipeID string
	Average        float64 // mean of all ratings (unrounded; caller may round for display)
	ReviewCount    int     // number of reviews contributing to the mean
}

// Favorite is an explicit, person-scoped preference over a recipe. It is
// never derived from ratings or reactions — only created (and removed) by
// an explicit action. A recipe with a low average rating can still be a
// favorite (e.g. a child's comfort food); a recipe with a high average
// rating is not automatically a favorite.
//
// Household-scoped favorites are not modeled at the schema level in this
// change. They can be derived by querying all household members'
// individual favorites. The schema leaves room for a future
// household_id column if the need arises.
//
// See migrations/0012_meal_history.sql favorite.
type Favorite struct {
	ID             int64
	PersonID       string
	MealieRecipeID string
	CreatedAt      time.Time
}
