// Package service implements the application service layer for spisordning.
// Services depend on persistence.Store for data access and on other services
// for cross-cutting concerns (e.g., Meals uses Preferences for reaction
// learning). They never import httpapi (the DI contract lives in httpapi) or
// cmd (the composition root).
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/androidand/spisordning/internal/httpapi"
	"github.com/androidand/spisordning/internal/persistence"
	"github.com/jackc/pgx/v5"
)

// Store is the subset of persistence.Store that services need. It is defined
// here so services depend on an interface rather than the concrete Store type,
// making unit testing possible with a fake implementation.
type Store interface {
	CreatePerson(ctx context.Context, p persistence.Person) error
	GetPerson(ctx context.Context, id string) (persistence.Person, error)
	ListPeople(ctx context.Context) ([]persistence.Person, error)
	UpsertPreference(ctx context.Context, p persistence.PersonPreference) error
	ListPreferences(ctx context.Context, personID string) ([]persistence.PersonPreference, error)
	RecordObservation(ctx context.Context, o persistence.PreferenceObservation) error
	ListRecipeRefs(ctx context.Context) ([]persistence.RecipeRef, error)
	GetRecipeRef(ctx context.Context, id string) (persistence.RecipeRef, error)
	CreateMealEvent(ctx context.Context, mealieRecipeID string, servedOn time.Time, planID *int64, planSlotDate *time.Time) (int64, error)
	AddMealReaction(ctx context.Context, r persistence.MealReaction) error
	ListMealReactions(ctx context.Context, eventID int64) ([]persistence.MealReaction, error)
	GetMealPlan(ctx context.Context, id int64) (persistence.MealPlan, error)
	GetOrCreateMealPlan(ctx context.Context, weekStart time.Time) (persistence.MealPlan, error)
	SetMealPlanStatus(ctx context.Context, id int64, status string) error
	InsertCandidate(ctx context.Context, c persistence.MealPlanCandidate) error
	ListCandidates(ctx context.Context, planID int64) ([]persistence.MealPlanCandidate, error)
	SetDecision(ctx context.Context, d persistence.MealPlanDecision) error
	ListDecisions(ctx context.Context, planID int64) ([]persistence.MealPlanDecision, error)
	InsertShoppingRequirement(ctx context.Context, r persistence.ShoppingRequirement) error
	ListShoppingRequirements(ctx context.Context, planID int64) ([]persistence.ShoppingRequirement, error)
	UpsertIngredientMapping(ctx context.Context, m persistence.IngredientMapping) error
	BeginTx(ctx context.Context) (pgx.Tx, error)
	CreateInventoryLocation(ctx context.Context, l persistence.InventoryLocation) error
	GetInventoryLocation(ctx context.Context, id string) (persistence.InventoryLocation, error)
	ListLotsUnderLocation(ctx context.Context, id string) ([]persistence.InventoryLot, error)
	ListInventoryLocations(ctx context.Context, householdID string) ([]persistence.InventoryLocation, error)
	RecordPurchase(ctx context.Context, ingredientID, productID, locationID string, quantity float64, unit string, bestBefore *time.Time, source string) (int64, error)
	RecordConsume(ctx context.Context, lotID int64, quantity float64, estimated bool, source string) error
	GetInventoryLot(ctx context.Context, id int64) (persistence.InventoryLot, error)
	ListMealEvents(ctx context.Context, mealieRecipeID, servedOn string) ([]persistence.MealEvent, error)
	GetMealEvent(ctx context.Context, id int64) (persistence.MealEvent, error)
	ListMealPlans(ctx context.Context) ([]persistence.MealPlan, error)
	GetIngredientMapping(ctx context.Context, mealieFoodID string) (persistence.IngredientMapping, error)
}

// txConn is the minimal transaction surface the Meals service needs.
// It is separate from pgx.Tx so tests can inject a fake without importing pgx.
type txConn interface {
	Exec(ctx context.Context, query string, args ...interface{}) (interface{}, error)
	QueryRow(ctx context.Context, query string, args ...interface{}) pgx.Row
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// People implements the PersonService interface defined in httpapi.
type People struct{ db Store }

// NewPeople returns a People service backed by db.
func NewPeople(db Store) *People { return &People{db: db} }

func (s *People) ListPeople(ctx context.Context) ([]httpapi.PersonResponse, error) {
	people, err := s.db.ListPeople(ctx)
	if err != nil {
		return nil, fmt.Errorf("service: list people: %w", err)
	}
	out := make([]httpapi.PersonResponse, 0, len(people))
	for _, p := range people {
		out = append(out, httpapi.PersonResponse{
			ID: p.ID, Name: p.Name, Weight: p.Weight, CreatedAt: p.CreatedAt,
		})
	}
	return out, nil
}

func (s *People) GetPerson(ctx context.Context, id string) (httpapi.PersonResponse, error) {
	p, err := s.db.GetPerson(ctx, id)
	if err != nil {
		return httpapi.PersonResponse{}, fmt.Errorf("service: get person: %w", err)
	}
	return httpapi.PersonResponse{
		ID: p.ID, Name: p.Name, Weight: p.Weight, CreatedAt: p.CreatedAt,
	}, nil
}

func (s *People) CreatePerson(ctx context.Context, in httpapi.PersonInput) (httpapi.PersonResponse, error) {
	weight := in.Weight
	if weight <= 0 {
		weight = 1.0
	}
	p := persistence.Person{
		ID:        generateID(),
		Name:      in.Name,
		Weight:    weight,
		CreatedAt: time.Now(),
	}
	if err := s.db.CreatePerson(ctx, p); err != nil {
		return httpapi.PersonResponse{}, fmt.Errorf("service: create person: %w", err)
	}
	return httpapi.PersonResponse{
		ID: p.ID, Name: p.Name, Weight: p.Weight, CreatedAt: p.CreatedAt,
	}, nil
}

// Preferences implements the PreferencesService interface defined in httpapi.
type Preferences struct{ db Store }

// NewPreferences returns a Preferences service backed by db.
func NewPreferences(db Store) *Preferences { return &Preferences{db: db} }

func (s *Preferences) ListPreferences(ctx context.Context, personID string) ([]httpapi.PersonPreferenceResponse, error) {
	prefs, err := s.db.ListPreferences(ctx, personID)
	if err != nil {
		return nil, fmt.Errorf("service: list preferences: %w", err)
	}
	out := make([]httpapi.PersonPreferenceResponse, 0, len(prefs))
	for _, p := range prefs {
		out = append(out, httpapi.PersonPreferenceResponse{
			PersonID: p.PersonID, Tag: p.Tag, Sentiment: int(p.Sentiment),
			Confidence: p.Confidence, UpdatedAt: p.UpdatedAt,
		})
	}
	return out, nil
}

// Recipes implements the RecipesService interface defined in httpapi.
type Recipes struct{ db Store }

// NewRecipes returns a Recipes service backed by db.
func NewRecipes(db Store) *Recipes { return &Recipes{db: db} }

func (s *Recipes) ListRecipes(ctx context.Context) ([]httpapi.RecipeRefResponse, error) {
	refs, err := s.db.ListRecipeRefs(ctx)
	if err != nil {
		return nil, fmt.Errorf("service: list recipes: %w", err)
	}
	out := make([]httpapi.RecipeRefResponse, 0, len(refs))
	for _, r := range refs {
		out = append(out, httpapi.RecipeRefResponse{
			MealieRecipeID: r.MealieRecipeID, Title: r.Title, Tags: r.Tags,
			Effort: r.Effort, LastSyncedAt: r.LastSyncedAt,
		})
	}
	return out, nil
}

// Meals implements the MealsService interface defined in httpapi.
type Meals struct {
	db    Store
	prefs httpapi.PreferencesService
}

// NewMeals returns a Meals service backed by db. prefs is used for reaction
// learning (recording preference observations); pass nil to skip.
func NewMeals(db Store, prefs httpapi.PreferencesService) *Meals {
	return &Meals{db: db, prefs: prefs}
}

func (s *Meals) CreateMealEvent(ctx context.Context, in httpapi.MealEventNew) (httpapi.MealEventResponse, error) {
	servedOn, err := time.Parse("2006-01-02", in.ServedOn)
	if err != nil {
		return httpapi.MealEventResponse{}, fmt.Errorf("service: meals create: invalid served_on %q: %w", in.ServedOn, err)
	}

	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		return httpapi.MealEventResponse{}, fmt.Errorf("service: meals create: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	const insertEventQ = `INSERT INTO meal_event (mealie_recipe_id, served_on) VALUES ($1, $2) RETURNING id`
	var eventID int64
	if err := tx.QueryRow(ctx, insertEventQ, in.MealieRecipeID, servedOn).Scan(&eventID); err != nil {
		return httpapi.MealEventResponse{}, fmt.Errorf("service: meals create: insert event: %w", err)
	}

	for _, rx := range in.Reactions {
		const insertRxQ = `INSERT INTO meal_reaction (meal_event_id, person_id, sentiment, note)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (meal_event_id, person_id) DO UPDATE SET sentiment = EXCLUDED.sentiment,
				note = EXCLUDED.note`
		if _, err := tx.Exec(ctx, insertRxQ, eventID, rx.PersonID, rx.Sentiment, ""); err != nil {
			return httpapi.MealEventResponse{}, fmt.Errorf("service: meals create: add reaction: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return httpapi.MealEventResponse{}, fmt.Errorf("service: meals create: commit tx: %w", err)
	}

	// Record preference observations outside the transaction — non-fatal.
	for _, rx := range in.Reactions {
		obs := persistence.PreferenceObservation{
			PersonID:   rx.PersonID,
			Tag:        "meal",
			Sentiment:  rx.Sentiment,
			Source:     "reaction",
			ObservedAt: time.Now(),
		}
		if err := s.db.RecordObservation(ctx, obs); err != nil {
			_ = err
		}
	}

	rxns, err := s.db.ListMealReactions(ctx, eventID)
	if err != nil {
		return httpapi.MealEventResponse{}, fmt.Errorf("service: meals create: read reactions: %w", err)
	}
	out := httpapi.MealEventResponse{
		ID: eventID, MealieRecipeID: in.MealieRecipeID,
		ServedOn:  in.ServedOn,
		CreatedAt: time.Now(),
		Reactions: make([]httpapi.MealReactionResponse, 0, len(rxns)),
	}
	for _, r := range rxns {
		out.Reactions = append(out.Reactions, httpapi.MealReactionResponse{
			PersonID: r.PersonID, Sentiment: r.Sentiment,
		})
	}
	return out, nil
}

func (s *Meals) GetMeal(ctx context.Context, id int64) (httpapi.MealEventResponse, error) {
	event, err := s.db.GetMealEvent(ctx, id)
	if err != nil {
		return httpapi.MealEventResponse{}, fmt.Errorf("service: get meal: %w", err)
	}
	rxns, err := s.db.ListMealReactions(ctx, event.ID)
	if err != nil {
		return httpapi.MealEventResponse{}, fmt.Errorf("service: get meal: list reactions: %w", err)
	}
	out := httpapi.MealEventResponse{
		ID: event.ID, MealieRecipeID: event.MealieRecipeID,
		ServedOn:  event.ServedOn.Format("2006-01-02"),
		CreatedAt: event.CreatedAt,
		Reactions: make([]httpapi.MealReactionResponse, 0, len(rxns)),
	}
	for _, r := range rxns {
		out.Reactions = append(out.Reactions, httpapi.MealReactionResponse{
			PersonID: r.PersonID, Sentiment: r.Sentiment,
		})
	}
	return out, nil
}

func (s *Meals) ListMeals(ctx context.Context, mealieRecipeID, servedOn string) ([]httpapi.MealEventResponse, error) {
	events, err := s.db.ListMealEvents(ctx, mealieRecipeID, servedOn)
	if err != nil {
		return nil, fmt.Errorf("service: list meals: %w", err)
	}
	out := make([]httpapi.MealEventResponse, 0, len(events))
	for _, event := range events {
		rxns, err := s.db.ListMealReactions(ctx, event.ID)
		if err != nil {
			return nil, fmt.Errorf("service: list meals: list reactions: %w", err)
		}
		resp := httpapi.MealEventResponse{
			ID: event.ID, MealieRecipeID: event.MealieRecipeID,
			ServedOn:  event.ServedOn.Format("2006-01-02"),
			CreatedAt: event.CreatedAt,
			Reactions: make([]httpapi.MealReactionResponse, 0, len(rxns)),
		}
		for _, r := range rxns {
			resp.Reactions = append(resp.Reactions, httpapi.MealReactionResponse{
				PersonID: r.PersonID, Sentiment: r.Sentiment,
			})
		}
		out = append(out, resp)
	}
	return out, nil
}

// generateID generates a deterministic 16-char hex id for tests.
func generateID() string {
	var b [8]byte
	for i := range b {
		b[i] = byte(i + 1)
	}
	return fmt.Sprintf("%016x", b)
}
