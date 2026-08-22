package domain

import "time"

// MealParticipant records that a person was actually present/ate at a
// meal_event. Distinct from MealReaction (who reacted and how) — a person can
// attend without reacting, and a reaction can exist without recorded attendance.
type MealParticipant struct {
	ID         int64
	MealEventID int64
	PersonID   string
	CreatedAt  time.Time
}

// MealReview is a person's considered, post-meal rating of one specific
// meal_event instance (1..5). It is a sibling of MealReaction, not a
// replacement: reaction captures a quick directional feeling (-2..2) at
// serving time; review captures a considered opinion after the meal.
//
// Recipe-level rating is an aggregate computed from MealReview rows, not
// stored as a denormalized column on any entity.
type MealReview struct {
	ID          int64
	MealEventID int64
	PersonID    string
	Rating      int // 1..5
	Note        string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// FavoriteRating is the read-side aggregate of MealReview rows for a recipe.
// Computed on read; never cached. Weighted by each reviewer's EffectiveWeight.
type FavoriteRating struct {
	MealieRecipeID string
	Average        float64 // weighted mean, 1.0..5.0
	ReviewCount    int     // number of reviews in the aggregate
}

// Favorite is an explicit, person- or household-scoped preference marker on a
// recipe. Created only by an explicit action — never derived from ratings or
// reactions. Exactly one of PersonID or HouseholdID is non-empty.
type Favorite struct {
	ID             int64
	PersonID       string // empty when household-scoped
	HouseholdID    string // empty when person-scoped
	MealieRecipeID string
	CreatedAt      time.Time
}

// IsPersonScoped returns true when this favorite is scoped to a person rather
// than a household.
func (f Favorite) IsPersonScoped() bool { return f.PersonID != "" }

// IsHouseholdScoped returns true when this favorite is scoped to a household
// rather than a person.
func (f Favorite) IsHouseholdScoped() bool { return f.HouseholdID != "" }
