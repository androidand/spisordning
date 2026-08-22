// Package main is the composition root: it owns the only edge that may import both
// the persistence layer (cmd -> persistence is allowed; internal packages may not
// import cmd) and the httpapi layer, bridging the two with small adapters that keep
// httpapi dependency-free of persistence (enforced by internal/architecturetest).
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/androidand/spisordning/internal/httpapi"
	"github.com/androidand/spisordning/internal/persistence"
	"github.com/jackc/pgx/v5"
)

// storeAdapter adapts *persistence.Store to every httpapi service interface.
// It is the sole place that knows both the persistence row types and the httpapi
// response DTOs; httpapi sees only the interfaces it defines itself.
type storeAdapter struct {
	db *persistence.Store
}

func (a storeAdapter) ListPeople(ctx context.Context) ([]httpapi.PersonResponse, error) {
	people, err := a.db.ListPeople(ctx)
	if err != nil {
		return nil, fmt.Errorf("people list: %w", err)
	}
	out := make([]httpapi.PersonResponse, 0, len(people))
	for _, p := range people {
		out = append(out, httpapi.PersonResponse{
			ID: p.ID, Name: p.Name, Weight: p.Weight, CreatedAt: p.CreatedAt,
		})
	}
	return out, nil
}

func (a storeAdapter) GetPerson(ctx context.Context, id string) (httpapi.PersonResponse, error) {
	p, err := a.db.GetPerson(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return httpapi.PersonResponse{}, httpapi.ErrNotFound
	}
	if err != nil {
		return httpapi.PersonResponse{}, fmt.Errorf("people get: %w", err)
	}
	return httpapi.PersonResponse{
		ID: p.ID, Name: p.Name, Weight: p.Weight, CreatedAt: p.CreatedAt,
	}, nil
}

func (a storeAdapter) CreatePerson(ctx context.Context, in httpapi.PersonInput) (httpapi.PersonResponse, error) {
	if in.Weight <= 0 {
		in.Weight = 1.0
	}
	id, err := newPersonID()
	if err != nil {
		return httpapi.PersonResponse{}, fmt.Errorf("people create: generate id: %w", err)
	}
	p := persistence.Person{ID: id, Name: in.Name, Weight: in.Weight, CreatedAt: time.Now()}
	if err := a.db.CreatePerson(ctx, p); err != nil {
		return httpapi.PersonResponse{}, fmt.Errorf("people create: %w", err)
	}
	return httpapi.PersonResponse{
		ID: p.ID, Name: p.Name, Weight: p.Weight, CreatedAt: p.CreatedAt,
	}, nil
}

func (a storeAdapter) ListPreferences(ctx context.Context, personID string) ([]httpapi.PersonPreferenceResponse, error) {
	prefs, err := a.db.ListPreferences(ctx, personID)
	if err != nil {
		return nil, fmt.Errorf("preferences list: %w", err)
	}
	out := make([]httpapi.PersonPreferenceResponse, 0, len(prefs))
	for _, p := range prefs {
		out = append(out, httpapi.PersonPreferenceResponse{
			PersonID: p.PersonID, Tag: p.Tag, Sentiment: int(p.Sentiment),
			Confidence: p.Confidence, UpdatedAt: p.UpdatedAt,
		})
	}
	return out, nil
}

func (a storeAdapter) ListRecipes(ctx context.Context) ([]httpapi.RecipeRefResponse, error) {
	refs, err := a.db.ListRecipeRefs(ctx)
	if err != nil {
		return nil, fmt.Errorf("recipes list: %w", err)
	}
	out := make([]httpapi.RecipeRefResponse, 0, len(refs))
	for _, r := range refs {
		out = append(out, httpapi.RecipeRefResponse{
			MealieRecipeID: r.MealieRecipeID, Title: r.Title, Tags: r.Tags,
			Effort: r.Effort, LastSyncedAt: r.LastSyncedAt,
		})
	}
	return out, nil
}

func (a storeAdapter) CreateMealEvent(ctx context.Context, in httpapi.MealEventNew) (httpapi.MealEventResponse, error) {
	servedOn, err := time.Parse("2006-01-02", in.ServedOn)
	if err != nil {
		return httpapi.MealEventResponse{}, fmt.Errorf("meals create: invalid served_on %q: %w", in.ServedOn, err)
	}
	// Wrap event + reactions in a transaction so they commit atomically.
	tx, err := a.db.BeginTx(ctx)
	if err != nil {
		return httpapi.MealEventResponse{}, fmt.Errorf("meals create: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Insert the event inside the tx. Ad-hoc meals have no plan link.
	const insertEventQ = `INSERT INTO meal_event (mealie_recipe_id, served_on) VALUES ($1, $2) RETURNING id`
	var eventID int64
	if err := tx.QueryRow(ctx, insertEventQ, in.MealieRecipeID, servedOn).Scan(&eventID); err != nil {
		return httpapi.MealEventResponse{}, fmt.Errorf("meals create: insert event: %w", err)
	}
	if _, err := a.db.CreateMealEvent(ctx, in.MealieRecipeID, servedOn, nil, nil); err != nil {
		return httpapi.MealEventResponse{}, fmt.Errorf("meals create: persist event: %w", err)
	}

	// Insert reactions inside the same tx.
	for _, rx := range in.Reactions {
		const insertRxQ = `INSERT INTO meal_reaction (meal_event_id, person_id, sentiment, note)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (meal_event_id, person_id) DO UPDATE SET sentiment = EXCLUDED.sentiment,
				note = EXCLUDED.note`
		if _, err := tx.Exec(ctx, insertRxQ, eventID, rx.PersonID, rx.Sentiment, ""); err != nil {
			return httpapi.MealEventResponse{}, fmt.Errorf("meals create: add reaction: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return httpapi.MealEventResponse{}, fmt.Errorf("meals create: commit tx: %w", err)
	}

	rxns, err := a.db.ListMealReactions(ctx, eventID)
	if err != nil {
		return httpapi.MealEventResponse{}, fmt.Errorf("meals create: read reactions: %w", err)
	}
	out := httpapi.MealEventResponse{
		ID: eventID, MealieRecipeID: in.MealieRecipeID,
		ServedOn:  in.ServedOn,
		CreatedAt: time.Now(),
		Reactions: make([]httpapi.MealReactionResponse, 0, len(rxns)),
	}
	for _, r := range rxns {
		out.Reactions = append(out.Reactions, httpapi.MealReactionResponse{
			PersonID: r.PersonID, Sentiment: r.Sentiment,
		})
	}
	return out, nil
}

// newPersonID generates a 16-char hex id from crypto/rand (stdlib only — no new dep).
func newPersonID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
