package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/androidand/spisordning/internal/ambient"
)

// newTestEnv wires the complete pipe against fake services (Mealie with 2
// recipes, Skolmaten with fish at school on Monday of 2026-W31, a fake adapter
// that resolves ingredients and captures wishlist creation) plus a family
// config. The Olla behavior is the caller's choice (prose reply, hard failure,
// ...) via ollaHandler. It returns the family file path and a thread-safe
// accessor for the last wishlist body the fake adapter received.
func newTestEnv(t *testing.T, ollaHandler http.HandlerFunc) (familyPath string, wishlistBody func() map[string]any) {
	t.Helper()

	// ── Fake Mealie ────────────────────────────────────────────────────────────
	fakeMealie := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/recipes":
			w.Write([]byte(`{"page":1,"perPage":50,"total":2,"items":[
				{"id":"r-pasta","slug":"pasta","name":"Pasta Bolognese"},
				{"id":"r-fisk","slug":"fisk","name":"Ugnslax"}]}`))
		case "/api/recipes/pasta":
			w.Write([]byte(`{"id":"r-pasta","slug":"pasta","name":"Pasta Bolognese","totalTime":"20 min",
				"tags":[{"name":"pasta"}],
				"recipeIngredient":[{"quantity":400,"unit":{"name":"g"},"food":{"id":"f1","name":"köttfärs"}}]}`))
		case "/api/recipes/fisk":
			w.Write([]byte(`{"id":"r-fisk","slug":"fisk","name":"Ugnslax","totalTime":"30 min",
				"tags":[{"name":"fisk"}],
				"recipeIngredient":[{"quantity":500,"unit":{"name":"g"},"food":{"id":"f2","name":"laxfilé"}}]}`))
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(fakeMealie.Close)

	// ── Fake Skolmaten: fish served at school on Monday of 2026-W31 ───────────
	fakeSkolmaten := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"id":"m","name":"Skolan","WeekState":{"week":31,"year":2026,"Days":[
			{"date":"2026-07-27T00:00:00Z","Meals":[{"id":"a","name":"Stekt fisk"}]}
		]},"School":{"id":"s","name":"Skolan"}}`))
	}))
	t.Cleanup(fakeSkolmaten.Close)

	// ── Fake Olla ──────────────────────────────────────────────────────────────
	fakeOlla := httptest.NewServer(ollaHandler)
	t.Cleanup(fakeOlla.Close)

	// ── Fake adapter: resolve + capture the wishlist creation ────────────────
	var mu sync.Mutex
	var lastWishlist map[string]any
	fakeAdapter := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/resolve":
			var in struct {
				Requirements []struct {
					IngredientID string `json:"ingredientId"`
				} `json:"requirements"`
			}
			_ = json.NewDecoder(r.Body).Decode(&in)
			var resolutions []map[string]any
			for _, req := range in.Requirements {
				if req.IngredientID == "köttfärs" {
					resolutions = append(resolutions, map[string]any{
						"ingredientId": req.IngredientID, "retailerProductId": "willys-1",
						"productName": "Köttfärs 20%", "packages": 1, "resolvedQuantity": 500.0,
						"matchType": "exact", "confidence": 0.95, "needsReview": false,
					})
				} else {
					resolutions = append(resolutions, map[string]any{
						"ingredientId": req.IngredientID, "retailerProductId": nil,
						"packages": 0, "matchType": "none", "confidence": 0.2, "needsReview": true,
					})
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"resolutions": resolutions})
		case "/shopping-lists":
			mu.Lock()
			_ = json.NewDecoder(r.Body).Decode(&lastWishlist)
			mu.Unlock()
			w.WriteHeader(201)
			_ = json.NewEncoder(w).Encode(map[string]string{"wishlistId": "wl-1", "name": "Vecka 31"})
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(fakeAdapter.Close)

	// ── Family config in a temp dir ───────────────────────────────────────────
	dir := t.TempDir()
	familyPath = filepath.Join(dir, "family.json")
	familyJSON := `{
		"people":[{"id":"01900000-0000-7000-8000-000000000001","name":"Kid","weight":2}],
		"preferences":[{"personId":"01900000-0000-7000-8000-000000000001","tag":"pasta","sentiment":2,"confidence":0.9},
		               {"personId":"01900000-0000-7000-8000-000000000001","tag":"fisk","sentiment":-1,"confidence":0.8}],
		"kitchenEnergy":{"mon":2,"tue":2,"wed":2,"thu":2,"fri":2,"sat":3,"sun":3}
	}`
	if err := os.WriteFile(familyPath, []byte(familyJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	// This test drives the Mealie pipeline end-to-end (no database), so pin
	// the recipe source to mealie regardless of the ambient RECIPE_SOURCE.
	t.Setenv("RECIPE_SOURCE", "mealie")
	t.Setenv("MEALIE_BASE_URL", fakeMealie.URL)
	t.Setenv("MEALIE_API_TOKEN", "tok")
	t.Setenv("SKOLMATEN_BASE_URL", fakeSkolmaten.URL)
	t.Setenv("SKOLMATEN_CLIENT_TOKEN", "")
	t.Setenv("OLLA_OPENAI_BASE_URL", fakeOlla.URL)
	t.Setenv("OLLA_MODEL", "test-model")
	t.Setenv("ADAPTER_URL", fakeAdapter.URL)

	return familyPath, func() map[string]any {
		mu.Lock()
		defer mu.Unlock()
		return lastWishlist
	}
}

// assertWishlist checks the wishlist the fake adapter received: one item, the
// confidently-resolved köttfärs product, and nothing silently added for the
// needs-review ingredient.
func assertWishlist(t *testing.T, body map[string]any, run int) {
	t.Helper()
	if body == nil {
		t.Fatal("no wishlist was created")
	}
	if body["name"] != "Vecka 31" {
		t.Errorf("run %d: unexpected wishlist name %v", run, body["name"])
	}
	items, _ := body["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("run %d: expected exactly 1 confidently-resolved item, got %d (%v)", run, len(items), items)
	}
	item := items[0].(map[string]any)
	if item["productCode"] != "willys-1" {
		t.Errorf("run %d: expected willys-1 in wishlist, got %v", run, item["productCode"])
	}
}

// TestRunPlan_EndToEnd drives the complete pipe against fake services:
// Mealie (2 recipes) → scorer with Skolmaten dedup (fish at school Monday) →
// Olla (returns prose; plan proceeds on scorer order) → adapter resolution →
// wishlist creation. Also exercises --write-tonight (task 5.2): asserts the
// ambient projection file is written and carries the planned dinners. Asserts
// the wishlist request contains the confidently resolved product and excludes
// the needs-review one.
func TestRunPlan_EndToEnd(t *testing.T) {
	// Fake Olla returns prose (unparseable) — pipe must proceed anyway.
	proseOlla := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"role": "assistant", "content": "Ät gott!"}}},
		})
	})
	familyPath, wishlistBody := newTestEnv(t, proseOlla)
	tonightPath := filepath.Join(filepath.Dir(familyPath), "tonight.json")

	err := runPlan([]string{
		"--family", familyPath,
		"--school", "skolan",
		"--week", "2026-W31",
		"--days", "2",
		"--create-wishlist",
		"--write-tonight", tonightPath,
	})
	if err != nil {
		t.Fatalf("runPlan: %v", err)
	}

	// The ambient projection (task 5.2) must be written and carry the planned dinners.
	projBuf, err := os.ReadFile(tonightPath)
	if err != nil {
		t.Fatalf("tonight projection not written: %v", err)
	}
	var proj ambient.PlanFile
	if err := json.Unmarshal(projBuf, &proj); err != nil {
		t.Fatalf("parse tonight projection: %v", err)
	}
	if proj.Week != "2026-W31" {
		t.Errorf("unexpected projection week %q", proj.Week)
	}
	if len(proj.Slots) != 2 {
		t.Fatalf("expected 2 projected dinners, got %d", len(proj.Slots))
	}
	if proj.Slots[0].Tags == nil || proj.Slots[0].Title == "" {
		t.Errorf("projected slot missing title/tags: %+v", proj.Slots[0])
	}

	// laxfilé was needs-review and must NOT have been silently added.
	assertWishlist(t, wishlistBody(), 0)
}

// TestRunPlan_EndToEnd_OllaUnavailable asserts implement-recommendations task
// 6.2: with the LLM down, ranking and the resulting plan are identical across
// repeated runs — the pipe proceeds on the deterministic scorer order.
func TestRunPlan_EndToEnd_OllaUnavailable(t *testing.T) {
	downOlla := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	familyPath, wishlistBody := newTestEnv(t, downOlla)

	args := []string{
		"--family", familyPath,
		"--school", "skolan",
		"--week", "2026-W31",
		"--days", "2",
		"--create-wishlist",
	}
	for run := 0; run < 2; run++ {
		if err := runPlan(args); err != nil {
			t.Fatalf("runPlan (run %d): %v", run, err)
		}
		assertWishlist(t, wishlistBody(), run)
	}
}
