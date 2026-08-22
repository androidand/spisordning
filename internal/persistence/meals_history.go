package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// MealParticipant mirrors migrations/0012_meal_history.sql meal_participant.
type MealParticipant struct {
	ID          int64
	MealEventID int64
	PersonID    string
	CreatedAt   time.Time
}

// AddMealParticipant records that a person was present at a meal_event.
// Idempotent: safe to call multiple times for the same (event, person) pair.
func (s *Store) AddMealParticipant(ctx context.Context, eventID int64, personID string) error {
	const q = `INSERT INTO meal_participant (meal_event_id, person_id)
		VALUES ($1, $2)
		ON CONFLICT (meal_event_id, person_id) DO NOTHING`
	if _, err := s.db.Exec(ctx, q, eventID, personID); err != nil {
		return fmt.Errorf("persistence: add meal_participant: %w", err)
	}
	return nil
}

// ListMealParticipants returns all participants for a meal_event.
func (s *Store) ListMealParticipants(ctx context.Context, eventID int64) ([]MealParticipant, error) {
	rows, err := s.db.Query(ctx, `SELECT id, meal_event_id, person_id, created_at
		FROM meal_participant WHERE meal_event_id = $1 ORDER BY created_at`, eventID)
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

// MealReview mirrors migrations/0012_meal_history.sql meal_review.
type MealReview struct {
	ID          int64
	MealEventID int64
	PersonID    string
	Rating      int // 1..5
	Note        string
	CreatedAt   time.Time
}

// AddMealReview records one person's review of a meal_event. The UNIQUE
// (meal_event_id, person_id) makes this an upsert per person per meal.
func (s *Store) AddMealReview(ctx context.Context, r MealReview) error {
	const q = `INSERT INTO meal_review (meal_event_id, person_id, rating, note)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (meal_event_id, person_id) DO UPDATE SET
			rating = EXCLUDED.rating,
			note = EXCLUDED.note`
	if _, err := s.db.Exec(ctx, q, r.MealEventID, r.PersonID, r.Rating, r.Note); err != nil {
		return fmt.Errorf("persistence: add meal_review: %w", err)
	}
	return nil
}

// ListMealReviews returns all reviews for a meal_event.
func (s *Store) ListMealReviews(ctx context.Context, eventID int64) ([]MealReview, error) {
	rows, err := s.db.Query(ctx, `SELECT id, meal_event_id, person_id, rating, note, created_at
		FROM meal_review WHERE meal_event_id = $1 ORDER BY created_at`, eventID)
	if err != nil {
		return nil, fmt.Errorf("persistence: list meal_reviews: %w", err)
	}
	defer rows.Close()
	var out []MealReview
	for rows.Next() {
		var r MealReview
		var note *string
		if err := rows.Scan(&r.ID, &r.MealEventID, &r.PersonID, &r.Rating, &note, &r.CreatedAt); err != nil {
			return nil, err
		}
		if note != nil {
			r.Note = *note
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RecipeRating mirrors the read-side aggregate computed from meal_review rows.
// See internal/domain/meals.go RecipeRating.
type RecipeRating struct {
	MealieRecipeID string
	Average        float64
	ReviewCount    int
}

// GetRecipeRating computes the mean rating and review count for a recipe
// across all meal_event instances that used it. Computed on read; never
// cached in the database.
func (s *Store) GetRecipeRating(ctx context.Context, mealieRecipeID string) (RecipeRating, error) {
	const q = `
		SELECT COALESCE(AVG(mr.rating), 0)::DOUBLE PRECISION, COUNT(mr.id)
		FROM meal_review mr
		JOIN meal_event me ON me.id = mr.meal_event_id
		WHERE me.mealie_recipe_id = $1`
	var avg float64
	var count int
	if err := s.db.QueryRow(ctx, q, mealieRecipeID).Scan(&avg, &count); err != nil {
		return RecipeRating{}, fmt.Errorf("persistence: get recipe rating: %w", err)
	}
	return RecipeRating{
		MealieRecipeID: mealieRecipeID,
		Average:        avg,
		ReviewCount:    count,
	}, nil
}

// Favorite mirrors migrations/0012_meal_history.sql favorite.
type Favorite struct {
	ID             int64
	PersonID       string
	MealieRecipeID string
	CreatedAt      time.Time
}

// AddFavorite creates a person-scoped favorite for a recipe.
// Idempotent: safe to call multiple times for the same (person, recipe) pair.
func (s *Store) AddFavorite(ctx context.Context, personID, mealieRecipeID string) error {
	const q = `INSERT INTO favorite (person_id, mealie_recipe_id)
		VALUES ($1, $2)
		ON CONFLICT (person_id, mealie_recipe_id) DO NOTHING`
	if _, err := s.db.Exec(ctx, q, personID, mealieRecipeID); err != nil {
		return fmt.Errorf("persistence: add favorite: %w", err)
	}
	return nil
}

// RemoveFavorite deletes a person-scoped favorite for a recipe.
func (s *Store) RemoveFavorite(ctx context.Context, personID, mealieRecipeID string) error {
	const q = `DELETE FROM favorite WHERE person_id = $1 AND mealie_recipe_id = $2`
	if _, err := s.db.Exec(ctx, q, personID, mealieRecipeID); err != nil {
		return fmt.Errorf("persistence: remove favorite: %w", err)
	}
	return nil
}

// ListPersonFavorites returns all favorites for a person.
func (s *Store) ListPersonFavorites(ctx context.Context, personID string) ([]Favorite, error) {
	rows, err := s.db.Query(ctx, `SELECT id, person_id, mealie_recipe_id, created_at
		FROM favorite WHERE person_id = $1 ORDER BY created_at`, personID)
	if err != nil {
		return nil, fmt.Errorf("persistence: list person favorites: %w", err)
	}
	defer rows.Close()
	var out []Favorite
	for rows.Next() {
		var f Favorite
		if err := rows.Scan(&f.ID, &f.PersonID, &f.MealieRecipeID, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// ListRecipeFavorites returns all persons who have favorited a recipe.
func (s *Store) ListRecipeFavorites(ctx context.Context, mealieRecipeID string) ([]Favorite, error) {
	rows, err := s.db.Query(ctx, `SELECT id, person_id, mealie_recipe_id, created_at
		FROM favorite WHERE mealie_recipe_id = $1 ORDER BY created_at`, mealieRecipeID)
	if err != nil {
		return nil, fmt.Errorf("persistence: list recipe favorites: %w", err)
	}
	defer rows.Close()
	var out []Favorite
	for rows.Next() {
		var f Favorite
		if err := rows.Scan(&f.ID, &f.PersonID, &f.MealieRecipeID, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// IsPersonFavorite returns true if the person has favorited the recipe.
func (s *Store) IsPersonFavorite(ctx context.Context, personID, mealieRecipeID string) (bool, error) {
	const q = `SELECT 1 FROM favorite WHERE person_id = $1 AND mealie_recipe_id = $2 LIMIT 1`
	var id int64
	err := s.db.QueryRow(ctx, q, personID, mealieRecipeID).Scan(&id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("persistence: check favorite: %w", err)
	}
	return true, nil
}

// LinkMealEventToPlan sets the optional plan link on a meal_event. Both
// planID and planSlotDate must be non-nil together, or both nil.
// Passing a nil planID (and nil planSlotDate) clears an existing link.
// Mixed input (one nil, one non-nil) returns an error.
func (s *Store) LinkMealEventToPlan(ctx context.Context, eventID int64, planID *int64, planSlotDate *time.Time) error {
	if (planID == nil) != (planSlotDate == nil) {
		return fmt.Errorf("persistence: link meal_event to plan: plan_id and plan_slot_date must be both set or both nil")
	}
	var pID *int64
	var pDate *time.Time
	if planID != nil && planSlotDate != nil {
		pID = planID
		pDate = planSlotDate
	}
	const q = `UPDATE meal_event SET plan_id = $1, plan_slot_date = $2 WHERE id = $3`
	if _, err := s.db.Exec(ctx, q, pID, pDate, eventID); err != nil {
		return fmt.Errorf("persistence: link meal_event to plan: %w", err)
	}
	return nil
}

// GetMealEventPlanLink returns the optional plan link for a meal_event, or
// nil values if none is set.
func (s *Store) GetMealEventPlanLink(ctx context.Context, eventID int64) (planID *int64, planSlotDate *time.Time, err error) {
	const q = `SELECT plan_id, plan_slot_date FROM meal_event WHERE id = $1`
	if err := s.db.QueryRow(ctx, q, eventID).Scan(&planID, &planSlotDate); err != nil {
		return nil, nil, fmt.Errorf("persistence: get meal_event plan link: %w", err)
	}
	return planID, planSlotDate, nil
}
