package persistence

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store wraps the Postgres connection pool and exposes the repository methods
// this package owns. It is the only persistence entry point the rest of the
// module uses; cmd/food-brain constructs it once and passes it down.
//
// All operations run outside an explicit transaction here; callers that need
// multi-statement atomicity should use a pgx.Tx obtained from the pool.
type Store struct {
	db *pgxpool.Pool
}

// New opens a pooled connection to Postgres using cfg.
func New(ctx context.Context, cfg Config) (*Store, error) {
	pool, err := NewPool(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &Store{db: pool}, nil
}

// Close releases the connection pool.
func (s *Store) Close() { s.db.Close() }

// Ping verifies connectivity.
func (s *Store) Ping(ctx context.Context) error { return s.db.Ping(ctx) }

// MealPlan is a planned week (one row of migrations/0001_init.sql meal_plan).
type MealPlan struct {
	ID        int64
	WeekStart time.Time // PostgreSQL DATE → Go time.Time at midnight UTC
	Status    string    // 'draft' | 'approved' | 'archived'
	CreatedAt time.Time
}

// CreateMealPlan inserts a plan for weekStart and returns its id. The UNIQUE
// week_start constraint makes this idempotent: on conflict the no-op DO UPDATE
// still yields a row, so the existing plan's id is returned rather than erroring.
func (s *Store) CreateMealPlan(ctx context.Context, weekStart time.Time) (int64, error) {
	const q = `INSERT INTO meal_plan (week_start, status) VALUES ($1, 'draft')
		ON CONFLICT (week_start) DO UPDATE SET week_start = EXCLUDED.week_start RETURNING id`
	var id int64
	err := s.db.QueryRow(ctx, q, weekStart).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("persistence: create meal_plan: %w", err)
	}
	return id, nil
}

// GetMealPlan fetches a plan by id.
func (s *Store) GetMealPlan(ctx context.Context, id int64) (MealPlan, error) {
	const q = `SELECT id, week_start, status, created_at FROM meal_plan WHERE id = $1`
	var m MealPlan
	if err := s.db.QueryRow(ctx, q, id).Scan(&m.ID, &m.WeekStart, &m.Status, &m.CreatedAt); err != nil {
		return MealPlan{}, fmt.Errorf("persistence: get meal_plan: %w", err)
	}
	return m, nil
}

// GetMealPlanByWeek fetches the plan for a given week's Monday.
func (s *Store) GetMealPlanByWeek(ctx context.Context, weekStart time.Time) (MealPlan, error) {
	const q = `SELECT id, week_start, status, created_at FROM meal_plan WHERE week_start = $1`
	var m MealPlan
	if err := s.db.QueryRow(ctx, q, weekStart).Scan(&m.ID, &m.WeekStart, &m.Status, &m.CreatedAt); err != nil {
		return MealPlan{}, fmt.Errorf("persistence: get meal_plan by week: %w", err)
	}
	return m, nil
}

// GetOrCreateMealPlan returns the plan row for weekStart, creating it as 'draft'
// if none exists yet (idempotent). This is the entry point the planner (2.3) uses
// to anchor candidates, decisions and shopping requirements to a plan.
func (s *Store) GetOrCreateMealPlan(ctx context.Context, weekStart time.Time) (MealPlan, error) {
	m, err := s.GetMealPlanByWeek(ctx, weekStart)
	if err == nil {
		return m, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return MealPlan{}, err
	}
	id, err := s.CreateMealPlan(ctx, weekStart)
	if err != nil {
		return MealPlan{}, err
	}
	return s.GetMealPlan(ctx, id)
}

// SetMealPlanStatus updates a plan's status.
func (s *Store) SetMealPlanStatus(ctx context.Context, id int64, status string) error {
	const q = `UPDATE meal_plan SET status = $1 WHERE id = $2`
	c, err := s.db.Exec(ctx, q, status, id)
	if err != nil {
		return fmt.Errorf("persistence: set meal_plan status: %w", err)
	}
	if c.RowsAffected() == 0 {
		return notFound("meal_plan", id)
	}
	return nil
}

// MealPlanCandidate mirrors migrations/0001_init.sql meal_plan_candidate.
type MealPlanCandidate struct {
	ID             int64
	PlanID         int64
	SlotDate       time.Time
	MealieRecipeID string
	Score          float64
	Breakdown      map[string]float64 // JSONB; nil-safe
	Feasible       bool
	Rank           int
}

// InsertCandidate writes one ranked candidate for a plan/slot.
func (s *Store) InsertCandidate(ctx context.Context, c MealPlanCandidate) error {
	breakdown := c.Breakdown
	if breakdown == nil {
		breakdown = map[string]float64{}
	}
	const q = `INSERT INTO meal_plan_candidate
		(plan_id, slot_date, mealie_recipe_id, score, breakdown, feasible, rank)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`
	if _, err := s.db.Exec(ctx, q, c.PlanID, c.SlotDate, c.MealieRecipeID, c.Score, breakdown, c.Feasible, c.Rank); err != nil {
		return fmt.Errorf("persistence: insert candidate: %w", err)
	}
	return nil
}

// ListCandidates returns a plan's candidates ordered by (slot_date, rank).
func (s *Store) ListCandidates(ctx context.Context, planID int64) ([]MealPlanCandidate, error) {
	const q = `SELECT id, plan_id, slot_date, mealie_recipe_id, score, breakdown, feasible, rank
		FROM meal_plan_candidate WHERE plan_id = $1 ORDER BY slot_date, rank`
	rows, err := s.db.Query(ctx, q, planID)
	if err != nil {
		return nil, fmt.Errorf("persistence: list candidates: %w", err)
	}
	defer rows.Close()
	var out []MealPlanCandidate
	for rows.Next() {
		var c MealPlanCandidate
		var breakdown map[string]float64
		if err := rows.Scan(&c.ID, &c.PlanID, &c.SlotDate, &c.MealieRecipeID, &c.Score, &breakdown, &c.Feasible, &c.Rank); err != nil {
			return nil, err
		}
		c.Breakdown = breakdown
		out = append(out, c)
	}
	return out, rows.Err()
}

// MealPlanDecision mirrors migrations/0001_init.sql meal_plan_decision.
type MealPlanDecision struct {
	PlanID         int64
	SlotDate       time.Time
	MealieRecipeID string
	DecidedAt      time.Time
}

// SetDecision upserts the chosen recipe for a plan/slot.
func (s *Store) SetDecision(ctx context.Context, d MealPlanDecision) error {
	const q = `INSERT INTO meal_plan_decision (plan_id, slot_date, mealie_recipe_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (plan_id, slot_date) DO UPDATE SET mealie_recipe_id = EXCLUDED.mealie_recipe_id`
	if _, err := s.db.Exec(ctx, q, d.PlanID, d.SlotDate, d.MealieRecipeID); err != nil {
		return fmt.Errorf("persistence: set decision: %w", err)
	}
	return nil
}

// ListDecisions returns a plan's decisions ordered by slot_date.
func (s *Store) ListDecisions(ctx context.Context, planID int64) ([]MealPlanDecision, error) {
	const q = `SELECT plan_id, slot_date, mealie_recipe_id, decided_at
		FROM meal_plan_decision WHERE plan_id = $1 ORDER BY slot_date`
	rows, err := s.db.Query(ctx, q, planID)
	if err != nil {
		return nil, fmt.Errorf("persistence: list decisions: %w", err)
	}
	defer rows.Close()
	var out []MealPlanDecision
	for rows.Next() {
		var d MealPlanDecision
		if err := rows.Scan(&d.PlanID, &d.SlotDate, &d.MealieRecipeID, &d.DecidedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ShoppingRequirement mirrors migrations/0001_init.sql shopping_requirement.
// (Distinct from the retailer-independent domain.ShoppingRequirement value:
// this struct carries the persistence row's id and plan_id.)
type ShoppingRequirement struct {
	ID              int64
	PlanID          int64
	IngredientID    string
	Quantity        float64
	Unit            string
	AcceptableForms []string
	PreferredForm   *string
}

// InsertShoppingRequirement inserts requirements derived from a plan's
// decisions. Duplicate (plan_id, ingredient_id) rows merge by summing
// quantity, matching the schema's UNIQUE constraint.
func (s *Store) InsertShoppingRequirement(ctx context.Context, r ShoppingRequirement) error {
	const q = `INSERT INTO shopping_requirement
		(plan_id, ingredient_id, quantity, unit, acceptable_forms, preferred_form)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (plan_id, ingredient_id) DO UPDATE
		SET quantity = shopping_requirement.quantity + EXCLUDED.quantity`
	forms := r.AcceptableForms
	if forms == nil {
		forms = []string{}
	}
	if _, err := s.db.Exec(ctx, q, r.PlanID, r.IngredientID, r.Quantity, r.Unit, forms, r.PreferredForm); err != nil {
		return fmt.Errorf("persistence: insert shopping_requirement: %w", err)
	}
	return nil
}

// ListShoppingRequirements returns a plan's requirements, sorted.
func (s *Store) ListShoppingRequirements(ctx context.Context, planID int64) ([]ShoppingRequirement, error) {
	const q = `SELECT id, plan_id, ingredient_id, quantity, unit, acceptable_forms, preferred_form
		FROM shopping_requirement WHERE plan_id = $1 ORDER BY ingredient_id, unit`
	rows, err := s.db.Query(ctx, q, planID)
	if err != nil {
		return nil, fmt.Errorf("persistence: list shopping_requirements: %w", err)
	}
	defer rows.Close()
	var out []ShoppingRequirement
	for rows.Next() {
		var r ShoppingRequirement
		if err := rows.Scan(&r.ID, &r.PlanID, &r.IngredientID, &r.Quantity, &r.Unit, &r.AcceptableForms, &r.PreferredForm); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func notFound(table string, id int64) error {
	return fmt.Errorf("persistence: %s not found (id %s)", table, strconv.FormatInt(id, 10))
}
