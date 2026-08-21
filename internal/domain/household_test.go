package domain

import (
	"testing"
	"time"
)

func TestNewPersonRestriction(t *testing.T) {
	at := time.Now()

	valid := []struct {
		name string
		kind RestrictionKind
	}{
		{"allergy", RestrictionAllergy},
		{"hard restriction", RestrictionHardRestriction},
	}
	for _, c := range valid {
		t.Run(c.name, func(t *testing.T) {
			r, err := NewPersonRestriction("p1", "peanut", c.kind, "peanut allergy", "parent", at)
			if err != nil {
				t.Fatalf("NewPersonRestriction(%s): %v", c.kind, err)
			}
			if !r.Active() {
				t.Error("a freshly set restriction should be active")
			}
			if r.RecordedBy != "parent" {
				t.Errorf("RecordedBy = %q, want %q", r.RecordedBy, "parent")
			}
		})
	}

	invalid := []struct {
		name       string
		personID   string
		tag        string
		kind       RestrictionKind
		recordedBy string
	}{
		{"invalid kind", "p1", "peanut", RestrictionKind("LIKES"), "parent"},
		{"empty person", "", "peanut", RestrictionAllergy, "parent"},
		{"empty tag", "p1", "", RestrictionAllergy, "parent"},
		{"unattributed", "p1", "peanut", RestrictionAllergy, ""},
	}
	for _, c := range invalid {
		t.Run(c.name, func(t *testing.T) {
			if _, err := NewPersonRestriction(c.personID, c.tag, c.kind, "", c.recordedBy, at); err == nil {
				t.Error("expected an error, got nil")
			}
		})
	}
}

func TestPersonRestrictionClear(t *testing.T) {
	at := time.Now()
	r, err := NewPersonRestriction("p1", "peanut", RestrictionAllergy, "", "parent", at)
	if err != nil {
		t.Fatalf("NewPersonRestriction: %v", err)
	}

	// Clearing without attribution is rejected (invariant 3).
	if _, err := r.Clear("", at); err == nil {
		t.Error("expected an error for unattributed clear, got nil")
	}

	cleared, err := r.Clear("parent", at)
	if err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if cleared.Active() {
		t.Error("a cleared restriction should not be active")
	}
	// The original is unchanged (Clear returns a copy).
	if !r.Active() {
		t.Error("the original restriction should remain active after Clear returns a copy")
	}
}

func TestAvoidTagsOnlyFromRestrictions(t *testing.T) {
	at := time.Now()
	allergy, _ := NewPersonRestriction("p1", "peanut", RestrictionAllergy, "", "parent", at)
	hard, _ := NewPersonRestriction("p1", "gluten", RestrictionHardRestriction, "", "parent", at)
	cleared, _ := NewPersonRestriction("p1", "dairy", RestrictionAllergy, "", "parent", at)
	cleared, _ = cleared.Clear("parent", at)

	avoid := AvoidTags([]PersonRestriction{allergy, hard, cleared})

	if !avoid["peanut"] || !avoid["gluten"] {
		t.Errorf("active restrictions missing from avoid set: %v", avoid)
	}
	if avoid["dairy"] {
		t.Error("a cleared restriction must not be in the avoid set")
	}

	// Invariant 2/3 made concrete: a scored DISLIKE is a Preference, a different
	// type that cannot reach AvoidTags. A strong negative preference is advisory,
	// never a safety constraint — the avoid set is built only from restrictions.
	_ = []Preference{{PersonID: "p1", Tag: "peanut", Sentiment: Hates, Confidence: 1.0}}
}

func TestHouseholdMembershipActive(t *testing.T) {
	if !(HouseholdMembership{HouseholdID: "h1", PersonID: "p1"}).Active() {
		t.Error("a membership with no ended_at should be active")
	}
	if (HouseholdMembership{HouseholdID: "h1", PersonID: "p1", EndedAt: time.Now()}).Active() {
		t.Error("a membership with an ended_at should not be active")
	}
}
