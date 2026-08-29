package dto

import (
	"context"
	"time"
)

// PersonResponse is the JSON view of a household person (openapi: components/schemas/Person).
// It is a transport-layer DTO: httpapi never imports persistence, so this is shaped
// independently of persistence.Person and mapped by the cmd composition root.
type PersonResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Weight    float64   `json:"weight"`
	CreatedAt time.Time `json:"created_at"`
}

// PersonInput is the body for POST /people (openapi: components/schemas/PersonNew).
// Weight is optional; the adapter/server apply 1.0 when zero.
type PersonInput struct {
	Name   string  `json:"name"`
	Weight float64 `json:"weight"`
}

// PersonUpdate is the body for PATCH /people/{id} (openapi: components/schemas/PersonUpdate).
// Name is required; Weight is optional (0 leaves the existing weight unchanged).
type PersonUpdate struct {
	Name   string  `json:"name"`
	Weight float64 `json:"weight"`
}

// PersonService is the subset of the application surface the /people handlers
// need. It is defined here (not imported from persistence) so the httpapi layer
// stays dependency-free of the persistence layer — the architecture test forbids
// httpapi -> persistence. The cmd composition root supplies an implementation
// backed by persistence.Store.
type PersonService interface {
	ListPeople(ctx context.Context) ([]PersonResponse, error)
	GetPerson(ctx context.Context, id string) (PersonResponse, error)
	CreatePerson(ctx context.Context, in PersonInput) (PersonResponse, error)
	UpdatePerson(ctx context.Context, id string, in PersonUpdate) (PersonResponse, error)
}
