package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestRunPlan_EndToEnd drives the complete pipe against fake services:
// Mealie (2 recipes) → scorer with Skolmaten dedup (fish at school Monday) →
// Olla (returns prose; plan proceeds on scorer order) → adapter resolution →
// wishlist creation. Asserts the wishlist request contains the confidently
// resolved product and excludes the needs-review one.
func TestRunPlan_EndToEnd(t *testing.T) {
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
	defer fakeMealie.Close()

	// ── Fake Skolmaten: fish served at school on Monday of 2026-W31 ───────────
	fakeSkolmaten := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"id":"m","name":"Skolan","WeekState":{"week":31,"year":2026,"Days":[
			{"date":"2026-07-27T00:00:00Z","Meals":[{"id":"a","name":"Stekt fisk"}]}
		]},"School":{"id":"s","name":"Skolan"}}`))
	}))
	defer fakeSkolmaten.Close()

	// ── Fake Olla: returns prose (unparseable) — pipe must proceed anyway ─────
	fakeOlla := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"role": "assistant", "content": "Ät gott!"}}},
		})
	}))
	defer fakeOlla.Close()

	// ── Fake adapter: resolve + capture the wishlist creation ────────────────
	var mu sync.Mutex
	var wishlistBody map[string]any
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
			_ = json.NewDecoder(r.Body).Decode(&wishlistBody)
			mu.Unlock()
			w.WriteHeader(201)
			_ = json.NewEncoder(w).Encode(map[string]string{"wishlistId": "wl-1", "name": "Vecka 31"})
		default:
			w.WriteHeader(404)
		}
	}))
	defer fakeAdapter.Close()

	// ── Family config in a temp dir ───────────────────────────────────────────
	dir := t.TempDir()
	familyPath := filepath.Join(dir, "family.json")
	familyJSON := `{
		"people":[{"id":"kid","name":"Kid","weight":2}],
		"preferences":[{"personId":"kid","tag":"pasta","sentiment":2,"confidence":0.9},
		               {"personId":"kid","tag":"fisk","sentiment":-1,"confidence":0.8}],
		"kitchenEnergy":{"mon":2,"tue":2,"wed":2,"thu":2,"fri":2,"sat":3,"sun":3}
	}`
	if err := os.WriteFile(familyPath, []byte(familyJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("MEALIE_BASE_URL", fakeMealie.URL)
	t.Setenv("MEALIE_API_TOKEN", "tok")
	t.Setenv("SKOLMATEN_BASE_URL", fakeSkolmaten.URL)
	t.Setenv("SKOLMATEN_CLIENT_TOKEN", "")
	t.Setenv("OLLA_OPENAI_BASE_URL", fakeOlla.URL)
	t.Setenv("OLLA_MODEL", "test-model")
	t.Setenv("ADAPTER_URL", fakeAdapter.URL)

	err := runPlan([]string{
		"--family", familyPath,
		"--school", "skolan",
		"--week", "2026-W31",
		"--days", "2",
		"--create-wishlist",
	})
	if err != nil {
		t.Fatalf("runPlan: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if wishlistBody == nil {
		t.Fatal("no wishlist was created")
	}
	if wishlistBody["name"] != "Vecka 31" {
		t.Errorf("unexpected wishlist name %v", wishlistBody["name"])
	}
	items, _ := wishlistBody["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected exactly 1 confidently-resolved item, got %d (%v)", len(items), items)
	}
	item := items[0].(map[string]any)
	if item["productCode"] != "willys-1" {
		t.Errorf("expected willys-1 in wishlist, got %v", item["productCode"])
	}
	// laxfilé was needs-review and must NOT have been silently added.
}
