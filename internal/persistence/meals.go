package persistence

import (
	"context"
	"fmt"
	"time"
)

// MealEvent mirrors migrations/0001_init.sql meal_event.
type MealEvent struct {
	ID             int64
	MealieRecipeID string
	ServedOn       time.Time
	CreatedAt      time.Time
}

// CreateMealEvent records that a recipe was served on a day.
func (s *Store) CreateMealEvent(ctx context.Context, mealieRecipeID string, servedOn time.Time) (int64, error) {
	const q = `INSERT INTO meal_event (mealie_recipe_id, served_on) VALUES ($1, $2) RETURNING id`
	var id int64
	if err := s.db.QueryRow(ctx, q, mealieRecipeID, servedOn).Scan(&id); err != nil {
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
