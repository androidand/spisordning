package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/androidand/spisordning/internal/mealie"
)

// The worked example from
// openspec/changes/implement-recipe-structuring-from-text/proposal.md,
// verbatim (Andreas's own real input, 2026-08-29) — this is the shape the
// design has to actually handle, not a clean synthetic case.
const workedExampleRecipe = `Pasta och tacokyckling i ugn
500g kokt pasta
1 påse fryst tacokyckling (Ica basic) 600g
1 burk tacosås
1 stor burk Creme fraiche
salt
svartpeppar
salladskrydda (eller något annat?)
Riven ost

Gör så här:
Koka pastan och lägg i en ungnsfast form
Strö över kycklingen över pastan
Blanda såsen: creme fraiche, tacosås och kryddor
Strö över osten

Grädda i mitten av ugnen på 200 grader C, ca 20-32 min`

func TestSectionRecipeText_WorkedExample(t *testing.T) {
	got := sectionRecipeText(workedExampleRecipe)

	if got.Title != "Pasta och tacokyckling i ugn" {
		t.Errorf("Title = %q", got.Title)
	}

	wantIngredients := []string{
		"500g kokt pasta",
		"1 påse fryst tacokyckling (Ica basic) 600g",
		"1 burk tacosås",
		"1 stor burk Creme fraiche",
		"salt",
		"svartpeppar",
		"salladskrydda (eller något annat?)",
		"Riven ost",
	}
	if !reflect.DeepEqual(got.IngredientLines, wantIngredients) {
		t.Errorf("IngredientLines = %#v, want %#v", got.IngredientLines, wantIngredients)
	}

	wantSteps := []string{
		"Koka pastan och lägg i en ungnsfast form",
		"Strö över kycklingen över pastan",
		"Blanda såsen: creme fraiche, tacosås och kryddor",
		"Strö över osten",
		"Grädda i mitten av ugnen på 200 grader C, ca 20-32 min",
	}
	if !reflect.DeepEqual(got.InstructionSteps, wantSteps) {
		t.Errorf("InstructionSteps = %#v, want %#v", got.InstructionSteps, wantSteps)
	}
}

func TestSectionRecipeText_NoMarker(t *testing.T) {
	// Best-effort: no recognized marker line means everything after the
	// title is treated as ingredients, and there are no instructions — not a
	// hard failure.
	got := sectionRecipeText("Enkel sallad\nSallad\nTomat\nGurka")
	if got.Title != "Enkel sallad" {
		t.Errorf("Title = %q", got.Title)
	}
	if len(got.IngredientLines) != 3 {
		t.Errorf("IngredientLines = %#v, want 3 lines", got.IngredientLines)
	}
	if len(got.InstructionSteps) != 0 {
		t.Errorf("InstructionSteps = %#v, want none", got.InstructionSteps)
	}
}

func TestSectionRecipeText_MarkerCaseAndColonInsensitive(t *testing.T) {
	got := sectionRecipeText("Titel\ningrediens\nINSTRUKTIONER\nsteg ett")
	if len(got.IngredientLines) != 1 || got.IngredientLines[0] != "ingrediens" {
		t.Errorf("IngredientLines = %#v", got.IngredientLines)
	}
	if len(got.InstructionSteps) != 1 || got.InstructionSteps[0] != "steg ett" {
		t.Errorf("InstructionSteps = %#v", got.InstructionSteps)
	}
}

// TestStructureFromText_WorkedExample drives StructureFromText against a fake
// Mealie server through the full worked example from proposal.md, asserting
// the create/set-ingredients/set-instructions sequence happens with the right
// content, and that a genuinely ambiguous line (bare "salt", no quantity or
// unit) is surfaced as low-confidence rather than silently presented as a
// resolved ingredient.
func TestStructureFromText_WorkedExample(t *testing.T) {
	var createdName string
	var patchedIngredients, patchedInstructions, patchedTags json.RawMessage

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/organizers/tags" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"items":[]}`)) // no existing tags — chat-import must be created
		case r.URL.Path == "/api/organizers/tags" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"tag-id-1","name":"chat-import","slug":"chat-import"}`))
		case r.URL.Path == "/api/recipes" && r.Method == http.MethodPost:
			var body struct {
				Name string `json:"name"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			createdName = body.Name
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`"pasta-och-tacokyckling-i-ugn"`))
		case r.URL.Path == "/api/parser/ingredients":
			var body struct {
				Ingredients []string `json:"ingredients"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			results := make([]map[string]any, len(body.Ingredients))
			for i, note := range body.Ingredients {
				// Lines with a recognizable quantity+food parse confidently;
				// bare single-word lines (salt, svartpeppar) and the
				// parenthetical aside don't — matching how Mealie's real
				// brute parser actually behaves on this kind of input.
				switch note {
				case "500g kokt pasta":
					results[i] = confidentResult(0.95, 500, "g", "pasta")
				case "1 påse fryst tacokyckling (Ica basic) 600g":
					results[i] = confidentResult(0.8, 600, "g", "tacokyckling")
				case "1 burk tacosås":
					results[i] = confidentResult(0.85, 1, "burk", "tacosås")
				case "1 stor burk Creme fraiche":
					results[i] = confidentResult(0.75, 1, "burk", "Creme fraiche")
				case "Riven ost":
					results[i] = confidentResult(0.7, 0, "", "ost")
				default: // salt, svartpeppar, "salladskrydda (eller något annat?)"
					results[i] = confidentResult(0.1, 0, "", "")
				}
			}
			b, _ := json.Marshal(results)
			_, _ = w.Write(b)
		case r.Method == http.MethodPatch:
			var body struct {
				RecipeIngredient  json.RawMessage `json:"recipeIngredient"`
				RecipeInstruction json.RawMessage `json:"recipeInstructions"`
				Tags              json.RawMessage `json:"tags"`
			}
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &body)
			if body.RecipeIngredient != nil {
				patchedIngredients = body.RecipeIngredient
			}
			if body.RecipeInstruction != nil {
				patchedInstructions = body.RecipeInstruction
			}
			if body.Tags != nil {
				patchedTags = body.Tags
			}
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	// This test exercises the legacy Mealie write path (no database), so pin
	// the recipe source to mealie regardless of the ambient RECIPE_SOURCE.
	t.Setenv("RECIPE_SOURCE", "mealie")
	svc := NewRecipes(nil, mealie.New(srv.URL, "tok"))
	got, err := svc.StructureFromText(context.Background(), workedExampleRecipe)
	if err != nil {
		t.Fatalf("StructureFromText: %v", err)
	}

	if createdName != "Pasta och tacokyckling i ugn" {
		t.Errorf("CreateRecipe called with name %q", createdName)
	}
	if got.RecipeID != "pasta-och-tacokyckling-i-ugn" {
		t.Errorf("RecipeID = %q", got.RecipeID)
	}
	if got.Title != "Pasta och tacokyckling i ugn" {
		t.Errorf("Title = %q", got.Title)
	}
	if len(got.Ingredients) != 8 {
		t.Fatalf("expected 8 ingredients, got %d: %+v", len(got.Ingredients), got.Ingredients)
	}
	if len(got.Instructions) != 5 {
		t.Fatalf("expected 5 instruction steps, got %d: %+v", len(got.Instructions), got.Instructions)
	}
	if patchedIngredients == nil {
		t.Error("expected a recipeIngredient PATCH")
	}
	if patchedInstructions == nil {
		t.Error("expected a recipeInstructions PATCH")
	}
	if patchedTags == nil {
		t.Error("expected a tags PATCH (chat-import)")
	} else if !strings.Contains(string(patchedTags), "chat-import") {
		t.Errorf("expected the chat-import tag in the PATCH body, got %s", patchedTags)
	}

	// The genuinely ambiguous lines must be flagged, not silently guessed.
	wantLow := map[string]bool{"salt": true, "svartpeppar": true, "salladskrydda (eller något annat?)": true}
	gotLow := map[string]bool{}
	for _, l := range got.LowConfidence {
		gotLow[l] = true
	}
	for note := range wantLow {
		if !gotLow[note] {
			t.Errorf("expected %q to be flagged low-confidence, got LowConfidence=%v", note, got.LowConfidence)
		}
	}
	// A confidently-parsed line must NOT be flagged.
	if gotLow["500g kokt pasta"] {
		t.Errorf("did not expect the confidently-parsed pasta line to be flagged: %v", got.LowConfidence)
	}
}

func confidentResult(confidence, quantity float64, unit, food string) map[string]any {
	ing := map[string]any{"quantity": quantity}
	if unit != "" {
		ing["unit"] = map[string]any{"name": unit}
	} else {
		ing["unit"] = nil
	}
	if food != "" {
		ing["food"] = map[string]any{"name": food}
	} else {
		ing["food"] = nil
	}
	return map[string]any{
		"confidence": map[string]any{"average": confidence},
		"ingredient": ing,
	}
}

func TestSectionRecipeText_MarkerSubstringDoesNotTrigger(t *testing.T) {
	// "method" appearing inside a longer line must not be mistaken for the
	// English "Method" marker — only a whole-line match counts.
	got := sectionRecipeText("Titel\n1 msk metod-ost\nsteg utan markör")
	if len(got.IngredientLines) != 2 {
		t.Errorf("IngredientLines = %#v, want both lines treated as ingredients (no marker found)", got.IngredientLines)
	}
}
