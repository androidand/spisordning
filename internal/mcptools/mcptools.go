// Package mcptools exposes Spisordning's application layer as MCP (Model Context
// Protocol) tools. It is a thin adapter: it owns the tool input/output schemas,
// the service interfaces the tools call, and the mapping of application-layer
// errors to MCP tool-call errors. It imports the MCP SDK and nothing from the
// persistence layer — the composition root (cmd/mcp-server) implements the
// service interfaces against internal/persistence and internal/planning. That
// keeps "AI never touches raw SQL" structurally true, and internal/architecturetest
// enforces that this package never imports persistence, clients, httpapi, or cmd.
package mcptools

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Name is the server's implementation name reported during MCP initialization.
const Name = "spisordning-mcp"

// Version identifies this MCP server implementation.
const Version = "0.1.0"

// ---- Tool output (structured content) ----

// PlannedSlot is one planned meal for a date and slot kind, produced by the
// application-layer weekly planner.
type PlannedSlot struct {
	Date   string  `json:"date"`
	Slot   string  `json:"slot"`   // "dinner" | "breakfast" | "snack"
	Recipe string  `json:"recipe"` // Mealie recipe id (empty for fallback snacks)
	Title  string  `json:"title"`
	Score  float64 `json:"score"`
}

// RecordReactionResult confirms a meal reaction recorded by the application layer.
type RecordReactionResult struct {
	MealEventID int64  `json:"meal_event_id"`
	Recipe      string `json:"recipe"`
	ServedOn    string `json:"served_on"`
	PersonID    string `json:"person_id"`
	Sentiment   int    `json:"sentiment"`
}

// ShoppingRequirement is one canonical, retailer-independent shopping line.
type ShoppingRequirement struct {
	Ingredient      string   `json:"ingredient"`
	Quantity        float64  `json:"quantity"`
	Unit            string   `json:"unit"`
	AcceptableForms []string `json:"acceptable_forms,omitempty"`
	PreferredForm   string   `json:"preferred_form,omitempty"`
}

// ---- Tool inputs ----

// ListCandidatesInput is the input for the list_recipe_candidates tool.
type ListCandidatesInput struct {
	// Date is the day to plan, YYYY-MM-DD. Empty means today.
	Date string `json:"date,omitempty"`
	// Days is how many consecutive days to plan starting at Date. <=0 means 1.
	Days int `json:"days,omitempty"`
	// Slots is the set of slot kinds to plan. Empty means dinner only
	// (backward compatible). Valid values: "dinner", "breakfast", "snack".
	Slots []string `json:"slots,omitempty"`
}

// RecordReactionInput is the input for the record_meal_reaction tool.
type RecordReactionInput struct {
	// Recipe is the Mealie recipe id that was served.
	Recipe string `json:"recipe"`
	// ServedOn is the date the meal was served, YYYY-MM-DD.
	ServedOn string `json:"served_on"`
	// PersonID is the household member who reacted.
	PersonID string `json:"person_id"`
	// Sentiment is -2 (hates) .. 2 (loves).
	Sentiment int `json:"sentiment"`
	// Slot is the slot kind the meal belongs to. Defaults to "dinner" when omitted.
	Slot string `json:"slot,omitempty"`
}

// GetRequirementsInput is the input for the get_shopping_requirements tool.
type GetRequirementsInput struct {
	// Recipes is the set of Mealie recipe ids to aggregate into requirements.
	Recipes []string `json:"recipes"`
}

// ---- Service interfaces (implemented by the composition root) ----

// PlannerService plans meals for a date range. The composition root implements
// it by loading the household and recipe candidates from persistence and
// delegating to the application-layer planner.
type PlannerService interface {
	// PlanDinners plans dinners (backward compatible, dinner-only).
	PlanDinners(ctx context.Context, date time.Time, days int) ([]PlannedSlot, error)
	// PlanSlots plans the requested slot kinds for a date range.
	PlanSlots(ctx context.Context, date time.Time, days int, slots []string) ([]PlannedSlot, error)
}

// MealReactionService records a household member's reaction to a served meal.
type MealReactionService interface {
	RecordReaction(ctx context.Context, in RecordReactionInput) (RecordReactionResult, error)
}

// RequirementsService aggregates a set of recipes into shopping requirements.
type RequirementsService interface {
	ShoppingRequirements(ctx context.Context, recipeIDs []string) ([]ShoppingRequirement, error)
}

// Dependencies carries the application-layer services the tools call. A nil
// field means that tool is not registered.
type Dependencies struct {
	Planner           PlannerService
	Reactions         MealReactionService
	Requirements      RequirementsService
	ShoppingList      ShoppingListService
	Compare           PriceComparisonService
	Wishlist          WishlistService
	RecipeStructuring RecipeStructuringService
}

// RegisterTools adds the initial tool set to s. Each tool calls exactly one
// application-layer service and maps its errors to MCP tool-call errors.
func RegisterTools(s *mcp.Server, deps Dependencies) {
	if deps.Planner != nil {
		mcp.AddTool(s, &mcp.Tool{
			Name:        "list_recipe_candidates",
			Description: "Plan the best meal candidate(s) for a given day (and optionally the following days) using the household's recipes, people, and preferences. By default plans dinner only; pass slots=[\"dinner\",\"breakfast\",\"snack\"] to plan multiple slot kinds.",
		}, listCandidatesHandler(deps.Planner))
	}
	if deps.Reactions != nil {
		mcp.AddTool(s, &mcp.Tool{
			Name:        "record_meal_reaction",
			Description: "Record a household member's reaction (sentiment -2..2) to a meal that was served on a given date. Optionally specify the slot kind (dinner, breakfast, snack); defaults to dinner.",
		}, recordReactionHandler(deps.Reactions))
	}
	if deps.Requirements != nil {
		mcp.AddTool(s, &mcp.Tool{
			Name:        "get_shopping_requirements",
			Description: "Return the canonical shopping requirements (ingredient + amount per line) for the given recipe ids, so they can be sent to a retailer adapter for product resolution.",
		}, requirementsHandler(deps.Requirements))
	}
	if deps.ShoppingList != nil {
		mcp.AddTool(s, &mcp.Tool{
			Name:        "create_shopping_list",
			Description: "Create a new spisordning shopping list from a set of canonical shopping requirements (ingredient + amount per line). Returns the new list id.",
		}, createShoppingListHandler(deps.ShoppingList))
	}
	if deps.Compare != nil {
		mcp.AddTool(s, &mcp.Tool{
			Name:        "compare_shopping_prices",
			Description: "Compare the price of a set of shopping requirements across retailers (Willys and ICA). Returns per-item each retailer's product + price, the cheapest, and per-retailer availability. A stale or unavailable retailer degrades to available:false instead of failing the call.",
		}, comparePricesHandler(deps.Compare))
	}
	if deps.Wishlist != nil {
		mcp.AddTool(s, &mcp.Tool{
			Name:        "push_shopping_wishlist",
			Description: "Push a chosen set of resolved products to a named retailer's (willys or ica) wishlist. Stops at the wishlist — it never fills a cart, checks out, or books a delivery slot. Optionally binds the wishlist to an existing spisordning shopping list.",
		}, pushWishlistHandler(deps.Wishlist))
	}
	if deps.RecipeStructuring != nil {
		mcp.AddTool(s, &mcp.Tool{
			Name:        "structure_recipe",
			Description: "Turn a freeform pasted recipe (a title line, a loose ingredient list, and an optional \"Gör så här\"/\"Instruktioner\"-style steps block) into a real, structured Mealie recipe. Handles real home-cook shorthand: inconsistent quantities, brand asides, bare ingredients with no amount. Returns the created recipe plus which ingredient lines it wasn't confident about, so the caller can flag them rather than presenting a guess as fact.",
		}, structureRecipeHandler(deps.RecipeStructuring))
	}
}

// ---- Tool handlers ----

// listCandidatesHandler plans meals for the requested day(s). On success it
// returns the structured result and lets the SDK populate the text content; on
// error it returns the error so the SDK reports it as an MCP tool-call error.
func listCandidatesHandler(p PlannerService) mcp.ToolHandlerFor[ListCandidatesInput, []PlannedSlot] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListCandidatesInput) (*mcp.CallToolResult, []PlannedSlot, error) {
		date, err := parseDate(in.Date)
		if err != nil {
			return nil, nil, err
		}
		days := in.Days
		if days <= 0 {
			days = 1
		}
		var slots []PlannedSlot
		if len(in.Slots) > 0 {
			slots, err = p.PlanSlots(ctx, date, days, in.Slots)
		} else {
			slots, err = p.PlanDinners(ctx, date, days)
		}
		if err != nil {
			return nil, nil, err
		}
		return nil, slots, nil
	}
}

func recordReactionHandler(r MealReactionService) mcp.ToolHandlerFor[RecordReactionInput, RecordReactionResult] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in RecordReactionInput) (*mcp.CallToolResult, RecordReactionResult, error) {
		if in.Recipe == "" {
			return nil, RecordReactionResult{}, fmt.Errorf("record_meal_reaction: recipe is required")
		}
		if in.PersonID == "" {
			return nil, RecordReactionResult{}, fmt.Errorf("record_meal_reaction: person_id is required")
		}
		if _, err := parseDate(in.ServedOn); err != nil {
			return nil, RecordReactionResult{}, fmt.Errorf("record_meal_reaction: %w", err)
		}
		if in.Sentiment < -2 || in.Sentiment > 2 {
			return nil, RecordReactionResult{}, fmt.Errorf("record_meal_reaction: sentiment must be in [-2, 2], got %d", in.Sentiment)
		}
		// Default slot to "dinner" when omitted.
		if in.Slot == "" {
			in.Slot = "dinner"
		}
		res, err := r.RecordReaction(ctx, in)
		if err != nil {
			return nil, RecordReactionResult{}, err
		}
		return nil, res, nil
	}
}

func requirementsHandler(r RequirementsService) mcp.ToolHandlerFor[GetRequirementsInput, []ShoppingRequirement] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in GetRequirementsInput) (*mcp.CallToolResult, []ShoppingRequirement, error) {
		if len(in.Recipes) == 0 {
			return nil, nil, fmt.Errorf("get_shopping_requirements: at least one recipe is required")
		}
		reqs, err := r.ShoppingRequirements(ctx, in.Recipes)
		if err != nil {
			return nil, nil, err
		}
		return nil, reqs, nil
	}
}

// parseDate parses a YYYY-MM-DD date, defaulting to today when empty.
func parseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Now(), nil
	}
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q: want YYYY-MM-DD", s)
	}
	return d, nil
}
