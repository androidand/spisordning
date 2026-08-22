package httpapi

import (
	"context"
	"encoding/json"
	"errors"
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
	People                    PersonService
	Preferences               PreferencesService
	Recipes                   RecipesService
	Meals                     MealsService
	Tonight                   TonightService
	Reactions                 ReactionService
	Plans                     PlanService
	EffortProfiles            EffortProfileService
	PlanningConstraints       PlanningConstraintService
}

type peopleHandler struct {
	svc PersonService
}

// RegisterHandlers wires every implemented route onto mux. /health is always
// available; resource routes are registered only when their service is provided,
// so a partial build (no DB) still serves the probe.
func RegisterHandlers(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("/health", healthHandler)

	if deps.People != nil {
		h := peopleHandler{svc: deps.People}
		mux.HandleFunc("GET /people", h.listPeople)
		mux.HandleFunc("POST /people", h.createPerson)
		mux.HandleFunc("GET /people/{id}", h.getPerson)
	}
	if deps.Preferences != nil {
		h := prefsHandler{svc: deps.Preferences}
		mux.HandleFunc("GET /preferences", h.listPreferences)
	}
	if deps.Recipes != nil {
		h := recipesHandler{svc: deps.Recipes}
		mux.HandleFunc("GET /recipes", h.listRecipes)
	}
	if deps.Meals != nil {
		h := mealsHandler{svc: deps.Meals}
		mux.HandleFunc("POST /meals", h.createMealEvent)
	}
	if deps.Tonight != nil {
		h := tonightHandler{svc: deps.Tonight}
		mux.HandleFunc("GET /tonight", h.getTonight)
	}
	if deps.Reactions != nil {
		h := reactionsHandler{svc: deps.Reactions}
		mux.HandleFunc("POST /reactions", h.createReaction)
	}
	if deps.Plans != nil {
		h := planRunHandler{svc: deps.Plans}
		mux.HandleFunc("POST /plans/run", h.runPlan)
		h2 := planListHandler{svc: deps.Plans}
		mux.HandleFunc("GET /plans", h2.listPlans)
		h3 := planCreateHandler{svc: deps.Plans}
		mux.HandleFunc("POST /plans", h3.createPlan)
		h4 := planGetHandler{svc: deps.Plans}
		mux.HandleFunc("GET /plans/{planId}", h4.getPlan)
		h5 := planUpdateHandler{svc: deps.Plans}
		mux.HandleFunc("PATCH /plans/{planId}", h5.updatePlan)
		h6 := planDecisionsHandler{svc: deps.Plans}
		mux.HandleFunc("POST /plans/{planId}/decisions", h6.setDecisions)
		h7 := planCandidatesHandler{svc: deps.Plans}
		mux.HandleFunc("GET /plans/{planId}/candidates", h7.listCandidates)
		h8 := planShoppingRequirementsHandler{svc: deps.Plans}
		mux.HandleFunc("GET /plans/{planId}/shopping-requirements", h8.listShoppingRequirements)
	}
	if deps.EffortProfiles != nil {
		h := effortProfileListHandler{svc: deps.EffortProfiles}
		mux.HandleFunc("GET /effort-profiles", h.listEffortProfiles)
		h2 := effortProfileUpsertHandler{svc: deps.EffortProfiles}
		mux.HandleFunc("POST /effort-profiles", h2.upsertEffortProfile)
	}
	if deps.PlanningConstraints != nil {
		h := planningConstraintListHandler{svc: deps.PlanningConstraints}
		mux.HandleFunc("GET /constraints", h.listConstraints)
		h2 := planningConstraintCreateHandler{svc: deps.PlanningConstraints}
		mux.HandleFunc("POST /constraints", h2.createConstraint)
	}
}

// Serve starts the HTTP server on addr (e.g. ":8080"). Handlers are sourced from
// api/openapi.yaml; /health, /people, /preferences, /recipes and /meals are wired
// today. Persistence-backed handlers are injected via deps from the cmd root.
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
