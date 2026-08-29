// Package mealie is mostly a read-only client for the Mealie REST API — Mealie
// is the recipe source of truth, and this package fetches recipes and
// normalizes them into references the Food Brain stores (id + tags +
// ingredient lines + snapshot), never authoritative copies. The one deliberate
// exception is CreateRecipe/SetIngredients/SetInstructions, added for
// implement-recipe-structuring-from-text: turning a household member's
// freeform pasted recipe into a real Mealie recipe requires writing one. It
// stays narrowly scoped to recipe creation, not general Mealie mutation.
package mealie

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

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
	// Confidence is Mealie brute parser's own average confidence score for
	// this line (0..1), populated only for lines that went through
	// parseUnstructured (i.e. arrived with no FoodName). Zero for lines that
	// were already structured on read, or that the parser failed to handle
	// at all (batch and per-note retry both failed).
	Confidence float64
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
	Confidence struct {
		Average float64 `json:"average"`
	} `json:"confidence"`
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
	line.Confidence = p.Confidence.Average
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

func (c *Client) patchRaw(ctx context.Context, path string, payload any) (json.RawMessage, error) {
	var raw json.RawMessage
	if err := c.http.PatchJSON(ctx, path, payload, &raw, c.authHeaders); err != nil {
		return nil, err
	}
	return raw, nil
}

// CreateRecipe creates a new, empty Mealie recipe by name and returns its
// slug. POST /api/recipes' response body is a bare JSON string (the slug),
// not an object — verified against the live Mealie instance.
func (c *Client) CreateRecipe(ctx context.Context, name string) (string, error) {
	raw, err := c.postRaw(ctx, "/api/recipes", map[string]any{"name": name})
	if err != nil {
		return "", fmt.Errorf("mealie: create recipe %q: %w", name, err)
	}
	var slug string
	if err := json.Unmarshal(raw, &slug); err != nil {
		return "", fmt.Errorf("mealie: create recipe %q: decode slug: %w", name, err)
	}
	return slug, nil
}

// recipeIngredientPatch is the outgoing shape for one recipeIngredient entry.
// Every field is always present (never a partially-populated object) because
// PATCHing recipeIngredient with referenceId omitted/null permanently
// corrupts the recipe server-side: reference_id ends up NULL, and the whole
// recipe becomes unreadable via GET forever after. Found and fixed manually
// earlier in the session that produced this client's write path; see
// openspec/changes/implement-recipe-structuring-from-text/proposal.md.
type recipeIngredientPatch struct {
	ReferenceID      string      `json:"referenceId"`
	Note             string      `json:"note"`
	Display          string      `json:"display"`
	Quantity         float64     `json:"quantity"`
	Unit             *namedThing `json:"unit"`
	Food             *namedThing `json:"food"`
	Title            *string     `json:"title"`
	OriginalText     *string     `json:"originalText"`
	ReferencedRecipe *string     `json:"referencedRecipe"`
}

type namedThing struct {
	Name string `json:"name"`
}

// SetIngredients replaces slug's recipeIngredient list. Each line is resolved
// via Mealie's own brute parser (parseUnstructured's parseNotes, same
// per-note-retry path used on the read side) before writing, so a recipe
// created from freeform text gets structured food/unit/quantity wherever the
// parser can manage it, and a clean unstructured note otherwise — never a
// partial object, which Mealie rejects with a 500 for food (ValueError:
// Expected 'id' to be provided for food).
func (c *Client) SetIngredients(ctx context.Context, slug string, lines []IngredientLine) error {
	c.parseUnstructured(ctx, lines)

	// food/unit are ALWAYS written null, never a parsed name. Verified live
	// against the real Mealie instance: {"food": {"name": "pasta"}} (a name
	// with no id) 500s with "ValueError: Expected 'id' to be provided for
	// food" — the same class of error as the referenceId corruption bug, just
	// triggered by a different missing key. Mealie's food/unit are references
	// into its own catalog, which requires a real id we don't have (the brute
	// parser returns names, not catalog ids) — so unlike quantity (a plain
	// number, safe to write directly), a resolved food/unit name can only be
	// safely reported back to the caller (see Recipes.StructureFromText's
	// FoodName/Unit fields on the returned lines), never written here.
	patch := make([]recipeIngredientPatch, len(lines))
	for i, l := range lines {
		patch[i] = recipeIngredientPatch{
			ReferenceID: uuid.NewString(),
			Note:        l.Note,
			Display:     l.Note,
			Quantity:    l.Quantity,
		}
	}

	if _, err := c.patchRaw(ctx, "/api/recipes/"+slug, map[string]any{"recipeIngredient": patch}); err != nil {
		return fmt.Errorf("mealie: set ingredients for %q: %w", slug, err)
	}
	return nil
}

// recipeInstructionPatch is the outgoing shape for one recipeInstructions
// entry. PATCHing with just {"text": "..."} throws a 500 (TypeError) — Mealie
// requires the full object shape, including a fresh id per step. Found and
// fixed manually while importing recipes earlier in the session that produced
// this client's write path.
type recipeInstructionPatch struct {
	ID                   string   `json:"id"`
	Title                string   `json:"title"`
	Summary              string   `json:"summary"`
	Text                 string   `json:"text"`
	IngredientReferences []string `json:"ingredientReferences"`
}

type tagListResponse struct {
	Items []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Slug string `json:"slug"`
	} `json:"items"`
}

type tagRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// getOrCreateTag returns the id of the Mealie tag named name, creating it if
// it doesn't exist. POST /api/organizers/tags is NOT idempotent — it 500s if
// the tag already exists (verified against the live instance) — so this
// always lists first rather than create-then-fall-back-on-error.
func (c *Client) getOrCreateTag(ctx context.Context, name string) (tagRef, error) {
	var list tagListResponse
	if err := c.get(ctx, "/api/organizers/tags", &list); err != nil {
		return tagRef{}, fmt.Errorf("mealie: list tags: %w", err)
	}
	for _, t := range list.Items {
		if strings.EqualFold(t.Name, name) {
			return tagRef{ID: t.ID, Name: t.Name, Slug: t.Slug}, nil
		}
	}
	raw, err := c.postRaw(ctx, "/api/organizers/tags", map[string]any{"name": name})
	if err != nil {
		return tagRef{}, fmt.Errorf("mealie: create tag %q: %w", name, err)
	}
	var created tagRef
	if err := json.Unmarshal(raw, &created); err != nil {
		return tagRef{}, fmt.Errorf("mealie: create tag %q: decode: %w", name, err)
	}
	return created, nil
}

// SetTags replaces slug's tags with tagNames, creating any tag that doesn't
// already exist in Mealie. PATCHing tags without a resolved id throws a 500
// (TypeError) — found while importing recipes earlier in the session that
// produced this write path — so every tag is resolved via getOrCreateTag
// first.
func (c *Client) SetTags(ctx context.Context, slug string, tagNames []string) error {
	tags := make([]tagRef, 0, len(tagNames))
	for _, name := range tagNames {
		t, err := c.getOrCreateTag(ctx, name)
		if err != nil {
			return fmt.Errorf("mealie: set tags for %q: %w", slug, err)
		}
		tags = append(tags, t)
	}
	if _, err := c.patchRaw(ctx, "/api/recipes/"+slug, map[string]any{"tags": tags}); err != nil {
		return fmt.Errorf("mealie: set tags for %q: %w", slug, err)
	}
	return nil
}

// SetInstructions replaces slug's recipeInstructions list, one entry per step.
func (c *Client) SetInstructions(ctx context.Context, slug string, steps []string) error {
	patch := make([]recipeInstructionPatch, len(steps))
	for i, s := range steps {
		patch[i] = recipeInstructionPatch{
			ID:                   uuid.NewString(),
			Text:                 s,
			IngredientReferences: []string{},
		}
	}
	if _, err := c.patchRaw(ctx, "/api/recipes/"+slug, map[string]any{"recipeInstructions": patch}); err != nil {
		return fmt.Errorf("mealie: set instructions for %q: %w", slug, err)
	}
	return nil
}
