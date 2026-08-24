package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/androidand/spisordning/internal/ambient"
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

// runPlan executes the live weekly pipe:
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
	writeTonight := fs.String("write-tonight", "", "write the week projection for the ambient surface (task 5.2), e.g. tonight.json")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx := context.Background()

	// ── Config ────────────────────────────────────────────────────────────────
	fam, err := loadFamily(*family)
	if err != nil {
		return err
	}
	mealieURL, mealieToken := os.Getenv("MEALIE_BASE_URL"), os.Getenv("MEALIE_API_TOKEN")
	if mealieURL == "" || mealieToken == "" {
		return fmt.Errorf("MEALIE_BASE_URL and MEALIE_API_TOKEN must be set")
	}
	adapterURL := envOr("ADAPTER_URL", "http://localhost:8402")

	year, week := nextISOWeek(time.Now())
	if *weekStr != "" {
		if _, err := fmt.Sscanf(*weekStr, "%d-W%d", &year, &week); err != nil {
			return fmt.Errorf("invalid --week %q (want e.g. 2026-W31)", *weekStr)
		}
	}
	monday := mondayOfISOWeek(year, week)
	fmt.Printf("Planning week %d-W%02d (%s) — %d dinners\n", year, week, monday.Format("2006-01-02"), *days)

	// ── Inputs ────────────────────────────────────────────────────────────────
	fmt.Println("Syncing recipes from Mealie...")
	refs, err := mealie.New(mealieURL, mealieToken).SyncRecipes(ctx)
	if err != nil {
		return fmt.Errorf("mealie sync: %w", err)
	}
	if len(refs) == 0 {
		return fmt.Errorf("no recipes in Mealie — add some first")
	}
	fmt.Printf("  %d recipes synced\n", len(refs))
	candidates, ingredientLines := candidatesFromRefs(refs)

	schoolTags := map[string][]string{} // date -> tags
	if *school != "" {
		sk := skolmaten.New(envOr("SKOLMATEN_BASE_URL", "http://192.168.1.120:8787"), os.Getenv("SKOLMATEN_CLIENT_TOKEN"))
		menu, err := sk.WeekMenu(ctx, *school, year, week)
		if err != nil {
			fmt.Printf("  ⚠️  skolmaten unavailable (%v) — planning without school dedup\n", err)
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
	}, monday, *days)

	// ── Present the plan ──────────────────────────────────────────────────────
	fmt.Println("\nProposed week:")
	byDate := make(map[string]planning.PlannedSlot, len(planned))
	for _, s := range planned {
		byDate[s.Date.Format("2006-01-02")] = s
	}
	projection := ambient.PlanFile{Week: fmt.Sprintf("%d-W%02d", year, week)}
	for i := 0; i < *days; i++ {
		date := monday.AddDate(0, 0, i)
		s, ok := byDate[date.Format("2006-01-02")]
		if !ok {
			fmt.Printf("  %s: no feasible meal (take-away night?)\n", date.Format("Mon 2006-01-02"))
			continue
		}
		reason := s.Winner.Reason
		if olla != nil {
			if expl, err := llm.Explain(olla, ctx, s.Winner); err == nil && expl != "" {
				reason = strings.TrimSpace(expl)
			}
		}
		fmt.Printf("  %s  %-30s %s\n", date.Format("Mon 02/01"), s.Winner.Candidate.Title, reason)
		projection.Slots = append(projection.Slots, ambient.Slot{
			Date:   date.Format("2006-01-02"),
			Title:  s.Winner.Candidate.Title,
			Reason: reason,
			Tags:   s.Winner.Candidate.Tags,
		})
	}

	// ── Ambient projection (task 5.2) ─────────────────────────────────────────
	// Written before the dry-run return so a plain `plan` run still feeds the
	// Home Assistant surface.
	if *writeTonight != "" {
		buf, err := json.MarshalIndent(projection, "", "  ")
		if err != nil {
			return fmt.Errorf("write tonight: %w", err)
		}
		if err := os.WriteFile(*writeTonight, append(buf, '\n'), 0o644); err != nil {
			return fmt.Errorf("write tonight: %w", err)
		}
		fmt.Printf("\nWrote ambient projection to %s (%d dinners)\n", *writeTonight, len(projection.Slots))
	}

	// ── Shopping requirements ─────────────────────────────────────────────────
	var meals []planning.ChosenMeal
	for _, s := range planned {
		meals = append(meals, ingredientLines[s.Winner.Candidate.MealieRecipeID])
	}
	allReqs := planning.BuildRequirements(meals)
	// Drop pantry staples (salt, pepper, oil, …) — assumed on hand, never bought.
	reqs, staples := planning.PartitionStaples(allReqs)

	// ── Persist the plan to Postgres (task 2.3) ─────────────────────────────────
	// No-op when no database is configured (openStore returns nil); the CLI
	// behavior is otherwise unchanged.
	if store, perr := openStore(ctx); perr != nil {
		fmt.Fprintf(os.Stderr, "⚠ persistence unavailable; plan not persisted: %v\n", perr)
	} else if store != nil {
		if err := persistPlan(ctx, store, monday, planned, reqs); err != nil {
			fmt.Fprintf(os.Stderr, "⚠ could not persist plan to Postgres: %v\n", err)
		} else {
			fmt.Printf("✅ persisted plan for week %d-W%02d to Postgres (meal_plan + candidates + decisions + shopping_requirements)\n", year, week)
		}
	}

	fmt.Printf("\nShopping requirements (%d; %d assumed-staple item(s) skipped):\n", len(reqs), len(staples))
	for _, r := range reqs {
		fmt.Printf("  - %-24s %g %s\n", r.IngredientID, r.Quantity, r.Unit)
	}
	if len(staples) > 0 {
		dropped := make([]string, 0, len(staples))
		for _, s := range staples {
			dropped = append(dropped, s.IngredientID)
		}
		fmt.Printf("  (skipped staples: %s)\n", strings.Join(dropped, ", "))
	}

	if !*createWishlist {
		fmt.Println("\nDry-run: no products resolved, no wishlist created. Re-run with --create-wishlist to apply.")
		return nil
	}

	// ── Resolve + wishlist via the adapter ────────────────────────────────────
	rc := retailer.New(adapterURL)
	terms := retailer.SearchTerms{}
	for _, meal := range meals {
		for _, line := range meal.Ingredients {
			terms[line.IngredientID] = line.IngredientID // canonical id doubles as Swedish term
		}
	}
	fmt.Println("\nResolving products via willys-adapter...")
	resolutions, err := rc.ResolveRequirements(ctx, reqs, terms)
	if err != nil {
		return fmt.Errorf("resolve: %w", err)
	}

	var items []retailer.ShoppingListItem
	var review []retailer.Resolution
	for _, res := range resolutions {
		if res.NeedsReview || res.RetailerProductID == nil {
			review = append(review, res)
			continue
		}
		items = append(items, retailer.ShoppingListItem{ProductCode: *res.RetailerProductID, Quantity: res.Packages})
		fmt.Printf("  ✓ %-24s → %s (%d pkg, conf %.2f)\n", res.IngredientID, res.ProductName, res.Packages, res.Confidence)
	}
	for _, res := range review {
		fmt.Printf("  ⚠ %-24s needs review (conf %.2f)\n", res.IngredientID, res.Confidence)
	}

	if len(items) == 0 {
		return fmt.Errorf("nothing confidently resolved — review the mappings first")
	}
	name := fmt.Sprintf("Vecka %d", week)
	created, err := rc.CreateShoppingList(ctx, name, items)
	if err != nil {
		return fmt.Errorf("create wishlist: %w", err)
	}
	fmt.Printf("\n✅ Wishlist %q created (id %s) with %d items — review it in the Willys app.\n", created.Name, created.WishlistID, len(items))
	if len(review) > 0 {
		fmt.Printf("   %d item(s) need manual review and were NOT added.\n", len(review))
	}
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
