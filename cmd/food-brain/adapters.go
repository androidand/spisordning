// Package main is the composition root: it owns the only edge that may import both
// the persistence layer (cmd -> persistence is allowed; internal packages may not
// import cmd) and the httpapi layer, bridging the two with small adapters that keep
// httpapi dependency-free of persistence (enforced by internal/architecturetest).
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/androidand/spisordning/internal/httpapi"
	"github.com/androidand/spisordning/internal/persistence"
	"github.com/jackc/pgx/v5"
	"github.com/oapi-codegen/runtime/types"
)

// storeAdapter adapts *persistence.Store to every httpapi service interface.
// It is the sole place that knows both the persistence row types and the httpapi
// response DTOs; httpapi sees only the interfaces it defines itself.
type storeAdapter struct {
	db *persistence.Store
}

func (a storeAdapter) ListPeople(ctx context.Context) ([]httpapi.PersonResponse, error) {
	people, err := a.db.ListPeople(ctx)
	if err != nil {
		return nil, fmt.Errorf("people list: %w", err)
	}
	out := make([]httpapi.PersonResponse, 0, len(people))
	for _, p := range people {
		out = append(out, httpapi.PersonResponse{
			ID: p.ID, Name: p.Name, Weight: p.Weight, CreatedAt: p.CreatedAt,
		})
	}
	return out, nil
}

func (a storeAdapter) GetPerson(ctx context.Context, id string) (httpapi.PersonResponse, error) {
	p, err := a.db.GetPerson(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return httpapi.PersonResponse{}, httpapi.ErrNotFound
	}
	if err != nil {
		return httpapi.PersonResponse{}, fmt.Errorf("people get: %w", err)
	}
	return httpapi.PersonResponse{
		ID: p.ID, Name: p.Name, Weight: p.Weight, CreatedAt: p.CreatedAt,
	}, nil
}

func (a storeAdapter) CreatePerson(ctx context.Context, in httpapi.PersonInput) (httpapi.PersonResponse, error) {
	if in.Weight <= 0 {
		in.Weight = 1.0
	}
	id, err := newPersonID()
	if err != nil {
		return httpapi.PersonResponse{}, fmt.Errorf("people create: generate id: %w", err)
	}
	p := persistence.Person{ID: id, Name: in.Name, Weight: in.Weight, CreatedAt: time.Now()}
	if err := a.db.CreatePerson(ctx, p); err != nil {
		return httpapi.PersonResponse{}, fmt.Errorf("people create: %w", err)
	}
	return httpapi.PersonResponse{
		ID: p.ID, Name: p.Name, Weight: p.Weight, CreatedAt: p.CreatedAt,
	}, nil
}

func (a storeAdapter) ListPreferences(ctx context.Context, personID string) ([]httpapi.PersonPreferenceResponse, error) {
	prefs, err := a.db.ListPreferences(ctx, personID)
	if err != nil {
		return nil, fmt.Errorf("preferences list: %w", err)
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

func (a storeAdapter) ListRecipes(ctx context.Context) ([]httpapi.RecipeRefResponse, error) {
	refs, err := a.db.ListRecipeRefs(ctx)
	if err != nil {
		return nil, fmt.Errorf("recipes list: %w", err)
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

func (a storeAdapter) GetTonight(ctx context.Context) (httpapi.TonightView, error) {
	// Use local midnight so "today" matches the household's timezone (see TZ
	// env var in docker-compose.yml). time.Now().Truncate(24h) truncates to UTC
	// midnight, which is wrong for UTC+1/+2 households in the early morning.
	today := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local)
	meal, err := a.db.GetTonightMeal(ctx, today)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return httpapi.TonightView{}, httpapi.ErrNoMealTonight
		}
		return httpapi.TonightView{}, fmt.Errorf("tonight get: %w", err)
	}
	out := httpapi.TonightView{
		ServedOn: meal.ServedOn.Format("2006-01-02"),
		Recipe: httpapi.RecipeRefResponse{
			MealieRecipeID: meal.MealieRecipeID, Title: meal.RecipeTitle,
			Tags: meal.RecipeTags, Effort: meal.RecipeEffort,
		},
		Reactions: make([]httpapi.MealReactionResponse, 0, len(meal.Reactions)),
	}
	for _, r := range meal.Reactions {
		out.Reactions = append(out.Reactions, httpapi.MealReactionResponse{
			PersonID: r.PersonID, Sentiment: r.Sentiment,
		})
	}
	return out, nil
}

func (a storeAdapter) CreateReaction(ctx context.Context, in httpapi.ReactionNew) (httpapi.MealReactionResponse, error) {
	// Find today's meal event to attach the reaction to.
	// Use local midnight so "today" matches the household's timezone (see TZ
	// env var in docker-compose.yml). time.Now().Truncate(24h) truncates to UTC
	// midnight, which is wrong for UTC+1/+2 households in the early morning.
	today := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local)
	meal, err := a.db.GetTonightMeal(ctx, today)
	if err != nil {
		return httpapi.MealReactionResponse{}, fmt.Errorf("reaction: no meal tonight: %w", err)
	}
	// Find or create the meal event for today.
	eventID, err := a.db.GetOrCreateMealEventForToday(ctx, meal.MealieRecipeID, today)
	if err != nil {
		return httpapi.MealReactionResponse{}, fmt.Errorf("reaction: find meal event: %w", err)
	}
	r, err := a.db.CreateReaction(ctx, eventID, in.PersonID, in.Sentiment, in.Note)
	if err != nil {
		return httpapi.MealReactionResponse{}, fmt.Errorf("reaction: create: %w", err)
	}
	return httpapi.MealReactionResponse{PersonID: r.PersonID, Sentiment: r.Sentiment}, nil
}

func (a storeAdapter) ListPlans(ctx context.Context) ([]httpapi.PlanResponse, error) {
	plans, err := a.db.ListMealPlans(ctx)
	if err != nil {
		return nil, fmt.Errorf("plans list: %w", err)
	}
	out := make([]httpapi.PlanResponse, 0, len(plans))
	for _, p := range plans {
		out = append(out, httpapi.PlanResponse{
			ID: int(p.ID), WeekStart: types.Date{Time: p.WeekStart}, Status: p.Status, CreatedAt: p.CreatedAt,
		})
	}
	return out, nil
}

func (a storeAdapter) CreatePlan(ctx context.Context, weekStart time.Time) (httpapi.PlanResponse, error) {
	p, err := a.db.GetOrCreateMealPlan(ctx, weekStart)
	if err != nil {
		return httpapi.PlanResponse{}, fmt.Errorf("plan create/get: %w", err)
	}
	return httpapi.PlanResponse{
		ID: int(p.ID), WeekStart: types.Date{Time: p.WeekStart}, Status: p.Status, CreatedAt: p.CreatedAt,
	}, nil
}

func (a storeAdapter) GetPlan(ctx context.Context, planID int64) (httpapi.PlanView, error) {
	plan, err := a.db.GetMealPlan(ctx, planID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return httpapi.PlanView{}, httpapi.ErrNotFound
		}
		return httpapi.PlanView{}, fmt.Errorf("plan get: %w", err)
	}
	candidates, err := a.db.ListCandidates(ctx, planID)
	if err != nil {
		return httpapi.PlanView{}, fmt.Errorf("candidates list: %w", err)
	}
	decisions, err := a.db.ListDecisions(ctx, planID)
	if err != nil {
		return httpapi.PlanView{}, fmt.Errorf("decisions list: %w", err)
	}

	// Fetch recipe refs for all candidates and decisions.
	recipeIDs := make(map[string]struct{})
	for _, c := range candidates {
		recipeIDs[c.MealieRecipeID] = struct{}{}
	}
	for _, d := range decisions {
		recipeIDs[d.MealieRecipeID] = struct{}{}
	}
	recipes := make(map[string]persistence.RecipeRef)
	for id := range recipeIDs {
		r, err := a.db.GetRecipeRef(ctx, id)
		if err != nil {
			// Recipe not found — continue without it (may be unseeded).
			continue
		}
		recipes[id] = r
	}

	view := httpapi.PlanView{
		Plan: httpapi.PlanResponse{
			ID: int(plan.ID), WeekStart: types.Date{Time: plan.WeekStart}, Status: plan.Status, CreatedAt: plan.CreatedAt,
		},
		Candidates: make([]httpapi.PlanCandidateResponse, 0, len(candidates)),
	}
	for _, c := range candidates {
		var recipe httpapi.RecipeRefResponse
		if r, ok := recipes[c.MealieRecipeID]; ok {
			recipe = httpapi.RecipeRefResponse{
				MealieRecipeID: r.MealieRecipeID, Title: r.Title, Tags: r.Tags, Effort: r.Effort,
			}
		}
		view.Candidates = append(view.Candidates, httpapi.PlanCandidateResponse{
			ID: int(c.ID), SlotDate: types.Date{Time: c.SlotDate}, Rank: c.Rank, Score: c.Score,
			Breakdown: c.Breakdown, Feasible: c.Feasible, Recipe: recipe,
		})
	}
	if len(decisions) > 0 {
		ds := make([]httpapi.PlanDecisionResponse, 0, len(decisions))
		for _, d := range decisions {
			ds = append(ds, httpapi.PlanDecisionResponse{
				PlanID: int(d.PlanID), SlotDate: types.Date{Time: d.SlotDate},
				MealieRecipeID: d.MealieRecipeID, DecidedAt: &d.DecidedAt,
			})
		}
		view.Decisions = &ds
	}
	return view, nil
}

func (a storeAdapter) UpdatePlan(ctx context.Context, planID int64, status string) (httpapi.PlanResponse, error) {
	if err := a.db.SetMealPlanStatus(ctx, planID, status); err != nil {
		if strings.Contains(err.Error(), "meal_plan not found") {
			return httpapi.PlanResponse{}, httpapi.ErrNotFound
		}
		return httpapi.PlanResponse{}, fmt.Errorf("plan update: %w", err)
	}
	p, err := a.db.GetMealPlan(ctx, planID)
	if err != nil {
		return httpapi.PlanResponse{}, fmt.Errorf("plan get after update: %w", err)
	}
	return httpapi.PlanResponse{
		ID: int(p.ID), WeekStart: types.Date{Time: p.WeekStart}, Status: p.Status, CreatedAt: p.CreatedAt,
	}, nil
}

func (a storeAdapter) SetDecisions(ctx context.Context, planID int64, decisions []httpapi.PlanDecisionInput) error {
	for _, d := range decisions {
		if err := a.db.SetDecision(ctx, persistence.MealPlanDecision{
			PlanID: planID, SlotDate: d.SlotDate.Time, MealieRecipeID: d.MealieRecipeID,
		}); err != nil {
			return fmt.Errorf("decision set: %w", err)
		}
	}
	return nil
}

func (a storeAdapter) ListCandidates(ctx context.Context, planID int64) ([]httpapi.PlanCandidateResponse, error) {
	candidates, err := a.db.ListCandidates(ctx, planID)
	if err != nil {
		return nil, fmt.Errorf("candidates list: %w", err)
	}
	out := make([]httpapi.PlanCandidateResponse, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, httpapi.PlanCandidateResponse{
			ID: int(c.ID), SlotDate: types.Date{Time: c.SlotDate}, Rank: c.Rank, Score: c.Score,
			Breakdown: c.Breakdown, Feasible: c.Feasible,
		})
	}
	return out, nil
}

func (a storeAdapter) InsertCandidates(ctx context.Context, candidates []httpapi.PlanCandidateInput) error {
	for _, c := range candidates {
		if err := a.db.InsertCandidate(ctx, persistence.MealPlanCandidate{
			PlanID: c.PlanID, SlotDate: c.SlotDate, MealieRecipeID: c.MealieRecipeID,
			Score: c.Score, Breakdown: c.Breakdown, Feasible: c.Feasible, Rank: c.Rank,
		}); err != nil {
			return fmt.Errorf("candidate insert: %w", err)
		}
	}
	return nil
}

func (a storeAdapter) ListShoppingRequirements(ctx context.Context, planID int64) ([]httpapi.ShoppingRequirementResponse, error) {
	reqs, err := a.db.ListShoppingRequirements(ctx, planID)
	if err != nil {
		return nil, fmt.Errorf("shopping requirements list: %w", err)
	}
	out := make([]httpapi.ShoppingRequirementResponse, 0, len(reqs))
	for _, r := range reqs {
		out = append(out, httpapi.ShoppingRequirementResponse{
			ID: int(r.ID), IngredientID: r.IngredientID, Quantity: r.Quantity,
			Unit: r.Unit, AcceptableForms: r.AcceptableForms, PreferredForm: r.PreferredForm,
		})
	}
	return out, nil
}

func (a storeAdapter) RunPlan(ctx context.Context, in httpapi.PlanRunInput) (httpapi.PlanRunResult, error) {
	result, err := RunPlan(ctx, RunPlanInput{
		Week:           in.Week,
		Days:           in.Days,
		CreateWishlist: in.CreateWishlist,
		Family:         "family.json",
		School:         "",
	})
	if err != nil {
		return httpapi.PlanRunResult{}, fmt.Errorf("plan run: %w", err)
	}
	out := httpapi.PlanRunResult{
		Status:   "accepted",
		Message:  result.Message,
		WeekStart: func() *string { s := result.WeekStart.Format("2006-01-02"); return &s }(),
	}
	return out, nil
}

func (a storeAdapter) CreateMealEvent(ctx context.Context, in httpapi.MealEventNew) (httpapi.MealEventResponse, error) {
	servedOn, err := time.Parse("2006-01-02", in.ServedOn)
	if err != nil {
		return httpapi.MealEventResponse{}, fmt.Errorf("meals create: invalid served_on %q: %w", in.ServedOn, err)
	}
	eventID, err := a.db.CreateMealEvent(ctx, in.MealieRecipeID, servedOn)
	if err != nil {
		return httpapi.MealEventResponse{}, fmt.Errorf("meals create: %w", err)
	}
	for _, rx := range in.Reactions {
		if err := a.db.AddMealReaction(ctx, persistence.MealReaction{
			MealEventID: eventID, PersonID: rx.PersonID, Sentiment: rx.Sentiment,
		}); err != nil {
			return httpapi.MealEventResponse{}, fmt.Errorf("meals create: add reaction: %w", err)
		}
	}
	rxns, err := a.db.ListMealReactions(ctx, eventID)
	if err != nil {
		return httpapi.MealEventResponse{}, fmt.Errorf("meals create: read reactions: %w", err)
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

func (a storeAdapter) ListEffortProfiles(ctx context.Context) ([]httpapi.EffortProfileResponse, error) {
	profiles, err := a.db.ListEffortProfiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("effort profiles list: %w", err)
	}
	out := make([]httpapi.EffortProfileResponse, 0, len(profiles))
	for _, e := range profiles {
		out = append(out, httpapi.EffortProfileResponse{
			Weekday: e.Weekday, KitchenEnergy: e.KitchenEnergy,
		})
	}
	return out, nil
}

func (a storeAdapter) UpsertEffortProfile(ctx context.Context, in httpapi.EffortProfileInput) error {
	if err := a.db.UpsertEffortProfile(ctx, persistence.EffortProfile{
		Weekday: in.Weekday, KitchenEnergy: in.KitchenEnergy,
	}); err != nil {
		return fmt.Errorf("effort profile upsert: %w", err)
	}
	return nil
}

func (a storeAdapter) ListPlanningConstraints(ctx context.Context) ([]httpapi.PlanningConstraintResponse, error) {
	constraints, err := a.db.ListPlanningConstraints(ctx)
	if err != nil {
		return nil, fmt.Errorf("constraints list: %w", err)
	}
	out := make([]httpapi.PlanningConstraintResponse, 0, len(constraints))
	for _, c := range constraints {
		out = append(out, httpapi.PlanningConstraintResponse{
			ID: int(c.ID), Kind: c.Kind, Value: c.Value, Active: c.Active,
		})
	}
	return out, nil
}

func (a storeAdapter) CreatePlanningConstraint(ctx context.Context, in httpapi.PlanningConstraintInput) (httpapi.PlanningConstraintResponse, error) {
	id, err := a.db.CreatePlanningConstraint(ctx, persistence.PlanningConstraint{
		Kind: in.Kind, Value: in.Value, Active: in.Active,
	})
	if err != nil {
		return httpapi.PlanningConstraintResponse{}, fmt.Errorf("constraint create: %w", err)
	}
	return httpapi.PlanningConstraintResponse{ID: int(id), Kind: in.Kind, Value: in.Value, Active: in.Active}, nil
}

// newPersonID generates a 16-char hex id from crypto/rand (stdlib only — no new dep).
func newPersonID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
