package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/androidand/spisordning/internal/domain"
)

// MealEvent mirrors migrations/0001_init.sql meal_event plus the plan link
// added by migrations/0010_meals_and_preferences.sql.
type MealEvent struct {
	ID               domain.MealEventID
	RecipeRefID      domain.RecipeRefID
	ServedOn         time.Time
	MealPlanID       *domain.MealPlanID // nil for ad-hoc meals
	MealPlanSlotDate *time.Time         // nil for ad-hoc meals; paired with MealPlanID
	MealPlanSlotKind *string            // nil for ad-hoc meals; paired with MealPlanID
	CreatedAt        time.Time
}

// CreateMealEvent records that a recipe was served on a day. When planID and
// planSlotDate are both non-nil they form a composite FK to the specific
// meal_plan_decision row that produced this meal; both must be nil for ad-hoc
// (unplanned) meals.
func (s *Store) CreateMealEvent(ctx context.Context, recipeRefID domain.RecipeRefID, servedOn time.Time, planID *domain.MealPlanID, planSlotDate *time.Time) (domain.MealEventID, error) {
	id := domain.NewMealEventID()
	const q = `INSERT INTO meal_event (id, recipe_ref_id, served_on, meal_plan_id, meal_plan_slot_date)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`
	var returnedID domain.MealEventID
	if err := s.db.QueryRow(ctx, q, id, recipeRefID, servedOn, planID, planSlotDate).Scan(&returnedID); err != nil {
		return domain.MealEventID{}, fmt.Errorf("persistence: create meal_event: %w", err)
	}
	return returnedID, nil
}

// CreateMealEventWithSlot records that a recipe was served on a day, with an
// explicit slot_kind. When planID and planSlotDate are both non-nil they form
// a composite FK to the specific meal_plan_decision row that produced this meal.
func (s *Store) CreateMealEventWithSlot(ctx context.Context, recipeRefID domain.RecipeRefID, servedOn time.Time, planID *domain.MealPlanID, planSlotDate *time.Time, planSlotKind *string) (domain.MealEventID, error) {
	id := domain.NewMealEventID()
	const q = `INSERT INTO meal_event (id, recipe_ref_id, served_on, meal_plan_id, meal_plan_slot_date, meal_plan_slot_kind)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`
	var returnedID domain.MealEventID
	if err := s.db.QueryRow(ctx, q, id, recipeRefID, servedOn, planID, planSlotDate, planSlotKind).Scan(&returnedID); err != nil {
		return domain.MealEventID{}, fmt.Errorf("persistence: create meal_event with slot: %w", err)
	}
	return returnedID, nil
}

// MealReaction mirrors migrations/0001_init.sql meal_reaction.
type MealReaction struct {
	MealEventID domain.MealEventID
	PersonID    domain.PersonID
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
func (s *Store) ListMealReactions(ctx context.Context, eventID domain.MealEventID) ([]MealReaction, error) {
	rows, err := s.db.Query(ctx, `SELECT meal_event_id, person_id, sentiment, note, created_at
		FROM meal_reaction WHERE meal_event_id = $1 ORDER BY created_at`, eventID)
	if err != nil {
		return nil, fmt.Errorf("persistence: list meal_reactions: %w", err)
	}
	defer rows.Close()
	var out []MealReaction
	for rows.Next() {
		var r MealReaction
		var note *string
		if err := rows.Scan(&r.MealEventID, &r.PersonID, &r.Sentiment, &note, &r.CreatedAt); err != nil {
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
	MealEventID domain.MealEventID
	PersonID    domain.PersonID
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
func (s *Store) ListMealParticipants(ctx context.Context, eventID domain.MealEventID) ([]MealParticipant, error) {
	rows, err := s.db.Query(ctx, `SELECT meal_event_id, person_id, created_at
		FROM meal_participant WHERE meal_event_id = $1 ORDER BY created_at`, eventID)
	if err != nil {
		return nil, fmt.Errorf("persistence: list meal_participants: %w", err)
	}
	defer rows.Close()
	var out []MealParticipant
	for rows.Next() {
		var p MealParticipant
		if err := rows.Scan(&p.MealEventID, &p.PersonID, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// MealReview mirrors migrations/0010_meals_and_preferences.sql meal_review.
type MealReview struct {
	MealEventID domain.MealEventID
	PersonID    domain.PersonID
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
func (s *Store) ListMealReviews(ctx context.Context, eventID domain.MealEventID) ([]MealReview, error) {
	rows, err := s.db.Query(ctx, `SELECT meal_event_id, person_id, rating, note, created_at, updated_at
		FROM meal_review WHERE meal_event_id = $1 ORDER BY created_at`, eventID)
	if err != nil {
		return nil, fmt.Errorf("persistence: list meal_reviews: %w", err)
	}
	defer rows.Close()
	var out []MealReview
	for rows.Next() {
		var r MealReview
		var note *string
		if err := rows.Scan(&r.MealEventID, &r.PersonID, &r.Rating, &note, &r.CreatedAt, &r.UpdatedAt); err != nil {
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
	RecipeRefID domain.RecipeRefID
	Average     float64
	ReviewCount int
}

// GetRecipeRating computes the weighted average rating for a recipe from its
// meal_review history. Weighted by each reviewer's person.weight. Returns zero
// values when the recipe has no reviews.
func (s *Store) GetRecipeRating(ctx context.Context, recipeRefID domain.RecipeRefID) (RecipeRating, error) {
	const q = `
		SELECT COALESCE(SUM(p.weight * mr.rating) / NULLIF(SUM(p.weight), 0), 0)::double precision,
		       COUNT(*)
		FROM meal_review mr
		JOIN meal_event me ON me.id = mr.meal_event_id
		JOIN person p ON p.id = mr.person_id
		WHERE me.recipe_ref_id = $1`
	var rating RecipeRating
	rating.RecipeRefID = recipeRefID
	if err := s.db.QueryRow(ctx, q, recipeRefID).Scan(&rating.Average, &rating.ReviewCount); err != nil {
		return RecipeRating{}, fmt.Errorf("persistence: get recipe rating: %w", err)
	}
	return rating, nil
}

// Favorite mirrors migrations/0010_meals_and_preferences.sql favorite.
// Uses a bounded scope_type/scope_id discriminator (D7).
type Favorite struct {
	ScopeType   string // 'person' | 'household'
	ScopeID     string
	RecipeRefID domain.RecipeRefID
	CreatedAt   time.Time
}

// UpsertFavorite adds or replaces a person- or household-scoped favorite.
// scopeType must be 'person' or 'household'; scopeID is the corresponding UUID.
func (s *Store) UpsertFavorite(ctx context.Context, scopeType, scopeID string, recipeRefID domain.RecipeRefID) error {
	if scopeType != "person" && scopeType != "household" {
		return fmt.Errorf("persistence: favorite: scope_type must be 'person' or 'household'")
	}
	if scopeID == "" {
		return fmt.Errorf("persistence: favorite: scope_id must be set")
	}
	const q = `INSERT INTO favorite (scope_type, scope_id, recipe_ref_id) VALUES ($1, $2, $3)
		ON CONFLICT (scope_type, scope_id, recipe_ref_id) DO NOTHING`
	if _, err := s.db.Exec(ctx, q, scopeType, scopeID, recipeRefID); err != nil {
		return fmt.Errorf("persistence: upsert favorite: %w", err)
	}
	return nil
}

// DeleteFavorite removes a person- or household-scoped favorite.
// scopeType must be 'person' or 'household'; scopeID is the corresponding UUID.
func (s *Store) DeleteFavorite(ctx context.Context, scopeType, scopeID string, recipeRefID domain.RecipeRefID) error {
	if scopeType != "person" && scopeType != "household" {
		return fmt.Errorf("persistence: delete favorite: scope_type must be 'person' or 'household'")
	}
	if scopeID == "" {
		return fmt.Errorf("persistence: delete favorite: scope_id must be set")
	}
	const q = `DELETE FROM favorite WHERE scope_type = $1 AND scope_id = $2 AND recipe_ref_id = $3`
	if _, err := s.db.Exec(ctx, q, scopeType, scopeID, recipeRefID); err != nil {
		return fmt.Errorf("persistence: delete favorite: %w", err)
	}
	return nil
}

// ListFavoritesForRecipe returns all favorites (person and household scoped)
// for a given recipe.
func (s *Store) ListFavoritesForRecipe(ctx context.Context, recipeRefID domain.RecipeRefID) ([]Favorite, error) {
	rows, err := s.db.Query(ctx, `SELECT scope_type, scope_id, recipe_ref_id, created_at
		FROM favorite WHERE recipe_ref_id = $1 ORDER BY created_at`, recipeRefID)
	if err != nil {
		return nil, fmt.Errorf("persistence: list favorites: %w", err)
	}
	defer rows.Close()
	var out []Favorite
	for rows.Next() {
		var f Favorite
		if err := rows.Scan(&f.ScopeType, &f.ScopeID, &f.RecipeRefID, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// GetMealEvent fetches a meal event by id.
func (s *Store) GetMealEvent(ctx context.Context, id domain.MealEventID) (MealEvent, error) {
	const q = `SELECT id, recipe_ref_id, served_on, meal_plan_id, meal_plan_slot_date, meal_plan_slot_kind, created_at
		FROM meal_event WHERE id = $1`
	var m MealEvent
	var planID *domain.MealPlanID
	var planSlot *time.Time
	var planSlotKind *string
	if err := s.db.QueryRow(ctx, q, id).Scan(&m.ID, &m.RecipeRefID, &m.ServedOn, &planID, &planSlot, &planSlotKind, &m.CreatedAt); err != nil {
		return MealEvent{}, fmt.Errorf("persistence: get meal_event: %w", err)
	}
	m.MealPlanID = planID
	m.MealPlanSlotDate = planSlot
	m.MealPlanSlotKind = planSlotKind
	return m, nil
}

// ListMealEvents returns meal events optionally filtered by recipeRefID
// and/or servedOn (date-only), ordered by served_on descending.
func (s *Store) ListMealEvents(ctx context.Context, recipeRefID domain.RecipeRefID, servedOn string) ([]MealEvent, error) {
	var q string
	var args []interface{}
	if recipeRefID != (domain.RecipeRefID{}) && servedOn != "" {
		q = `SELECT id, recipe_ref_id, served_on, meal_plan_id, meal_plan_slot_date, meal_plan_slot_kind, created_at
			 FROM meal_event WHERE recipe_ref_id = $1 AND served_on = $2 ORDER BY served_on DESC`
		args = []interface{}{recipeRefID, servedOn}
	} else if recipeRefID != (domain.RecipeRefID{}) {
		q = `SELECT id, recipe_ref_id, served_on, meal_plan_id, meal_plan_slot_date, meal_plan_slot_kind, created_at
			 FROM meal_event WHERE recipe_ref_id = $1 ORDER BY served_on DESC`
		args = []interface{}{recipeRefID}
	} else if servedOn != "" {
		q = `SELECT id, recipe_ref_id, served_on, meal_plan_id, meal_plan_slot_date, meal_plan_slot_kind, created_at
			 FROM meal_event WHERE served_on = $1 ORDER BY served_on DESC`
		args = []interface{}{servedOn}
	} else {
		q = `SELECT id, recipe_ref_id, served_on, meal_plan_id, meal_plan_slot_date, meal_plan_slot_kind, created_at
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
		var planID *domain.MealPlanID
		var planSlot *time.Time
		var planSlotKind *string
		if err := rows.Scan(&m.ID, &m.RecipeRefID, &m.ServedOn, &planID, &planSlot, &planSlotKind, &m.CreatedAt); err != nil {
			return nil, err
		}
		if planID != nil {
			m.MealPlanID = planID
		}
		if planSlot != nil {
			m.MealPlanSlotDate = planSlot
		}
		m.MealPlanSlotKind = planSlotKind
		out = append(out, m)
	}
	return out, rows.Err()
}

// TonightMeal is the recipe info + reactions for today's planned meal.
type TonightMeal struct {
	ServedOn       time.Time
	RecipeRefID    domain.RecipeRefID
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
		SELECT me.served_on, me.recipe_ref_id, rr.mealie_recipe_id,
		       rr.title, rr.tags, rr.effort,
		       mr.meal_event_id, mr.person_id, mr.sentiment, mr.note, mr.created_at
		FROM meal_plan mp
		JOIN meal_plan_decision mpd ON mpd.plan_id = mp.id
		JOIN meal_event me ON me.served_on = mpd.slot_date AND me.recipe_ref_id = mpd.recipe_ref_id
			AND (me.meal_plan_slot_kind IS NULL OR me.meal_plan_slot_kind = mpd.slot_kind)
		JOIN recipe_ref rr ON rr.id = me.recipe_ref_id
		LEFT JOIN meal_reaction mr ON mr.meal_event_id = me.id
		WHERE mp.week_start = (SELECT week_start FROM meal_plan WHERE week_start <= $1 ORDER BY week_start DESC LIMIT 1)
		  AND mp.status = 'approved'
		  AND mpd.slot_date = $1
		  AND mpd.slot_kind = 'dinner'
		ORDER BY mr.meal_event_id, mr.person_id`
	rows, err := s.db.Query(ctx, q, today)
	if err != nil {
		return TonightMeal{}, fmt.Errorf("persistence: get tonight meal: %w", err)
	}
	defer rows.Close()

	var meal TonightMeal
	var recipeTags []string
	var reactions []MealReaction
	seenReactions := make(map[string]bool)

	for rows.Next() {
		var servedOn time.Time
		var recipeRefID domain.RecipeRefID
		var mealieRecipeID string
		var title string
		var tags []string
		var effort int
		var reactionEventID *domain.MealEventID
		var personID *domain.PersonID
		var sentiment *int
		var note *string
		var createdAt time.Time

		if err := rows.Scan(&servedOn, &recipeRefID, &mealieRecipeID, &title, &tags, &effort,
			&reactionEventID, &personID, &sentiment, &note, &createdAt); err != nil {
			return TonightMeal{}, err
		}

		if meal.ServedOn.IsZero() {
			meal.ServedOn = servedOn
			meal.RecipeRefID = recipeRefID
			meal.MealieRecipeID = mealieRecipeID
			meal.RecipeTitle = title
			recipeTags = tags
			meal.RecipeEffort = effort
		} else if meal.RecipeRefID != recipeRefID {
			continue
		}

		if reactionEventID != nil && sentiment != nil {
			key := reactionEventID.String() + ":" + (func() string { if personID != nil { return personID.String() }; return "" })()
			if !seenReactions[key] {
				seenReactions[key] = true
				r := MealReaction{
					MealEventID: *reactionEventID,
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
func (s *Store) CreateReaction(ctx context.Context, eventID domain.MealEventID, personID string, sentiment int, note *string) (MealReaction, error) {
	noteVal := ""
	if note != nil {
		noteVal = *note
	}
	const q = `INSERT INTO meal_reaction (meal_event_id, person_id, sentiment, note)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (meal_event_id, person_id) DO UPDATE SET sentiment = EXCLUDED.sentiment,
			note = EXCLUDED.note
		RETURNING meal_event_id, person_id, sentiment, note, created_at`
	var r MealReaction
	if err := s.db.QueryRow(ctx, q, eventID, personID, sentiment, noteVal).Scan(
		&r.MealEventID, &r.PersonID, &r.Sentiment, &r.Note, &r.CreatedAt); err != nil {
		return MealReaction{}, fmt.Errorf("persistence: create reaction: %w", err)
	}
	return r, nil
}

// GetOrCreateMealEventForToday returns the meal event ID for today's recipe,
// creating it if none exists yet. This is the entry point for one-tap reactions.
func (s *Store) GetOrCreateMealEventForToday(ctx context.Context, recipeRefID domain.RecipeRefID, today time.Time) (domain.MealEventID, error) {
	const q = `SELECT id FROM meal_event WHERE recipe_ref_id = $1 AND served_on = $2 LIMIT 1`
	var id domain.MealEventID
	err := s.db.QueryRow(ctx, q, recipeRefID, today).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.MealEventID{}, fmt.Errorf("persistence: find meal event: %w", err)
	}
	id, err = s.CreateMealEvent(ctx, recipeRefID, today, nil, nil)
	if err != nil {
		return domain.MealEventID{}, fmt.Errorf("persistence: create meal event: %w", err)
	}
	return id, nil
}

// GetMealEventWithReactions fetches a meal event and all its reactions.
func (s *Store) GetMealEventWithReactions(ctx context.Context, eventID domain.MealEventID) (MealEvent, []MealReaction, error) {
	const q = `SELECT id, recipe_ref_id, served_on, created_at FROM meal_event WHERE id = $1`
	var evt MealEvent
	if err := s.db.QueryRow(ctx, q, eventID).Scan(&evt.ID, &evt.RecipeRefID, &evt.ServedOn, &evt.CreatedAt); err != nil {
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
	ID     string
	Kind   string
	Value  string
	Active bool
}

// CreatePlanningConstraint inserts one.
func (s *Store) CreatePlanningConstraint(ctx context.Context, c PlanningConstraint) (string, error) {
	id := domain.NewPlanningConstraintID()
	const q = `INSERT INTO planning_constraint (id, kind, value, active) VALUES ($1, $2, $3, $4) RETURNING id`
	var returnedID string
	if err := s.db.QueryRow(ctx, q, id, c.Kind, c.Value, c.Active).Scan(&returnedID); err != nil {
		return "", fmt.Errorf("persistence: create constraint: %w", err)
	}
	return returnedID, nil
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
