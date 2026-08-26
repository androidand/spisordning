package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Person mirrors migrations/0001_init.sql person.
type Person struct {
	ID        string
	Name      string
	Weight    float64
	CreatedAt time.Time
}

// CreatePerson inserts a person. A zero weight falls back to the column default (1.0),
// since weight must be > 0 and 0 means "unspecified".
func (s *Store) CreatePerson(ctx context.Context, p Person) error {
	const q = `INSERT INTO person (id, name, weight) VALUES ($1, $2, COALESCE(NULLIF($3, 0), 1.0))`
	if _, err := s.db.Exec(ctx, q, p.ID, p.Name, p.Weight); err != nil {
		return fmt.Errorf("persistence: create person: %w", err)
	}
	return nil
}

// GetPerson fetches one person by id.
func (s *Store) GetPerson(ctx context.Context, id string) (Person, error) {
	const q = `SELECT id, name, weight, created_at FROM person WHERE id = $1`
	var p Person
	if err := s.db.QueryRow(ctx, q, id).Scan(&p.ID, &p.Name, &p.Weight, &p.CreatedAt); err != nil {
		return Person{}, fmt.Errorf("persistence: get person: %w", err)
	}
	return p, nil
}

// ListPeople returns all people.
func (s *Store) ListPeople(ctx context.Context) ([]Person, error) {
	rows, err := s.db.Query(ctx, `SELECT id, name, weight, created_at FROM person ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("persistence: list people: %w", err)
	}
	return scanPeople(rows)
}

func scanPeople(rows pgx.Rows) ([]Person, error) {
	defer rows.Close()
	var out []Person
	for rows.Next() {
		var p Person
		if err := rows.Scan(&p.ID, &p.Name, &p.Weight, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PersonPreference is the current confidence-weighted sentiment toward a tag.
type PersonPreference struct {
	PersonID   string
	Tag        string
	Sentiment  int // -2..2
	Confidence float64
	UpdatedAt  time.Time
}

// UpsertPreference inserts or updates one (person,tag) preference.
func (s *Store) UpsertPreference(ctx context.Context, p PersonPreference) error {
	const q = `INSERT INTO person_preference (person_id, tag, sentiment, confidence)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (person_id, tag) DO UPDATE SET sentiment = EXCLUDED.sentiment,
			confidence = EXCLUDED.confidence, updated_at = now()`
	if _, err := s.db.Exec(ctx, q, p.PersonID, p.Tag, p.Sentiment, p.Confidence); err != nil {
		return fmt.Errorf("persistence: upsert preference: %w", err)
	}
	return nil
}

// ListPreferences returns preferences, optionally filtered to personID when
// non-empty. An empty personID returns all preferences (used by the unfiltered
// GET /preferences endpoint).
func (s *Store) ListPreferences(ctx context.Context, personID string) ([]PersonPreference, error) {
	var rows pgx.Rows
	var err error
	if personID == "" {
		rows, err = s.db.Query(ctx,
			`SELECT person_id, tag, sentiment, confidence, updated_at
			 FROM person_preference ORDER BY person_id, tag`)
	} else {
		rows, err = s.db.Query(ctx,
			`SELECT person_id, tag, sentiment, confidence, updated_at
			 FROM person_preference WHERE person_id = $1 ORDER BY tag`, personID)
	}
	if err != nil {
		return nil, fmt.Errorf("persistence: list preferences: %w", err)
	}
	defer rows.Close()
	var out []PersonPreference
	for rows.Next() {
		var p PersonPreference
		if err := rows.Scan(&p.PersonID, &p.Tag, &p.Sentiment, &p.Confidence, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PreferenceObservation is an append-only evidence row feeding confidence.
type PreferenceObservation struct {
	ID         int64
	PersonID   string
	Tag        string
	Sentiment  int
	Source     string // 'reaction' | 'manual' | 'import'
	ObservedAt time.Time
}

// RecordObservation appends evidence and re-derives the aggregate preference is
// handled by the application layer; this only writes the observation.
func (s *Store) RecordObservation(ctx context.Context, o PreferenceObservation) error {
	const q = `INSERT INTO preference_observation (person_id, tag, sentiment, source)
		VALUES ($1, $2, $3, $4)`
	if _, err := s.db.Exec(ctx, q, o.PersonID, o.Tag, o.Sentiment, o.Source); err != nil {
		return fmt.Errorf("persistence: record observation: %w", err)
	}
	return nil
}
