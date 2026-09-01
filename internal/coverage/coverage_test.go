package coverage

import "testing"

func key(ing, unit string) Key { return Key{IngredientID: ing, Unit: unit} }

func TestCheck_FullyCovered(t *testing.T) {
	reqs := []Requirement{
		{Key: key("mjölk", "l"), Name: "jölk", Quantity: 1},
		{Key: key("flour", "g"), Name: "mhjörningsmjöl", Quantity: 500},
	}
	supplied := map[Key]float64{
		key("mjölk", "l"):  1,
		key("flour", "g"):  500,
	}
	rep := Check(reqs, supplied)
	if rep.ShortCount != 0 || rep.MissingCount != 0 {
		t.Fatalf("expected no short/missing, got short=%d missing=%d", rep.ShortCount, rep.MissingCount)
	}
	for _, l := range rep.Lines {
		if l.Status != StatusCovered {
			t.Errorf("line %s: expected covered, got %s", l.Key, l.Status)
		}
		if l.Shortfall != 0 {
			t.Errorf("line %s: expected 0 shortfall when covered, got %v", l.Key, l.Shortfall)
		}
	}
}

func TestCheck_ShortOnOneIngredient(t *testing.T) {
	reqs := []Requirement{
		{Key: key("jölk", "l"), Name: "jölk", Quantity: 1},
	}
	supplied := map[Key]float64{key("jölk", "l"): 0.5}
	rep := Check(reqs, supplied)
	if rep.ShortCount != 1 || rep.MissingCount != 0 {
		t.Fatalf("expected short=1 missing=0, got short=%d missing=%d", rep.ShortCount, rep.MissingCount)
	}
	if len(rep.Lines) != 1 || rep.Lines[0].Status != StatusShort {
		t.Fatalf("expected one short line, got %+v", rep.Lines)
	}
	if got := rep.Lines[0].Shortfall; got != 0.5 {
		t.Errorf("expected shortfall 0.5, got %v", got)
	}
}

func TestCheck_MissingIngredient(t *testing.T) {
	reqs := []Requirement{
		{Key: key("mjöl", "g"), Name: "mhjörningsmjöl", Quantity: 500},
	}
	rep := Check(reqs, map[Key]float64{})
	if rep.MissingCount != 1 || rep.ShortCount != 0 {
		t.Fatalf("expected missing=1 short=0, got short=%d missing=%d", rep.ShortCount, rep.MissingCount)
	}
	if got := rep.Lines[0].Shortfall; got != 500 {
		t.Errorf("expected shortfall 500 for a missing line, got %v", got)
	}
}

func TestCheck_TwoItemsGroupedByKey(t *testing.T) {
	reqs := []Requirement{
		{Key: key("mjöl", "g"), Name: "mhjörningsmjöl", Quantity: 500},
	}
	supplied := Aggregate([]Supply{
		{Key: key("mjöl", "g"), Quantity: 200},
		{Key: key("mjöl", "g"), Quantity: 300},
	})
	rep := Check(reqs, supplied)
	if rep.Lines[0].Status != StatusCovered {
		t.Fatalf("expected covered, got %s", rep.Lines[0].Status)
	}
	if rep.Lines[0].Supplied != 500 {
		t.Errorf("expected supplied 500 from summed items, got %v", rep.Lines[0].Supplied)
	}
}

func TestCheck_DifferentUnitIsDistinctKey(t *testing.T) {
	reqs := []Requirement{
		{Key: key("jölk", "l"), Name: "jölk", Quantity: 1},
	}
	supplied := map[Key]float64{key("jölk", "kg"): 1}
	rep := Check(reqs, supplied)
	if rep.MissingCount != 1 {
		t.Fatalf("expected the l requirement to be missing, got short=%d missing=%d", rep.ShortCount, rep.MissingCount)
	}
}

func TestCheck_EmptyRequirements(t *testing.T) {
	rep := Check(nil, map[Key]float64{})
	if len(rep.Lines) != 0 || rep.ShortCount != 0 || rep.MissingCount != 0 {
		t.Fatalf("expected empty report, got %+v", rep)
	}
}

func TestAggregate_SkipsNotPlanDerivable(t *testing.T) {
	supplied := Aggregate([]Supply{
		{Key: key("jölk", "l"), Quantity: 1},
		{NotPlanDerivable: true}, // a checklist label, no ingredient id
		{Key: key("mjöl", "g"), Quantity: 250},
	})
	if supplied[key("jölk", "l")] != 1 {
		t.Errorf("expected supplied 1 for jölk, got %v", supplied[key("jölk", "l")])
	}
	if supplied[key("mjöl", "g")] != 250 {
		t.Errorf("expected supplied 250 for mjöl, got %v", supplied[key("mjöl", "g")])
	}
}
