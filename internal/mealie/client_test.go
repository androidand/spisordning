package mealie

import (
	"context"
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

func TestEffortFromTotalTime(t *testing.T) {
	cases := []struct {
		in   string
		want domain.Effort
	}{
		{"15 min", domain.EffortLow},
		{"25 minuter", domain.EffortLow},
		{"40 min", domain.EffortMedium},
		{"1 timme 30 minuter", domain.EffortHigh},
		{"", domain.EffortMedium},     // unknown defaults to medium
		{"snabbt", domain.EffortMedium},
	}
	for _, c := range cases {
		if got := effortFromTotalTime(c.in); got != c.want {
			t.Errorf("effortFromTotalTime(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}
