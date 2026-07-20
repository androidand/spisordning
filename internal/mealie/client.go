// Package mealie is a read-only client for the Mealie REST API. Mealie is the
// recipe source of truth; this package fetches recipes and normalizes them into
// references the Food Brain stores (id + tags + ingredient lines + snapshot),
// never authoritative copies.
package mealie

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/androidand/spisordning/internal/domain"
)

// Client talks to a Mealie instance (e.g. the tengil-deployed hlab-mealie).
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// New returns a Client for the Mealie at baseURL using an API token.
func New(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: 60 * time.Second},
	}
}

// IngredientLine is one normalized ingredient row from a Mealie recipe.
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

// parseUnstructured fills in FoodName/Unit/Quantity for ingredient lines that
// Mealie left as raw notes, using Mealie's brute parser. Best-effort: parser
// failures leave the lines as-is (they get skipped downstream, same as before).
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

	body, err := json.Marshal(map[string]any{"parser": "brute", "ingredients": notes})
	if err != nil {
		return
	}
	raw, err := c.postRaw(ctx, "/api/parser/ingredients", body)
	if err != nil {
		return // parser unavailable; leave lines unstructured
	}

	var parsed []struct {
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
	if err := json.Unmarshal(raw, &parsed); err != nil || len(parsed) != len(targets) {
		return
	}
	for n, p := range parsed {
		i := targets[n].idx
		if p.Ingredient.Food != nil && p.Ingredient.Food.Name != "" {
			lines[i].FoodName = p.Ingredient.Food.Name
		}
		if p.Ingredient.Unit != nil {
			lines[i].Unit = p.Ingredient.Unit.Name
		}
		if p.Ingredient.Quantity > 0 {
			lines[i].Quantity = p.Ingredient.Quantity
		}
	}
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mealie: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mealie: HTTP %d for %s", res.StatusCode, path)
	}

	var buf json.RawMessage
	if err := json.NewDecoder(res.Body).Decode(&buf); err != nil {
		return nil, fmt.Errorf("mealie: decode %s: %w", path, err)
	}
	return buf, nil
}

func (c *Client) postRaw(ctx context.Context, path string, body []byte) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mealie: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mealie: HTTP %d for %s", res.StatusCode, path)
	}

	var buf json.RawMessage
	if err := json.NewDecoder(res.Body).Decode(&buf); err != nil {
		return nil, fmt.Errorf("mealie: decode %s: %w", path, err)
	}
	return buf, nil
}
