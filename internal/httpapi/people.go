package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/androidand/spisordning/internal/dto"
)

// ErrNotFound signals a resource miss so handlers can map it to 404 without
// httpapi knowing about pgx sentinel errors.
var ErrNotFound = errors.New("not found")

// Dependencies are the services httpapi's handlers call. Add new services here as
// routes are implemented from api/openapi.yaml.
type Dependencies struct {
	People      dto.PersonService
	Preferences dto.PreferencesService
	Recipes     dto.RecipesService
	Meals       dto.MealsService
	Planning    dto.PlanningService
	Pantry      dto.PantryService
	Ingredients dto.IngredientsService
	Stores      dto.StoresService
}

type peopleHandler struct {
	svc dto.PersonService
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
		mux.HandleFunc("GET /meals", h.listMeals)
		mux.HandleFunc("GET /meals/{id}", h.getMeal)
		mux.HandleFunc("POST /meals", h.createMealEvent)
	}
	if deps.Planning != nil {
		h := plansHandler{svc: deps.Planning}
		mux.HandleFunc("GET /plans", h.listPlans)
		mux.HandleFunc("POST /plans", h.createPlan)
		mux.HandleFunc("GET /plans/{id}", h.getPlan)
		mux.HandleFunc("PATCH /plans/{id}", h.updatePlan)
		mux.HandleFunc("POST /plans/{id}/decisions", h.setDecisions)
		mux.HandleFunc("GET /plans/{id}/shopping-requirements", h.listShoppingRequirements)
	}
	if deps.Pantry != nil {
		h := pantryHandler{svc: deps.Pantry}
		mux.HandleFunc("GET /pantry/locations", h.listLocations)
		mux.HandleFunc("POST /pantry/locations", h.createLocation)
		mux.HandleFunc("GET /pantry/locations/{id}/lots", h.listLots)
		mux.HandleFunc("POST /pantry/lots/purchase", h.purchase)
		mux.HandleFunc("POST /pantry/lots/{id}/consume", h.consume)
	}
	if deps.Ingredients != nil {
		h := ingredientsHandler{svc: deps.Ingredients}
		mux.HandleFunc("GET /ingredients/search", h.searchFood)
		mux.HandleFunc("GET /ingredients/nutrition/{nummer}", h.lookupNutrition)
		mux.HandleFunc("GET /ingredients/dabas/search", h.searchDabas)
		mux.HandleFunc("GET /ingredients/matpriskollen/search", h.searchMatpriskollen)
		mux.HandleFunc("PATCH /ingredient-mappings/{mealieFoodId}", h.resolveMapping)
	}
	if deps.Stores != nil {
		h := storesHandler{svc: deps.Stores}
		mux.HandleFunc("GET /products/search", h.searchProducts)
		mux.HandleFunc("GET /products/by-gtin", h.searchByGTIN)
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
	var in dto.PersonInput
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
