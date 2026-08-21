package domain

import (
	"fmt"
	"time"
)

// Household is the unit that plans, cooks, and shops together. It owns
// membership, preferences, and the cookbook. Mutable (rename); never hard-deleted
// while it has history (establish-household-and-catalog design.md Step 4).
type Household struct {
	ID   string
	Name string
}

// Account is a login identity (credential, email, auth), entirely outside the
// food domain (design.md invariant 1). It is referenced by a Person (optional)
// but never owned by a Household — an Account is not invited to or removed from a
// household; a HouseholdMembership is. A Person may exist with no Account (a
// child), and deleting an Account never deletes a Person or their history.
type Account struct {
	ID    string
	Email string
}

// HouseholdMembership is the join between a Household and a Person, with a
// lifecycle (joined/left). Append + close: created on join, closed (EndedAt set)
// on leaving, never deleted (invariant 10 — "who was in the household when this
// meal was rated" must stay answerable).
type HouseholdMembership struct {
	HouseholdID string
	PersonID    string
	JoinedAt    time.Time
	// EndedAt is the zero time while the membership is active.
	EndedAt time.Time
}

// Active reports whether the membership is currently in force.
func (m HouseholdMembership) Active() bool {
	return m.EndedAt.IsZero()
}

// RestrictionKind is the categorical class of a PersonRestriction. It is never a
// sentiment: a restriction is a hard constraint, not a scored preference
// (invariant 2).
type RestrictionKind string

const (
	RestrictionAllergy         RestrictionKind = "ALLERGY"
	RestrictionHardRestriction RestrictionKind = "HARD_RESTRICTION"
)

// Valid reports whether the kind is one of the two restriction classes.
func (k RestrictionKind) Valid() bool {
	return k == RestrictionAllergy || k == RestrictionHardRestriction
}

// PersonRestriction is an ALLERGY or HARD_RESTRICTION a Person holds against a
// tag. Categorical, never scored, safety-critical (invariants 2 and 3). It is a
// deliberately separate model from Preference: it carries no Sentiment or
// Confidence, so it can never be fed into preference scoring. Set and cleared
// only by an explicit, attributed command.
type PersonRestriction struct {
	PersonID   string
	Tag        string
	Kind       RestrictionKind
	Note       string
	RecordedBy string
	RecordedAt time.Time
	// ClearedBy/ClearedAt are set when the restriction is cleared; the row is
	// kept for the audit trail, never deleted (invariant 3).
	ClearedBy string
	ClearedAt time.Time
}

// NewPersonRestriction validates a restriction at construction: the kind must be
// one of the two classes (invariant 2) and the change must be attributed to an
// actor (invariant 3 — safety-critical, so who recorded it is required).
func NewPersonRestriction(personID, tag string, kind RestrictionKind, note, recordedBy string, at time.Time) (PersonRestriction, error) {
	if !kind.Valid() {
		return PersonRestriction{}, fmt.Errorf("domain: invalid restriction kind %q", kind)
	}
	if personID == "" {
		return PersonRestriction{}, fmt.Errorf("domain: restriction requires a person id")
	}
	if tag == "" {
		return PersonRestriction{}, fmt.Errorf("domain: restriction requires a tag")
	}
	if recordedBy == "" {
		return PersonRestriction{}, fmt.Errorf("domain: restriction change must be attributed to an actor")
	}
	return PersonRestriction{PersonID: personID, Tag: tag, Kind: kind, Note: note, RecordedBy: recordedBy, RecordedAt: at}, nil
}

// Active reports whether the restriction is currently in force (not cleared).
func (r PersonRestriction) Active() bool {
	return r.ClearedAt.IsZero()
}

// Clear returns a copy of the restriction with its cleared attribution set. The
// row is kept (audit trail), never deleted (invariant 3).
func (r PersonRestriction) Clear(clearedBy string, at time.Time) (PersonRestriction, error) {
	if clearedBy == "" {
		return PersonRestriction{}, fmt.Errorf("domain: clearing a restriction must be attributed to an actor")
	}
	r.ClearedBy = clearedBy
	r.ClearedAt = at
	return r, nil
}

// AvoidTags returns the set of tags a person must avoid, drawn ONLY from their
// active restrictions. This is the hard-constraint input to planning. It never
// reads from preferences: a scored DISLIKE is an advisory signal, not a safety
// constraint, and must not leak into this set (invariants 2 and 3). The function
// accepts only []PersonRestriction — a Preference is a different type and cannot
// be passed here, which is the structural guarantee that a restriction is never
// scored as a preference and a preference is never a restriction.
func AvoidTags(restrictions []PersonRestriction) map[string]bool {
	out := make(map[string]bool, len(restrictions))
	for _, r := range restrictions {
		if r.Active() {
			out[r.Tag] = true
		}
	}
	return out
}
