package mealie

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/androidand/spisordning/internal/domain"
)

func TestSyncRecipes_NormalizesReferences(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("missing bearer token")
		}
		switch r.URL.Path {
		case "/api/recipes":
			w.Write([]byte(`{"page":1,"perPage":50,"total":2,"items":[
				{"id":"r1","slug":"pasta-bolognese","name":"Pasta Bolognese"},
				{"id":"r2","slug":"ugnslax","name":"Ugnslax"}
			]}`))
		case "/api/recipes/pasta-bolognese":
			w.Write([]byte(`{
				"id":"r1","slug":"pasta-bolognese","name":"Pasta Bolognese",
				"totalTime":"20 minuter",
				"tags":[{"name":"Pasta"},{"name":"Barnfavorit"}],
				"recipeIngredient":[
					{"quantity":400,"unit":{"name":"g"},"food":{"id":"f-mince","name":"Köttfärs"},"note":""},
					{"quantity":1,"unit":{"name":"st"},"food":{"id":"f-onion","name":"Gul lök"},"note":"hackad"}
				]
			}`))
		case "/api/recipes/ugnslax":
			w.Write([]byte(`{
				"id":"r2","slug":"ugnslax","name":"Ugnslax",
				"totalTime":"1 timme",
				"tags":[{"name":"Fisk"}],
				"recipeIngredient":[
					{"quantity":500,"unit":{"name":"g"},"food":{"id":"f-salmon","name":"Laxfilé"},"note":""}
				]
			}`))
		case "/api/parser/ingredients":
			t.Errorf("parser should not be called: recipe %s had structured foods", r.URL.Path)
			w.WriteHeader(500)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	refs, err := New(srv.URL, "tok").SyncRecipes(context.Background())
	if err != nil {
		t.Fatalf("SyncRecipes: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("expected 2 recipes, got %d", len(refs))
	}

	pasta := refs[0]
	if pasta.MealieRecipeID != "r1" || pasta.Title != "Pasta Bolognese" {
		t.Errorf("unexpected ref: %+v", pasta)
	}
	if len(pasta.Tags) != 2 || pasta.Tags[0] != "pasta" {
		t.Errorf("tags should be lowercased: %v", pasta.Tags)
	}
	if pasta.Effort != domain.EffortLow {
		t.Errorf("20 minutes should be low effort, got %d", pasta.Effort)
	}
	if len(pasta.Ingredients) != 2 || pasta.Ingredients[0].FoodID != "f-mince" ||
		pasta.Ingredients[0].Quantity != 400 || pasta.Ingredients[0].Unit != "g" {
		t.Errorf("unexpected ingredients: %+v", pasta.Ingredients)
	}
	if len(pasta.Raw) == 0 {
		t.Errorf("raw snapshot should be retained")
	}

	if refs[1].Effort != domain.EffortHigh {
		t.Errorf("1 timme should be high effort, got %d", refs[1].Effort)
	}
}

func TestSyncRecipes_ParsesUnstructuredIngredientsViaMealieParser(t *testing.T) {
	var parserCalled bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/recipes":
			w.Write([]byte(`{"page":1,"perPage":50,"total":1,"items":[
				{"id":"r1","slug":"korvstroganoff","name":"Korvstroganoff"}]}`))
		case "/api/recipes/korvstroganoff":
			// URL-imported recipe: ingredients are raw notes, no food/unit/quantity.
			w.Write([]byte(`{
				"id":"r1","slug":"korvstroganoff","name":"Korvstroganoff","totalTime":"30 minutes",
				"tags":[],
				"recipeIngredient":[
					{"quantity":0,"note":"550 g falukorv"},
					{"quantity":0,"note":"1 gul lök"}
				]}`))
		case "/api/parser/ingredients":
			parserCalled = true
			w.Write([]byte(`[
				{"ingredient":{"quantity":550,"unit":{"name":"g"},"food":{"name":"falukorv"}}},
				{"ingredient":{"quantity":1,"unit":{"name":"gul"},"food":{"name":"lök"}}}
			]`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	refs, err := New(srv.URL, "tok").SyncRecipes(context.Background())
	if err != nil {
		t.Fatalf("SyncRecipes: %v", err)
	}
	if !parserCalled {
		t.Fatal("expected the Mealie parser to be called for unstructured ingredients")
	}
	ings := refs[0].Ingredients
	if len(ings) != 2 {
		t.Fatalf("expected 2 ingredients, got %d", len(ings))
	}
	if ings[0].FoodName != "falukorv" || ings[0].Quantity != 550 || ings[0].Unit != "g" {
		t.Errorf("first ingredient not parsed: %+v", ings[0])
	}
	if ings[1].FoodName != "lök" {
		t.Errorf("second ingredient food not parsed: %+v", ings[1])
	}
}

// TestSyncRecipes_IsolatesBadNoteOnBatchParserFailure reproduces a real bug
// found against the live Mealie instance: the brute parser 500s on at least
// one input shape (a note containing a comma, e.g. "Ris, 4 portioner"), and
// batching a recipe's notes in one parser call meant that single bad note
// discarded every ingredient in the recipe, not just the unparseable one.
// The fix retries one note at a time when the batch fails, so a bad note is
// isolated instead of poisoning the whole recipe.
func TestSyncRecipes_IsolatesBadNoteOnBatchParserFailure(t *testing.T) {
	var batchCalls, singleCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/recipes":
			w.Write([]byte(`{"page":1,"perPage":50,"total":1,"items":[
				{"id":"r1","slug":"korvstroganoff","name":"Korvstroganoff"}]}`))
		case "/api/recipes/korvstroganoff":
			w.Write([]byte(`{
				"id":"r1","slug":"korvstroganoff","name":"Korvstroganoff","totalTime":"30 minutes",
				"tags":[],
				"recipeIngredient":[
					{"quantity":0,"note":"550 g falukorv"},
					{"quantity":0,"note":"Ris, 4 portioner"}
				]}`))
		case "/api/parser/ingredients":
			var body struct {
				Ingredients []string `json:"ingredients"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if len(body.Ingredients) > 1 {
				batchCalls++
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			singleCalls++
			if body.Ingredients[0] == "Ris, 4 portioner" {
				w.WriteHeader(http.StatusInternalServerError) // still fails alone, like the real bug
				return
			}
			w.Write([]byte(`[{"ingredient":{"quantity":550,"unit":{"name":"g"},"food":{"name":"falukorv"}}}]`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	refs, err := New(srv.URL, "tok").SyncRecipes(context.Background())
	if err != nil {
		t.Fatalf("SyncRecipes: %v", err)
	}
	if batchCalls != 1 {
		t.Fatalf("expected 1 batch attempt, got %d", batchCalls)
	}
	if singleCalls != 2 {
		t.Fatalf("expected a per-note retry for both lines after the batch failed, got %d", singleCalls)
	}
	ings := refs[0].Ingredients
	if len(ings) != 2 {
		t.Fatalf("expected 2 ingredients, got %d", len(ings))
	}
	if ings[0].FoodName != "falukorv" || ings[0].Quantity != 550 || ings[0].Unit != "g" {
		t.Errorf("good note should still parse via the per-note fallback: %+v", ings[0])
	}
	if ings[1].FoodName != "" {
		t.Errorf("bad note should stay unparsed (not crash or fabricate data): %+v", ings[1])
	}
}

func TestCreateRecipe_DecodesBareStringResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/recipes" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body struct {
			Name string `json:"name"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Name != "Pasta och tacokyckling i ugn" {
			t.Errorf("unexpected name in request: %q", body.Name)
		}
		w.WriteHeader(http.StatusCreated)
		// Verified against the live Mealie instance: the response body is a bare
		// JSON string (the slug), not an object.
		_, _ = w.Write([]byte(`"pasta-och-tacokyckling-i-ugn"`))
	}))
	defer srv.Close()

	slug, err := New(srv.URL, "tok").CreateRecipe(context.Background(), "Pasta och tacokyckling i ugn")
	if err != nil {
		t.Fatalf("CreateRecipe: %v", err)
	}
	if slug != "pasta-och-tacokyckling-i-ugn" {
		t.Errorf("slug = %q, want %q", slug, "pasta-och-tacokyckling-i-ugn")
	}
}

// TestSetIngredients_AlwaysSendsReferenceIDAndCleanNulls guards the
// corruption bug found and fixed manually earlier in the session that
// produced this write path: PATCHing recipeIngredient with referenceId
// omitted/null permanently corrupts the recipe (reference_id ends up NULL
// server-side, and the recipe becomes unreadable via GET forever after). It
// also guards the food/unit shape: a partial object like {"id": null} throws
// a 500 (ValueError: Expected 'id' to be provided for food) — food/unit must
// be either populated with a name, or clean null, never partial.
func TestSetIngredients_AlwaysSendsReferenceIDAndCleanNulls(t *testing.T) {
	var patched []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/parser/ingredients":
			// Resolve "500 g falukorv" (whichever call it arrives in — batch or
			// the per-note retry); everything else stays unresolved (food: null),
			// simulating a note the parser genuinely can't extract a food from.
			var body struct {
				Ingredients []string `json:"ingredients"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			results := make([]map[string]any, len(body.Ingredients))
			for i, note := range body.Ingredients {
				ing := map[string]any{"quantity": 0, "unit": nil, "food": nil}
				if note == "500 g falukorv" {
					ing = map[string]any{"quantity": 500, "unit": map[string]any{"name": "g"}, "food": map[string]any{"name": "falukorv"}}
				}
				results[i] = map[string]any{"ingredient": ing}
			}
			b, _ := json.Marshal(results)
			w.Write(b)
		case r.URL.Path == "/api/recipes/korvstroganoff" && r.Method == http.MethodPatch:
			var body struct {
				RecipeIngredient []map[string]any `json:"recipeIngredient"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			patched = body.RecipeIngredient
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	lines := []IngredientLine{
		{Note: "500 g falukorv"},   // unstructured, parser resolves it
		{Note: "Peppar", Unit: ""}, // parser gets no useful call target here in this test setup
	}
	if err := New(srv.URL, "tok").SetIngredients(context.Background(), "korvstroganoff", lines); err != nil {
		t.Fatalf("SetIngredients: %v", err)
	}
	if len(patched) != 2 {
		t.Fatalf("expected 2 patched ingredient rows, got %d", len(patched))
	}
	for i, p := range patched {
		refID, _ := p["referenceId"].(string)
		if refID == "" {
			t.Errorf("row %d: referenceId must never be empty (this is the corruption bug), got %+v", i, p)
		}
		if _, hasFood := p["food"]; !hasFood {
			t.Errorf("row %d: food key must be present (even if null), got %+v", i, p)
		}
		if _, hasUnit := p["unit"]; !hasUnit {
			t.Errorf("row %d: unit key must be present (even if null), got %+v", i, p)
		}
	}
	if patched[0]["food"] == nil || patched[0]["food"].(map[string]any)["name"] != "falukorv" {
		t.Errorf("expected the parser-resolved food name on row 0, got %+v", patched[0])
	}
	if patched[1]["food"] != nil {
		t.Errorf("expected clean null food for the unresolved row, got %+v", patched[1])
	}
}

// TestSetInstructions_AlwaysSendsFullObjectShape guards the 500-causing bug
// found while importing recipes earlier in the session that produced this
// write path: PATCHing recipeInstructions with just {"text": "..."} throws a
// TypeError — Mealie requires id/title/summary/ingredientReferences alongside
// text on every entry.
func TestSetInstructions_AlwaysSendsFullObjectShape(t *testing.T) {
	var patched []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/recipes/pasta" || r.Method != http.MethodPatch {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body struct {
			RecipeInstructions []map[string]any `json:"recipeInstructions"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		patched = body.RecipeInstructions
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	steps := []string{"Koka pastan", "Grädda i ugnen"}
	if err := New(srv.URL, "tok").SetInstructions(context.Background(), "pasta", steps); err != nil {
		t.Fatalf("SetInstructions: %v", err)
	}
	if len(patched) != 2 {
		t.Fatalf("expected 2 patched instruction rows, got %d", len(patched))
	}
	for i, p := range patched {
		for _, field := range []string{"id", "title", "summary", "text", "ingredientReferences"} {
			if _, ok := p[field]; !ok {
				t.Errorf("row %d: missing required field %q (this is the 500-causing shape), got %+v", i, field, p)
			}
		}
		if id, _ := p["id"].(string); id == "" {
			t.Errorf("row %d: id must never be empty, got %+v", i, p)
		}
	}
	if patched[0]["text"] != "Koka pastan" || patched[1]["text"] != "Grädda i ugnen" {
		t.Errorf("step text/order mismatch: %+v", patched)
	}
}

// TestSetTags_ReusesExistingTagAndCreatesMissing guards two real bugs found
// while importing recipes earlier in the session that produced this write
// path: (1) POST /api/organizers/tags is NOT idempotent — it 500s if the tag
// already exists — so an existing tag must be looked up, never blindly
// (re)created; (2) PATCHing tags without a resolved id throws a 500.
func TestSetTags_ReusesExistingTagAndCreatesMissing(t *testing.T) {
	var createCalls int
	var patchedTags []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/organizers/tags" && r.Method == http.MethodGet:
			w.Write([]byte(`{"items":[{"id":"existing-id","name":"middag","slug":"middag"}]}`))
		case r.URL.Path == "/api/organizers/tags" && r.Method == http.MethodPost:
			createCalls++
			var body struct {
				Name string `json:"name"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id":"new-id","name":"` + body.Name + `","slug":"` + body.Name + `"}`))
		case r.URL.Path == "/api/recipes/tacopaj" && r.Method == http.MethodPatch:
			var body struct {
				Tags []map[string]any `json:"tags"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			patchedTags = body.Tags
			w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	if err := New(srv.URL, "tok").SetTags(context.Background(), "tacopaj", []string{"middag", "chat-import"}); err != nil {
		t.Fatalf("SetTags: %v", err)
	}
	if createCalls != 1 {
		t.Fatalf("expected exactly 1 tag creation (only for the missing tag), got %d", createCalls)
	}
	if len(patchedTags) != 2 {
		t.Fatalf("expected 2 patched tags, got %+v", patchedTags)
	}
	if patchedTags[0]["id"] != "existing-id" {
		t.Errorf("expected the existing tag's real id to be reused, got %+v", patchedTags[0])
	}
	if patchedTags[1]["id"] != "new-id" || patchedTags[1]["name"] != "chat-import" {
		t.Errorf("expected the newly-created tag's id, got %+v", patchedTags[1])
	}
	for i, tag := range patchedTags {
		if id, _ := tag["id"].(string); id == "" {
			t.Errorf("tag %d: id must never be empty (this is the 500-causing shape), got %+v", i, tag)
		}
	}
}

func TestEffortFromTotalTime(t *testing.T) {
	cases := []struct {
		in   string
		want domain.Effort
	}{
		{"15 min", domain.EffortLow},
		{"25 minuter", domain.EffortLow},
		{"40 min", domain.EffortMedium},
		{"1 timme 30 minuter", domain.EffortHigh},
		{"", domain.EffortMedium}, // unknown defaults to medium
		{"snabbt", domain.EffortMedium},
	}
	for _, c := range cases {
		if got := effortFromTotalTime(c.in); got != c.want {
			t.Errorf("effortFromTotalTime(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}
