// Package mealie is a read-only client for the Mealie REST API. Mealie is the
// recipe source of truth; this package fetches recipes and normalizes them into
// references the Food Brain stores (id + tags + ingredient lines + snapshot),
// never authoritative copies.
package mealie

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/androidand/spisordning/internal/httpclient"
)

// Client talks to a Mealie instance (e.g. the tengil-deployed hlab-mealie).
type Client struct {
	token string
	http  *httpclient.Client
}

// New returns a Client for the Mealie at baseURL using an API token.
func New(baseURL, token string) *Client {
	return &Client{
		token: token,
		http:  httpclient.New(baseURL, "mealie", 60*time.Second),
	}
}

// authHeaders attaches the bearer token every Mealie request needs.
func (c *Client) authHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
}

// IngredientLine is one normalized ingredient row from a Mealie recipe — the
// SOURCE shape, naming the food as Mealie spells it. Downstream consumers map
// it onto the canonical domain.Ingredient (id via domain.CanonicalIngredientID);
// it is deliberately not the canonical type.
type IngredientLine struct {
	FoodID   string
	FoodName string
	Quantity float64
	Unit     string
	Note     string
}

// RecipeRef is the Food Brain's reference to a Mealie recipe: identity, the
// planner-relevant metadata, and the raw payload as a cache snapshot.
type RecipeRef struct {
	MealieRecipeID string
	Slug           string
	Title          string
	Tags           []string
	Effort         domain.Effort
	Ingredients    []IngredientLine
	Raw            json.RawMessage
}

type listResponse struct {
	Page    int `json:"page"`
	PerPage int `json:"perPage"`
	Total   int `json:"total"`
	Items   []struct {
		ID   string `json:"id"`
		Slug string `json:"slug"`
		Name string `json:"name"`
	} `json:"items"`
}

type fullRecipe struct {
	ID        string `json:"id"`
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	TotalTime string `json:"totalTime"`
	Tags      []struct {
		Name string `json:"name"`
	} `json:"tags"`
	RecipeIngredient []struct {
		Quantity float64 `json:"quantity"`
		Note     string  `json:"note"`
		Unit     *struct {
			Name string `json:"name"`
		} `json:"unit"`
		Food *struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"food"`
	} `json:"recipeIngredient"`
}

// SyncRecipes fetches every recipe (paged list, then full detail per recipe)
// and returns normalized references.
func (c *Client) SyncRecipes(ctx context.Context) ([]RecipeRef, error) {
	var refs []RecipeRef
	for page := 1; ; page++ {
		var list listResponse
		if err := c.get(ctx, fmt.Sprintf("/api/recipes?page=%d&perPage=50", page), &list); err != nil {
			return nil, err
		}
		for _, item := range list.Items {
			ref, err := c.fetchRecipe(ctx, item.Slug)
			if err != nil {
				return nil, fmt.Errorf("recipe %s: %w", item.Slug, err)
			}
			refs = append(refs, *ref)
		}
		if len(list.Items) < list.PerPage || len(list.Items) == 0 {
			return refs, nil
		}
	}
}

func (c *Client) fetchRecipe(ctx context.Context, slug string) (*RecipeRef, error) {
	raw, err := c.getRaw(ctx, "/api/recipes/"+slug)
	if err != nil {
		return nil, err
	}
	var r fullRecipe
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, err
	}

	ref := RecipeRef{
		MealieRecipeID: r.ID,
		Slug:           r.Slug,
		Title:          r.Name,
		Effort:         effortFromTotalTime(r.TotalTime),
		Raw:            raw,
	}
	for _, t := range r.Tags {
		if t.Name != "" {
			ref.Tags = append(ref.Tags, strings.ToLower(t.Name))
		}
	}
	for _, line := range r.RecipeIngredient {
		il := IngredientLine{Quantity: line.Quantity, Note: line.Note}
		if line.Food != nil {
			il.FoodID = line.Food.ID
			il.FoodName = line.Food.Name
		}
		if line.Unit != nil {
			il.Unit = line.Unit.Name
		}
		ref.Ingredients = append(ref.Ingredients, il)
	}

	// Mealie's URL import stores ingredients as raw text without structured
	// food/unit/quantity (the common real-world case). Fall back to Mealie's
	// own ingredient parser for any line that arrived unstructured, so
	// downstream shopping-requirement resolution has something to work with.
	c.parseUnstructured(ctx, ref.Ingredients)
	return &ref, nil
}

type parsedIngredient struct {
	Ingredient struct {
		Quantity float64 `json:"quantity"`
		Unit     *struct {
			Name string `json:"name"`
		} `json:"unit"`
		Food *struct {
			Name string `json:"name"`
		} `json:"food"`
	} `json:"ingredient"`
}

// parseUnstructured fills in FoodName/Unit/Quantity for ingredient lines that
// Mealie left as raw notes, using Mealie's brute parser. Best-effort: parser
// failures leave the affected line(s) as-is (skipped downstream, same as
// before).
//
// Mealie's brute parser 500s on at least one real input shape (a note
// containing a comma, e.g. "Ris, 4 portioner" — reproduced directly against
// the live parser endpoint). Batching all of a recipe's notes in one call
// means a single bad note previously discarded every ingredient in that
// recipe, not just the unparseable one. So: try the whole batch first (one
// request, the common case), and only fall back to parsing notes one at a
// time — isolating a bad note to just that line — when the batch call fails.
func (c *Client) parseUnstructured(ctx context.Context, lines []IngredientLine) {
	type target struct{ idx int }
	var notes []string
	var targets []target
	for i := range lines {
		if lines[i].FoodName == "" && strings.TrimSpace(lines[i].Note) != "" {
			notes = append(notes, lines[i].Note)
			targets = append(targets, target{idx: i})
		}
	}
	if len(notes) == 0 {
		return
	}

	if parsed, ok := c.parseNotes(ctx, notes); ok && len(parsed) == len(targets) {
		for n, p := range parsed {
			applyParsed(&lines[targets[n].idx], p)
		}
		return
	}

	// Batch failed (or returned a mismatched count) — isolate the bad note(s)
	// by retrying one at a time instead of losing the whole recipe's ingredients.
	for n, note := range notes {
		parsed, ok := c.parseNotes(ctx, []string{note})
		if !ok || len(parsed) != 1 {
			continue
		}
		applyParsed(&lines[targets[n].idx], parsed[0])
	}
}

func applyParsed(line *IngredientLine, p parsedIngredient) {
	if p.Ingredient.Food != nil && p.Ingredient.Food.Name != "" {
		line.FoodName = p.Ingredient.Food.Name
	}
	if p.Ingredient.Unit != nil {
		line.Unit = p.Ingredient.Unit.Name
	}
	if p.Ingredient.Quantity > 0 {
		line.Quantity = p.Ingredient.Quantity
	}
}

// parseNotes calls Mealie's brute ingredient parser for notes. ok is false on
// any transport/decode failure — callers fall back accordingly.
func (c *Client) parseNotes(ctx context.Context, notes []string) ([]parsedIngredient, bool) {
	raw, err := c.postRaw(ctx, "/api/parser/ingredients", map[string]any{"parser": "brute", "ingredients": notes})
	if err != nil {
		return nil, false
	}
	var parsed []parsedIngredient
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, false
	}
	return parsed, true
}

// effortFromTotalTime maps Mealie's free-text totalTime to an effort class.
// Mealie stores strings like "30 minuter", "1 timme", "45 min"; unparseable or
// missing values default to medium.
func effortFromTotalTime(totalTime string) domain.Effort {
	minutes := parseMinutes(totalTime)
	switch {
	case minutes == 0:
		return domain.EffortMedium
	case minutes <= 25:
		return domain.EffortLow
	case minutes <= 50:
		return domain.EffortMedium
	default:
		return domain.EffortHigh
	}
}

func parseMinutes(s string) int {
	s = strings.ToLower(s)
	total := 0
	fields := strings.Fields(s)
	for i, f := range fields {
		n := 0
		if _, err := fmt.Sscanf(f, "%d", &n); err != nil || n <= 0 {
			continue
		}
		unit := ""
		if i+1 < len(fields) {
			unit = fields[i+1]
		}
		switch {
		case strings.HasPrefix(unit, "tim"), strings.HasPrefix(unit, "hour"), unit == "h":
			total += n * 60
		default: // minutes are the common case ("min", "minuter", bare number)
			total += n
		}
	}
	return total
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	raw, err := c.getRaw(ctx, path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func (c *Client) getRaw(ctx context.Context, path string) (json.RawMessage, error) {
	var raw json.RawMessage
	if err := c.http.GetJSON(ctx, path, &raw, c.authHeaders); err != nil {
		return nil, err
	}
	return raw, nil
}

func (c *Client) postRaw(ctx context.Context, path string, payload any) (json.RawMessage, error) {
	var raw json.RawMessage
	if err := c.http.PostJSON(ctx, path, payload, &raw, c.authHeaders); err != nil {
		return nil, err
	}
	return raw, nil
}
