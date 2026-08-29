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
	Pantry      dto.PantryService
	Ingredients dto.IngredientsService
	Stores      dto.StoresService
	// Tonight, Reactions, Plans, EffortProfiles, PlanningConstraints,
	// ShoppingLists, ShoppingListItems, ShoppingPush, and Orders are defined
	// locally in this package (not internal/dto) — they predate the dto
	// extraction and were added straight against httpapi's own response DTOs.
	// Plans supersedes the old dto.PlanningService-backed /plans routes (same
	// paths plus POST /plans/run and GET /plans/{id}/candidates).
	Tonight             TonightService
	Reactions           ReactionService
	Plans               PlanService
	EffortProfiles      EffortProfileService
	PlanningConstraints PlanningConstraintService
	ShoppingLists       ShoppingListService
	ShoppingListItems   ShoppingListItemService
	ShoppingPush        ShoppingPushService
	Orders              OrderService
	// PriceComparison compares prices across retailers (POST /compare). It
	// resolves each requirement against every retailer and reports the cheapest;
	// a stale/unavailable retailer degrades to available:false per item.
	PriceComparison PriceComparisonService
	// RecipeFamily is the git-like recipe hierarchy (family -> variant ->
	// revision). Backs the /recipe-families routes.
	RecipeFamily dto.RecipeFamilyService
	// Favorites is the explicit recipe favorite + rating surface. Backs the
	// /recipes/{id}/favorites and /recipes/{id}/rating routes.
	Favorites dto.FavoritesService
	// PriceIntelligence reads current prices per retailer product across
	// stores, with the cheapest store computed. Backs the /prices routes.
	PriceIntelligence dto.PriceIntelligenceService
	// Dashboard aggregates tonight's meal, a pantry summary, and expiring items
	// into a single read model. Backs the /widgets routes.
	Dashboard dto.DashboardService
	// IngredientAlias manages household nicknames → canonical ingredient
	// (configurable nickname matching). Backs the /ingredient-aliases routes.
	IngredientAlias dto.IngredientAliasService
	// Inspiration ranks recipes by pantry coverage ("what can I make from my
	// pantry"). Backs the /inspiration routes.
	Inspiration dto.InspirationService
	// Grocy bridges a running Grocy instance (products, stock, shopping list).
	// Backs the /grocy routes; degrades to 503 when not configured.
	Grocy dto.GrocyService
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
		mux.HandleFunc("PATCH /people/{id}", h.updatePerson)
	}
	if deps.Preferences != nil {
		h := prefsHandler{svc: deps.Preferences}
		mux.HandleFunc("GET /preferences", h.listPreferences)
		mux.HandleFunc("POST /preferences", h.setPreference)
	}
	if deps.Recipes != nil {
		h := recipesHandler{svc: deps.Recipes}
		mux.HandleFunc("GET /recipes", h.listRecipes)
		mux.HandleFunc("GET /recipes/{id}", h.getRecipe)
	}
	if deps.Meals != nil {
		h := mealsHandler{svc: deps.Meals}
		mux.HandleFunc("GET /meals", h.listMeals)
		mux.HandleFunc("GET /meals/{id}", h.getMeal)
		mux.HandleFunc("POST /meals", h.createMealEvent)
	}
	if deps.Pantry != nil {
		h := pantryHandler{svc: deps.Pantry}
		mux.HandleFunc("GET /pantry/locations", h.listLocations)
		mux.HandleFunc("POST /pantry/locations", h.createLocation)
		mux.HandleFunc("GET /pantry/locations/{id}/lots", h.listLots)
		mux.HandleFunc("POST /pantry/lots/purchase", h.purchase)
		mux.HandleFunc("POST /pantry/lots/{id}/consume", h.consume)
		mux.HandleFunc("GET /pantry/expiring", h.listExpiring)
	}
	if deps.Ingredients != nil {
		h := ingredientsHandler{svc: deps.Ingredients}
		mux.HandleFunc("GET /ingredients/search", h.searchFood)
		mux.HandleFunc("GET /ingredients/by-id/{id}/nutrition", h.nutritionByID)
		mux.HandleFunc("GET /ingredients/nutrition/{nummer}", h.lookupNutrition)
		mux.HandleFunc("GET /ingredients/dabas/search", h.searchDabas)
		mux.HandleFunc("GET /ingredients/matpriskollen/search", h.searchMatpriskollen)
		mux.HandleFunc("PATCH /ingredient-mappings/{mealieFoodId}", h.resolveMapping)
	}
	if deps.Stores != nil {
		h := storesHandler{svc: deps.Stores}
		mux.HandleFunc("GET /stores", h.listStores)
		mux.HandleFunc("GET /stores/{id}/offers", h.listStoreOffers)
		mux.HandleFunc("GET /products/search", h.searchProducts)
		mux.HandleFunc("GET /products/by-gtin", h.searchByGTIN)
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
		hp := planProgressHandler{svc: deps.Plans}
		mux.HandleFunc("POST /plans/run/stream", hp.runPlanStream)
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
	if deps.ShoppingLists != nil {
		h := shoppingListListHandler{svc: deps.ShoppingLists}
		mux.HandleFunc("GET /shopping-lists", h.listShoppingLists)
		h2 := shoppingListCreateHandler{svc: deps.ShoppingLists}
		mux.HandleFunc("POST /shopping-lists", h2.createShoppingList)
		h3 := shoppingListFromChecklistHandler{svc: deps.ShoppingLists}
		mux.HandleFunc("POST /shopping-lists/from-checklist", h3.createFromChecklist)
		h4 := shoppingListGetHandler{svc: deps.ShoppingLists}
		mux.HandleFunc("GET /shopping-lists/{listId}", h4.getShoppingList)
		h5 := shoppingListArchiveHandler{svc: deps.ShoppingLists}
		mux.HandleFunc("DELETE /shopping-lists/{listId}", h5.archiveShoppingList)
	}
	if deps.ShoppingListItems != nil {
		h := shoppingItemListHandler{svc: deps.ShoppingListItems}
		mux.HandleFunc("GET /shopping-lists/{listId}/items", h.listShoppingListItems)
		h2 := shoppingItemCreateHandler{svc: deps.ShoppingListItems}
		mux.HandleFunc("POST /shopping-lists/{listId}/items", h2.addShoppingListItem)
		h3 := shoppingItemToggleHandler{svc: deps.ShoppingListItems}
		mux.HandleFunc("PATCH /shopping-lists/{listId}/items/{itemId}", h3.toggleShoppingListItem)
		h4 := shoppingItemDeleteHandler{svc: deps.ShoppingListItems}
		mux.HandleFunc("DELETE /shopping-lists/{listId}/items/{itemId}", h4.deleteShoppingListItem)
	}
	if deps.ShoppingPush != nil {
		h := pushShoppingListHandler{svc: deps.ShoppingPush}
		mux.HandleFunc("POST /shopping-lists/{listId}/push", h.pushShoppingList)
		h2 := listShoppingCartsHandler{svc: deps.ShoppingPush}
		mux.HandleFunc("GET /shopping-lists/{listId}/carts", h2.listShoppingCarts)
		h3 := toCartHandler{svc: deps.ShoppingPush}
		mux.HandleFunc("POST /shopping-lists/{listId}/push/to-cart", h3.toCart)
	}
	if deps.Orders != nil {
		h := listOrdersHandler{svc: deps.Orders}
		mux.HandleFunc("GET /orders", h.listOrders)
		h2 := getOrderHandler{svc: deps.Orders}
		mux.HandleFunc("GET /orders/{orderId}", h2.getOrder)
		h3 := listOrderItemsHandler{svc: deps.Orders}
		mux.HandleFunc("GET /orders/{orderId}/items", h3.listOrderItems)
	}
	if deps.PriceComparison != nil {
		h := compareHandler{svc: deps.PriceComparison}
		mux.HandleFunc("POST /compare", h.compare)
	}
	if deps.Favorites != nil {
		h := favoritesHandler{svc: deps.Favorites}
		mux.HandleFunc("GET /recipes/{id}/favorites", h.listFavorites)
		mux.HandleFunc("POST /recipes/{id}/favorites", h.setFavorite)
		mux.HandleFunc("DELETE /recipes/{id}/favorites", h.unsetFavorite)
		mux.HandleFunc("GET /recipes/{id}/rating", h.getRating)
	}
	if deps.PriceIntelligence != nil {
		h := pricesHandler{svc: deps.PriceIntelligence}
		mux.HandleFunc("GET /prices", h.listProductPrices)
	}
	if deps.Dashboard != nil {
		h := dashboardHandler{svc: deps.Dashboard}
		mux.HandleFunc("GET /widgets/dashboard", h.getDashboard)
	}
	if deps.IngredientAlias != nil {
		h := ingredientAliasHandler{svc: deps.IngredientAlias}
		mux.HandleFunc("GET /ingredient-aliases", h.listAliases)
		mux.HandleFunc("POST /ingredient-aliases", h.createAlias)
		mux.HandleFunc("DELETE /ingredient-aliases/{alias}", h.deleteAlias)
		mux.HandleFunc("GET /ingredient-aliases/resolve/{alias}", h.resolveAlias)
	}
	if deps.Inspiration != nil {
		h := inspirationHandler{svc: deps.Inspiration}
		mux.HandleFunc("GET /inspiration", h.suggest)
	}
	if deps.Grocy != nil {
		h := grocyHandler{svc: deps.Grocy}
		mux.HandleFunc("GET /grocy/status", h.status)
		mux.HandleFunc("GET /grocy/products", h.listProducts)
		mux.HandleFunc("GET /grocy/stock", h.listStock)
		mux.HandleFunc("GET /grocy/shopping-list", h.listShoppingList)
		mux.HandleFunc("POST /grocy/stock/add", h.addStock)
		mux.HandleFunc("POST /grocy/stock/consume", h.consumeStock)
		mux.HandleFunc("POST /grocy/shopping-list/items", h.addShoppingItem)
	}
	if deps.RecipeFamily != nil {
		h := recipeFamilyHandler{svc: deps.RecipeFamily}
		mux.HandleFunc("GET /recipe-families", h.listFamilies)
		mux.HandleFunc("POST /recipe-families", h.createFamily)
		mux.HandleFunc("GET /recipe-families/{id}", h.getFamily)
		mux.HandleFunc("GET /recipe-families/{id}/variants", h.listVariants)
		mux.HandleFunc("POST /recipe-families/{id}/variants", h.createVariant)
		mux.HandleFunc("GET /recipe-families/{id}/variants/{variantId}/revisions", h.listRevisions)
		mux.HandleFunc("POST /recipe-families/{id}/variants/{variantId}/revisions", h.createRevision)
		mux.HandleFunc("GET /recipe-families/{id}/variants/{variantId}/revisions/{revisionId}", h.getRevision)
		mux.HandleFunc("POST /recipe-families/{id}/variants/{variantId}/default", h.setDefaultVariant)
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

func (h *peopleHandler) updatePerson(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in dto.PersonUpdate
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "field 'name' is required"})
		return
	}
	out, err := h.svc.UpdatePerson(r.Context(), id, in)
	if errors.Is(err, ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorBody{Message: "person " + id + " not found"})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "update person: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}
