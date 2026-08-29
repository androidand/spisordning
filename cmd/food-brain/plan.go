package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/androidand/spisordning/internal/ambient"
	"github.com/androidand/spisordning/internal/domain"
	"github.com/androidand/spisordning/internal/llm"
	"github.com/androidand/spisordning/internal/mealie"
	"github.com/androidand/spisordning/internal/planning"
	"github.com/androidand/spisordning/internal/retailer"
	"github.com/androidand/spisordning/internal/scoring"
	"github.com/androidand/spisordning/internal/service"
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

// RunPlanInput carries the parameters for a plan run from the HTTP API or CLI.
type RunPlanInput struct {
	Week           string
	Days           int
	CreateWishlist bool
	Family         string
	School         string
	// Retailer selects the shopping-list backend: "willys" (default) or "ica".
	// Empty defaults to "willys". Not yet exposed on the HTTP API.
	Retailer string
	// WriteTonight is a path to write the ambient week projection (task 5.2),
	// e.g. tonight.json. Empty skips the write. Not yet exposed on the HTTP API.
	WriteTonight string
	// Progress, when non-nil, is called with (phase, message) as the run
	// progresses. The SSE endpoint (POST /plans/run/stream) uses this to stream
	// progress events. Nil for the CLI and the synchronous POST /plans/run.
	Progress func(phase, message string)
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
	if in.Retailer == "" {
		in.Retailer = "willys"
	}

	// emit reports a progress phase to the optional Progress callback (SSE).
	// No-op when Progress is nil (CLI and synchronous POST /plans/run).
	// Note: "started" and "done" are emitted by the SSE handler (progress.go),
	// not here — the handler's "done" carries the full result.
	emit := func(phase, message string) {
		if in.Progress != nil {
			in.Progress(phase, message)
		}
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
	icaAdapterURL := envOr("ICA_ADAPTER_URL", "http://localhost:8403")
	hemkopAdapterURL := envOr("HEMKOP_ADAPTER_URL", "http://localhost:8404")
	kind := retailer.RetailerKind(in.Retailer)

	year, week := nextISOWeek(time.Now())
	if in.Week != "" {
		if _, err := fmt.Sscanf(in.Week, "%d-W%d", &year, &week); err != nil {
			return RunPlanResult{}, fmt.Errorf("invalid week %q (want e.g. 2026-W31)", in.Week)
		}
	}
	monday := mondayOfISOWeek(year, week)

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

	// ── Orchestrate via the planning service (tasks 7.2/7.4) ──────────────────
	// The service owns mealie sync, candidate conversion, scoring, explanations,
	// shopping requirements, and persistence. The composition root keeps only the
	// wiring (store + mealie client) and the CLI-specific display/wishlist path.
	store, err := openStore(ctx)
	if err != nil {
		return RunPlanResult{}, fmt.Errorf("open store: %w", err)
	}
	var db service.Store
	if store != nil {
		db = store
	}
	planningSvc := service.NewPlanning(db, mealie.New(mealieURL, mealieToken))

	emit("planning", "Planning week (syncing recipes, scoring candidates)")
	pw, err := planningSvc.PlanWeek(ctx, service.PlanWeekInput{
		WeekStart:   monday,
		Days:        in.Days,
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
		Olla: olla,
	})
	if err != nil {
		return RunPlanResult{}, err
	}

	// Aliases so the display and wishlist sections below read unchanged.
	planned := pw.Planned
	explanations := pw.Explanations
	meals := pw.Meals
	reqs := pw.Reqs

	// ── Present the plan ──────────────────────────────────────────────────────
	fmt.Println("\nProposed week:")
	byDate := make(map[string]planning.PlannedSlot, len(planned))
	for _, s := range planned {
		byDate[s.Date.Format("2006-01-02")] = s
	}
	projection := ambient.PlanFile{Week: fmt.Sprintf("%d-W%02d", year, week)}
	for i := 0; i < in.Days; i++ {
		date := monday.AddDate(0, 0, i)
		s, ok := byDate[date.Format("2006-01-02")]
		if !ok {
			fmt.Printf("  %s: no feasible meal (take-away night?)\n", date.Format("Mon 2006-01-02"))
			continue
		}
		reason := s.Winner.Reason
		if expl, ok := explanations[s.Winner.Candidate.MealieRecipeID]; ok {
			reason = expl
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
	if in.WriteTonight != "" {
		buf, err := json.MarshalIndent(projection, "", "  ")
		if err != nil {
			return RunPlanResult{}, fmt.Errorf("write tonight: %w", err)
		}
		if err := os.WriteFile(in.WriteTonight, append(buf, '\n'), 0o644); err != nil {
			return RunPlanResult{}, fmt.Errorf("write tonight: %w", err)
		}
		fmt.Printf("\nWrote ambient projection to %s (%d dinners)\n", in.WriteTonight, len(projection.Slots))
	}

	// ── Persist result (task 2.3) ─────────────────────────────────────────────
	// The service already persisted the plan (or skipped it when no database is
	// configured). Persistence is best-effort: a failure warns but does not fail
	// the run. The store is reused below by the catalog path (task 3.3).
	if pw.PersistError != nil {
		fmt.Fprintf(os.Stderr, "⚠ could not persist plan to Postgres: %v\n", pw.PersistError)
	} else if pw.Persisted {
		fmt.Printf("✅ persisted plan for week %d-W%02d to Postgres (meal_plan + candidates + decisions + shopping_requirements)\n", year, week)
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
	rc, err := retailer.NewFromKind(kind, adapterURL, icaAdapterURL, hemkopAdapterURL)
	if err != nil {
		return result, fmt.Errorf("retailer: %w", err)
	}
	rc.WithAuthFile(envOr("ICA_AUTH_FILE", ""))
	terms := retailer.SearchTerms{}
	for _, meal := range meals {
		for _, line := range meal.Ingredients {
			terms[line.IngredientID] = line.IngredientID // canonical id doubles as Swedish term
		}
	}
	fmt.Printf("\nResolving products via %s-adapter...\n", kind)
	emit("resolving", fmt.Sprintf("Resolving %d product(s) via %s-adapter", len(reqs), kind))
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

	// ── Wire resolved EANs into the catalog (task 3.3) ─────────────────────────
	if store != nil {
		for _, ean := range retailer.ExtractRetailerEANs(resolutions) {
			// Normalize to GTIN-14 and upsert the product_identifier link.
			gtin, nerr := domain.NormalizeGTIN(ean)
			if nerr != nil {
				fmt.Fprintf(os.Stderr, "  ⚠ bad EAN %q: %v (skipped)\n", ean, nerr)
				continue
			}
			if err := store.UpsertProductIdentifier(ctx, ean, gtin); err != nil {
				fmt.Fprintf(os.Stderr, "  ⚠ upsert product_identifier for %s: %v\n", gtin, err)
				continue
			}
			fmt.Printf("  📦 catalog: %s → %s\n", ean, gtin)
		}
		// ICA-specific: try barcode lookup for resolutions that got no product.
		if kind == retailer.RetailerICA {
			for _, res := range resolutions {
				if res.RetailerProductID != nil {
					continue // already resolved
				}
				// ICA adapter may return the product name as the "retailerProductId"
				// when no barcode is available; skip those.
				if res.ProductName == "" {
					continue
				}
				// For ICA, the /resolve endpoint may not return a barcode.
				// Skip barcode lookup here — it requires user-initiated scanning.
				_ = res
			}
		}
	}

	if len(items) == 0 {
		return result, fmt.Errorf("nothing confidently resolved — review the mappings first")
	}

	// Print what resolved before attempting the wishlist, so a wishlist-creation
	// failure (a real, observed Willys-side outage) doesn't discard already-done
	// resolution work — the caller can still act on these products manually.
	fmt.Printf("\nResolved %d item(s):\n", len(items))
	for _, res := range resolutions {
		if res.NeedsReview || res.RetailerProductID == nil {
			continue
		}
		fmt.Printf("  %s  %s  x%d\n", *res.RetailerProductID, res.ProductName, res.Packages)
	}
	if len(review) > 0 {
		fmt.Printf("%d item(s) need manual review:\n", len(review))
		for _, res := range review {
			fmt.Printf("  ⚠ %s (confidence %.2f)\n", res.IngredientID, res.Confidence)
		}
	}

	name := fmt.Sprintf("Vecka %d", week)
	emit("wishlist", fmt.Sprintf("Creating wishlist %q with %d item(s)", name, len(items)))
	created, err := rc.CreateShoppingList(ctx, name, items)
	if err != nil {
		return result, fmt.Errorf("create wishlist: %w", err)
	}
	fmt.Printf("\n✅ Wishlist %q created (id %s) with %d items — review it in the %s app.\n", created.Name, created.WishlistID, len(items), kind)
	if len(review) > 0 {
		fmt.Printf("   %d item(s) need manual review and were NOT added.\n", len(review))
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
// requirements → optional retailer-adapter resolution + wishlist.
func runPlan(args []string) error {
	fs := flag.NewFlagSet("plan", flag.ExitOnError)
	family := fs.String("family", "family.json", "path to the family config JSON")
	school := fs.String("school", os.Getenv("SKOLMATEN_SCHOOL"), "skolmaten school slug (empty = skip school dedup)")
	days := fs.Int("days", 7, "number of dinners to plan, starting Monday")
	weekStr := fs.String("week", "", "ISO week to plan, e.g. 2026-W31 (default: next week)")
	createWishlist := fs.Bool("create-wishlist", false, "resolve products and create the wishlist (default: dry-run print)")
	re := fs.String("retailer", "willys", "retailer backend: willys (default) or ica")
	writeTonight := fs.String("write-tonight", "", "write the week projection for the ambient surface (task 5.2), e.g. tonight.json")
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
		Retailer:       *re,
		WriteTonight:   *writeTonight,
	}
	result, err := RunPlan(ctx, in)
	if err != nil {
		return err
	}

	fmt.Printf("Planning week %d-W%02d (%s) — %d dinners\n", result.WeekYear, result.WeekNum, result.WeekStart.Format("2006-01-02"), result.PlanCount)
	fmt.Println(result.Message)
	return nil
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
