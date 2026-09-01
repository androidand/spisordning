// Package main is the composition root: it owns the only edge that may import both
// the persistence layer and the httpapi layer, wiring service implementations
// into the httpapi.Dependencies struct.
//
// Business logic lives in internal/service; this file is deliberately thin —
// it only constructs services and passes the persistence.Store to them.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/androidand/spisordning/internal/config"
	"github.com/androidand/spisordning/internal/dto"
	"github.com/androidand/spisordning/internal/domain"
	"github.com/androidand/spisordning/internal/grocy"
	"github.com/androidand/spisordning/internal/httpapi"
	"github.com/androidand/spisordning/internal/ingredients"
	"github.com/androidand/spisordning/internal/mealie"
	"github.com/androidand/spisordning/internal/persistence"
	"github.com/androidand/spisordning/internal/retailer"
	"github.com/androidand/spisordning/internal/service"
	"github.com/oapi-codegen/runtime/types"
)

// buildDependencies wires the persistence-backed services the HTTP layer exposes.
// It degrades gracefully: if Postgres isn't configured or unreachable, only the
// /health endpoint is served (resource routes are nil-guarded in RegisterHandlers).
// External client services (ingredients, stores) are wired only when their
// environment variables are set; missing clients result in nil service entries.
func buildDependencies() httpapi.Dependencies {
	deps := httpapi.Dependencies{}

	cfg := config.Load()

	pgCfg, err := persistence.FromEnv(os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "⚠ no database configured (POSTGRES_PASSWORD/DATABASE_URL unset); serving /health only")
		return deps
	}

	ctx := context.Background()
	store, err := persistence.New(ctx, pgCfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "⚠ persistence unavailable:", err)
		return deps
	}

	// Refuse to serve on a database whose schema is behind the embedded
	// migrations. serve never mutates the schema — run `food-brain migrate up`.
	pending, err := persistence.MigrationsPending(ctx, pgCfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "⚠ migration check failed:", err)
		return deps
	}
	if pending > 0 {
		fmt.Fprintf(os.Stderr, "❌ %d migration(s) pending — run `food-brain migrate up` before serving\n", pending)
		os.Exit(1)
	}

	var mealieClient *mealie.Client
	if cfg.HasMealie() {
		mealieClient = mealie.New(cfg.MealieBaseURL, cfg.MealieAPIToken)
	}
	deps.People = service.NewPeople(store)
	deps.Preferences = service.NewPreferences(store)
	deps.Recipes = service.NewRecipes(store, mealieClient)
	deps.Meals = service.NewMeals(store, nil)
	deps.Pantry = service.NewPantry(store)
	deps.RecipeFamily = service.NewRecipeFamily(store)
	// Discovery reuses the RecipeFamily service (promotion creates family /
	// variant / revision through it) and a default HTTP client for fetching
	// external recipe URLs.
	deps.Discovery = service.NewDiscovery(store, deps.RecipeFamily, nil)
	deps.Favorites = service.NewFavorites(store)
	deps.PriceIntelligence = service.NewPriceIntelligence(store)

	// Dashboard aggregates tonight + pantry + expiring into one read model. The
	// tonight provider adapts storeAdapter.GetTonight (httpapi view) to the
	// service-layer tonightView so the service layer never imports httpapi.
	adapter := storeAdapter{db: store, adapterURL: cfg.AdapterURL, school: cfg.SkolmatenSchool}
	deps.Dashboard = service.NewDashboard(store, dashboardTonightProvider{adapter}, deps.Pantry)
	deps.IngredientAlias = service.NewIngredientAlias(store)
	deps.Inspiration = service.NewInspiration(store)

	// Grocy is optional — only wired when GROCY_BASE_URL is set. When unset the
	// service is constructed with a nil client and reports "not configured"
	// (503) on every /grocy route, so the API degrades gracefully.
	var grocyClient *grocy.Client
	if cfg.HasGrocy() {
		grocyClient = grocy.New(cfg.GrocyBaseURL, cfg.GrocyAPIKey)
	}
	deps.Grocy = service.NewGrocy(grocyClient, cfg.GrocyBaseURL)

	// External clients are optional — only wired when configured.
	var slv *ingredients.Client
	if cfg.HasSLV() {
		slv = ingredients.NewLivsmedelsverket(cfg.SLVBaseURL)
	}
	var dabas *ingredients.DabasClient
	if cfg.DabasEnabled {
		dabas = ingredients.NewDabas()
	}
	var mpk *ingredients.MPKClient
	if cfg.MPKEnabled {
		mpk = ingredients.NewMatpriskollen()
	}
	deps.Ingredients = service.NewIngredients(store, slv, dabas, mpk)
	deps.Stores = service.NewStores(store, mpk)

	// storeAdapter implements the newer capabilities added straight against
	// persistence.Store (no internal/service/internal/dto indirection): tonight,
	// reactions, plan runs, effort profiles, planning constraints, shopping
	// lists/items/push, and orders. It supersedes the old dto.PlanningService
	// wiring for /plans (this Plans field is a strict superset — adds
	// POST /plans/run and GET /plans/{id}/candidates).
	adapters := storeAdapter{db: store, adapterURL: cfg.AdapterURL, school: cfg.SkolmatenSchool}
	deps.Tonight = adapters
	deps.Reactions = adapters
	deps.Plans = adapters
	deps.EffortProfiles = adapters
	deps.PlanningConstraints = adapters
	deps.ShoppingLists = adapters
	deps.ShoppingListItems = adapters
	deps.ShoppingPush = adapters
	deps.Orders = adapters

	// Price comparison resolves each requirement against every retailer via
	// internal/retailer.Compare. It needs the adapter base URLs; when neither is
	// configured the route is simply not registered (nil-guarded in
	// RegisterHandlers), matching the optional-client convention above.
	if cfg.HasWillys() {
		deps.PriceComparison = priceComparisonAdapter{
			willysURL: cfg.AdapterURL,
			icaURL:    cfg.ICAAdapterURL,
			hemkopURL: cfg.HemkopAdapterURL,
		}
	}

	// Retailer credentials (ICA elevated-auth credential upload) are stored via
	// the same storeAdapter; the credential methods live on storeAdapter below.
	deps.RetailerCredentials = adapters
	return deps
}

// priceComparisonAdapter adapts the retailer client to the httpapi
// PriceComparisonService interface. It is the sole place that knows both the
// retailer client and the httpapi compare DTOs; httpapi sees only the interface
// it defines itself (enforced by internal/architecturetest).
type priceComparisonAdapter struct {
	willysURL string
	icaURL    string
	hemkopURL string
}

func (a priceComparisonAdapter) ComparePrices(ctx context.Context, reqs []httpapi.CompareRequirement) (httpapi.PriceComparison, error) {
	domainReqs := make([]domain.ShoppingRequirement, 0, len(reqs))
	terms := retailer.SearchTerms{}
	for _, r := range reqs {
		domainReqs = append(domainReqs, domain.ShoppingRequirement{
			IngredientID:    r.Ingredient,
			Quantity:        r.Quantity,
			Unit:            r.Unit,
			AcceptableForms: r.AcceptableForms,
			PreferredForm:   r.PreferredForm,
		})
		terms[r.Ingredient] = r.Ingredient
	}
	cmp := retailer.Compare(ctx, domainReqs, terms, a.willysURL, a.icaURL, a.hemkopURL)
	return toHTTPComparison(cmp), nil
}

// toHTTPComparison maps a retailer.Comparison to the httpapi PriceComparison
// wire shape (the HTTP equivalent of the MCP tool output).
func toHTTPComparison(cmp *retailer.Comparison) httpapi.PriceComparison {
	out := httpapi.PriceComparison{Items: make([]httpapi.ItemComparison, 0, len(cmp.Items))}
	for _, item := range cmp.Items {
		mi := httpapi.ItemComparison{
			Ingredient: item.Requirement.IngredientID,
			Results:    make([]httpapi.RetailerPriceResult, 0, len(item.Results)),
			Unresolved: item.Unresolved,
		}
		for _, r := range item.Results {
			mi.Results = append(mi.Results, toHTTPResult(r))
		}
		if item.Cheapest != nil {
			c := toHTTPResult(*item.Cheapest)
			mi.Cheapest = &c
		}
		out.Items = append(out.Items, mi)
	}
	return out
}

// toHTTPResult maps a single retailer.RetailerResult to the httpapi wire shape.
func toHTTPResult(r retailer.RetailerResult) httpapi.RetailerPriceResult {
	mr := httpapi.RetailerPriceResult{
		Retailer:   string(r.Retailer),
		Available:  r.Available,
		PriceValue: r.PriceValue,
		Error:      r.Error,
	}
	if r.Resolution.RetailerProductID != nil {
		mr.ProductID = r.Resolution.RetailerProductID
	}
	if r.Resolution.ProductName != "" {
		mr.ProductName = &r.Resolution.ProductName
	}
	if r.Resolution.Price != nil {
		mr.Price = r.Resolution.Price
	}
	return mr
}

// storeAdapter adapts *persistence.Store to the newer httpapi service
// interfaces (Tonight, Reactions, Plans, EffortProfiles, PlanningConstraints,
// ShoppingLists/Items/Push, Orders) added straight against httpapi's own
// response DTOs, bypassing internal/service + internal/dto. People,
// Preferences, Recipes, and Meals are deliberately NOT wired through this
// adapter — those stay on the existing, tested internal/service/internal/dto
// layer (see buildDependencies); storeAdapter only supplies the capabilities
// that layer doesn't have yet.
type storeAdapter struct {
	db         *persistence.Store
	adapterURL string
	school     string
}

// dashboardTonightProvider adapts storeAdapter.GetTonight (which returns the
// httpapi view) to the service-layer TonightProvider, so the dashboard service
// never imports httpapi.
type dashboardTonightProvider struct {
	inner storeAdapter
}

func (p dashboardTonightProvider) GetTonight(ctx context.Context) (dto.TonightView, error) {
	view, err := p.inner.GetTonight(ctx)
	if err != nil {
		if errors.Is(err, dto.ErrNoMealTonight) {
			return dto.TonightView{}, dto.ErrNoMealTonight
		}
		return dto.TonightView{}, err
	}
	return view, nil
}

func (a storeAdapter) ListRecipes(ctx context.Context) ([]dto.RecipeRefResponse, error) {
	refs, err := a.db.ListRecipeRefs(ctx)
	if err != nil {
		return nil, fmt.Errorf("recipes list: %w", err)
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

func (a storeAdapter) GetTonight(ctx context.Context) (dto.TonightView, error) {
	// Use local midnight so "today" matches the household's timezone (see TZ
	// env var in docker-compose.yml). time.Now().Truncate(24h) truncates to UTC
	// midnight, which is wrong for UTC+1/+2 households in the early morning.
	today := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local)
	meal, err := a.db.GetTonightMeal(ctx, today)
	if err != nil {
		if errors.Is(err, persistence.ErrNoRows) {
			return dto.TonightView{}, dto.ErrNoMealTonight
		}
		return dto.TonightView{}, fmt.Errorf("tonight get: %w", err)
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
			PersonID: r.PersonID.String(), Sentiment: r.Sentiment,
		})
	}
	return out, nil
}

func (a storeAdapter) CreateReaction(ctx context.Context, in httpapi.ReactionNew) (dto.MealReactionResponse, error) {
	// Find today's meal event to attach the reaction to.
	// Use local midnight so "today" matches the household's timezone (see TZ
	// env var in docker-compose.yml). time.Now().Truncate(24h) truncates to UTC
	// midnight, which is wrong for UTC+1/+2 households in the early morning.
	today := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local)
	meal, err := a.db.GetTonightMeal(ctx, today)
	if err != nil {
		return dto.MealReactionResponse{}, fmt.Errorf("reaction: no meal tonight: %w", err)
	}
	// Find or create the meal event for today.
	eventID, err := a.db.GetOrCreateMealEventForToday(ctx, meal.RecipeRefID, today)
	if err != nil {
		return dto.MealReactionResponse{}, fmt.Errorf("reaction: find meal event: %w", err)
	}
	r, err := a.db.CreateReaction(ctx, eventID, in.PersonID, in.Sentiment, in.Note)
	if err != nil {
		return dto.MealReactionResponse{}, fmt.Errorf("reaction: create: %w", err)
	}
	return dto.MealReactionResponse{PersonID: r.PersonID.String(), Sentiment: r.Sentiment}, nil
}

func (a storeAdapter) ListPlans(ctx context.Context) ([]httpapi.PlanResponse, error) {
	plans, err := a.db.ListMealPlans(ctx)
	if err != nil {
		return nil, fmt.Errorf("plans list: %w", err)
	}
	out := make([]httpapi.PlanResponse, 0, len(plans))
	for _, p := range plans {
		out = append(out, httpapi.PlanResponse{
			ID: p.ID.String(), WeekStart: types.Date{Time: p.WeekStart}, Status: p.Status, CreatedAt: p.CreatedAt,
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
		ID: p.ID.String(), WeekStart: types.Date{Time: p.WeekStart}, Status: p.Status, CreatedAt: p.CreatedAt,
	}, nil
}

func (a storeAdapter) GetPlan(ctx context.Context, planID string) (httpapi.PlanView, error) {
	parsedPlanID, err := domain.ParseMealPlanID(planID)
	if err != nil {
		return httpapi.PlanView{}, fmt.Errorf("plan get: %w", err)
	}
	plan, err := a.db.GetMealPlan(ctx, parsedPlanID)
	if err != nil {
		if errors.Is(err, persistence.ErrNoRows) {
			return httpapi.PlanView{}, httpapi.ErrNotFound
		}
		return httpapi.PlanView{}, fmt.Errorf("plan get: %w", err)
	}
	candidates, err := a.db.ListCandidates(ctx, parsedPlanID)
	if err != nil {
		return httpapi.PlanView{}, fmt.Errorf("candidates list: %w", err)
	}
	decisions, err := a.db.ListDecisions(ctx, parsedPlanID)
	if err != nil {
		return httpapi.PlanView{}, fmt.Errorf("decisions list: %w", err)
	}

	// Fetch recipe refs for all candidates and decisions.
	recipeIDs := make(map[domain.RecipeRefID]struct{})
	for _, c := range candidates {
		recipeIDs[c.RecipeRefID] = struct{}{}
	}
	for _, d := range decisions {
		recipeIDs[d.RecipeRefID] = struct{}{}
	}
	recipes := make(map[domain.RecipeRefID]persistence.RecipeRef)
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
			ID: plan.ID.String(), WeekStart: types.Date{Time: plan.WeekStart}, Status: plan.Status, CreatedAt: plan.CreatedAt,
		},
		Candidates: make([]httpapi.PlanCandidateResponse, 0, len(candidates)),
	}
	for _, c := range candidates {
		var recipe dto.RecipeRefResponse
		if r, ok := recipes[c.RecipeRefID]; ok {
			recipe = dto.RecipeRefResponse{
				MealieRecipeID: r.MealieRecipeID, Title: r.Title, Tags: r.Tags, Effort: r.Effort,
			}
		}
		view.Candidates = append(view.Candidates, httpapi.PlanCandidateResponse{
			ID: c.ID.String(), SlotDate: types.Date{Time: c.SlotDate}, SlotKind: c.SlotKind,
			Rank: c.Rank, Score: c.Score, Breakdown: c.Breakdown, Feasible: c.Feasible, Recipe: recipe,
		})
	}
	if len(decisions) > 0 {
		ds := make([]httpapi.PlanDecisionResponse, 0, len(decisions))
		for _, d := range decisions {
			var mealieRecipeID string
			if r, ok := recipes[d.RecipeRefID]; ok {
				mealieRecipeID = r.MealieRecipeID
			}
			ds = append(ds, httpapi.PlanDecisionResponse{
				PlanID: d.PlanID.String(), SlotDate: types.Date{Time: d.SlotDate}, SlotKind: d.SlotKind,
				MealieRecipeID: mealieRecipeID, DecidedAt: &d.DecidedAt,
			})
		}
		view.Decisions = &ds
	}
	return view, nil
}

func (a storeAdapter) UpdatePlan(ctx context.Context, planID string, status string) (httpapi.PlanResponse, error) {
	parsedPlanID, err := domain.ParseMealPlanID(planID)
	if err != nil {
		return httpapi.PlanResponse{}, fmt.Errorf("plan update: %w", err)
	}
	if err := a.db.SetMealPlanStatus(ctx, parsedPlanID, status); err != nil {
		if strings.Contains(err.Error(), "meal_plan not found") {
			return httpapi.PlanResponse{}, httpapi.ErrNotFound
		}
		return httpapi.PlanResponse{}, fmt.Errorf("plan update: %w", err)
	}
	p, err := a.db.GetMealPlan(ctx, parsedPlanID)
	if err != nil {
		return httpapi.PlanResponse{}, fmt.Errorf("plan get after update: %w", err)
	}
	return httpapi.PlanResponse{
		ID: p.ID.String(), WeekStart: types.Date{Time: p.WeekStart}, Status: p.Status, CreatedAt: p.CreatedAt,
	}, nil
}

func (a storeAdapter) SetDecisions(ctx context.Context, planID string, decisions []httpapi.PlanDecisionInput) error {
	parsedPlanID, err := domain.ParseMealPlanID(planID)
	if err != nil {
		return fmt.Errorf("plan set decisions: %w", err)
	}
	for _, d := range decisions {
		ref, err := a.db.GetRecipeRefByMealieID(ctx, d.MealieRecipeID)
		if err != nil {
			return fmt.Errorf("plan set decisions: resolve recipe %q: %w", d.MealieRecipeID, err)
		}
		if err := a.db.SetDecision(ctx, persistence.MealPlanDecision{
			PlanID: parsedPlanID, SlotDate: d.SlotDate.Time, SlotKind: d.SlotKind, RecipeRefID: ref.ID,
		}); err != nil {
			return fmt.Errorf("decision set: %w", err)
		}
	}
	return nil
}

func (a storeAdapter) ListCandidates(ctx context.Context, planID string) ([]httpapi.PlanCandidateResponse, error) {
	parsedPlanID, err := domain.ParseMealPlanID(planID)
	if err != nil {
		return nil, fmt.Errorf("candidates list: %w", err)
	}
	candidates, err := a.db.ListCandidates(ctx, parsedPlanID)
	if err != nil {
		return nil, fmt.Errorf("candidates list: %w", err)
	}
	out := make([]httpapi.PlanCandidateResponse, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, httpapi.PlanCandidateResponse{
			ID: c.ID.String(), SlotDate: types.Date{Time: c.SlotDate}, SlotKind: c.SlotKind,
			Rank: c.Rank, Score: c.Score, Breakdown: c.Breakdown, Feasible: c.Feasible,
		})
	}
	return out, nil
}

func (a storeAdapter) InsertCandidates(ctx context.Context, candidates []httpapi.PlanCandidateInput) error {
	for _, c := range candidates {
		parsedPlanID, err := domain.ParseMealPlanID(c.PlanID)
		if err != nil {
			return fmt.Errorf("candidate insert: %w", err)
		}
		ref, err := a.db.GetRecipeRefByMealieID(ctx, c.MealieRecipeID)
		if err != nil {
			return fmt.Errorf("candidate insert: resolve recipe %q: %w", c.MealieRecipeID, err)
		}
		if err := a.db.InsertCandidate(ctx, persistence.MealPlanCandidate{
			PlanID: parsedPlanID, SlotDate: c.SlotDate, SlotKind: c.SlotKind, RecipeRefID: ref.ID,
			Score: c.Score, Breakdown: c.Breakdown, Feasible: c.Feasible, Rank: c.Rank,
		}); err != nil {
			return fmt.Errorf("candidate insert: %w", err)
		}
	}
	return nil
}

func (a storeAdapter) ListShoppingRequirements(ctx context.Context, planID string) ([]httpapi.ShoppingRequirementResponse, error) {
	parsedPlanID, err := domain.ParseMealPlanID(planID)
	if err != nil {
		return nil, fmt.Errorf("shopping requirements list: %w", err)
	}
	reqs, err := a.db.ListShoppingRequirements(ctx, parsedPlanID)
	if err != nil {
		return nil, fmt.Errorf("shopping requirements list: %w", err)
	}
	out := make([]httpapi.ShoppingRequirementResponse, 0, len(reqs))
	for _, r := range reqs {
		out = append(out, httpapi.ShoppingRequirementResponse{
			ID: r.ID.String(), IngredientID: r.IngredientID.String(), IngredientName: r.IngredientName,
			Quantity: r.Quantity, Unit: r.Unit, AcceptableForms: r.AcceptableForms, PreferredForm: r.PreferredForm,
		})
	}
	return out, nil
}

// resolvePlanSchool picks the skolmaten school slug for a plan run: the caller's
// request wins, otherwise the configured default (SKOLMATEN_SCHOOL) applies, and
// empty means the school-dedup signal stays off.
func resolvePlanSchool(req httpapi.PlanRunInput, configured string) string {
	if req.School != "" {
		return req.School
	}
	return configured
}

func (a storeAdapter) RunPlan(ctx context.Context, in httpapi.PlanRunInput) (httpapi.PlanRunResult, error) {
	school := resolvePlanSchool(in, a.school)
	result, err := RunPlan(ctx, RunPlanInput{
		Week:           in.Week,
		Days:           in.Days,
		CreateWishlist: in.CreateWishlist,
		Family:         "family.json",
		School:         school,
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

func (a storeAdapter) RunPlanWithProgress(ctx context.Context, in httpapi.PlanRunInput, progress func(httpapi.PlanProgress)) (httpapi.PlanRunResult, error) {
	school := resolvePlanSchool(in, a.school)
	result, err := RunPlan(ctx, RunPlanInput{
		Week:           in.Week,
		Days:           in.Days,
		CreateWishlist: in.CreateWishlist,
		Family:         "family.json",
		School:         school,
		Progress: func(phase, message string) {
			progress(httpapi.PlanProgress{Phase: phase, Message: message, At: time.Now()})
		},
	})
	if err != nil {
		return httpapi.PlanRunResult{}, fmt.Errorf("plan run: %w", err)
	}
	out := httpapi.PlanRunResult{
		Status:    "accepted",
		Message:   result.Message,
		WeekStart: func() *string { s := result.WeekStart.Format("2006-01-02"); return &s }(),
	}
	return out, nil
}

func (a storeAdapter) CreateMealEvent(ctx context.Context, in dto.MealEventNew) (dto.MealEventResponse, error) {
	servedOn, err := time.Parse("2006-01-02", in.ServedOn)
	if err != nil {
		return dto.MealEventResponse{}, fmt.Errorf("meals create: invalid served_on %q: %w", in.ServedOn, err)
	}
	ref, err := a.db.GetRecipeRefByMealieID(ctx, in.MealieRecipeID)
	if err != nil {
		return dto.MealEventResponse{}, fmt.Errorf("meals create: resolve recipe: %w", err)
	}
	eventID, err := a.db.CreateMealEvent(ctx, ref.ID, servedOn, nil, nil)
	if err != nil {
		return dto.MealEventResponse{}, fmt.Errorf("meals create: %w", err)
	}
	for _, rx := range in.Reactions {
		pid, perr := domain.ParsePersonID(rx.PersonID)
		if perr != nil {
			return dto.MealEventResponse{}, fmt.Errorf("meals create: parse person id: %w", perr)
		}
		if err := a.db.AddMealReaction(ctx, persistence.MealReaction{
			MealEventID: eventID, PersonID: pid, Sentiment: rx.Sentiment,
		}); err != nil {
			return dto.MealEventResponse{}, fmt.Errorf("meals create: add reaction: %w", err)
		}
	}
	rxns, err := a.db.ListMealReactions(ctx, eventID)
	if err != nil {
		return dto.MealEventResponse{}, fmt.Errorf("meals create: read reactions: %w", err)
	}
	out := dto.MealEventResponse{
		ID: eventID.String(), MealieRecipeID: in.MealieRecipeID,
		ServedOn:  in.ServedOn,
		CreatedAt: time.Now(),
		Reactions: make([]dto.MealReactionResponse, 0, len(rxns)),
	}
	for _, r := range rxns {
		out.Reactions = append(out.Reactions, dto.MealReactionResponse{
			PersonID: r.PersonID.String(), Sentiment: r.Sentiment,
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
			ID: c.ID, Kind: c.Kind, Value: c.Value, Active: c.Active,
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
	return httpapi.PlanningConstraintResponse{ID: id, Kind: in.Kind, Value: in.Value, Active: in.Active}, nil
}

func (a storeAdapter) ListShoppingLists(ctx context.Context) ([]httpapi.ShoppingListResponse, error) {
	lists, err := a.db.ListShoppingLists(ctx)
	if err != nil {
		return nil, fmt.Errorf("shopping lists list: %w", err)
	}
	out := make([]httpapi.ShoppingListResponse, 0, len(lists))
	for _, l := range lists {
		var owner *string
		if l.OwnerPersonID != nil {
			s := l.OwnerPersonID.String()
			owner = &s
		}
		out = append(out, httpapi.ShoppingListResponse{
			ID: l.ID.String(), OwnerPersonID: owner,
			Name: l.Name, Status: l.Status, CreatedAt: l.CreatedAt,
		})
	}
	return out, nil
}

func (a storeAdapter) CreateShoppingList(ctx context.Context, in httpapi.ShoppingListInput) (httpapi.ShoppingListResponse, error) {
	var owner *domain.PersonID
	if in.OwnerPersonID != nil {
		p, perr := domain.ParsePersonID(*in.OwnerPersonID)
		if perr != nil {
			return httpapi.ShoppingListResponse{}, fmt.Errorf("shopping list create: %w", perr)
		}
		owner = &p
	}
	l, err := a.db.CreateShoppingList(ctx, persistence.ShoppingList{
		OwnerPersonID: owner, Name: in.Name, Status: "active",
	})
	if err != nil {
		return httpapi.ShoppingListResponse{}, fmt.Errorf("shopping list create: %w", err)
	}
	return httpapi.ShoppingListResponse{ID: l.String(), Name: in.Name, Status: "active", CreatedAt: time.Now()}, nil
}

// CreateFromChecklist creates a shopping list plus its line items in one call, the
// ingestion target for the Mac-local Apple Notes reader (POST /shopping-lists/from-checklist).
func (a storeAdapter) CreateFromChecklist(ctx context.Context, in httpapi.ShoppingListFromChecklistInput) (httpapi.ShoppingListFromChecklistResponse, error) {
	items := make([]persistence.ShoppingListItem, 0, len(in.Items))
	for _, it := range in.Items {
		label := it.Label
		items = append(items, persistence.ShoppingListItem{
			Label: &label, Quantity: float64(it.Quantity), Unit: it.Unit, Checked: false,
		})
	}
	listID, itemIDs, err := a.db.CreateShoppingListWithItems(ctx,
		persistence.ShoppingList{Name: in.Name, Status: "active"}, items)
	if err != nil {
		return httpapi.ShoppingListFromChecklistResponse{}, fmt.Errorf("shopping list from-checklist create: %w", err)
	}
	respItems := make([]httpapi.ShoppingListItemResponse, 0, len(in.Items))
	for i, it := range in.Items {
		label := it.Label
		respItems = append(respItems, httpapi.ShoppingListItemResponse{
			ID: itemIDs[i].String(), ShoppingListID: listID.String(), Label: &label,
			Quantity: it.Quantity, Unit: it.Unit, Checked: false, AddedAt: time.Now(),
		})
	}
	return httpapi.ShoppingListFromChecklistResponse{
		ShoppingListResponse: httpapi.ShoppingListResponse{
			ID: listID.String(), Name: in.Name, Status: "active", CreatedAt: time.Now(),
		},
		Items: respItems,
	}, nil
}

// ListResolvedItemsSince returns the list's items that were confidently resolved
// and pushed to a retailer wishlist since the given time (all resolved items when
// since is the zero time). This is the cheap read the Apple Notes bridge polls to
// learn which checklist lines to check off.
func (a storeAdapter) ListResolvedItemsSince(ctx context.Context, listID string, since time.Time) ([]httpapi.ResolvedItemResponse, error) {
	id, err := domain.ParseShoppingListID(listID)
	if err != nil {
		return nil, httpapi.ErrNotFound
	}
	items, err := a.db.ListResolvedItemsSince(ctx, id, since)
	if err != nil {
		return nil, fmt.Errorf("list resolved items: %w", err)
	}
	out := make([]httpapi.ResolvedItemResponse, 0, len(items))
	for _, it := range items {
		ri := httpapi.ResolvedItemResponse{
			ID:           it.ID.String(),
			Label:        it.Label,
			NoteMatchKey: it.NoteMatchKey,
		}
		if it.ResolvedAt != nil {
			ri.ResolvedAt = *it.ResolvedAt
		}
		out = append(out, ri)
	}
	return out, nil
}

func (a storeAdapter) GetShoppingList(ctx context.Context, listID string) (httpapi.ShoppingListResponse, error) {
	id, err := domain.ParseShoppingListID(listID)
	if err != nil {
		return httpapi.ShoppingListResponse{}, httpapi.ErrNotFound
	}
	l, err := a.db.GetShoppingList(ctx, id)
	if err != nil {
		return httpapi.ShoppingListResponse{}, httpapi.ErrNotFound
	}
	var owner *string
	if l.OwnerPersonID != nil {
		s := l.OwnerPersonID.String()
		owner = &s
	}
	return httpapi.ShoppingListResponse{
		ID: l.ID.String(), OwnerPersonID: owner,
		Name: l.Name, Status: l.Status, CreatedAt: l.CreatedAt,
	}, nil
}

func (a storeAdapter) ArchiveShoppingList(ctx context.Context, listID string) error {
	id, err := domain.ParseShoppingListID(listID)
	if err != nil {
		return httpapi.ErrNotFound
	}
	_, err = a.db.GetShoppingList(ctx, id)
	if err != nil {
		return httpapi.ErrNotFound
	}
	if err := a.db.UpdateShoppingListStatus(ctx, id, "archived"); err != nil {
		return fmt.Errorf("shopping list archive: %w", err)
	}
	return nil
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

func (a storeAdapter) ListShoppingListItems(ctx context.Context, listID string) ([]httpapi.ShoppingListItemResponse, error) {
	id, err := domain.ParseShoppingListID(listID)
	if err != nil {
		return nil, fmt.Errorf("shopping list items list: %w", err)
	}
	items, err := a.db.ListShoppingListItems(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("shopping list items list: %w", err)
	}
	out := make([]httpapi.ShoppingListItemResponse, 0, len(items))
	for _, it := range items {
		var ingID *string
		if it.IngredientID != nil {
			s := it.IngredientID.String()
			ingID = &s
		}
		var reqID *string
		if it.ShoppingRequirementID != nil {
			s := it.ShoppingRequirementID.String()
			reqID = &s
		}
		out = append(out, httpapi.ShoppingListItemResponse{
			ID: it.ID.String(), ShoppingListID: it.ShoppingListID.String(),
			ShoppingRequirementID: reqID,
			IngredientID: ingID, Label: it.Label,
			Quantity: float64ToFloat32(it.Quantity), Unit: it.Unit, Checked: it.Checked, AddedAt: it.AddedAt,
		})
	}
	return out, nil
}

func (a storeAdapter) AddShoppingListItem(ctx context.Context, listID string, in httpapi.ShoppingListItemInput) (httpapi.ShoppingListItemResponse, error) {
	slid, err := domain.ParseShoppingListID(listID)
	if err != nil {
		return httpapi.ShoppingListItemResponse{}, fmt.Errorf("shopping list item create: %w", err)
	}
	var ingID *domain.IngredientID
	if in.IngredientID != nil {
		parsed, err := domain.ParseIngredientID(*in.IngredientID)
		if err != nil {
			return httpapi.ShoppingListItemResponse{}, fmt.Errorf("shopping list item create: %w", err)
		}
		ingID = &parsed
	}
	var reqID *domain.ShoppingRequirementID
	if in.ShoppingRequirementID != nil {
		parsed, err := domain.ParseShoppingRequirementID(*in.ShoppingRequirementID)
		if err != nil {
			return httpapi.ShoppingListItemResponse{}, fmt.Errorf("shopping list item create: %w", err)
		}
		reqID = &parsed
	}
	it, err := a.db.CreateShoppingListItem(ctx, persistence.ShoppingListItem{
		ShoppingListID: slid, ShoppingRequirementID: reqID,
		IngredientID: ingID, Label: in.Label,
		Quantity: float64(in.Quantity), Unit: in.Unit, Checked: false,
	})
	if err != nil {
		return httpapi.ShoppingListItemResponse{}, fmt.Errorf("shopping list item create: %w", err)
	}
	return httpapi.ShoppingListItemResponse{
		ID: it.String(), ShoppingListID: listID,
		ShoppingRequirementID: in.ShoppingRequirementID,
		IngredientID: in.IngredientID, Label: in.Label,
		Quantity: in.Quantity, Unit: in.Unit, Checked: false, AddedAt: time.Now(),
	}, nil
}

func (a storeAdapter) ToggleShoppingListItem(ctx context.Context, listID, itemID string, checked bool) (httpapi.ShoppingListItemResponse, error) {
	slid, err := domain.ParseShoppingListID(listID)
	if err != nil {
		return httpapi.ShoppingListItemResponse{}, httpapi.ErrNotFound
	}
	iiid, err := domain.ParseShoppingListItemID(itemID)
	if err != nil {
		return httpapi.ShoppingListItemResponse{}, httpapi.ErrNotFound
	}
	if err := a.db.UpdateShoppingListItemChecked(ctx, iiid, checked); err != nil {
		return httpapi.ShoppingListItemResponse{}, httpapi.ErrNotFound
	}
	items, err := a.db.ListShoppingListItems(ctx, slid)
	if err != nil {
		return httpapi.ShoppingListItemResponse{}, fmt.Errorf("shopping list item toggle: %w", err)
	}
	for _, it := range items {
		if it.ID == iiid {
			var ingID *string
			if it.IngredientID != nil {
				s := it.IngredientID.String()
				ingID = &s
			}
			var reqID *string
			if it.ShoppingRequirementID != nil {
				s := it.ShoppingRequirementID.String()
				reqID = &s
			}
			return httpapi.ShoppingListItemResponse{
				ID: it.ID.String(), ShoppingListID: it.ShoppingListID.String(),
				ShoppingRequirementID: reqID,
				IngredientID: ingID, Label: it.Label,
				Quantity: float64ToFloat32(it.Quantity), Unit: it.Unit, Checked: it.Checked, AddedAt: it.AddedAt,
			}, nil
		}
	}
	return httpapi.ShoppingListItemResponse{}, httpapi.ErrNotFound
}

func (a storeAdapter) DeleteShoppingListItem(ctx context.Context, listID, itemID string) error {
	iiid, err := domain.ParseShoppingListItemID(itemID)
	if err != nil {
		return httpapi.ErrNotFound
	}
	if err := a.db.DeleteShoppingListItem(ctx, iiid); err != nil {
		return httpapi.ErrNotFound
	}
	return nil
}

func (a storeAdapter) PushShoppingList(ctx context.Context, listID string, retailer string) (httpapi.RetailerListBindingResponse, error) {
	slid, err := domain.ParseShoppingListID(listID)
	if err != nil {
		return httpapi.RetailerListBindingResponse{}, err
	}
	if _, err := PushShoppingList(ctx, a.db, slid, a.adapterURL, retailer); err != nil {
		return httpapi.RetailerListBindingResponse{}, err
	}
	binding, err := a.db.GetRetailerListBinding(ctx, slid, retailer)
	if err != nil {
		return httpapi.RetailerListBindingResponse{}, fmt.Errorf("push shopping list: get binding: %w", err)
	}
	out := httpapi.RetailerListBindingResponse{
		ShoppingListID: binding.ShoppingListID.String(),
		Retailer: binding.Retailer, ExternalListID: binding.ExternalListID,
		SyncDirection: binding.SyncDirection, LastPushedAt: binding.LastPushedAt,
	}
	if binding.LastPushStatus != nil {
		s := httpapi.RetailerListBindingLastPushStatus(*binding.LastPushStatus)
		out.LastPushStatus = &s
	}
	return out, nil
}

func (a storeAdapter) ListShoppingCarts(ctx context.Context, listID string) ([]httpapi.ShoppingCartResponse, error) {
	slid, err := domain.ParseShoppingListID(listID)
	if err != nil {
		return nil, fmt.Errorf("shopping carts list: %w", err)
	}
	bindings, err := a.db.ListRetailerListBindings(ctx, slid)
	if err != nil {
		return nil, fmt.Errorf("shopping carts list: %w", err)
	}
	out := make([]httpapi.ShoppingCartResponse, 0)
	for _, b := range bindings {
		carts, err := a.db.ListShoppingCarts(ctx, b.ShoppingListID, b.Retailer)
		if err != nil {
			continue
		}
		for _, c := range carts {
			out = append(out, httpapi.ShoppingCartResponse{
				ID: c.ID.String(),
				CreatedAt: c.CreatedAt, Status: c.Status,
			})
		}
	}
	return out, nil
}

func (a storeAdapter) ToCart(ctx context.Context, listID string, retailer string) (httpapi.ShoppingCartResponse, error) {
	slid, err := domain.ParseShoppingListID(listID)
	if err != nil {
		return httpapi.ShoppingCartResponse{}, httpapi.ErrNotFound
	}
	binding, err := a.db.GetRetailerListBinding(ctx, slid, retailer)
	if err != nil {
		return httpapi.ShoppingCartResponse{}, httpapi.ErrNotFound
	}
	carts, err := a.db.ListShoppingCarts(ctx, binding.ShoppingListID, binding.Retailer)
	if err != nil {
		return httpapi.ShoppingCartResponse{}, fmt.Errorf("to cart: %w", err)
	}
	if len(carts) == 0 {
		return httpapi.ShoppingCartResponse{}, fmt.Errorf("to cart: no carts for binding %s", binding.ShoppingListID)
	}
	c := carts[0]
	return httpapi.ShoppingCartResponse{
		ID: c.ID.String(),
		CreatedAt: c.CreatedAt, Status: c.Status,
	}, nil
}

func (a storeAdapter) ListOrders(ctx context.Context, retailer *string, cartID *string) ([]httpapi.OrderResponse, error) {
	var cartIDTyped *domain.ShoppingCartID
	if cartID != nil {
		parsed, err := domain.ParseShoppingCartID(*cartID)
		if err != nil {
			return nil, fmt.Errorf("orders list: %w", err)
		}
		cartIDTyped = &parsed
	}
	orders, err := a.db.ListOrders(ctx, cartIDTyped, retailer)
	if err != nil {
		return nil, fmt.Errorf("orders list: %w", err)
	}
	out := make([]httpapi.OrderResponse, 0, len(orders))
	for _, o := range orders {
		var scid *string
		if o.ShoppingCartID != nil {
			s := o.ShoppingCartID.String()
			scid = &s
		}
		out = append(out, httpapi.OrderResponse{
			ID: o.ID.String(), ShoppingCartID: scid,
			Retailer: o.Retailer, Source: o.Source,
			OrderedAt: o.OrderedAt, TotalPriceMinor: o.TotalPriceMinor,
			Currency: o.Currency,
		})
	}
	return out, nil
}

func (a storeAdapter) GetOrder(ctx context.Context, orderID string) (httpapi.OrderViewResponse, error) {
	oid, err := domain.ParseOrderID(orderID)
	if err != nil {
		return httpapi.OrderViewResponse{}, fmt.Errorf("get order: %w", err)
	}
	o, err := a.db.GetOrder(ctx, oid)
	if err != nil {
		return httpapi.OrderViewResponse{}, httpapi.ErrNotFound
	}
	items, err := a.db.ListOrderItems(ctx, oid)
	if err != nil {
		return httpapi.OrderViewResponse{}, fmt.Errorf("get order: %w", err)
	}
	var scid *string
	if o.ShoppingCartID != nil {
		s := o.ShoppingCartID.String()
		scid = &s
	}
	out := httpapi.OrderViewResponse{
		Order: httpapi.OrderResponse{
			ID: o.ID.String(), ShoppingCartID: scid,
			Retailer: o.Retailer, Source: o.Source,
			OrderedAt: o.OrderedAt, TotalPriceMinor: o.TotalPriceMinor,
			Currency: o.Currency,
		},
	}
	for _, it := range items {
		var subFor *string
		if it.SubstitutedForItemID != nil {
			s := it.SubstitutedForItemID.String()
			subFor = &s
		}
		out.Items = append(out.Items, httpapi.OrderItemResponse{
			ID: it.ID.String(), OrderID: it.OrderID.String(),
			RetailerProductID: it.RetailerProductID.String(),
			Quantity: float64ToFloat32(it.Quantity), UnitPrice: float64PtrToFloat32Ptr(it.UnitPrice),
			TotalPriceMinor: it.TotalPriceMinor, Currency: it.Currency, SubstitutedForItemID: subFor,
		})
	}
	return out, nil
}

func (a storeAdapter) ListOrderItems(ctx context.Context, orderID string) ([]httpapi.OrderItemResponse, error) {
	oid, err := domain.ParseOrderID(orderID)
	if err != nil {
		return nil, fmt.Errorf("order items list: %w", err)
	}
	items, err := a.db.ListOrderItems(ctx, oid)
	if err != nil {
		return nil, fmt.Errorf("order items list: %w", err)
	}
	out := make([]httpapi.OrderItemResponse, 0, len(items))
	for _, it := range items {
		var subFor *string
		if it.SubstitutedForItemID != nil {
			s := it.SubstitutedForItemID.String()
			subFor = &s
		}
		out = append(out, httpapi.OrderItemResponse{
			ID: it.ID.String(), OrderID: it.OrderID.String(),
			RetailerProductID: it.RetailerProductID.String(),
			Quantity: float64ToFloat32(it.Quantity), UnitPrice: float64PtrToFloat32Ptr(it.UnitPrice),
			TotalPriceMinor: it.TotalPriceMinor, Currency: it.Currency, SubstitutedForItemID: subFor,
		})
	}
	return out, nil
}

func (a storeAdapter) UploadRetailerCredential(ctx context.Context, retailer string, payload json.RawMessage) (httpapi.RetailerCredentialResponse, error) {
	if err := a.db.UpsertRetailerCredential(ctx, retailer, payload); err != nil {
		return httpapi.RetailerCredentialResponse{}, fmt.Errorf("upload retailer credential: %w", err)
	}
	return a.GetRetailerCredential(ctx, retailer)
}

func (a storeAdapter) GetRetailerCredential(ctx context.Context, retailer string) (httpapi.RetailerCredentialResponse, error) {
	cred, found, err := a.db.GetRetailerCredential(ctx, retailer)
	if err != nil {
		return httpapi.RetailerCredentialResponse{}, fmt.Errorf("get retailer credential: %w", err)
	}
	if !found {
		return httpapi.RetailerCredentialResponse{}, httpapi.ErrNotFound
	}
	return httpapi.RetailerCredentialResponse{
		Retailer:   cred.Retailer,
		Payload:    json.RawMessage(cred.Payload),
		UploadedAt: cred.UploadedAt,
	}, nil
}

// newPersonID generates a 16-char hex id from crypto/rand (stdlib only — no new dep).
func newPersonID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
