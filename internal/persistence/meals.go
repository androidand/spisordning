package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// MealEvent mirrors migrations/0001_init.sql meal_event plus the plan link
// added by migrations/0010_meals_and_preferences.sql.
type MealEvent struct {
	ID               int64
	MealieRecipeID   string
	ServedOn         time.Time
	MealPlanID       *int64     // nil for ad-hoc meals
	MealPlanSlotDate *time.Time // nil for ad-hoc meals; paired with MealPlanID
	CreatedAt        time.Time
}

// CreateMealEvent records that a recipe was served on a day. When planID and
// planSlotDate are both non-nil they form a composite FK to the specific
// meal_plan_decision row that produced this meal; both must be nil for ad-hoc
// (unplanned) meals.
func (s *Store) CreateMealEvent(ctx context.Context, mealieRecipeID string, servedOn time.Time, planID *int64, planSlotDate *time.Time) (int64, error) {
	const q = `INSERT INTO meal_event (mealie_recipe_id, served_on, meal_plan_id, meal_plan_slot_date)
		VALUES ($1, $2, $3, $4) RETURNING id`
	var id int64
	if err := s.db.QueryRow(ctx, q, mealieRecipeID, servedOn, planID, planSlotDate).Scan(&id); err != nil {
		return 0, fmt.Errorf("persistence: create meal_event: %w", err)
	}
	return id, nil
}

// MealReaction mirrors migrations/0001_init.sql meal_reaction.
type MealReaction struct {
	ID          int64
	MealEventID int64
	PersonID    string
	Sentiment   int // -2..2
	Note        string
	CreatedAt   time.Time
}

// AddMealReaction records one person's reaction to a served meal. The UNIQUE
// (meal_event_id, person_id) makes this an upsert per person per meal.
func (s *Store) AddMealReaction(ctx context.Context, r MealReaction) error {
	const q = `INSERT INTO meal_reaction (meal_event_id, person_id, sentiment, note)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (meal_event_id, person_id) DO UPDATE SET sentiment = EXCLUDED.sentiment,
			note = EXCLUDED.note`
	if _, err := s.db.Exec(ctx, q, r.MealEventID, r.PersonID, r.Sentiment, r.Note); err != nil {
		return fmt.Errorf("persistence: add meal_reaction: %w", err)
	}
	return nil
}

// ListMealReactions returns all reactions for an event.
func (s *Store) ListMealReactions(ctx context.Context, eventID int64) ([]MealReaction, error) {
	rows, err := s.db.Query(ctx, `SELECT id, meal_event_id, person_id, sentiment, note, created_at
		FROM meal_reaction WHERE meal_event_id = $1 ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("persistence: list meal_reactions: %w", err)
	}
	defer rows.Close()
	var out []MealReaction
	for rows.Next() {
		var r MealReaction
		var note *string
		if err := rows.Scan(&r.ID, &r.MealEventID, &r.PersonID, &r.Sentiment, &note, &r.CreatedAt); err != nil {
			return nil, err
		}
		if note != nil {
			r.Note = *note
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// MealParticipant mirrors migrations/0010_meals_and_preferences.sql meal_participant.
type MealParticipant struct {
	ID          int64
	MealEventID int64
	PersonID    string
	CreatedAt   time.Time
}

// AddMealParticipant records that a person was present at a meal. The UNIQUE
// (meal_event_id, person_id) makes this an upsert per person per meal.
func (s *Store) AddMealParticipant(ctx context.Context, p MealParticipant) error {
	const q = `INSERT INTO meal_participant (meal_event_id, person_id) VALUES ($1, $2)
		ON CONFLICT (meal_event_id, person_id) DO NOTHING`
	if _, err := s.db.Exec(ctx, q, p.MealEventID, p.PersonID); err != nil {
		return fmt.Errorf("persistence: add meal_participant: %w", err)
	}
	return nil
}

// ListMealParticipants returns all participants for an event.
func (s *Store) ListMealParticipants(ctx context.Context, eventID int64) ([]MealParticipant, error) {
	rows, err := s.db.Query(ctx, `SELECT id, meal_event_id, person_id, created_at
		FROM meal_participant WHERE meal_event_id = $1 ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("persistence: list meal_participants: %w", err)
	}
	defer rows.Close()
	var out []MealParticipant
	for rows.Next() {
		var p MealParticipant
		if err := rows.Scan(&p.ID, &p.MealEventID, &p.PersonID, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// MealReview mirrors migrations/0010_meals_and_preferences.sql meal_review.
type MealReview struct {
	ID          int64
	MealEventID int64
	PersonID    string
	Rating      int // 1..5
	Note        string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// UpsertMealReview records or updates one person's review of one meal_event.
// The UNIQUE (meal_event_id, person_id) makes this an upsert per person per meal.
func (s *Store) UpsertMealReview(ctx context.Context, r MealReview) error {
	const q = `INSERT INTO meal_review (meal_event_id, person_id, rating, note)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (meal_event_id, person_id) DO UPDATE SET
			rating = EXCLUDED.rating,
			note = EXCLUDED.note,
			updated_at = now()`
	if _, err := s.db.Exec(ctx, q, r.MealEventID, r.PersonID, r.Rating, r.Note); err != nil {
		return fmt.Errorf("persistence: upsert meal_review: %w", err)
	}
	return nil
}

// ListMealReviews returns all reviews for an event.
func (s *Store) ListMealReviews(ctx context.Context, eventID int64) ([]MealReview, error) {
	rows, err := s.db.Query(ctx, `SELECT id, meal_event_id, person_id, rating, note, created_at, updated_at
		FROM meal_review WHERE meal_event_id = $1 ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("persistence: list meal_reviews: %w", err)
	}
	defer rows.Close()
	var out []MealReview
	for rows.Next() {
		var r MealReview
		var note *string
		if err := rows.Scan(&r.ID, &r.MealEventID, &r.PersonID, &r.Rating, &note, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		if note != nil {
			r.Note = *note
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RecipeRating is a read-side aggregate of MealReview rows for a recipe.
// Computed from all reviews across all meal_events that reference the recipe.
type RecipeRating struct {
	MealieRecipeID string
	Average        float64
	ReviewCount    int
}

// GetRecipeRating computes the weighted average rating for a recipe from its
// meal_review history. Weighted by each reviewer's person.weight. Returns zero
// values when the recipe has no reviews.
func (s *Store) GetRecipeRating(ctx context.Context, mealieRecipeID string) (RecipeRating, error) {
	const q = `
		SELECT COALESCE(SUM(p.weight * mr.rating) / NULLIF(SUM(p.weight), 0), 0)::double precision,
		       COUNT(*)
		FROM meal_review mr
		JOIN meal_event me ON me.id = mr.meal_event_id
		JOIN person p ON p.id = mr.person_id
		WHERE me.mealie_recipe_id = $1`
	var rating RecipeRating
	rating.MealieRecipeID = mealieRecipeID
	if err := s.db.QueryRow(ctx, q, mealieRecipeID).Scan(&rating.Average, &rating.ReviewCount); err != nil {
		return RecipeRating{}, fmt.Errorf("persistence: get recipe rating: %w", err)
	}
	return rating, nil
}

// Favorite mirrors migrations/0010_meals_and_preferences.sql favorite.
type Favorite struct {
	ID             int64
	PersonID       *string // nil when household-scoped
	HouseholdID    *string // nil when person-scoped
	MealieRecipeID string
	CreatedAt      time.Time
}

// UpsertFavorite adds or replaces a person- or household-scoped favorite.
// Exactly one of personID/householdID must be non-nil; they map to the two
// unique constraints on the table.
func (s *Store) UpsertFavorite(ctx context.Context, personID, householdID, mealieRecipeID string) error {
	if personID != "" && householdID != "" {
		return fmt.Errorf("persistence: favorite: exactly one of person_id/household_id must be set")
	}
	if personID == "" && householdID == "" {
		return fmt.Errorf("persistence: favorite: either person_id or household_id must be set")
	}
	var q string
	var args []interface{}
	if personID != "" {
		q = `INSERT INTO favorite (person_id, mealie_recipe_id) VALUES ($1, $2)
			ON CONFLICT (person_id, mealie_recipe_id) DO NOTHING`
		args = []interface{}{personID, mealieRecipeID}
	} else {
		q = `INSERT INTO favorite (household_id, mealie_recipe_id) VALUES ($1, $2)
			ON CONFLICT (household_id, mealie_recipe_id) DO NOTHING`
		args = []interface{}{householdID, mealieRecipeID}
	}
	if _, err := s.db.Exec(ctx, q, args...); err != nil {
		return fmt.Errorf("persistence: upsert favorite: %w", err)
	}
	return nil
}

// DeleteFavorite removes a person- or household-scoped favorite.
// Exactly one of personID/householdID must be non-empty; mirrors UpsertFavorite's
// validation.
func (s *Store) DeleteFavorite(ctx context.Context, personID, householdID, mealieRecipeID string) error {
	if personID != "" && householdID != "" {
		return fmt.Errorf("persistence: delete favorite: exactly one of person_id/household_id must be set")
	}
	if personID == "" && householdID == "" {
		return fmt.Errorf("persistence: delete favorite: either person_id or household_id must be set")
	}
	var q string
	var args []interface{}
	if personID != "" {
		q = `DELETE FROM favorite WHERE person_id = $1 AND mealie_recipe_id = $2`
		args = []interface{}{personID, mealieRecipeID}
	} else {
		q = `DELETE FROM favorite WHERE household_id = $1 AND mealie_recipe_id = $2`
		args = []interface{}{householdID, mealieRecipeID}
	}
	if _, err := s.db.Exec(ctx, q, args...); err != nil {
		return fmt.Errorf("persistence: delete favorite: %w", err)
	}
	return nil
}

// ListFavoritesForRecipe returns all favorites (person and household scoped)
// for a given recipe.
func (s *Store) ListFavoritesForRecipe(ctx context.Context, mealieRecipeID string) ([]Favorite, error) {
	rows, err := s.db.Query(ctx, `SELECT id, person_id, household_id, mealie_recipe_id, created_at
		FROM favorite WHERE mealie_recipe_id = $1 ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("persistence: list favorites: %w", err)
	}
	defer rows.Close()
	var out []Favorite
	for rows.Next() {
		var f Favorite
		if err := rows.Scan(&f.ID, &f.PersonID, &f.HouseholdID, &f.MealieRecipeID, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// GetMealEvent fetches a meal event by id.
func (s *Store) GetMealEvent(ctx context.Context, id int64) (MealEvent, error) {
	const q = `SELECT id, mealie_recipe_id, served_on, meal_plan_id, meal_plan_slot_date, created_at
		FROM meal_event WHERE id = $1`
	var m MealEvent
	if err := s.db.QueryRow(ctx, q, id).Scan(&m.ID, &m.MealieRecipeID, &m.ServedOn, nil, nil, &m.CreatedAt); err != nil {
		return MealEvent{}, fmt.Errorf("persistence: get meal_event: %w", err)
	}
	return m, nil
}

// ListMealEvents returns meal events optionally filtered by mealieRecipeID
// and/or servedOn (date-only), ordered by served_on descending.
func (s *Store) ListMealEvents(ctx context.Context, mealieRecipeID, servedOn string) ([]MealEvent, error) {
	var q string
	var args []interface{}
	if mealieRecipeID != "" && servedOn != "" {
		q = `SELECT id, mealie_recipe_id, served_on, meal_plan_id, meal_plan_slot_date, created_at
			 FROM meal_event WHERE mealie_recipe_id = $1 AND served_on = $2 ORDER BY served_on DESC`
		args = []interface{}{mealieRecipeID, servedOn}
	} else if mealieRecipeID != "" {
		q = `SELECT id, mealie_recipe_id, served_on, meal_plan_id, meal_plan_slot_date, created_at
			 FROM meal_event WHERE mealie_recipe_id = $1 ORDER BY served_on DESC`
		args = []interface{}{mealieRecipeID}
	} else if servedOn != "" {
		q = `SELECT id, mealie_recipe_id, served_on, meal_plan_id, meal_plan_slot_date, created_at
			 FROM meal_event WHERE served_on = $1 ORDER BY served_on DESC`
		args = []interface{}{servedOn}
	} else {
		q = `SELECT id, mealie_recipe_id, served_on, meal_plan_id, meal_plan_slot_date, created_at
			 FROM meal_event ORDER BY served_on DESC`
	}
	rows, err := s.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("persistence: list meal_events: %w", err)
	}
	defer rows.Close()
	return scanMealEvents(rows)
}

func scanMealEvents(rows pgx.Rows) ([]MealEvent, error) {
	defer rows.Close()
	var out []MealEvent
	for rows.Next() {
		var m MealEvent
		var planID *int64
		var planSlot *time.Time
		if err := rows.Scan(&m.ID, &m.MealieRecipeID, &m.ServedOn, &planID, &planSlot, &m.CreatedAt); err != nil {
			return nil, err
		}
		if planID != nil {
			m.MealPlanID = planID
		}
		if planSlot != nil {
			m.MealPlanSlotDate = planSlot
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// EffortProfile mirrors migrations/0001_init.sql effort_profile.
type EffortProfile struct {
	Weekday       int // 0=Sunday..6=Saturday
	KitchenEnergy int // 1..3
}

// UpsertEffortProfile sets the kitchen energy for a weekday.
func (s *Store) UpsertEffortProfile(ctx context.Context, e EffortProfile) error {
	const q = `INSERT INTO effort_profile (weekday, kitchen_energy)
		VALUES ($1, $2)
		ON CONFLICT (weekday) DO UPDATE SET kitchen_energy = EXCLUDED.kitchen_energy`
	if _, err := s.db.Exec(ctx, q, e.Weekday, e.KitchenEnergy); err != nil {
		return fmt.Errorf("persistence: upsert effort_profile: %w", err)
	}
	return nil
}

// PlanningConstraint mirrors migrations/0001_init.sql planning_constraint.
type PlanningConstraint struct {
	ID     int64
	Kind   string
	Value  string
	Active bool
}

// CreatePlanningConstraint inserts one.
func (s *Store) CreatePlanningConstraint(ctx context.Context, c PlanningConstraint) (int64, error) {
	const q = `INSERT INTO planning_constraint (kind, value, active) VALUES ($1, $2, $3) RETURNING id`
	var id int64
	if err := s.db.QueryRow(ctx, q, c.Kind, c.Value, c.Active).Scan(&id); err != nil {
		return 0, fmt.Errorf("persistence: create constraint: %w", err)
	}
	return id, nil
}
