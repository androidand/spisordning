package skolmaten

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWeekMenu_ParsesDaysAndMeals(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/4/menu/school/mariaskolan" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if r.URL.Query().Get("year") != "2026" || r.URL.Query().Get("week") != "30" {
			t.Errorf("unexpected query %s", r.URL.RawQuery)
		}
		if r.Header.Get("Client-Token") != "tok" {
			t.Errorf("missing Client-Token header")
		}
		w.Write([]byte(`{
			"id": "x", "name": "Mariaskolan",
			"WeekState": {
				"week": 30, "year": 2026,
				"Days": [
					{"date": "2026-07-20T00:00:00Z", "Meals": [{"id":"a","name":"Stekt fisk med potatismos"}]},
					{"date": "2026-07-21T00:00:00Z", "Meals": [{"id":"b","name":"Pasta med köttfärssås"}, {"id":"c","name":"Vegetarisk lasagne"}]}
				]
			},
			"School": {"id":"s","name":"Mariaskolan"}
		}`))
	}))
	defer srv.Close()

	days, err := New(srv.URL, "tok").WeekMenu(context.Background(), "mariaskolan", 2026, 30)
	if err != nil {
		t.Fatalf("WeekMenu: %v", err)
	}
	if len(days) != 2 {
		t.Fatalf("expected 2 days, got %d", len(days))
	}
	if days[0].Meals[0] != "Stekt fisk med potatismos" {
		t.Errorf("unexpected meal: %q", days[0].Meals[0])
	}
	if len(days[1].Meals) != 2 {
		t.Errorf("expected 2 meals on day 2, got %d", len(days[1].Meals))
	}
}

func TestWeekMenu_UnpublishedWeekIsEmptyNotError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"id":"x","name":"Mariaskolan","WeekState":null,"School":{"id":"s","name":"m"}}`))
	}))
	defer srv.Close()

	days, err := New(srv.URL, "").WeekMenu(context.Background(), "mariaskolan", 2026, 52)
	if err != nil {
		t.Fatalf("unpublished week should not error: %v", err)
	}
	if len(days) != 0 {
		t.Errorf("expected empty menu, got %d days", len(days))
	}
}

func TestTagsForDay_TokenizesAndFiltersStopwords(t *testing.T) {
	day := DayMenu{Meals: []string{"Stekt fisk med potatismos", "Fisk i ugn"}}
	tags := TagsForDay(day)

	want := map[string]bool{"stekt": true, "fisk": true, "potatismos": true, "ugn": true}
	if len(tags) != len(want) {
		t.Fatalf("expected %d tags, got %v", len(want), tags)
	}
	for _, tag := range tags {
		if !want[tag] {
			t.Errorf("unexpected tag %q (stopword leak?)", tag)
		}
	}
}
