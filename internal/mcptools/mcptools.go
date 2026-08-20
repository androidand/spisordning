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

// PlannedDinner is one planned dinner for a date, produced by the
// application-layer weekly planner.
type PlannedDinner struct {
	Date   string  `json:"date"`
	Recipe string  `json:"recipe"` // Mealie recipe id
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
}

// GetRequirementsInput is the input for the get_shopping_requirements tool.
type GetRequirementsInput struct {
	// Recipes is the set of Mealie recipe ids to aggregate into requirements.
	Recipes []string `json:"recipes"`
}

// ---- Service interfaces (implemented by the composition root) ----

// PlannerService plans dinners for a date range. The composition root implements
// it by loading the household and recipe candidates from persistence and
// delegating to the application-layer planner.
type PlannerService interface {
	PlanDinners(ctx context.Context, date time.Time, days int) ([]PlannedDinner, error)
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
	Planner      PlannerService
	Reactions    MealReactionService
	Requirements RequirementsService
}

// RegisterTools adds the initial tool set to s. Each tool calls exactly one
// application-layer service and maps its errors to MCP tool-call errors.
func RegisterTools(s *mcp.Server, deps Dependencies) {
	if deps.Planner != nil {
		mcp.AddTool(s, &mcp.Tool{
			Name:        "list_recipe_candidates",
			Description: "Plan the best dinner candidate for a given day (and optionally the following days) using the household's recipes, people, and preferences.",
		}, listCandidatesHandler(deps.Planner))
	}
	if deps.Reactions != nil {
		mcp.AddTool(s, &mcp.Tool{
			Name:        "record_meal_reaction",
			Description: "Record a household member's reaction (sentiment -2..2) to a meal that was served on a given date.",
		}, recordReactionHandler(deps.Reactions))
	}
	if deps.Requirements != nil {
		mcp.AddTool(s, &mcp.Tool{
			Name:        "get_shopping_requirements",
			Description: "Aggregate a set of recipes into canonical, retailer-independent shopping requirements (ingredient + amount per line).",
		}, requirementsHandler(deps.Requirements))
	}
}

// ---- Tool handlers ----

// listCandidatesHandler plans dinners for the requested day(s). On success it
// returns the structured result and lets the SDK populate the text content; on
// error it returns the error so the SDK reports it as an MCP tool-call error.
func listCandidatesHandler(p PlannerService) mcp.ToolHandlerFor[ListCandidatesInput, []PlannedDinner] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListCandidatesInput) (*mcp.CallToolResult, []PlannedDinner, error) {
		date, err := parseDate(in.Date)
		if err != nil {
			return nil, nil, err
		}
		days := in.Days
		if days <= 0 {
			days = 1
		}
		dinners, err := p.PlanDinners(ctx, date, days)
		if err != nil {
			return nil, nil, err
		}
		return nil, dinners, nil
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
