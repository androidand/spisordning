// Package main is the composition root for the MCP server: it owns the only edge
// that may import both the persistence layer and the mcptools layer, bridging the
// two with a small adapter that keeps internal/mcptools free of persistence
// (enforced by internal/architecturetest). The adapter loads the household and
// recipe candidates from persistence and delegates planning to the application
// layer (internal/planning) — it never constructs or executes SQL itself.
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/androidand/spisordning/internal/config"
	"github.com/androidand/spisordning/internal/domain"
	"github.com/androidand/spisordning/internal/mcptools"
	"github.com/androidand/spisordning/internal/persistence"
	"github.com/androidand/spisordning/internal/planning"
	"github.com/androidand/spisordning/internal/retailer"
	"github.com/androidand/spisordning/internal/service"
	"github.com/androidand/spisordning/internal/skolmaten"
)

// mcpStoreAdapter adapts *persistence.Store to the mcptools service interfaces.
// It is the sole place that knows both the persistence row types and the
// mcptools DTOs; mcptools sees only the interfaces it defines itself.
type mcpStoreAdapter struct {
	db        *persistence.Store
	willysURL string
	icaURL    string
	hemkopURL string
	cfg       config.Config
	// recipes is nil when Mealie isn't configured; StructureRecipe reports
	// that as an error rather than nil-dereferencing, same degrade-gracefully
	// pattern as the rest of buildMCPDeps.
	recipes *service.Recipes
}

// PlanDinners loads the household and recipe candidates, then delegates to the
// application-layer planner.
func (a mcpStoreAdapter) PlanDinners(ctx context.Context, date time.Time, days int) ([]mcptools.PlannedSlot, error) {
	candidates, err := a.loadCandidates(ctx)
	if err != nil {
		return nil, fmt.Errorf("plan dinners: load candidates: %w", err)
	}
	people, prefs, err := a.loadHousehold(ctx)
	if err != nil {
		return nil, fmt.Errorf("plan dinners: load household: %w", err)
	}
	energy, err := a.loadEnergyFor(ctx)
	if err != nil {
		return nil, fmt.Errorf("plan dinners: load effort profile: %w", err)
	}
	schoolTags, err := a.loadSchoolTagsFor(ctx, date)
	if err != nil {
		return nil, fmt.Errorf("plan dinners: load school tags: %w", err)
	}

	slots := planning.PlanWeek(ctx, planning.WeekConfig{
		Candidates:    candidates,
		People:        people,
		Preferences:   prefs,
		EnergyFor:     energy,
		SchoolTagsFor: schoolTags,
	}, date, days)

	out := make([]mcptools.PlannedSlot, 0, len(slots))
	for _, slot := range slots {
		out = append(out, mcptools.PlannedSlot{
			Date:   slot.Date.Format("2006-01-02"),
			Slot:   string(slot.Slot),
			Recipe: slot.Winner.Candidate.MealieRecipeID,
			Title:  slot.Winner.Candidate.Title,
			Score:  slot.Winner.Score,
		})
	}
	return out, nil
}

// PlanSlots plans the requested slot kinds for a date range.
func (a mcpStoreAdapter) PlanSlots(ctx context.Context, date time.Time, days int, slots []string) ([]mcptools.PlannedSlot, error) {
	candidates, err := a.loadCandidates(ctx)
	if err != nil {
		return nil, fmt.Errorf("plan slots: load candidates: %w", err)
	}
	people, prefs, err := a.loadHousehold(ctx)
	if err != nil {
		return nil, fmt.Errorf("plan slots: load household: %w", err)
	}
	energy, err := a.loadEnergyFor(ctx)
	if err != nil {
		return nil, fmt.Errorf("plan slots: load effort profile: %w", err)
	}
	schoolTags, err := a.loadSchoolTagsFor(ctx, date)
	if err != nil {
		return nil, fmt.Errorf("plan slots: load school tags: %w", err)
	}

	cfg := planning.WeekConfig{
		Candidates:    candidates,
		People:        people,
		Preferences:   prefs,
		EnergyFor:     energy,
		SchoolTagsFor: schoolTags,
	}

	var wantDinner, wantBreakfast, wantSnack bool
	for _, s := range slots {
		switch s {
		case "dinner":
			wantDinner = true
		case "breakfast":
			wantBreakfast = true
		case "snack":
			wantSnack = true
		}
	}

	var allSlots []planning.PlannedSlot

	if wantDinner || (!wantBreakfast && !wantSnack) {
		// Default to dinner when no slots specified.
		allSlots = append(allSlots, planning.PlanWeek(ctx, cfg, date, days)...)
	}

	if wantBreakfast {
		for i := 0; i < days; i++ {
			d := date.AddDate(0, 0, i)
			breakfastCands := planning.BreakfastCandidates(candidates, d)
			if bs := planning.PlanSimpleSlot(ctx, cfg, breakfastCands, d, domain.SlotBreakfast); bs != nil {
				allSlots = append(allSlots, *bs)
			}
		}
	}

	if wantSnack {
		snackCands := planning.SnackCandidates(candidates)
		for i := 0; i < days; i++ {
			d := date.AddDate(0, 0, i)
			if ss := planning.PlanSimpleSlot(ctx, cfg, snackCands, d, domain.SlotSnack); ss != nil {
				allSlots = append(allSlots, *ss)
			}
		}
	}

	out := make([]mcptools.PlannedSlot, 0, len(allSlots))
	for _, slot := range allSlots {
		out = append(out, mcptools.PlannedSlot{
			Date:   slot.Date.Format("2006-01-02"),
			Slot:   string(slot.Slot),
			Recipe: slot.Winner.Candidate.MealieRecipeID,
			Title:  slot.Winner.Candidate.Title,
			Score:  slot.Winner.Score,
		})
	}
	return out, nil
}

// RecordReaction creates a meal event for the served recipe/date and records the
// household member's reaction.
func (a mcpStoreAdapter) RecordReaction(ctx context.Context, in mcptools.RecordReactionInput) (mcptools.RecordReactionResult, error) {
	servedOn, err := time.Parse("2006-01-02", in.ServedOn)
	if err != nil {
		return mcptools.RecordReactionResult{}, fmt.Errorf("record reaction: invalid served_on %q: %w", in.ServedOn, err)
	}
	// planID/planSlotDate: not available at this call site (MCP-driven reaction
	// recording isn't tied to a specific plan slot) — nil means "unlinked".
	// When a slot kind is specified, pass it through so the meal_event row
	// carries the slot_kind for future plan-linking.
	var slotKind *string
	if in.Slot != "" && in.Slot != "dinner" {
		s := in.Slot
		slotKind = &s
	}
	ref, err := a.db.GetRecipeRefByMealieID(ctx, in.Recipe)
	if err != nil {
		return mcptools.RecordReactionResult{}, fmt.Errorf("record reaction: resolve recipe: %w", err)
	}
	var eventID domain.MealEventID
	if slotKind != nil {
		eventID, err = a.db.CreateMealEventWithSlot(ctx, ref.ID, servedOn, nil, nil, slotKind)
	} else {
		eventID, err = a.db.CreateMealEvent(ctx, ref.ID, servedOn, nil, nil)
	}
	if err != nil {
		return mcptools.RecordReactionResult{}, fmt.Errorf("record reaction: create meal event: %w", err)
	}
	pid, perr := domain.ParsePersonID(in.PersonID)
	if perr != nil {
		return mcptools.RecordReactionResult{}, fmt.Errorf("record reaction: parse person id: %w", perr)
	}
	if err := a.db.AddMealReaction(ctx, persistence.MealReaction{
		MealEventID: eventID,
		PersonID:    pid,
		Sentiment:   in.Sentiment,
	}); err != nil {
		return mcptools.RecordReactionResult{}, fmt.Errorf("record reaction: add reaction: %w", err)
	}
	return mcptools.RecordReactionResult{
		MealEventID: eventID.String(),
		Recipe:      in.Recipe,
		ServedOn:    in.ServedOn,
		PersonID:    in.PersonID,
		Sentiment:   in.Sentiment,
	}, nil
}

// ShoppingRequirements loads each recipe's canonical ingredient lines and
// aggregates them into shopping requirements via the application layer.
func (a mcpStoreAdapter) ShoppingRequirements(ctx context.Context, recipeIDs []string) ([]mcptools.ShoppingRequirement, error) {
	meals := make([]planning.ChosenMeal, 0, len(recipeIDs))
	for _, id := range recipeIDs {
		ref, err := a.db.GetRecipeRefByMealieID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("shopping requirements: resolve recipe %q: %w", id, err)
		}
		lines, err := a.db.ListRecipeIngredients(ctx, ref.ID)
		if err != nil {
			return nil, fmt.Errorf("shopping requirements: list ingredients for %q: %w", id, err)
		}
		ings := make([]domain.Ingredient, 0, len(lines))
		for _, l := range lines {
			ings = append(ings, domain.Ingredient{
				IngredientID: l.IngredientID.String(),
				Quantity:     l.Quantity,
				Unit:         l.Unit,
			})
		}
		meals = append(meals, planning.ChosenMeal{MealieRecipeID: id, Ingredients: ings})
	}

	reqs := planning.BuildRequirements(meals)
	out := make([]mcptools.ShoppingRequirement, 0, len(reqs))
	for _, r := range reqs {
		out = append(out, mcptools.ShoppingRequirement{
			Ingredient:      r.IngredientID,
			Quantity:        r.Quantity,
			Unit:            r.Unit,
			AcceptableForms: r.AcceptableForms,
			PreferredForm:   r.PreferredForm,
		})
	}
	return out, nil
}

// StructureRecipe turns freeform recipe text into a real Mealie recipe via the
// application layer, and maps the result onto the mcptools DTO.
func (a mcpStoreAdapter) StructureRecipe(ctx context.Context, rawText string) (mcptools.StructureRecipeResult, error) {
	if a.recipes == nil {
		return mcptools.StructureRecipeResult{}, fmt.Errorf("structure_recipe: no Mealie instance configured")
	}
	res, err := a.recipes.StructureFromText(ctx, rawText)
	if err != nil {
		return mcptools.StructureRecipeResult{}, err
	}
	out := mcptools.StructureRecipeResult{
		RecipeID:      res.RecipeID,
		Title:         res.Title,
		Instructions:  res.Instructions,
		LowConfidence: res.LowConfidence,
	}
	for _, ing := range res.Ingredients {
		out.Ingredients = append(out.Ingredients, mcptools.StructuredIngredientResult{
			Note: ing.Note, FoodName: ing.FoodName, Quantity: ing.Quantity, Unit: ing.Unit,
		})
	}
	return out, nil
}

// loadCandidates builds the planner's candidate list from the cached recipe
// references and their canonical ingredient lines.
func (a mcpStoreAdapter) loadCandidates(ctx context.Context) ([]domain.Candidate, error) {
	refs, err := a.db.ListRecipeRefs(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Candidate, 0, len(refs))
	for _, ref := range refs {
		lines, err := a.db.ListRecipeIngredients(ctx, ref.ID)
		if err != nil {
			return nil, fmt.Errorf("ingredients for %q: %w", ref.MealieRecipeID, err)
		}
		ids := make([]string, 0, len(lines))
		for _, l := range lines {
			ids = append(ids, l.IngredientID.String())
		}
		out = append(out, domain.Candidate{
			MealieRecipeID: ref.MealieRecipeID,
			Title:          ref.Title,
			Tags:           ref.Tags,
			Ingredients:    ids,
			Effort:         domain.Effort(ref.Effort),
		})
	}
	return out, nil
}

// loadHousehold builds the planner's people and their preferences.
func (a mcpStoreAdapter) loadHousehold(ctx context.Context) ([]domain.Person, []domain.Preference, error) {
	people, err := a.db.ListPeople(ctx)
	if err != nil {
		return nil, nil, err
	}
	domainPeople := make([]domain.Person, 0, len(people))
	for _, p := range people {
		pid, err := domain.ParsePersonID(p.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("parse person id %q: %w", p.ID, err)
		}
		domainPeople = append(domainPeople, domain.Person{ID: pid, Name: p.Name, Weight: p.Weight})
	}

	var prefs []domain.Preference
	for _, p := range people {
		pid, err := domain.ParsePersonID(p.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("parse person id %q: %w", p.ID, err)
		}
		rows, err := a.db.ListPreferences(ctx, pid)
		if err != nil {
			return nil, nil, fmt.Errorf("preferences for %q: %w", p.ID, err)
		}
		for _, pr := range rows {
			prefs = append(prefs, domain.Preference{
				PersonID:   pr.PersonID,
				Tag:        pr.Tag,
				Sentiment:  domain.Sentiment(pr.Sentiment),
				Confidence: pr.Confidence,
			})
		}
	}
	return domainPeople, prefs, nil
}

// loadEnergyFor builds the planner's per-weekday kitchen-energy function from
// the effort_profile table, mirroring the CLI's family.json KitchenEnergy.
// Weekdays with no profile row default to EffortMedium, so an unconfigured
// household still gets a sane budget instead of an infeasible zero.
func (a mcpStoreAdapter) loadEnergyFor(ctx context.Context) (func(time.Time) domain.Effort, error) {
	rows, err := a.db.ListEffortProfiles(ctx)
	if err != nil {
		return nil, err
	}
	energy := make(map[int]domain.Effort, len(rows))
	for _, e := range rows {
		energy[e.Weekday] = domain.Effort(e.KitchenEnergy)
	}
	return func(date time.Time) domain.Effort {
		if e, ok := energy[int(date.Weekday())]; ok {
			return e
		}
		return domain.EffortMedium
	}, nil
}

// loadSchoolTagsFor builds the planner's school-lunch tag function from the
// skolmaten service, mirroring the CLI's --school flag. If SKOLMATEN_SCHOOL is
// unset, it returns a no-op closure (no school dedup). Errors from the
// skolmaten service are non-fatal: the planner continues without school tags.
func (a mcpStoreAdapter) loadSchoolTagsFor(ctx context.Context, date time.Time) (func(time.Time) []string, error) {
	school := a.cfg.SkolmatenSchool
	if school == "" {
		return func(time.Time) []string { return nil }, nil
	}
	baseURL := a.cfg.SkolmatenBaseURL
	if baseURL == "" {
		baseURL = "http://192.168.1.120:8787"
	}
	token := a.cfg.SkolmatenClientToken
	sk := skolmaten.New(baseURL, token)

	year, week := date.AddDate(0, 0, 7).ISOWeek()
	menu, err := sk.WeekMenu(ctx, school, year, week)
	if err != nil {
		// Non-fatal: continue without school dedup.
		return func(time.Time) []string { return nil }, nil
	}

	tagsByDate := make(map[string][]string, len(menu))
	for _, day := range menu {
		tagsByDate[day.Date.Format("2006-01-02")] = skolmaten.TagsForDay(day)
	}
	return func(d time.Time) []string {
		return tagsByDate[d.Format("2006-01-02")]
	}, nil
}

// ── Shopping services ───────────────────────────────────────────────────────

// CreateShoppingList creates a spisordning shopping list and its items from
// canonical requirements.
func (a mcpStoreAdapter) CreateShoppingList(ctx context.Context, in mcptools.CreateShoppingListInput) (mcptools.CreateShoppingListResult, error) {
	items := make([]persistence.ShoppingListItem, 0, len(in.Items))
	for _, item := range in.Items {
		ingredientID, err := domain.ParseIngredientID(item.Ingredient)
		if err != nil {
			return mcptools.CreateShoppingListResult{}, fmt.Errorf("create shopping list: %w", err)
		}
		items = append(items, persistence.ShoppingListItem{
			IngredientID: &ingredientID,
			Quantity:     item.Quantity,
			Unit:         item.Unit,
		})
	}
	listID, _, err := a.db.CreateShoppingListWithItems(ctx, persistence.ShoppingList{Name: in.Name, Status: "active"}, items)
	if err != nil {
		return mcptools.CreateShoppingListResult{}, fmt.Errorf("create shopping list: %w", err)
	}
	return mcptools.CreateShoppingListResult{
		ListID: listID.String(),
		Name:   in.Name,
		Status: "active",
		Items:  len(in.Items),
	}, nil
}

// ComparePrices compares prices across retailers for the given requirements.
// A stale or unavailable retailer degrades to available:false per item rather
// than failing the whole call.
func (a mcpStoreAdapter) ComparePrices(ctx context.Context, reqs []mcptools.ShoppingRequirement) (mcptools.PriceComparison, error) {
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
	return toMCPComparison(cmp), nil
}

// PushToWishlist pushes resolved lines to a retailer's wishlist and records
// the binding. It never fills a cart or checks out.
func (a mcpStoreAdapter) PushToWishlist(ctx context.Context, in mcptools.PushWishlistInput) (mcptools.PushWishlistResult, error) {
	rc, err := retailer.NewFromKind(retailer.RetailerKind(in.Retailer), a.willysURL, a.icaURL, a.hemkopURL)
	if err != nil {
		return mcptools.PushWishlistResult{}, fmt.Errorf("push wishlist: %w", err)
	}
	rc.WithAuthFile(a.cfg.ICAAuthFile)
	items := make([]retailer.ShoppingListItem, 0, len(in.Items))
	for _, it := range in.Items {
		items = append(items, retailer.ShoppingListItem{ProductCode: it.ProductCode, Quantity: it.Quantity})
	}
	created, err := rc.CreateShoppingList(ctx, in.ListName, items)
	if err != nil {
		return mcptools.PushWishlistResult{}, fmt.Errorf("push wishlist: create: %w", err)
	}
	if in.ShoppingListID != nil {
		slid, err := domain.ParseShoppingListID(*in.ShoppingListID)
		if err != nil {
			return mcptools.PushWishlistResult{}, fmt.Errorf("push wishlist: %w", err)
		}
		now := time.Now()
		status := "success"
		if err := a.db.CreateOrUpdateRetailerListBinding(ctx, persistence.RetailerListBinding{
			ShoppingListID: slid,
			Retailer:       in.Retailer,
			ExternalListID: created.WishlistID,
			SyncDirection:  "outbound",
			LastPushedAt:   &now,
			LastPushStatus: &status,
		}); err != nil {
			return mcptools.PushWishlistResult{}, fmt.Errorf("push wishlist: record binding: %w", err)
		}
	}
	return mcptools.PushWishlistResult{
		Retailer:       in.Retailer,
		WishlistID:     created.WishlistID,
		ListName:       created.Name,
		Items:          len(items),
		ShoppingListID: in.ShoppingListID,
	}, nil
}

// toMCPComparison maps a retailer.Comparison to the MCP tool output shape.
func toMCPComparison(cmp *retailer.Comparison) mcptools.PriceComparison {
	out := mcptools.PriceComparison{Items: make([]mcptools.ItemComparison, 0, len(cmp.Items))}
	for _, item := range cmp.Items {
		mi := mcptools.ItemComparison{
			Ingredient: item.Requirement.IngredientID,
			Results:    make([]mcptools.RetailerPriceResult, 0, len(item.Results)),
			Unresolved: item.Unresolved,
		}
		for _, r := range item.Results {
			mi.Results = append(mi.Results, toMCPResult(r))
		}
		if item.Cheapest != nil {
			c := toMCPResult(*item.Cheapest)
			mi.Cheapest = &c
		}
		out.Items = append(out.Items, mi)
	}
	return out
}

// toMCPResult maps a single retailer.RetailerResult to the MCP output shape.
func toMCPResult(r retailer.RetailerResult) mcptools.RetailerPriceResult {
	mr := mcptools.RetailerPriceResult{
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
