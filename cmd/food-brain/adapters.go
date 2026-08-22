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

func (a storeAdapter) ListShoppingLists(ctx context.Context) ([]httpapi.ShoppingListResponse, error) {
	lists, err := a.db.ListShoppingLists(ctx)
	if err != nil {
		return nil, fmt.Errorf("shopping lists list: %w", err)
	}
	out := make([]httpapi.ShoppingListResponse, 0, len(lists))
	for _, l := range lists {
		out = append(out, httpapi.ShoppingListResponse{
			ID: int(l.ID), OwnerPersonID: l.OwnerPersonID,
			Name: l.Name, Status: l.Status, CreatedAt: l.CreatedAt,
		})
	}
	return out, nil
}

func (a storeAdapter) CreateShoppingList(ctx context.Context, in httpapi.ShoppingListInput) (httpapi.ShoppingListResponse, error) {
	l, err := a.db.CreateShoppingList(ctx, persistence.ShoppingList{
		OwnerPersonID: in.OwnerPersonID, Name: in.Name, Status: "active",
	})
	if err != nil {
		return httpapi.ShoppingListResponse{}, fmt.Errorf("shopping list create: %w", err)
	}
	return httpapi.ShoppingListResponse{ID: int(l), Name: in.Name, Status: "active", CreatedAt: time.Now()}, nil
}

func (a storeAdapter) GetShoppingList(ctx context.Context, listID int64) (httpapi.ShoppingListResponse, error) {
	l, err := a.db.GetShoppingList(ctx, listID)
	if err != nil {
		return httpapi.ShoppingListResponse{}, httpapi.ErrNotFound
	}
	return httpapi.ShoppingListResponse{
		ID: int(l.ID), OwnerPersonID: l.OwnerPersonID,
		Name: l.Name, Status: l.Status, CreatedAt: l.CreatedAt,
	}, nil
}

func (a storeAdapter) ArchiveShoppingList(ctx context.Context, listID int64) error {
	_, err := a.db.GetShoppingList(ctx, listID)
	if err != nil {
		return httpapi.ErrNotFound
	}
	if err := a.db.UpdateShoppingListStatus(ctx, listID, "archived"); err != nil {
		return fmt.Errorf("shopping list archive: %w", err)
	}
	return nil
}

func intPtr64ToIntPtr(v *int64) *int {
	if v == nil {
		return nil
	}
	r := int(*v)
	return &r
}

func float64ToFloat32(v float64) float32 {
	return float32(v)
}

func float64PtrToFloat32Ptr(v *float64) *float32 {
	if v == nil {
		return nil
	}
	r := float32(*v)
	return &r
}

func int64PtrToIntPtr(v *int64) *int {
	if v == nil {
		return nil
	}
	r := int(*v)
	return &r
}

func (a storeAdapter) ListShoppingListItems(ctx context.Context, listID int64) ([]httpapi.ShoppingListItemResponse, error) {
	items, err := a.db.ListShoppingListItems(ctx, listID)
	if err != nil {
		return nil, fmt.Errorf("shopping list items list: %w", err)
	}
	out := make([]httpapi.ShoppingListItemResponse, 0, len(items))
	for _, it := range items {
		out = append(out, httpapi.ShoppingListItemResponse{
			ID: int(it.ID), ShoppingListID: int(it.ShoppingListID),
			ShoppingRequirementID: int64PtrToIntPtr(it.ShoppingRequirementID),
			IngredientID: it.IngredientID, Label: it.Label,
			Quantity: float64ToFloat32(it.Quantity), Unit: it.Unit, Checked: it.Checked, AddedAt: it.AddedAt,
		})
	}
	return out, nil
}

func (a storeAdapter) AddShoppingListItem(ctx context.Context, listID int64, in httpapi.ShoppingListItemInput) (httpapi.ShoppingListItemResponse, error) {
	it, err := a.db.CreateShoppingListItem(ctx, persistence.ShoppingListItem{
		ShoppingListID: listID, ShoppingRequirementID: func() *int64 {
			if in.ShoppingRequirementID != nil {
				v := int64(*in.ShoppingRequirementID)
				return &v
			}
			return nil
		}(),
		IngredientID: in.IngredientID, Label: in.Label,
		Quantity: float64(in.Quantity), Unit: in.Unit, Checked: false,
	})
	if err != nil {
		return httpapi.ShoppingListItemResponse{}, fmt.Errorf("shopping list item create: %w", err)
	}
	return httpapi.ShoppingListItemResponse{
		ID: int(it), ShoppingListID: int(listID),
		ShoppingRequirementID: inPtrToIntPtr(in.ShoppingRequirementID),
		IngredientID: in.IngredientID, Label: in.Label,
		Quantity: in.Quantity, Unit: in.Unit, Checked: false, AddedAt: time.Now(),
	}, nil
}

func inPtrToIntPtr(v *int) *int { return v }

func (a storeAdapter) ToggleShoppingListItem(ctx context.Context, listID, itemID int64, checked bool) (httpapi.ShoppingListItemResponse, error) {
	if err := a.db.UpdateShoppingListItemChecked(ctx, itemID, checked); err != nil {
		return httpapi.ShoppingListItemResponse{}, httpapi.ErrNotFound
	}
	items, err := a.db.ListShoppingListItems(ctx, listID)
	if err != nil {
		return httpapi.ShoppingListItemResponse{}, fmt.Errorf("shopping list item toggle: %w", err)
	}
	for _, it := range items {
		if it.ID == itemID {
			return httpapi.ShoppingListItemResponse{
				ID: int(it.ID), ShoppingListID: int(it.ShoppingListID),
				ShoppingRequirementID: int64PtrToIntPtr(it.ShoppingRequirementID),
				IngredientID: it.IngredientID, Label: it.Label,
				Quantity: float64ToFloat32(it.Quantity), Unit: it.Unit, Checked: it.Checked, AddedAt: it.AddedAt,
			}, nil
		}
	}
	return httpapi.ShoppingListItemResponse{}, httpapi.ErrNotFound
}

func (a storeAdapter) DeleteShoppingListItem(ctx context.Context, listID, itemID int64) error {
	if err := a.db.DeleteShoppingListItem(ctx, itemID); err != nil {
		return httpapi.ErrNotFound
	}
	return nil
}

func (a storeAdapter) PushShoppingList(ctx context.Context, listID int64, retailer string) (httpapi.RetailerListBindingResponse, error) {
	if _, err := PushShoppingList(ctx, a.db, listID, envOr("ADAPTER_URL", "http://localhost:8402"), retailer); err != nil {
		return httpapi.RetailerListBindingResponse{}, err
	}
	binding, err := a.db.GetRetailerListBinding(ctx, listID, retailer)
	if err != nil {
		return httpapi.RetailerListBindingResponse{}, fmt.Errorf("push shopping list: get binding: %w", err)
	}
	out := httpapi.RetailerListBindingResponse{
		ID: int(binding.ID), ShoppingListID: int(binding.ShoppingListID),
		Retailer: binding.Retailer, ExternalListID: binding.ExternalListID,
		SyncDirection: binding.SyncDirection, LastPushedAt: binding.LastPushedAt,
	}
	if binding.LastPushStatus != nil {
		s := httpapi.RetailerListBindingLastPushStatus(*binding.LastPushStatus)
		out.LastPushStatus = &s
	}
	return out, nil
}

func (a storeAdapter) ListShoppingCarts(ctx context.Context, listID int64) ([]httpapi.ShoppingCartResponse, error) {
	bindings, err := a.db.ListRetailerListBindings(ctx, listID)
	if err != nil {
		return nil, fmt.Errorf("shopping carts list: %w", err)
	}
	out := make([]httpapi.ShoppingCartResponse, 0)
	for _, b := range bindings {
		carts, err := a.db.ListShoppingCarts(ctx, b.ID)
		if err != nil {
			continue
		}
		for _, c := range carts {
			out = append(out, httpapi.ShoppingCartResponse{
				ID: int(c.ID), RetailerListBindingID: int(c.RetailerListBindingID),
				CreatedAt: c.CreatedAt, Status: c.Status,
			})
		}
	}
	return out, nil
}

func (a storeAdapter) ToCart(ctx context.Context, listID int64, retailer string) (httpapi.ShoppingCartResponse, error) {
	binding, err := a.db.GetRetailerListBinding(ctx, listID, retailer)
	if err != nil {
		return httpapi.ShoppingCartResponse{}, httpapi.ErrNotFound
	}
	carts, err := a.db.ListShoppingCarts(ctx, binding.ID)
	if err != nil {
		return httpapi.ShoppingCartResponse{}, fmt.Errorf("to cart: %w", err)
	}
	if len(carts) == 0 {
		return httpapi.ShoppingCartResponse{}, fmt.Errorf("to cart: no carts for binding %d", binding.ID)
	}
	c := carts[0]
	return httpapi.ShoppingCartResponse{
		ID: int(c.ID), RetailerListBindingID: int(c.RetailerListBindingID),
		CreatedAt: c.CreatedAt, Status: c.Status,
	}, nil
}

func (a storeAdapter) ListOrders(ctx context.Context, retailer *string, cartID *int64) ([]httpapi.OrderResponse, error) {
	orders, err := a.db.ListOrders(ctx, cartID, retailer)
	if err != nil {
		return nil, fmt.Errorf("orders list: %w", err)
	}
	out := make([]httpapi.OrderResponse, 0, len(orders))
	for _, o := range orders {
		out = append(out, httpapi.OrderResponse{
			ID: int(o.ID), ShoppingCartID: int64PtrToIntPtr(o.ShoppingCartID),
			Retailer: o.Retailer, Source: o.Source,
			OrderedAt: o.OrderedAt, TotalPrice: float64PtrToFloat32Ptr(o.TotalPrice),
		})
	}
	return out, nil
}

func (a storeAdapter) GetOrder(ctx context.Context, orderID int64) (httpapi.OrderViewResponse, error) {
	o, err := a.db.GetOrder(ctx, orderID)
	if err != nil {
		return httpapi.OrderViewResponse{}, httpapi.ErrNotFound
	}
	items, err := a.db.ListOrderItems(ctx, orderID)
	if err != nil {
		return httpapi.OrderViewResponse{}, fmt.Errorf("get order: %w", err)
	}
	out := httpapi.OrderViewResponse{
		Order: httpapi.OrderResponse{
			ID: int(o.ID), ShoppingCartID: int64PtrToIntPtr(o.ShoppingCartID),
			Retailer: o.Retailer, Source: o.Source,
			OrderedAt: o.OrderedAt, TotalPrice: float64PtrToFloat32Ptr(o.TotalPrice),
		},
	}
	for _, it := range items {
		out.Items = append(out.Items, httpapi.OrderItemResponse{
			ID: int(it.ID), OrderID: int(it.OrderID),
			RetailerProductID: it.RetailerProductID,
			Quantity: float64ToFloat32(it.Quantity), UnitPrice: float64PtrToFloat32Ptr(it.UnitPrice),
			TotalPrice: float64PtrToFloat32Ptr(it.TotalPrice), SubstitutedForItemID: int64PtrToIntPtr(it.SubstitutedForItemID),
		})
	}
	return out, nil
}

func (a storeAdapter) ListOrderItems(ctx context.Context, orderID int64) ([]httpapi.OrderItemResponse, error) {
	items, err := a.db.ListOrderItems(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("order items list: %w", err)
	}
	out := make([]httpapi.OrderItemResponse, 0, len(items))
	for _, it := range items {
		out = append(out, httpapi.OrderItemResponse{
			ID: int(it.ID), OrderID: int(it.OrderID),
			RetailerProductID: it.RetailerProductID,
			Quantity: float64ToFloat32(it.Quantity), UnitPrice: float64PtrToFloat32Ptr(it.UnitPrice),
			TotalPrice: float64PtrToFloat32Ptr(it.TotalPrice), SubstitutedForItemID: int64PtrToIntPtr(it.SubstitutedForItemID),
		})
	}
	return out, nil
}


// newPersonID generates a 16-char hex id from crypto/rand (stdlib only — no new dep).
func newPersonID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
