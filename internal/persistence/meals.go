package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
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
		FROM meal_reaction WHERE meal_event_id = $1 ORDER BY created_at`, eventID)
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

// TonightMeal is the recipe info + reactions for today's planned meal.
type TonightMeal struct {
	ServedOn       time.Time
	MealieRecipeID string
	RecipeTitle    string
	RecipeTags     []string
	RecipeEffort   int
	Reactions      []MealReaction
}

// GetTonightMeal returns tonight's meal from the approved plan's decision for today,
// including the recipe reference and any reactions already recorded.
// Returns pgx.ErrNoRows when there is no approved plan with a decision for today.
func (s *Store) GetTonightMeal(ctx context.Context, today time.Time) (TonightMeal, error) {
	const q = `
		SELECT me.served_on, me.mealie_recipe_id,
		       rr.title, rr.tags, rr.effort,
		       mr.id, mr.person_id, mr.sentiment, mr.note, mr.created_at
		FROM meal_plan mp
		JOIN meal_plan_decision mpd ON mpd.plan_id = mp.id
		JOIN meal_event me ON me.served_on = mpd.slot_date AND me.mealie_recipe_id = mpd.mealie_recipe_id
		JOIN recipe_ref rr ON rr.mealie_recipe_id = me.mealie_recipe_id
		LEFT JOIN meal_reaction mr ON mr.meal_event_id = me.id
		WHERE mp.week_start = (SELECT week_start FROM meal_plan WHERE week_start <= $1 ORDER BY week_start DESC LIMIT 1)
		  AND mp.status = 'approved'
		  AND mpd.slot_date = $1
		ORDER BY mr.id`
	rows, err := s.db.Query(ctx, q, today)
	if err != nil {
		return TonightMeal{}, fmt.Errorf("persistence: get tonight meal: %w", err)
	}
	defer rows.Close()

	var meal TonightMeal
	var recipeTags []string
	var reactions []MealReaction
	var seenReactionIDs map[int64]bool

	for rows.Next() {
		var servedOn time.Time
		var mealieRecipeID string
		var title string
		var tags []string
		var effort int
		var reactionID *int64
		var personID *string
		var sentiment *int
		var note *string
		var createdAt time.Time

		if err := rows.Scan(&servedOn, &mealieRecipeID, &title, &tags, &effort,
			&reactionID, &personID, &sentiment, &note, &createdAt); err != nil {
			return TonightMeal{}, err
		}

		if meal.ServedOn.IsZero() {
			meal.ServedOn = servedOn
			meal.MealieRecipeID = mealieRecipeID
			meal.RecipeTitle = title
			recipeTags = tags
			meal.RecipeEffort = effort
			seenReactionIDs = make(map[int64]bool)
		} else if meal.MealieRecipeID != mealieRecipeID {
			// Should not happen with the query, but guard against it.
			continue
		}

		if reactionID != nil && !seenReactionIDs[*reactionID] {
			seenReactionIDs[*reactionID] = true
			r := MealReaction{
				ID:          *reactionID,
				MealEventID: 0, // not needed for the response
				Sentiment:   *sentiment,
				CreatedAt:   createdAt,
			}
			if note != nil {
				r.Note = *note
			}
			if personID != nil {
				r.PersonID = *personID
			}
			reactions = append(reactions, r)
		}
	}
	if err := rows.Err(); err != nil {
		return TonightMeal{}, fmt.Errorf("persistence: scan tonight meal: %w", err)
	}

	if meal.ServedOn.IsZero() {
		return TonightMeal{}, pgx.ErrNoRows
	}

	meal.RecipeTags = recipeTags
	meal.Reactions = reactions
	return meal, nil
}

// CreateReaction records one person's reaction to a specific meal event.
// Returns the created/updated reaction.
func (s *Store) CreateReaction(ctx context.Context, eventID int64, personID string, sentiment int, note *string) (MealReaction, error) {
	noteVal := ""
	if note != nil {
		noteVal = *note
	}
	const q = `INSERT INTO meal_reaction (meal_event_id, person_id, sentiment, note)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (meal_event_id, person_id) DO UPDATE SET sentiment = EXCLUDED.sentiment,
			note = EXCLUDED.note
		RETURNING id, meal_event_id, person_id, sentiment, note, created_at`
	var r MealReaction
	if err := s.db.QueryRow(ctx, q, eventID, personID, sentiment, noteVal).Scan(
		&r.ID, &r.MealEventID, &r.PersonID, &r.Sentiment, &r.Note, &r.CreatedAt); err != nil {
		return MealReaction{}, fmt.Errorf("persistence: create reaction: %w", err)
	}
	return r, nil
}

// GetOrCreateMealEventForToday returns the meal event ID for today's recipe,
// creating it if none exists yet. This is the entry point for one-tap reactions.
func (s *Store) GetOrCreateMealEventForToday(ctx context.Context, mealieRecipeID string, today time.Time) (int64, error) {
	const q = `SELECT id FROM meal_event WHERE mealie_recipe_id = $1 AND served_on = $2 LIMIT 1`
	var id int64
	err := s.db.QueryRow(ctx, q, mealieRecipeID, today).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf("persistence: find meal event: %w", err)
	}
	id, err = s.CreateMealEvent(ctx, mealieRecipeID, today)
	if err != nil {
		return 0, fmt.Errorf("persistence: create meal event: %w", err)
	}
	return id, nil
}

// GetMealEventWithReactions fetches a meal event and all its reactions.
func (s *Store) GetMealEventWithReactions(ctx context.Context, eventID int64) (MealEvent, []MealReaction, error) {
	const q = `SELECT id, mealie_recipe_id, served_on, created_at FROM meal_event WHERE id = $1`
	var evt MealEvent
	if err := s.db.QueryRow(ctx, q, eventID).Scan(&evt.ID, &evt.MealieRecipeID, &evt.ServedOn, &evt.CreatedAt); err != nil {
		return MealEvent{}, nil, fmt.Errorf("persistence: get meal event: %w", err)
	}
	rxns, err := s.ListMealReactions(ctx, eventID)
	if err != nil {
		return MealEvent{}, nil, fmt.Errorf("persistence: list reactions: %w", err)
	}
	return evt, rxns, nil
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

// ListPlanningConstraints returns all constraints ordered by id.
func (s *Store) ListPlanningConstraints(ctx context.Context) ([]PlanningConstraint, error) {
	const q = `SELECT id, kind, value, active FROM planning_constraint ORDER BY id`
	rows, err := s.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("persistence: list constraints: %w", err)
	}
	defer rows.Close()
	var out []PlanningConstraint
	for rows.Next() {
		var c PlanningConstraint
		if err := rows.Scan(&c.ID, &c.Kind, &c.Value, &c.Active); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListEffortProfiles returns all effort profiles ordered by weekday.
func (s *Store) ListEffortProfiles(ctx context.Context) ([]EffortProfile, error) {
	const q = `SELECT weekday, kitchen_energy FROM effort_profile ORDER BY weekday`
	rows, err := s.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("persistence: list effort_profiles: %w", err)
	}
	defer rows.Close()
	var out []EffortProfile
	for rows.Next() {
		var e EffortProfile
		if err := rows.Scan(&e.Weekday, &e.KitchenEnergy); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
