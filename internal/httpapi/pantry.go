package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// PantryLocation is the HTTP-side view of an inventory location.
type PantryLocation struct {
	ID               string    `json:"id"`
	HouseholdID      string    `json:"household_id"`
	Name             string    `json:"name"`
	LocationType     string    `json:"location_type"`
	ParentLocationID string    `json:"parent_location_id"`
	ArchivedAt       time.Time `json:"archived_at,omitempty"`
}

// PantryLot is the HTTP-side view of an inventory lot.
type PantryLot struct {
	ID           int64     `json:"id"`
	IngredientID string    `json:"ingredient_id"`
	ProductID    string    `json:"product_id"`
	LocationID   string    `json:"location_id"`
	Quantity     float64   `json:"quantity"`
	Unit         string    `json:"unit"`
	Confidence   string    `json:"confidence"`
	BestBefore   time.Time `json:"best_before,omitempty"`
	OpenedAt     time.Time `json:"opened_at,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// PantryLocationNew is the POST /pantry/locations request body.
type PantryLocationNew struct {
	HouseholdID      string `json:"household_id"`
	Name             string `json:"name"`
	LocationType     string `json:"location_type"`
	ParentLocationID string `json:"parent_location_id"`
}

// PantryPurchaseInput is the POST /pantry/lots/purchase request body.
type PantryPurchaseInput struct {
	IngredientID string  `json:"ingredient_id"`
	ProductID    string  `json:"product_id"`
	LocationID   string  `json:"location_id"`
	Quantity     float64 `json:"quantity"`
	Unit         string  `json:"unit"`
	BestBefore   string  `json:"best_before,omitempty"`
	Source       string  `json:"source"`
}

// PantryConsumeInput is the POST /pantry/lots/{id}/consume request body.
type PantryConsumeInput struct {
	Quantity  float64 `json:"quantity"`
	Estimated bool    `json:"estimated"`
	Source    string  `json:"source"`
}

// PantryService is the surface the /pantry handlers need.
type PantryService interface {
	ListLocations(ctx context.Context, householdID string) ([]PantryLocation, error)
	CreateLocation(ctx context.Context, in PantryLocationNew) (PantryLocation, error)
	ListLots(ctx context.Context, locationID string) ([]PantryLot, error)
	Purchase(ctx context.Context, in PantryPurchaseInput) (PantryLot, error)
	Consume(ctx context.Context, lotID int64, in PantryConsumeInput) error
}

// pantryHandler handles /pantry routes.
type pantryHandler struct {
	svc PantryService
}

func (h *pantryHandler) listLocations(w http.ResponseWriter, r *http.Request) {
	householdID := r.URL.Query().Get("householdId")
	out, err := h.svc.ListLocations(r.Context(), householdID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list locations: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *pantryHandler) createLocation(w http.ResponseWriter, r *http.Request) {
	var in PantryLocationNew
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	out, err := h.svc.CreateLocation(r.Context(), in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create location: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *pantryHandler) listLots(w http.ResponseWriter, r *http.Request) {
	locationID := r.PathValue("id")
	out, err := h.svc.ListLots(r.Context(), locationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list lots: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *pantryHandler) purchase(w http.ResponseWriter, r *http.Request) {
	var in PantryPurchaseInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	if in.Quantity <= 0 {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "field 'quantity' must be > 0"})
		return
	}
	out, err := h.svc.Purchase(r.Context(), in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "purchase: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *pantryHandler) consume(w http.ResponseWriter, r *http.Request) {
	lotIDStr := r.PathValue("id")
	lotID, err := strconv.ParseInt(lotIDStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid lot id: " + lotIDStr})
		return
	}
	var in PantryConsumeInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	if in.Quantity <= 0 {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "field 'quantity' must be > 0"})
		return
	}
	if err := h.svc.Consume(r.Context(), lotID, in); err != nil {
		writeError(w, http.StatusInternalServerError, "consume: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
