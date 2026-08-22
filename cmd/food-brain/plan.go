package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/androidand/spisordning/internal/llm"
	"github.com/androidand/spisordning/internal/mealie"
	"github.com/androidand/spisordning/internal/planning"
	"github.com/androidand/spisordning/internal/retailer"
	"github.com/androidand/spisordning/internal/scoring"
	"github.com/androidand/spisordning/internal/skolmaten"
)

// familyConfig is the local family file (people, preferences, weekday energy)
// used until these domains live in Postgres. See family.example.json.
type familyConfig struct {
	People      []domain.Person     `json:"people"`
	Preferences []domain.Preference `json:"preferences"`
	// KitchenEnergy per weekday: mon..sun -> 1 (low) .. 3 (high).
	KitchenEnergy map[string]int `json:"kitchenEnergy"`
}

var weekdayKeys = [...]string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}

// RunPlanInput carries the parameters for a plan run from the HTTP API.
type RunPlanInput struct {
	Week           string
	Days           int
	CreateWishlist bool
	Family         string
	School         string
}

// RunPlanResult carries the outcome of a plan run.
type RunPlanResult struct {
	WeekStart    time.Time
	WeekYear     int
	WeekNum      int
	PlanCount    int
	WishlistName string
	WishlistID   string
	ItemCount    int
	ReviewCount  int
	Message      string
	Errors       []string
}

// RunPlan executes the weekly planning pipe with the given parameters.
// It is the HTTP-api-compatible entry point; the CLI entry point (runPlan)
// parses flags and delegates here. Errors are collected and returned as a
// single error so the caller can report them without stdout noise.
func RunPlan(ctx context.Context, in RunPlanInput) (RunPlanResult, error) {
	if in.Family == "" {
		in.Family = "family.json"
	}
	if in.Days <= 0 {
		in.Days = 7
	}

	fam, err := loadFamily(in.Family)
	if err != nil {
		return RunPlanResult{}, err
	}
	mealieURL, mealieToken := os.Getenv("MEALIE_BASE_URL"), os.Getenv("MEALIE_API_TOKEN")
	if mealieURL == "" || mealieToken == "" {
		return RunPlanResult{}, fmt.Errorf("MEALIE_BASE_URL and MEALIE_API_TOKEN must be set")
	}
	adapterURL := envOr("ADAPTER_URL", "http://localhost:8402")

	year, week := nextISOWeek(time.Now())
	if in.Week != "" {
		if _, err := fmt.Sscanf(in.Week, "%d-W%d", &year, &week); err != nil {
			return RunPlanResult{}, fmt.Errorf("invalid week %q (want e.g. 2026-W31)", in.Week)
		}
	}
	monday := mondayOfISOWeek(year, week)

	// ── Inputs ────────────────────────────────────────────────────────────────
	refs, err := mealie.New(mealieURL, mealieToken).SyncRecipes(ctx)
	if err != nil {
		return RunPlanResult{}, fmt.Errorf("mealie sync: %w", err)
	}
	if len(refs) == 0 {
		return RunPlanResult{}, fmt.Errorf("no recipes in Mealie — add some first")
	}
	candidates, ingredientLines := candidatesFromRefs(refs)

	schoolTags := map[string][]string{} // date -> tags
	if in.School != "" {
		sk := skolmaten.New(envOr("SKOLMATEN_BASE_URL", "http://192.168.1.120:8787"), os.Getenv("SKOLMATEN_CLIENT_TOKEN"))
		menu, err := sk.WeekMenu(ctx, in.School, year, week)
		if err != nil {
			// Non-fatal: continue without school dedup.
		}
		for _, day := range menu {
			schoolTags[day.Date.Format("2006-01-02")] = skolmaten.TagsForDay(day)
		}
	}

	var olla llm.Provider
	if base := os.Getenv("OLLA_OPENAI_BASE_URL"); base != "" && os.Getenv("OLLA_MODEL") != "" {
		olla = llm.New(base, os.Getenv("OLLA_MODEL"))
	}

	// ── Per-day planning ──────────────────────────────────────────────────────
	planned := planning.PlanWeek(ctx, planning.WeekConfig{
		Candidates:  candidates,
		People:      fam.People,
		Preferences: fam.Preferences,
		EnergyFor: func(date time.Time) domain.Effort {
			return domain.Effort(fam.KitchenEnergy[weekdayKeys[int(date.Weekday())]])
		},
		SchoolTagsFor: func(date time.Time) []string {
			return schoolTags[date.Format("2006-01-02")]
		},
		Reorder: func(ctx context.Context, ranked []scoring.ScoredCandidate) []scoring.ScoredCandidate {
			if olla == nil {
				return ranked
			}
			if reordered, err := llm.ProposeOrder(olla, ctx, ranked); err == nil && len(reordered) > 0 {
				return reordered
			}
			return ranked
		},
	}, monday, in.Days)

	// ── Shopping requirements ─────────────────────────────────────────────────
	var meals []planning.ChosenMeal
	for _, s := range planned {
		meals = append(meals, ingredientLines[s.Winner.Candidate.MealieRecipeID])
	}
	allReqs := planning.BuildRequirements(meals)
	// Drop pantry staples (salt, pepper, oil, …) — assumed on hand, never bought.
	reqs, _ := planning.PartitionStaples(allReqs)

	// ── Persist the plan to Postgres (task 2.3) ───────────────────────────────
	if store, perr := openStore(ctx); perr != nil {
		// Non-fatal: persistence failure does not abort planning.
	} else if store != nil {
		if err := persistPlan(ctx, store, monday, planned, reqs); err != nil {
			// Non-fatal: persistence failure does not abort planning.
		}
	}

	result := RunPlanResult{
		WeekStart: monday,
		WeekYear:  year,
		WeekNum:   week,
		PlanCount: len(planned),
	}

	if !in.CreateWishlist {
		result.Message = fmt.Sprintf("planned %d dinners for week %d-W%02d (dry-run, no wishlist)", len(planned), year, week)
		return result, nil
	}

	// ── Resolve + wishlist via the adapter ────────────────────────────────────
	rc := retailer.New(adapterURL)
	terms := retailer.SearchTerms{}
	for _, meal := range meals {
		for _, line := range meal.Ingredients {
			terms[line.IngredientID] = line.IngredientID // canonical id doubles as Swedish term
		}
	}
	resolutions, err := rc.ResolveRequirements(ctx, reqs, terms)
	if err != nil {
		return result, fmt.Errorf("resolve: %w", err)
	}

	var items []retailer.ShoppingListItem
	var review []retailer.Resolution
	for _, res := range resolutions {
		if res.NeedsReview || res.RetailerProductID == nil {
			review = append(review, res)
			continue
		}
		items = append(items, retailer.ShoppingListItem{ProductCode: *res.RetailerProductID, Quantity: res.Packages})
	}
	result.ReviewCount = len(review)

	if len(items) == 0 {
		return result, fmt.Errorf("nothing confidently resolved — review the mappings first")
	}
	name := fmt.Sprintf("Vecka %d", week)
	created, err := rc.CreateShoppingList(ctx, name, items)
	if err != nil {
		return result, fmt.Errorf("create wishlist: %w", err)
	}
	result.WishlistName = created.Name
	result.WishlistID = created.WishlistID
	result.ItemCount = len(items)
	result.Message = fmt.Sprintf("planned %d dinners, wishlist %q created with %d items (%d need review)", len(planned), name, len(items), len(review))
	return result, nil
}

// runPlan executes the live weekly pipe from CLI flags.
// Mealie recipes → per-day deterministic scoring (Skolmaten dedup, in-week
// repetition avoidance) → optional Olla explanations → canonical shopping
// requirements → optional willys-adapter resolution + wishlist.
func runPlan(args []string) error {
	fs := flag.NewFlagSet("plan", flag.ExitOnError)
	family := fs.String("family", "family.json", "path to the family config JSON")
	school := fs.String("school", os.Getenv("SKOLMATEN_SCHOOL"), "skolmaten school slug (empty = skip school dedup)")
	days := fs.Int("days", 7, "number of dinners to plan, starting Monday")
	weekStr := fs.String("week", "", "ISO week to plan, e.g. 2026-W31 (default: next week)")
	createWishlist := fs.Bool("create-wishlist", false, "resolve products and create the Willys wishlist (default: dry-run print)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx := context.Background()
	in := RunPlanInput{
		Family:         *family,
		School:         *school,
		Days:           *days,
		Week:           *weekStr,
		CreateWishlist: *createWishlist,
	}
	result, err := RunPlan(ctx, in)
	if err != nil {
		return err
	}

	fmt.Printf("Planning week %d-W%02d (%s) — %d dinners\n", result.WeekYear, result.WeekNum, result.WeekStart.Format("2006-01-02"), result.PlanCount)
	fmt.Println(result.Message)
	return nil
}

// candidatesFromRefs converts Mealie references into scorer candidates and a
// lookup of each recipe's canonical ingredient lines for requirements.
func candidatesFromRefs(refs []mealie.RecipeRef) ([]domain.Candidate, map[string]planning.ChosenMeal) {
	var candidates []domain.Candidate
	lines := map[string]planning.ChosenMeal{}

	for _, ref := range refs {
		c := domain.Candidate{
			MealieRecipeID: ref.MealieRecipeID,
			Title:          ref.Title,
			Tags:           ref.Tags,
			Effort:         ref.Effort,
		}
		meal := planning.ChosenMeal{MealieRecipeID: ref.MealieRecipeID}
		for _, ing := range ref.Ingredients {
			if ing.FoodName == "" {
				continue // unmapped free-text line; ingredient_mapping review picks these up
			}
			// Canonical id: lowercase food name until the mapping table refines it.
			id := domain.CanonicalIngredientID(ing.FoodName)
			c.Ingredients = append(c.Ingredients, id)
			qty := ing.Quantity
			if qty <= 0 {
				qty = 1
			}
			unit := ing.Unit
			if unit == "" {
				unit = "st"
			}
			meal.Ingredients = append(meal.Ingredients, domain.Ingredient{
				IngredientID: id, Quantity: qty, Unit: unit,
			})
		}
		candidates = append(candidates, c)
		lines[ref.MealieRecipeID] = meal
	}
	return candidates, lines
}

func loadFamily(path string) (*familyConfig, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("family config: %w (copy family.example.json to %s)", err, path)
	}
	var fam familyConfig
	if err := json.Unmarshal(buf, &fam); err != nil {
		return nil, fmt.Errorf("family config: %w", err)
	}
	if len(fam.People) == 0 {
		return nil, fmt.Errorf("family config has no people")
	}
	return &fam, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// nextISOWeek returns the ISO year/week of the week after t's.
func nextISOWeek(t time.Time) (int, int) {
	return t.AddDate(0, 0, 7).ISOWeek()
}

// mondayOfISOWeek returns the Monday of the given ISO week.
func mondayOfISOWeek(year, week int) time.Time {
	// Jan 4 is always in ISO week 1.
	t := time.Date(year, 1, 4, 0, 0, 0, 0, time.UTC)
	for t.Weekday() != time.Monday {
		t = t.AddDate(0, 0, -1)
	}
	return t.AddDate(0, 0, (week-1)*7)
}
