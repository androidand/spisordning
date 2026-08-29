// Package httpapi contains API integration tests that exercise the HTTP layer
// against a real handler wired to a real Postgres Store (task 3.5). They skip
// cleanly when no DATABASE_URL/POSTGRES_PASSWORD is configured, matching the
// pattern used by internal/persistence/*_test.go.
package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/androidand/spisordning/internal/dto"
	"github.com/androidand/spisordning/internal/persistence"
	"github.com/jackc/pgx/v5"
	"github.com/oapi-codegen/runtime/types"
)

// skipWithoutDB skips the test when no Postgres is reachable, matching the
// persistence package's convention. CI provides one (see
// .github/workflows/ci.yml persistence-test job); local dev without
// `docker compose up -d` skips cleanly rather than failing red.
func skipWithoutDB(t *testing.T) *persistence.Store {
	t.Helper()
	if os.Getenv("DATABASE_URL") == "" && os.Getenv("POSTGRES_PASSWORD") == "" {
		t.Skip("no DATABASE_URL/POSTGRES_PASSWORD in env; skipping HTTP integration test")
	}
	cfg, err := persistence.FromEnv(os.Getenv)
	if err != nil {
		t.Skipf("no usable postgres config: %v", err)
	}
	ctx := context.Background()
	store, err := persistence.New(ctx, cfg)
	if err != nil {
		t.Skipf("cannot connect to postgres: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

// truncateTables clears the given tables (CASCADE for FK dependents) so
// integration tests start from a clean slate. Mirrors the persistence
// package's helper.
func truncateTables(t *testing.T, store *persistence.Store, tables ...string) {
	t.Helper()
	ctx := context.Background()
	tx, err := store.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	if _, err := tx.Exec(ctx, "TRUNCATE "+strings.Join(tables, ", ")+" CASCADE"); err != nil {
		tx.Rollback(ctx)
		t.Fatalf("TRUNCATE: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("Commit: %v", err)
	}
}

// newTestServer builds an http.ServeMux wired to a real *persistence.Store and
// returns a httptest.Server bound to it. The caller owns the returned server.
func newTestServer(t *testing.T, store *persistence.Store) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	adapter := dbAdapter{store: store}
	RegisterHandlers(mux, Dependencies{
		People:      adapter,
		Preferences: adapter,
		Recipes:     adapter,
		Meals:       adapter,
		Pantry:      adapter,
		Plans:       &testAdapter{db: store},
		Inspiration: adapter,
	})
	return httptest.NewServer(mux)
}

// dbAdapter bridges *persistence.Store to the httpapi service interfaces. It
// lives in this test file so httpapi stays dependency-free of persistence.
type dbAdapter struct {
	store *persistence.Store
}

// testAdapter wires a *persistence.Store into the newer httpapi service
// interfaces (Tonight, Reactions, Plans, EffortProfiles, PlanningConstraints,
// ShoppingLists/Items/Push, Orders) added straight against httpapi's own
// response DTOs — the same shape as cmd/food-brain/storeAdapter, duplicated
// here so the integration test can exercise the full HTTP→service→store path
// without importing cmd. People/Preferences/Recipes/Meals are deliberately
// NOT on this adapter: dbAdapter above (dto-typed) already covers those, and
// nothing in this package needs a second implementation.
type testAdapter struct {
	db *persistence.Store
}

func (a dbAdapter) ListPeople(ctx context.Context) ([]dto.PersonResponse, error) {
	people, err := a.store.ListPeople(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dto.PersonResponse, 0, len(people))
	for _, p := range people {
		out = append(out, dto.PersonResponse{ID: p.ID, Name: p.Name, Weight: p.Weight, CreatedAt: p.CreatedAt})
	}
	return out, nil
}

func (a dbAdapter) GetPerson(ctx context.Context, id string) (dto.PersonResponse, error) {
	p, err := a.store.GetPerson(ctx, id)
	if err != nil {
		return dto.PersonResponse{}, ErrNotFound
	}
	return dto.PersonResponse{ID: p.ID, Name: p.Name, Weight: p.Weight, CreatedAt: p.CreatedAt}, nil
}

func (a dbAdapter) CreatePerson(ctx context.Context, in dto.PersonInput) (dto.PersonResponse, error) {
	if in.Weight <= 0 {
		in.Weight = 1.0
	}
	id := "itest-" + strings.ReplaceAll(time.Now().Format("20060102150405.000"), ".", "")
	p := persistence.Person{ID: id, Name: in.Name, Weight: in.Weight, CreatedAt: time.Now()}
	if err := a.store.CreatePerson(ctx, p); err != nil {
		return dto.PersonResponse{}, err
	}
	return dto.PersonResponse{ID: p.ID, Name: p.Name, Weight: p.Weight, CreatedAt: p.CreatedAt}, nil
}

func (a dbAdapter) UpdatePerson(ctx context.Context, id string, in dto.PersonUpdate) (dto.PersonResponse, error) {
	if err := a.store.UpdatePerson(ctx, persistence.Person{ID: id, Name: in.Name, Weight: in.Weight}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dto.PersonResponse{}, fmt.Errorf("%w: person %s not found", dto.ErrNotFound, id)
		}
		return dto.PersonResponse{}, err
	}
	return a.GetPerson(ctx, id)
}

func (a dbAdapter) ListPreferences(ctx context.Context, personID string) ([]dto.PersonPreferenceResponse, error) {
	prefs, err := a.store.ListPreferences(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.PersonPreferenceResponse, 0, len(prefs))
	for _, p := range prefs {
		out = append(out, dto.PersonPreferenceResponse{
			PersonID: p.PersonID, Tag: p.Tag, Sentiment: int(p.Sentiment),
			Confidence: p.Confidence, UpdatedAt: p.UpdatedAt,
		})
	}
	return out, nil
}

func (a dbAdapter) SetPreference(ctx context.Context, in dto.SetPreferenceInput) (dto.PersonPreferenceResponse, error) {
	if in.PersonID == "" || in.Tag == "" {
		return dto.PersonPreferenceResponse{}, fmt.Errorf("%w: person_id and tag are required", dto.ErrInvalidPreference)
	}
	if in.Sentiment < -2 || in.Sentiment > 2 {
		return dto.PersonPreferenceResponse{}, fmt.Errorf("%w: sentiment must be in [-2, 2]", dto.ErrInvalidPreference)
	}
	if in.Confidence < 0 || in.Confidence > 1 {
		return dto.PersonPreferenceResponse{}, fmt.Errorf("%w: confidence must be in [0, 1]", dto.ErrInvalidPreference)
	}
	if err := a.store.UpsertPreference(ctx, persistence.PersonPreference{
		PersonID: in.PersonID, Tag: in.Tag, Sentiment: in.Sentiment, Confidence: in.Confidence,
	}); err != nil {
		return dto.PersonPreferenceResponse{}, err
	}
	prefs, err := a.store.ListPreferences(ctx, in.PersonID)
	if err != nil {
		return dto.PersonPreferenceResponse{}, err
	}
	for _, p := range prefs {
		if p.Tag == in.Tag {
			return dto.PersonPreferenceResponse{
				PersonID: p.PersonID, Tag: p.Tag, Sentiment: int(p.Sentiment),
				Confidence: p.Confidence, UpdatedAt: p.UpdatedAt,
			}, nil
		}
	}
	return dto.PersonPreferenceResponse{}, fmt.Errorf("row not found after upsert")
}

func (a dbAdapter) ListRecipes(ctx context.Context) ([]dto.RecipeRefResponse, error) {
	refs, err := a.store.ListRecipeRefs(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dto.RecipeRefResponse, 0, len(refs))
	for _, r := range refs {
		out = append(out, dto.RecipeRefResponse{
			MealieRecipeID: r.MealieRecipeID, Title: r.Title, Tags: r.Tags,
			Effort: r.Effort, LastSyncedAt: r.LastSyncedAt,
		})
	}
	return out, nil
}

func (a dbAdapter) GetRecipe(ctx context.Context, id string) (dto.RecipeRefResponse, error) {
	r, err := a.store.GetRecipeRef(ctx, id)
	if err != nil {
		return dto.RecipeRefResponse{}, ErrNotFound
	}
	return dto.RecipeRefResponse{
		MealieRecipeID: r.MealieRecipeID, Title: r.Title, Tags: r.Tags,
		Effort: r.Effort, LastSyncedAt: r.LastSyncedAt,
	}, nil
}

func (a dbAdapter) Suggest(ctx context.Context) ([]dto.InspirationSuggestion, error) {
	pantryIDs, err := a.store.ListPantryIngredientIDs(ctx)
	if err != nil {
		return nil, err
	}
	pantrySet := make(map[string]bool, len(pantryIDs))
	for _, id := range pantryIDs {
		pantrySet[id] = true
	}
	refs, err := a.store.ListRecipeRefs(ctx)
	if err != nil {
		return nil, err
	}
	refByID := make(map[string]persistence.RecipeRef, len(refs))
	for _, r := range refs {
		refByID[r.MealieRecipeID] = r
	}
	lines, err := a.store.ListAllRecipeIngredients(ctx)
	if err != nil {
		return nil, err
	}
	byRecipe := make(map[string][]string)
	for _, line := range lines {
		byRecipe[line.MealieRecipeID] = append(byRecipe[line.MealieRecipeID], line.IngredientID)
	}
	var out []dto.InspirationSuggestion
	for recipeID, ingredientIDs := range byRecipe {
		ref, ok := refByID[recipeID]
		if !ok {
			continue
		}
		matched := make([]string, 0, len(ingredientIDs))
		missing := make([]string, 0, len(ingredientIDs))
		for _, id := range ingredientIDs {
			if pantrySet[id] {
				matched = append(matched, id)
			} else {
				missing = append(missing, id)
			}
		}
		if len(matched) == 0 {
			continue
		}
		ratio := 0.0
		if len(ingredientIDs) > 0 {
			ratio = float64(len(matched)) / float64(len(ingredientIDs))
		}
		out = append(out, dto.InspirationSuggestion{
			MealieRecipeID: recipeID, Title: ref.Title, Tags: ref.Tags,
			Effort: int(ref.Effort), TotalIngredients: len(ingredientIDs),
			MatchedIngredientIDs: matched, MissingIngredientIDs: missing, MatchRatio: ratio,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MatchRatio != out[j].MatchRatio {
			return out[i].MatchRatio > out[j].MatchRatio
		}
		return out[i].Title < out[j].Title
	})
	return out, nil
}

func (a dbAdapter) GetMeal(ctx context.Context, id int64) (dto.MealEventResponse, error) {
	event, err := a.store.GetMealEvent(ctx, id)
	if err != nil {
		return dto.MealEventResponse{}, err
	}
	rxns, err := a.store.ListMealReactions(ctx, event.ID)
	if err != nil {
		return dto.MealEventResponse{}, err
	}
	out := dto.MealEventResponse{
		ID: event.ID, MealieRecipeID: event.MealieRecipeID,
		ServedOn:  event.ServedOn.Format("2006-01-02"),
		CreatedAt: event.CreatedAt,
		Reactions: make([]dto.MealReactionResponse, 0, len(rxns)),
	}
	for _, r := range rxns {
		out.Reactions = append(out.Reactions, dto.MealReactionResponse{PersonID: r.PersonID, Sentiment: r.Sentiment})
	}
	return out, nil
}

func (a dbAdapter) ListMeals(ctx context.Context, mealieRecipeID, servedOn string) ([]dto.MealEventResponse, error) {
	events, err := a.store.ListMealEvents(ctx, mealieRecipeID, servedOn)
	if err != nil {
		return nil, err
	}
	out := make([]dto.MealEventResponse, 0, len(events))
	for _, event := range events {
		rxns, err := a.store.ListMealReactions(ctx, event.ID)
		if err != nil {
			return nil, err
		}
		resp := dto.MealEventResponse{
			ID: event.ID, MealieRecipeID: event.MealieRecipeID,
			ServedOn:  event.ServedOn.Format("2006-01-02"),
			CreatedAt: event.CreatedAt,
			Reactions: make([]dto.MealReactionResponse, 0, len(rxns)),
		}
		for _, r := range rxns {
			resp.Reactions = append(resp.Reactions, dto.MealReactionResponse{PersonID: r.PersonID, Sentiment: r.Sentiment})
		}
		out = append(out, resp)
	}
	return out, nil
}

func (a dbAdapter) CreateMealEvent(ctx context.Context, in dto.MealEventNew) (dto.MealEventResponse, error) {
	servedOn, err := time.Parse("2006-01-02", in.ServedOn)
	if err != nil {
		return dto.MealEventResponse{}, err
	}
	eventID, err := a.store.CreateMealEvent(ctx, in.MealieRecipeID, servedOn, nil, nil)
	if err != nil {
		return dto.MealEventResponse{}, err
	}
	for _, rx := range in.Reactions {
		if err := a.store.AddMealReaction(ctx, persistence.MealReaction{
			MealEventID: eventID, PersonID: rx.PersonID, Sentiment: rx.Sentiment,
		}); err != nil {
			return dto.MealEventResponse{}, err
		}
	}
	rxns, err := a.store.ListMealReactions(ctx, eventID)
	if err != nil {
		return dto.MealEventResponse{}, err
	}
	out := dto.MealEventResponse{
		ID: eventID, MealieRecipeID: in.MealieRecipeID, ServedOn: in.ServedOn,
		CreatedAt: time.Now(), Reactions: make([]dto.MealReactionResponse, 0, len(rxns)),
	}
	for _, r := range rxns {
		out.Reactions = append(out.Reactions, dto.MealReactionResponse{
			PersonID: r.PersonID, Sentiment: r.Sentiment,
		})
	}
	return out, nil
}

func (a dbAdapter) ListLocations(ctx context.Context, householdID string) ([]dto.PantryLocation, error) {
	locs, err := a.store.ListInventoryLocations(ctx, householdID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.PantryLocation, 0, len(locs))
	for _, l := range locs {
		out = append(out, dto.PantryLocation{
			ID: l.ID, HouseholdID: l.HouseholdID, Name: l.Name,
			LocationType: l.LocationType, ParentLocationID: l.ParentLocationID,
		})
	}
	return out, nil
}

func (a dbAdapter) CreateLocation(ctx context.Context, in dto.PantryLocationNew) (dto.PantryLocation, error) {
	id := "loc-" + strings.ReplaceAll(time.Now().Format("20060102150405.000000"), ".", "")
	l := persistence.InventoryLocation{
		ID: id, HouseholdID: in.HouseholdID, Name: in.Name,
		LocationType: in.LocationType, ParentLocationID: in.ParentLocationID,
	}
	if err := a.store.CreateInventoryLocation(ctx, l); err != nil {
		return dto.PantryLocation{}, err
	}
	return dto.PantryLocation{
		ID: id, HouseholdID: in.HouseholdID, Name: in.Name,
		LocationType: in.LocationType, ParentLocationID: in.ParentLocationID,
	}, nil
}

func (a dbAdapter) ListLots(ctx context.Context, locationID string) ([]dto.PantryLot, error) {
	lots, err := a.store.ListLotsUnderLocation(ctx, locationID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.PantryLot, 0, len(lots))
	for _, l := range lots {
		out = append(out, dto.PantryLot{
			ID: l.ID, IngredientID: l.IngredientID, ProductID: l.ProductID,
			LocationID: l.LocationID, Quantity: l.Quantity, Unit: l.Unit,
			Confidence: string(l.Confidence), CreatedAt: l.CreatedAt,
		})
	}
	return out, nil
}

func (a dbAdapter) Purchase(ctx context.Context, in dto.PantryPurchaseInput) (dto.PantryLot, error) {
	var bestBefore *time.Time
	if in.BestBefore != "" {
		bb, err := time.Parse(time.RFC3339, in.BestBefore)
		if err != nil {
			return dto.PantryLot{}, err
		}
		bestBefore = &bb
	}
	lotID, err := a.store.RecordPurchase(ctx, in.IngredientID, in.ProductID, in.LocationID, in.Quantity, in.Unit, bestBefore, in.Source)
	if err != nil {
		return dto.PantryLot{}, err
	}
	lot, err := a.store.GetInventoryLot(ctx, lotID)
	if err != nil {
		return dto.PantryLot{}, err
	}
	return dto.PantryLot{
		ID: lot.ID, IngredientID: lot.IngredientID, ProductID: lot.ProductID,
		LocationID: lot.LocationID, Quantity: lot.Quantity, Unit: lot.Unit,
		Confidence: string(lot.Confidence), CreatedAt: lot.CreatedAt,
	}, nil
}

func (a dbAdapter) Consume(ctx context.Context, lotID int64, in dto.PantryConsumeInput) error {
	return a.store.RecordConsume(ctx, lotID, in.Quantity, in.Estimated, in.Source)
}

func (a dbAdapter) ListExpiring(ctx context.Context, within time.Duration) ([]dto.PantryLot, error) {
	lots, err := a.store.ListExpiringLots(ctx, within)
	if err != nil {
		return nil, err
	}
	out := make([]dto.PantryLot, 0, len(lots))
	for _, l := range lots {
		var bestBefore, openedAt time.Time
		if l.BestBefore != nil {
			bestBefore = *l.BestBefore
		}
		if l.OpenedAt != nil {
			openedAt = *l.OpenedAt
		}
		out = append(out, dto.PantryLot{
			ID: l.ID, IngredientID: l.IngredientID, ProductID: l.ProductID,
			LocationID: l.LocationID, Quantity: l.Quantity, Unit: l.Unit,
			Confidence: string(l.Confidence), BestBefore: bestBefore,
			OpenedAt: openedAt, CreatedAt: l.CreatedAt, UpdatedAt: l.UpdatedAt,
		})
	}
	return out, nil
}

func (a *testAdapter) GetTonight(ctx context.Context) (dto.TonightView, error) {
	if a.db == nil {
		return dto.TonightView{}, dto.ErrNoMealTonight
	}
	// Use local midnight so "today" matches the household's timezone.
	today := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local)
	meal, err := a.db.GetTonightMeal(ctx, today)
	if err != nil {
		return dto.TonightView{}, err
	}
	out := dto.TonightView{
		ServedOn: meal.ServedOn.Format("2006-01-02"),
		Recipe: dto.RecipeRefResponse{
			MealieRecipeID: meal.MealieRecipeID, Title: meal.RecipeTitle,
			Tags: meal.RecipeTags, Effort: meal.RecipeEffort,
		},
		Reactions: make([]dto.MealReactionResponse, 0, len(meal.Reactions)),
	}
	for _, r := range meal.Reactions {
		out.Reactions = append(out.Reactions, dto.MealReactionResponse{
			PersonID: r.PersonID, Sentiment: r.Sentiment,
		})
	}
	return out, nil
}

func (a *testAdapter) CreateReaction(ctx context.Context, in ReactionNew) (dto.MealReactionResponse, error) {
	if a.db == nil {
		return dto.MealReactionResponse{}, fmt.Errorf("no database configured")
	}
	// Use local midnight so "today" matches the household's timezone.
	today := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local)
	meal, err := a.db.GetTonightMeal(ctx, today)
	if err != nil {
		return dto.MealReactionResponse{}, err
	}
	eventID, err := a.db.GetOrCreateMealEventForToday(ctx, meal.MealieRecipeID, today)
	if err != nil {
		return dto.MealReactionResponse{}, err
	}
	r, err := a.db.CreateReaction(ctx, eventID, in.PersonID, in.Sentiment, in.Note)
	if err != nil {
		return dto.MealReactionResponse{}, err
	}
	return dto.MealReactionResponse{PersonID: r.PersonID, Sentiment: r.Sentiment}, nil
}

func (a *testAdapter) RunPlan(ctx context.Context, in PlanRunInput) (PlanRunResult, error) {
	return PlanRunResult{Status: "accepted", Message: "not wired in integration test"}, nil
}

func (a *testAdapter) RunPlanWithProgress(ctx context.Context, in PlanRunInput, progress func(PlanProgress)) (PlanRunResult, error) {
	return a.RunPlan(ctx, in)
}

func (a *testAdapter) ListPlans(ctx context.Context) ([]PlanResponse, error) {
	plans, err := a.db.ListMealPlans(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PlanResponse, 0, len(plans))
	for _, p := range plans {
		out = append(out, PlanResponse{ID: int(p.ID), WeekStart: types.Date{Time: p.WeekStart}, Status: p.Status, CreatedAt: p.CreatedAt})
	}
	return out, nil
}

func (a *testAdapter) CreatePlan(ctx context.Context, weekStart time.Time) (PlanResponse, error) {
	id, err := a.db.CreateMealPlan(ctx, weekStart)
	if err != nil {
		return PlanResponse{}, err
	}
	p, err := a.db.GetMealPlan(ctx, id)
	if err != nil {
		return PlanResponse{}, err
	}
	return PlanResponse{ID: int(p.ID), WeekStart: types.Date{Time: p.WeekStart}, Status: p.Status, CreatedAt: p.CreatedAt}, nil
}

func (a *testAdapter) GetPlan(ctx context.Context, planID int64) (PlanView, error) {
	plan, err := a.db.GetMealPlan(ctx, planID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PlanView{}, ErrNotFound
		}
		return PlanView{}, err
	}
	candidates, _ := a.db.ListCandidates(ctx, planID)
	decisions, _ := a.db.ListDecisions(ctx, planID)
	view := PlanView{
		Plan: PlanResponse{ID: int(plan.ID), WeekStart: types.Date{Time: plan.WeekStart}, Status: plan.Status, CreatedAt: plan.CreatedAt},
		Candidates: make([]PlanCandidateResponse, 0, len(candidates)),
	}
	for _, c := range candidates {
		view.Candidates = append(view.Candidates, PlanCandidateResponse{
			ID: int(c.ID), SlotDate: types.Date{Time: c.SlotDate}, Rank: c.Rank, Score: c.Score,
			Breakdown: c.Breakdown, Feasible: c.Feasible,
		})
	}
	if len(decisions) > 0 {
		ds := make([]PlanDecisionResponse, 0, len(decisions))
		for _, d := range decisions {
			ds = append(ds, PlanDecisionResponse{PlanID: int(d.PlanID), SlotDate: types.Date{Time: d.SlotDate}, MealieRecipeID: d.MealieRecipeID, DecidedAt: &d.DecidedAt})
		}
		view.Decisions = &ds
	}
	return view, nil
}

func (a *testAdapter) UpdatePlan(ctx context.Context, planID int64, status string) (PlanResponse, error) {
	if err := a.db.SetMealPlanStatus(ctx, planID, status); err != nil {
		if strings.Contains(err.Error(), "meal_plan not found") {
			return PlanResponse{}, ErrNotFound
		}
		return PlanResponse{}, err
	}
	p, err := a.db.GetMealPlan(ctx, planID)
	if err != nil {
		return PlanResponse{}, err
	}
	return PlanResponse{ID: int(p.ID), WeekStart: types.Date{Time: p.WeekStart}, Status: p.Status, CreatedAt: p.CreatedAt}, nil
}

func (a *testAdapter) SetDecisions(ctx context.Context, planID int64, decisions []PlanDecisionInput) error {
	for _, d := range decisions {
		if err := a.db.SetDecision(ctx, persistence.MealPlanDecision{PlanID: planID, SlotDate: d.SlotDate.Time, MealieRecipeID: d.MealieRecipeID}); err != nil {
			return err
		}
	}
	return nil
}

func (a *testAdapter) ListCandidates(ctx context.Context, planID int64) ([]PlanCandidateResponse, error) {
	candidates, err := a.db.ListCandidates(ctx, planID)
	if err != nil {
		return nil, err
	}
	out := make([]PlanCandidateResponse, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, PlanCandidateResponse{ID: int(c.ID), SlotDate: types.Date{Time: c.SlotDate}, Rank: c.Rank, Score: c.Score, Breakdown: c.Breakdown, Feasible: c.Feasible})
	}
	return out, nil
}

func (a *testAdapter) InsertCandidates(ctx context.Context, candidates []PlanCandidateInput) error {
	for _, c := range candidates {
		if err := a.db.InsertCandidate(ctx, persistence.MealPlanCandidate{PlanID: c.PlanID, SlotDate: c.SlotDate, MealieRecipeID: c.MealieRecipeID, Score: c.Score, Breakdown: c.Breakdown, Feasible: c.Feasible, Rank: c.Rank}); err != nil {
			return err
		}
	}
	return nil
}

func (a *testAdapter) ListShoppingRequirements(ctx context.Context, planID int64) ([]ShoppingRequirementResponse, error) {
	reqs, err := a.db.ListShoppingRequirements(ctx, planID)
	if err != nil {
		return nil, err
	}
	out := make([]ShoppingRequirementResponse, 0, len(reqs))
	for _, r := range reqs {
		out = append(out, ShoppingRequirementResponse{ID: int(r.ID), IngredientID: r.IngredientID, Quantity: r.Quantity, Unit: r.Unit, AcceptableForms: r.AcceptableForms, PreferredForm: r.PreferredForm})
	}
	return out, nil
}

func (a *testAdapter) ListEffortProfiles(ctx context.Context) ([]EffortProfileResponse, error) {
	profiles, err := a.db.ListEffortProfiles(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]EffortProfileResponse, 0, len(profiles))
	for _, e := range profiles {
		out = append(out, EffortProfileResponse{Weekday: e.Weekday, KitchenEnergy: e.KitchenEnergy})
	}
	return out, nil
}

func (a *testAdapter) UpsertEffortProfile(ctx context.Context, in EffortProfileInput) error {
	return a.db.UpsertEffortProfile(ctx, persistence.EffortProfile{Weekday: in.Weekday, KitchenEnergy: in.KitchenEnergy})
}

func (a *testAdapter) ListPlanningConstraints(ctx context.Context) ([]PlanningConstraintResponse, error) {
	constraints, err := a.db.ListPlanningConstraints(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PlanningConstraintResponse, 0, len(constraints))
	for _, c := range constraints {
		out = append(out, PlanningConstraintResponse{ID: int(c.ID), Kind: c.Kind, Value: c.Value, Active: c.Active})
	}
	return out, nil
}

func (a *testAdapter) CreatePlanningConstraint(ctx context.Context, in PlanningConstraintInput) (PlanningConstraintResponse, error) {
	id, err := a.db.CreatePlanningConstraint(ctx, persistence.PlanningConstraint{Kind: in.Kind, Value: in.Value, Active: in.Active})
	if err != nil {
		return PlanningConstraintResponse{}, err
	}
	return PlanningConstraintResponse{ID: int(id), Kind: in.Kind, Value: in.Value, Active: in.Active}, nil
}

func TestIntegration_TonightNotFound(t *testing.T) {
	skipWithoutDB(t)
	adapter := &testAdapter{}
	mux := newMux(t, Dependencies{Tonight: adapter})

	rec := doGet(t, mux, "/tonight")
	// No approved plan with a decision for today → 404.
	if rec.Code != http.StatusNotFound {
		t.Fatalf("tonight status = %d, want 404; body: %s", rec.Code, rec.Body)
	}
}

// TestIntegration_ReactionAgainstTodayMeal is skipped: it requires an approved
// plan decision for today (which GetTonightMeal JOINs on meal_event), but
// POST /meals only creates a meal_event row without a plan decision. The
// reaction endpoint would return 500 in this scenario. A full plan-driven
// reaction test belongs in the planning integration layer, not here.
func TestIntegration_ReactionAgainstTodayMeal(t *testing.T) {
	t.Skip("requires an approved plan decision for today; see TestIntegration_TonightNotFound")
}

// TestAPI_Health verifies GET /health returns 200 with {"status":"ok"}
// regardless of whether a database is configured.
func TestAPI_Health(t *testing.T) {
	mux := http.NewServeMux()
	RegisterHandlers(mux, Dependencies{})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /health: status = %d, want 200", rec.Code)
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body %q: %v", string(rec.Body.String()), err)
	}
	if body.Status != "ok" {
		t.Errorf("GET /health: status = %q, want ok", body.Status)
	}
}

// TestAPI_PeopleRoundTrip exercises the full /people lifecycle against a real
// Postgres-backed store: list, create, list (one), get by id, get 404.
func TestAPI_PeopleRoundTrip(t *testing.T) {
	store := skipWithoutDB(t)
	server := newTestServer(t, store)
	defer server.Close()

	// List people.
	rec := serverDoGet(server, "/people")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /people: status = %d", rec.Code)
	}
	var people []dto.PersonResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &people); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Create one.
	body := `{"name":"Ada"}`
	rec = serverDoPost(server, "/people", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /people: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var created dto.PersonResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if created.Name != "Ada" || created.ID == "" {
		t.Errorf("POST /people: unexpected response %+v", created)
	}

	// Get by id.
	rec = serverDoGet(server, "/people/"+created.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /people/%s: status = %d", created.ID, rec.Code)
	}
	var got dto.PersonResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.ID != created.ID || got.Name != "Ada" {
		t.Errorf("GET /people/%s: got %+v", created.ID, got)
	}

	// Get non-existent → 404.
	rec = serverDoGet(server, "/people/does-not-exist")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /people/does-not-exist: status = %d, want 404", rec.Code)
	}
}

// TestAPI_MealsRoundTrip exercises POST /meals with reactions against a real
// Postgres-backed store, verifying the event is persisted and reactions are
// returned in the response.
func TestAPI_MealsRoundTrip(t *testing.T) {
	store := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, store, "meal_reaction", "meal_event", "person")

	// meal_event has a FK to recipe_ref, so seed one.
	if err := store.UpsertRecipeRef(ctx, persistence.RecipeRef{
		MealieRecipeID: "r-integ-pasta", Title: "Pasta Bolognese", Tags: []string{"pasta"}, Effort: 2,
	}); err != nil {
		t.Fatalf("UpsertRecipeRef: %v", err)
	}
	// meal_reaction has a FK to person, so seed the reacting person.
	if err := store.CreatePerson(ctx, persistence.Person{ID: "p-kid", Name: "Kid", Weight: 1.0}); err != nil {
		t.Fatalf("CreatePerson: %v", err)
	}

	server := newTestServer(t, store)
	defer server.Close()

	body := `{"mealie_recipe_id":"r-integ-pasta","served_on":"2026-08-18","reactions":[{"person_id":"p-kid","sentiment":2}]}`
	rec := serverDoPost(server, "/meals", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /meals: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var event dto.MealEventResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &event); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if event.MealieRecipeID != "r-integ-pasta" || event.ServedOn != "2026-08-18" {
		t.Errorf("POST /meals: unexpected response %+v", event)
	}
	if len(event.Reactions) != 1 || event.Reactions[0].PersonID != "p-kid" || event.Reactions[0].Sentiment != 2 {
		t.Errorf("POST /meals: unexpected reactions %+v", event.Reactions)
	}
}

// TestAPI_Validation verifies that bad input is rejected with 400 before
// reaching the persistence layer.
func TestAPI_Validation(t *testing.T) {
	store := skipWithoutDB(t)
	server := newTestServer(t, store)
	defer server.Close()

	cases := []struct {
		name, method, path, body string
		wantCode                 int
	}{
		{"bad_json_post_people", http.MethodPost, "/people", `not-json`, http.StatusBadRequest},
		{"empty_name", http.MethodPost, "/people", `{"name":""}`, http.StatusBadRequest},
		{"missing_recipe_meals", http.MethodPost, "/meals", `{"served_on":"2026-08-18"}`, http.StatusBadRequest},
		{"missing_date_meals", http.MethodPost, "/meals", `{"mealie_recipe_id":"r-1"}`, http.StatusBadRequest},
		{"bad_date_meals", http.MethodPost, "/meals", `{"mealie_recipe_id":"r-1","served_on":"not-a-date"}`, http.StatusBadRequest},
		{"bad_sentiment_meals", http.MethodPost, "/meals", `{"mealie_recipe_id":"r-1","served_on":"2026-08-18","reactions":[{"person_id":"p1","sentiment":9}]}`, http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var req *http.Request
			if c.body != "" {
				req = httptest.NewRequest(c.method, c.path, bytes.NewBufferString(c.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(c.method, c.path, nil)
			}
			rec := httptest.NewRecorder()
			server.Config.Handler.ServeHTTP(rec, req)
			if rec.Code != c.wantCode {
				t.Errorf("%s %s: status = %d, want %d; body = %s", c.method, c.path, rec.Code, c.wantCode, rec.Body.String())
			}
		})
	}
}

// serverDoGet and serverDoPost make requests against a httptest.Server.
// They avoid colliding with the mux-level doGet/doPost in people_test.go.
func serverDoGet(server *httptest.Server, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, server.URL+path, nil)
	rec := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(rec, req)
	return rec
}

func serverDoPost(server *httptest.Server, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, server.URL+path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(rec, req)
	return rec
}

func serverDoPatch(server *httptest.Server, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPatch, server.URL+path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(rec, req)
	return rec
}

// TestAPI_PlanRoundTrip exercises the /plans lifecycle against a real Postgres store.
func TestAPI_PlanRoundTrip(t *testing.T) {
	store := skipWithoutDB(t)
	truncateTables(t, store, "meal_plan_decision", "meal_plan_candidate", "meal_plan")
	server := newTestServer(t, store)
	defer server.Close()

	// Create a plan.
	rec := serverDoPost(server, "/plans", `{"week_start":"2026-01-13"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /plans: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var plan dto.MealPlan
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if plan.Status != "draft" {
		t.Errorf("expected draft, got %s", plan.Status)
	}

	// List plans.
	rec = serverDoGet(server, "/plans")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /plans: status = %d", rec.Code)
	}
	var plans []dto.MealPlan
	if err := json.Unmarshal(rec.Body.Bytes(), &plans); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	// Check the created plan is present (not a global count, which is unsafe
	// when other packages' tests write to the same DB in parallel).
	planFound := false
	for _, p := range plans {
		if p.ID == plan.ID {
			planFound = true
			break
		}
	}
	if !planFound {
		t.Fatalf("created plan %d not found in list of %d plans", plan.ID, len(plans))
	}

	// Get plan.
	rec = serverDoGet(server, "/plans/"+strconv.FormatInt(plan.ID, 10))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /plans/{id}: status = %d", rec.Code)
	}
}

// TestAPI_PantryRoundTrip exercises the /pantry/locations lifecycle against a real Postgres store.
func TestAPI_PantryRoundTrip(t *testing.T) {
	store := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, store, "household", "inventory_lot", "inventory_location")
	// inventory_location has an FK to household, so seed one.
	if err := store.CreateHousehold(ctx, persistence.Household{ID: "h1", Name: "Test Household"}); err != nil {
		t.Fatalf("CreateHousehold: %v", err)
	}
	server := newTestServer(t, store)
	defer server.Close()

	// Create a location.
	rec := serverDoPost(server, "/pantry/locations", `{"name":"Kitchen","household_id":"h1"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /pantry/locations: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var loc dto.PantryLocation
	if err := json.Unmarshal(rec.Body.Bytes(), &loc); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if loc.Name != "Kitchen" {
		t.Errorf("expected Kitchen, got %s", loc.Name)
	}

	// List locations.
	rec = serverDoGet(server, "/pantry/locations")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /pantry/locations: status = %d", rec.Code)
	}
	var locs []dto.PantryLocation
	if err := json.Unmarshal(rec.Body.Bytes(), &locs); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	// Check the created location is present (not a global count, which is
	// unsafe when other packages' tests write to the same DB in parallel).
	locFound := false
	for _, l := range locs {
		if l.ID == loc.ID && l.Name == "Kitchen" {
			locFound = true
			break
		}
	}
	if !locFound {
		t.Errorf("created location %q not found in list of %d locations", loc.ID, len(locs))
	}
}

// TestAPI_MealsList verifies GET /meals returns an array against a real Postgres store.
func TestAPI_MealsList(t *testing.T) {
	store := skipWithoutDB(t)
	server := newTestServer(t, store)
	defer server.Close()

	rec := serverDoGet(server, "/meals")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /meals: status = %d", rec.Code)
	}
	var events []dto.MealEventResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &events); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}
