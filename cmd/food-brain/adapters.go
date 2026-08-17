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

// personAdapter translates between httpapi's PersonService DTOs and persistence.Store.
// It is the only thing httpapi sees as a "person service"; swapping persistence for an
// in-memory fake (or a future Mealie-backed source) changes only this file.
type personAdapter struct {
	db *persistence.Store
}

func (a personAdapter) ListPeople(ctx context.Context) ([]httpapi.PersonResponse, error) {
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

func (a personAdapter) GetPerson(ctx context.Context, id string) (httpapi.PersonResponse, error) {
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

func (a personAdapter) CreatePerson(ctx context.Context, in httpapi.PersonInput) (httpapi.PersonResponse, error) {
	if in.Weight <= 0 {
		in.Weight = 1.0
	}
	id, err := newPersonID()
	if err != nil {
		return httpapi.PersonResponse{}, fmt.Errorf("people create: generate id: %w", err)
	}
	p := persistence.Person{
		ID: id, Name: in.Name, Weight: in.Weight, CreatedAt: time.Now(),
	}
	if err := a.db.CreatePerson(ctx, p); err != nil {
		return httpapi.PersonResponse{}, fmt.Errorf("people create: %w", err)
	}
	return httpapi.PersonResponse{
		ID: p.ID, Name: p.Name, Weight: p.Weight, CreatedAt: p.CreatedAt,
	}, nil
}

// newPersonID generates a 16-char hex id from crypto/rand (stdlib only — no new dep).
func newPersonID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
