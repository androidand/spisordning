package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/androidand/spisordning/internal/dto"
	"github.com/androidand/spisordning/internal/service"
)

// grocyHandler handles /grocy routes.
type grocyHandler struct {
	svc dto.GrocyService
}

func (h *grocyHandler) status(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Status(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "grocy status: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *grocyHandler) listProducts(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListProducts(r.Context())
	if err != nil {
		h.writeGrocyError(w, "list grocy products", err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *grocyHandler) listStock(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListStock(r.Context())
	if err != nil {
		h.writeGrocyError(w, "list grocy stock", err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *grocyHandler) listShoppingList(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListShoppingList(r.Context())
	if err != nil {
		h.writeGrocyError(w, "list grocy shopping list", err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *grocyHandler) addStock(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ProductID  int     `json:"product_id"`
		Amount     float64 `json:"amount"`
		BestBefore string  `json:"best_before,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	if err := h.svc.AddStock(r.Context(), in.ProductID, in.Amount, in.BestBefore); err != nil {
		h.writeGrocyError(w, "add grocy stock", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *grocyHandler) consumeStock(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ProductID int     `json:"product_id"`
		Amount    float64 `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	if err := h.svc.ConsumeStock(r.Context(), in.ProductID, in.Amount); err != nil {
		h.writeGrocyError(w, "consume grocy stock", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *grocyHandler) addShoppingItem(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ProductID int     `json:"product_id"`
		Note      string  `json:"note"`
		Amount    float64 `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	if err := h.svc.AddShoppingItem(r.Context(), in.ProductID, in.Note, in.Amount); err != nil {
		h.writeGrocyError(w, "add grocy shopping item", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// writeGrocyError maps service errors to HTTP: not-configured → 503, other → 502.
func (h *grocyHandler) writeGrocyError(w http.ResponseWriter, op string, err error) {
	if errors.Is(err, service.ErrGrocyNotConfigured) {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Message: "grocy is not configured"})
		return
	}
	writeError(w, http.StatusBadGateway, op+": "+err.Error())
}
