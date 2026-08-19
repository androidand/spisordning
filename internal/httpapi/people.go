package httpapi

import (
	"context"
	"errors"
	"encoding/json"
	"net/http"
	"strings"
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

// ErrNotFound signals a resource miss so handlers can map it to 404 without
// httpapi knowing about pgx sentinel errors.
var ErrNotFound = errors.New("not found")

// PersonService is the subset of the application surface the /people handlers
// need. It is defined here (not imported from persistence) so the httpapi layer
// stays dependency-free of the persistence layer — the architecture test forbids
// httpapi -> persistence. The cmd composition root supplies an implementation
// backed by persistence.Store.
type PersonService interface {
	ListPeople(ctx context.Context) ([]PersonResponse, error)
	GetPerson(ctx context.Context, id string) (PersonResponse, error)
	CreatePerson(ctx context.Context, in PersonInput) (PersonResponse, error)
}

// Dependencies are the services httpapi's handlers call. Add new services here as
// routes are implemented from api/openapi.yaml.
type Dependencies struct {
	People PersonService
}

type peopleHandler struct {
	svc PersonService
}

// RegisterHandlers wires every implemented route onto mux. /health is always
// available; resource routes are registered only when their service is provided,
// so a partial build (no DB) still serves the probe.
func RegisterHandlers(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("/health", healthHandler)

	if deps.People == nil {
		return
	}
	h := peopleHandler{svc: deps.People}

	mux.HandleFunc("GET /people", h.listPeople)
	mux.HandleFunc("POST /people", h.createPerson)
	mux.HandleFunc("GET /people/{id}", h.getPerson)
}

// Serve starts the HTTP server on addr (e.g. ":8080"). Handlers are sourced from
// api/openapi.yaml; /health and /people{,/id} are wired today. Persistence-backed
// handlers are injected via deps from the cmd composition root.
func Serve(addr string, deps Dependencies) error {
	mux := http.NewServeMux()
	RegisterHandlers(mux, deps)
	return http.ListenAndServe(addr, mux)
}

func (h *peopleHandler) listPeople(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	out, err := h.svc.ListPeople(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list people: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *peopleHandler) getPerson(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	out, err := h.svc.GetPerson(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorBody{Message: "person " + id + " not found"})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get person: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *peopleHandler) createPerson(w http.ResponseWriter, r *http.Request) {
	var in PersonInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "field 'name' is required"})
		return
	}
	out, err := h.svc.CreatePerson(r.Context(), in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create person: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, out)
}
