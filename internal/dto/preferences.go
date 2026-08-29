package dto

import (
	"context"
	"errors"
	"time"
)

// PersonPreferenceResponse mirrors api/openapi.yaml components/schemas/PersonPreference.
type PersonPreferenceResponse struct {
	PersonID   string    `json:"person_id"`
	Tag        string    `json:"tag"`
	Sentiment  int       `json:"sentiment"`  // -2 (hate) .. 2 (love)
	Confidence float64   `json:"confidence"` // [0,1]
	UpdatedAt  time.Time `json:"updated_at"` // rendered as RFC3339 by json.Marshal
}

// SetPreferenceInput is the body for POST /preferences (openapi: PersonPreferenceNew).
type SetPreferenceInput struct {
	PersonID   string  `json:"person_id"`
	Tag        string  `json:"tag"`
	Sentiment  int     `json:"sentiment"`  // -2 (hate) .. 2 (love)
	Confidence float64 `json:"confidence"` // [0,1]
}

// ErrInvalidPreference is returned when a preference is missing required fields
// or has an out-of-range sentiment/confidence.
var ErrInvalidPreference = errors.New("invalid preference")

// PreferencesService is the surface the /preferences handler needs.
// httpapi defines it (not importing persistence); the cmd root wires the impl.
type PreferencesService interface {
	// ListPreferences returns all preferences, or those for personID when non-empty.
	ListPreferences(ctx context.Context, personID string) ([]PersonPreferenceResponse, error)
	// SetPreference upserts a (person, tag) preference.
	SetPreference(ctx context.Context, in SetPreferenceInput) (PersonPreferenceResponse, error)
}
