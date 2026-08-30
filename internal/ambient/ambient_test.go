package ambient

import (
	"testing"

	"github.com/androidand/spisordning/internal/domain"
)

func TestRecordReactionSeedsMissingPreference(t *testing.T) {
	personID, err := domain.ParsePersonID("00000000-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatalf("parse personID: %v", err)
	}
	out := RecordReaction(nil, personID, []string{"pasta", "tomato"}, domain.Loves)
	if len(out) != 2 {
		t.Fatalf("want 2 prefs, got %d", len(out))
	}
	for _, p := range out {
		if p.PersonID != personID {
			t.Errorf("want personId %s, got %s", personID, p.PersonID)
		}
		if p.Sentiment != domain.Loves {
			t.Errorf("want Loves, got %v", p.Sentiment)
		}
		if p.Confidence != observationWeight {
			t.Errorf("want confidence %v, got %v", observationWeight, p.Confidence)
		}
	}
}

func TestRecordReactionIncreasesConfidenceAndMovesSentiment(t *testing.T) {
	dadID, err := domain.ParsePersonID("00000000-0000-0000-0000-000000000002")
	if err != nil {
		t.Fatalf("parse personID: %v", err)
	}
	prefs := []domain.Preference{
		{PersonID: dadID, Tag: "gryta", Sentiment: domain.Neutral, Confidence: 0.3},
	}
	out := RecordReaction(prefs, dadID, []string{"gryta"}, domain.Loves)
	if len(out) != 1 {
		t.Fatalf("want 1 pref, got %d", len(out))
	}
	p := out[0]
	if p.Confidence <= 0.3 {
		t.Errorf("want confidence > 0.3, got %v", p.Confidence)
	}
	// Weighted mean of (0.3*0 + 0.25*2) / 0.55 = 0.91 -> rounds to 1 (Likes).
	if p.Sentiment != domain.Likes {
		t.Errorf("want sentiment to move toward positive, got %v", p.Sentiment)
	}
}

func TestRecordReactionCapsConfidenceAtOne(t *testing.T) {
	kidID, err := domain.ParsePersonID("00000000-0000-0000-0000-000000000003")
	if err != nil {
		t.Fatalf("parse personID: %v", err)
	}
	prefs := []domain.Preference{
		{PersonID: kidID, Tag: "pasta", Sentiment: domain.Loves, Confidence: 0.9},
	}
	out := RecordReaction(prefs, kidID, []string{"pasta"}, domain.Loves)
	if len(out) != 1 {
		t.Fatalf("want 1 pref, got %d", len(out))
	}
	if out[0].Confidence > 1 {
		t.Errorf("want confidence <= 1, got %v", out[0].Confidence)
	}
	if out[0].Confidence != 1 {
		t.Errorf("want confidence capped at 1, got %v", out[0].Confidence)
	}
}

func TestRecordReactionDoesNotTouchOtherPeople(t *testing.T) {
	kidID, err := domain.ParsePersonID("00000000-0000-0000-0000-000000000004")
	if err != nil {
		t.Fatalf("parse personID: %v", err)
	}
	mumID, err := domain.ParsePersonID("00000000-0000-0000-0000-000000000005")
	if err != nil {
		t.Fatalf("parse personID: %v", err)
	}
	prefs := []domain.Preference{
		{PersonID: kidID, Tag: "pasta", Sentiment: domain.Hates, Confidence: 0.5},
	}
	out := RecordReaction(prefs, mumID, []string{"pasta"}, domain.Loves)
	if len(out) != 2 {
		t.Fatalf("want 2 prefs (kid unchanged + mum added), got %d", len(out))
	}
	if out[0].PersonID != kidID || out[0].Sentiment != domain.Hates || out[0].Confidence != 0.5 {
		t.Errorf("kid's preference changed: %+v", out[0])
	}
}

func TestRecordReactionDoesNotMutateInput(t *testing.T) {
	kidID, err := domain.ParsePersonID("00000000-0000-0000-0000-000000000006")
	if err != nil {
		t.Fatalf("parse personID: %v", err)
	}
	prefs := []domain.Preference{
		{PersonID: kidID, Tag: "pasta", Sentiment: domain.Likes, Confidence: 0.4},
	}
	_ = RecordReaction(prefs, kidID, []string{"pasta"}, domain.Loves)
	if prefs[0].Confidence != 0.4 || prefs[0].Sentiment != domain.Likes {
		t.Errorf("input was mutated: %+v", prefs[0])
	}
}

func TestRecordReactionDedupesTags(t *testing.T) {
	kidID, err := domain.ParsePersonID("00000000-0000-0000-0000-000000000007")
	if err != nil {
		t.Fatalf("parse personID: %v", err)
	}
	out := RecordReaction(nil, kidID, []string{"pasta", "pasta"}, domain.Loves)
	if len(out) != 1 {
		t.Fatalf("want 1 pref (deduped), got %d", len(out))
	}
	if out[0].Confidence != observationWeight {
		t.Errorf("duplicate tag double-counted: confidence %v", out[0].Confidence)
	}
}

func TestTonightPicksDateThenFallsBackToFirst(t *testing.T) {
	pf := PlanFile{Week: "2026-W31", Slots: []Slot{
		{Date: "2026-08-03", Title: "Köttbullar"},
		{Date: "2026-08-04", Title: "Pasta"},
	}}
	if s, _ := pf.Tonight("2026-08-04"); s.Title != "Pasta" {
		t.Errorf("want Pasta for 2026-08-04, got %q", s.Title)
	}
	if s, _ := pf.Tonight(""); s.Title != "Köttbullar" {
		t.Errorf("want first slot for empty date, got %q", s.Title)
	}
	if s, _ := pf.Tonight("2026-08-99"); s.Title != "Köttbullar" {
		t.Errorf("want first slot for missing date, got %q", s.Title)
	}
	if _, ok := (PlanFile{}).Tonight("2026-08-03"); ok {
		t.Errorf("empty projection should report no slot")
	}
}

func TestRender(t *testing.T) {
	if got := Render(Slot{Title: "Köttbullar", Reason: "Low effort"}); got != "Köttbullar — Low effort" {
		t.Errorf("unexpected render with reason: %q", got)
	}
	if got := Render(Slot{Title: "Köttbullar"}); got != "Köttbullar" {
		t.Errorf("unexpected render without reason: %q", got)
	}
}
