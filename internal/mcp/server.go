// Package mcp implements the Model Context Protocol server for spisordning.
//
// The server exposes tools that call application-layer service functions.
// It never imports internal/persistence or any SQL driver directly.
//
// Two transports are supported:
//   - Streamable HTTP (single POST endpoint) for remote clients
//   - stdio for local/subprocess use
package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/androidand/spisordning/internal/httpapi"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewServer returns an MCP server with the given service dependencies.
// Pass nil for any service that is not available; that tool will return an
// "unavailable" error instead.
func NewServer(recipes httpapi.RecipesService, meals httpapi.MealsService, planning httpapi.PlanningService, pantry httpapi.PantryService) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "spisordning", Version: "0.1.0"}, nil)

	if recipes != nil {
		srv.AddTool(&mcp.Tool{
			Name:        "list_recipes",
			Description: "List known recipe refs from Mealie",
			InputSchema: `{"type": "object", "properties": {}}`,
		}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			refs, err := recipes.ListRecipes(ctx)
			if err != nil {
				return errorResult(fmt.Sprintf("error: %v", err)), nil
			}
			data, _ := json.MarshalIndent(refs, "", "  ")
			return successResult(string(data)), nil
		})
	}

	if meals != nil {
		srv.AddTool(&mcp.Tool{
			Name:        "record_meal_reaction",
			Description: "Record that a recipe was served and capture one or more people's reactions",
			InputSchema: mealReactionSchema,
		}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var input recordReactionInput
			if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
				return errorResult("invalid JSON: " + err.Error()), nil
			}
			if input.MealieRecipeID == "" || input.ServedOn == "" || len(input.Reactions) == 0 {
				return errorResult("mealie_recipe_id, served_on, and reactions are required"), nil
			}
			for _, rx := range input.Reactions {
				if rx.PersonID == "" || rx.Sentiment < -2 || rx.Sentiment > 2 {
					return errorResult("each reaction needs a non-empty person_id and sentiment in [-2,2]"), nil
				}
			}
			resp, err := meals.CreateMealEvent(ctx, httpapi.MealEventNew{
				MealieRecipeID: input.MealieRecipeID,
				ServedOn:       input.ServedOn,
				Reactions:      toReactionInputs(input.Reactions),
			})
			if err != nil {
				return errorResult(fmt.Sprintf("error: %v", err)), nil
			}
			data, _ := json.MarshalIndent(resp, "", "  ")
			return successResult(string(data)), nil
		})
	}

	if planning != nil {
		srv.AddTool(&mcp.Tool{
			Name:        "get_shopping_requirements",
			Description: "Get the shopping requirements for a meal plan",
			InputSchema: shoppingReqSchema,
		}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var input getShoppingInput
			if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
				return errorResult("invalid JSON: " + err.Error()), nil
			}
			if input.PlanID <= 0 {
				return errorResult("plan_id is required and must be > 0"), nil
			}
			reqs, err := planning.ListShoppingRequirements(ctx, input.PlanID)
			if err != nil {
				return errorResult(fmt.Sprintf("error: %v", err)), nil
			}
			data, _ := json.MarshalIndent(reqs, "", "  ")
			return successResult(string(data)), nil
		})
	}

	if pantry != nil {
		srv.AddTool(&mcp.Tool{
			Name:        "list_pantry_locations",
			Description: "List inventory locations in the pantry",
			InputSchema: `{"type": "object", "properties": {}}`,
		}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var input listLocationsInput
			_ = json.Unmarshal(req.Params.Arguments, &input)
			locations, err := pantry.ListLocations(ctx, input.HouseholdID)
			if err != nil {
				return errorResult(fmt.Sprintf("error: %v", err)), nil
			}
			data, _ := json.MarshalIndent(locations, "", "  ")
			return successResult(string(data)), nil
		})
	}

	return srv
}

const mealReactionSchema = `{
  "type": "object",
  "required": ["mealie_recipe_id", "served_on", "reactions"],
  "properties": {
    "mealie_recipe_id": {"type": "string", "description": "Mealie recipe ID"},
    "served_on": {"type": "string", "description": "Date served (YYYY-MM-DD)"},
    "reactions": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["person_id", "sentiment"],
        "properties": {
          "person_id": {"type": "string"},
          "sentiment": {"type": "integer", "minimum": -2, "maximum": 2}
        }
      }
    }
  }
}`

const shoppingReqSchema = `{
  "type": "object",
  "required": ["plan_id"],
  "properties": {
    "plan_id": {"type": "integer", "description": "Plan ID"}
  }
}`

type recordReactionInput struct {
	MealieRecipeID string              `json:"mealie_recipe_id"`
	ServedOn       string              `json:"served_on"`
	Reactions      []reactionInputItem `json:"reactions"`
}

type reactionInputItem struct {
	PersonID  string `json:"person_id"`
	Sentiment int    `json:"sentiment"`
}

type getShoppingInput struct {
	PlanID int64 `json:"plan_id"`
}

type listLocationsInput struct {
	HouseholdID string `json:"household_id,omitempty"`
}

func toReactionInputs(items []reactionInputItem) []httpapi.MealReactionInput {
	out := make([]httpapi.MealReactionInput, 0, len(items))
	for _, item := range items {
		out = append(out, httpapi.MealReactionInput{PersonID: item.PersonID, Sentiment: item.Sentiment})
	}
	return out
}

func errorResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: msg}}}
}

func successResult(data string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: data}}}
}
