package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ── Shopping list service ─────────────────────────────────────────────────────

// ShoppingListService is the application surface for shopping lists.
type ShoppingListService interface {
	ListShoppingLists(ctx context.Context) ([]ShoppingListResponse, error)
	CreateShoppingList(ctx context.Context, in ShoppingListInput) (ShoppingListResponse, error)
	CreateFromChecklist(ctx context.Context, in ShoppingListFromChecklistInput) (ShoppingListFromChecklistResponse, error)
	GetShoppingList(ctx context.Context, listID string) (ShoppingListResponse, error)
	ArchiveShoppingList(ctx context.Context, listID string) error
	// ListResolvedItemsSince returns the list's items that were confidently
	// resolved and pushed to a retailer wishlist since the given time (all
	// resolved items when since is the zero time). This is the cheap read the
	// Apple Notes bridge polls to learn which checklist lines to check off.
	ListResolvedItemsSince(ctx context.Context, listID string, since time.Time) ([]ResolvedItemResponse, error)
}

// ResolvedItemResponse is the JSON view for GET /shopping-lists/{listId}/
// resolved-since. It carries the normalized note_match_key so the notes-bridge
// can match the item back to its checklist line without re-parsing the note.
type ResolvedItemResponse struct {
	ID           string    `json:"id"`
	Label        *string   `json:"label,omitempty"`
	NoteMatchKey *string   `json:"note_match_key,omitempty"`
	ResolvedAt   time.Time `json:"resolved_at"`
}

// ShoppingListResponse is the JSON view (openapi: components/schemas/ShoppingList).
type ShoppingListResponse struct {
	ID            string    `json:"id"`
	OwnerPersonID *string   `json:"owner_person_id,omitempty"`
	Name          string    `json:"name"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

// ShoppingListInput is the body for POST /shopping-lists
// (openapi: components/schemas/ShoppingListNew).
type ShoppingListInput struct {
	Name          string  `json:"name"`
	OwnerPersonID *string `json:"owner_person_id,omitempty"`
}

// ShoppingListFromChecklistInput is the body for POST /shopping-lists/from-checklist
// (openapi: components/schemas/ShoppingListFromChecklist). It is the ingestion target
// for the Mac-local Apple Notes reader: one named checklist becomes a new shopping
// list plus its line items in a single call.
type ShoppingListFromChecklistInput struct {
	Name  string             `json:"name"`
	Items []ChecklistItemInput `json:"items"`
}

// ChecklistItemInput is one line item in a from-checklist submission.
type ChecklistItemInput struct {
	Label    string  `json:"label"`
	Quantity float32 `json:"quantity"`
	Unit     string  `json:"unit"`
}

// ShoppingListFromChecklistResponse is the JSON view for POST /shopping-lists/from-checklist
// (openapi: components/schemas/ShoppingListFromChecklistResult). It embeds the created
// list and returns the created items so the caller can reference them without a second fetch.
type ShoppingListFromChecklistResponse struct {
	ShoppingListResponse
	Items []ShoppingListItemResponse `json:"items"`
}

type shoppingListListHandler struct{ svc ShoppingListService }
type shoppingListCreateHandler struct{ svc ShoppingListService }
type shoppingListFromChecklistHandler struct{ svc ShoppingListService }
type shoppingListGetHandler struct{ svc ShoppingListService }
type shoppingListArchiveHandler struct{ svc ShoppingListService }
type shoppingListResolvedSinceHandler struct{ svc ShoppingListService }

func (h *shoppingListListHandler) listShoppingLists(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListShoppingLists(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list shopping lists: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *shoppingListCreateHandler) createShoppingList(w http.ResponseWriter, r *http.Request) {
	var in ShoppingListInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "field 'name' is required"})
		return
	}
	out, err := h.svc.CreateShoppingList(r.Context(), in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create shopping list: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *shoppingListFromChecklistHandler) createFromChecklist(w http.ResponseWriter, r *http.Request) {
	var in ShoppingListFromChecklistInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "field 'name' is required"})
		return
	}
	if len(in.Items) == 0 {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "field 'items' must contain at least one item"})
		return
	}
	for i, it := range in.Items {
		if strings.TrimSpace(it.Label) == "" {
			writeJSON(w, http.StatusBadRequest, errorBody{Message: fmt.Sprintf("item %d: field 'label' is required", i)})
			return
		}
		if it.Quantity <= 0 {
			writeJSON(w, http.StatusBadRequest, errorBody{Message: fmt.Sprintf("item %d: quantity must be positive", i)})
			return
		}
		if it.Unit == "" {
			writeJSON(w, http.StatusBadRequest, errorBody{Message: fmt.Sprintf("item %d: field 'unit' is required", i)})
			return
		}
	}
	out, err := h.svc.CreateFromChecklist(r.Context(), in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create shopping list from checklist: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *shoppingListGetHandler) getShoppingList(w http.ResponseWriter, r *http.Request) {
	listID := r.PathValue("listId")
	out, err := h.svc.GetShoppingList(r.Context(), listID)
	if errors.Is(err, ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorBody{Message: "shopping list " + listID + " not found"})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get shopping list: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *shoppingListArchiveHandler) archiveShoppingList(w http.ResponseWriter, r *http.Request) {
	listID := r.PathValue("listId")
	if err := h.svc.ArchiveShoppingList(r.Context(), listID); err != nil {
		if errors.Is(err, ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errorBody{Message: "shopping list " + listID + " not found"})
			return
		}
		writeError(w, http.StatusInternalServerError, "archive shopping list: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ShoppingListResponse{})
}

// listResolvedItemsSince handles GET /shopping-lists/{listId}/resolved-since.
// The optional ?since=<RFC3339> bounds the result to items resolved at or after
// that time; omitting it returns every resolved item on the list.
func (h *shoppingListResolvedSinceHandler) listResolvedItemsSince(w http.ResponseWriter, r *http.Request) {
	listID := r.PathValue("listId")
	var since time.Time
	if v := r.URL.Query().Get("since"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid 'since' (want RFC3339)"})
			return
		}
		since = t
	}
	out, err := h.svc.ListResolvedItemsSince(r.Context(), listID, since)
	if errors.Is(err, ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorBody{Message: "shopping list " + listID + " not found"})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list resolved items: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// ── Shopping list items service ───────────────────────────────────────────────

// ShoppingListItemService is the application surface for shopping list items.
type ShoppingListItemService interface {
	ListShoppingListItems(ctx context.Context, listID string) ([]ShoppingListItemResponse, error)
	AddShoppingListItem(ctx context.Context, listID string, in ShoppingListItemInput) (ShoppingListItemResponse, error)
	ToggleShoppingListItem(ctx context.Context, listID, itemID string, checked bool) (ShoppingListItemResponse, error)
	DeleteShoppingListItem(ctx context.Context, listID, itemID string) error
}

// ShoppingListItemResponse is the JSON view (openapi: components/schemas/ShoppingListItem).
type ShoppingListItemResponse struct {
	ID                    string  `json:"id"`
	ShoppingListID        string  `json:"shopping_list_id"`
	ShoppingRequirementID *string `json:"shopping_requirement_id,omitempty"`
	IngredientID          *string `json:"ingredient_id,omitempty"`
	Label                 *string `json:"label,omitempty"`
	Quantity              float32 `json:"quantity"`
	Unit                  string  `json:"unit"`
	Checked               bool    `json:"checked"`
	AddedAt               time.Time `json:"added_at"`
}

// ShoppingListItemInput is the body for POST /shopping-lists/{listId}/items
// (openapi: components/schemas/ShoppingListItemNew).
type ShoppingListItemInput struct {
	ShoppingRequirementID *string `json:"shopping_requirement_id,omitempty"`
	IngredientID          *string `json:"ingredient_id,omitempty"`
	Label                 *string `json:"label,omitempty"`
	Quantity              float32 `json:"quantity"`
	Unit                  string  `json:"unit"`
}

type shoppingItemListHandler struct{ svc ShoppingListItemService }
type shoppingItemCreateHandler struct{ svc ShoppingListItemService }
type shoppingItemToggleHandler struct{ svc ShoppingListItemService }
type shoppingItemDeleteHandler struct{ svc ShoppingListItemService }

func (h *shoppingItemListHandler) listShoppingListItems(w http.ResponseWriter, r *http.Request) {
	listID := r.PathValue("listId")
	out, err := h.svc.ListShoppingListItems(r.Context(), listID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list shopping list items: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *shoppingItemCreateHandler) addShoppingListItem(w http.ResponseWriter, r *http.Request) {
	listID := r.PathValue("listId")
	var in ShoppingListItemInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	if in.Quantity <= 0 {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "quantity must be positive"})
		return
	}
	if in.Unit == "" {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "unit is required"})
		return
	}
	if in.ShoppingRequirementID == nil && in.IngredientID == nil && in.Label == nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "must provide shopping_requirement_id, ingredient_id, or label"})
		return
	}
	out, err := h.svc.AddShoppingListItem(r.Context(), listID, in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "add shopping list item: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *shoppingItemToggleHandler) toggleShoppingListItem(w http.ResponseWriter, r *http.Request) {
	listID := r.PathValue("listId")
	itemID := r.PathValue("itemId")
	var in struct {
		Checked bool `json:"checked"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	out, err := h.svc.ToggleShoppingListItem(r.Context(), listID, itemID, in.Checked)
	if errors.Is(err, ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorBody{Message: "shopping list item " + itemID + " not found"})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "toggle shopping list item: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *shoppingItemDeleteHandler) deleteShoppingListItem(w http.ResponseWriter, r *http.Request) {
	listID := r.PathValue("listId")
	itemID := r.PathValue("itemId")
	if err := h.svc.DeleteShoppingListItem(r.Context(), listID, itemID); err != nil {
		if errors.Is(err, ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errorBody{Message: "shopping list item " + itemID + " not found"})
			return
		}
		writeError(w, http.StatusInternalServerError, "delete shopping list item: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Push / cart service ───────────────────────────────────────────────────────

// ShoppingPushService is the application surface for pushing a list and creating carts.
type ShoppingPushService interface {
	PushShoppingList(ctx context.Context, listID string, retailer string) (RetailerListBindingResponse, error)
	ListShoppingCarts(ctx context.Context, listID string) ([]ShoppingCartResponse, error)
	ToCart(ctx context.Context, listID string, retailer string) (ShoppingCartResponse, error)
}

// RetailerListBindingResponse is the JSON view (openapi: components/schemas/RetailerListBinding).
type RetailerListBindingResponse struct {
	ShoppingListID string                             `json:"shopping_list_id"`
	Retailer       string                             `json:"retailer"`
	ExternalListID string                             `json:"external_list_id"`
	SyncDirection  string                             `json:"sync_direction"`
	LastPushedAt   *time.Time                         `json:"last_pushed_at,omitempty"`
	LastPushStatus *RetailerListBindingLastPushStatus `json:"last_push_status,omitempty"`
}

type RetailerListBindingLastPushStatus string

const (
	RetailerListBindingLastPushStatusSuccess RetailerListBindingLastPushStatus = "success"
	RetailerListBindingLastPushStatusError   RetailerListBindingLastPushStatus = "error"
)

// ShoppingCartResponse is the JSON view (openapi: components/schemas/ShoppingCart).
type ShoppingCartResponse struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Status    string    `json:"status"`
}

// ShoppingCartItemResponse is the JSON view (openapi: components/schemas/ShoppingCartItem).
type ShoppingCartItemResponse struct {
	ShoppingCartID     string   `json:"shopping_cart_id"`
	LineNo             int      `json:"line_no"`
	RetailerProductID  string   `json:"retailer_product_id"`
	Quantity           float32  `json:"quantity"`
	Unit               string   `json:"unit"`
	ResolvedPriceMinor *int64   `json:"resolved_price_minor,omitempty"`
	Currency           string   `json:"currency"`
}

type pushShoppingListHandler struct{ svc ShoppingPushService }
type listShoppingCartsHandler struct{ svc ShoppingPushService }
type toCartHandler struct{ svc ShoppingPushService }

func (h *pushShoppingListHandler) pushShoppingList(w http.ResponseWriter, r *http.Request) {
	listID := r.PathValue("listId")
	retailer := r.URL.Query().Get("retailer")
	if retailer == "" {
		retailer = "willys"
	}
	out, err := h.svc.PushShoppingList(r.Context(), listID, retailer)
	if errors.Is(err, ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorBody{Message: "shopping list " + listID + " not found"})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "push shopping list: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *listShoppingCartsHandler) listShoppingCarts(w http.ResponseWriter, r *http.Request) {
	listID := r.PathValue("listId")
	out, err := h.svc.ListShoppingCarts(r.Context(), listID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list shopping carts: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *toCartHandler) toCart(w http.ResponseWriter, r *http.Request) {
	listID := r.PathValue("listId")
	retailer := r.URL.Query().Get("retailer")
	if retailer == "" {
		retailer = "willys"
	}
	out, err := h.svc.ToCart(r.Context(), listID, retailer)
	if errors.Is(err, ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorBody{Message: "shopping list " + listID + " not found"})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "to cart: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

// ── Orders service ─────────────────────────────────────────────────────────────

// OrderService is the application surface for orders.
type OrderService interface {
	ListOrders(ctx context.Context, retailer *string, cartID *string) ([]OrderResponse, error)
	GetOrder(ctx context.Context, orderID string) (OrderViewResponse, error)
	ListOrderItems(ctx context.Context, orderID string) ([]OrderItemResponse, error)
}

// OrderResponse is the JSON view (openapi: components/schemas/Order).
type OrderResponse struct {
	ID               string    `json:"id"`
	ShoppingCartID   *string   `json:"shopping_cart_id,omitempty"`
	Retailer         string    `json:"retailer"`
	Source           string    `json:"source"`
	OrderedAt        time.Time `json:"ordered_at"`
	TotalPriceMinor  *int64    `json:"total_price_minor,omitempty"`
	Currency         string    `json:"currency"`
}

// OrderViewResponse is the JSON view (openapi: components/schemas/OrderView).
type OrderViewResponse struct {
	Order  OrderResponse       `json:"order"`
	Items  []OrderItemResponse `json:"items"`
}

// OrderItemResponse is the JSON view (openapi: components/schemas/OrderItem).
type OrderItemResponse struct {
	ID                   string   `json:"id"`
	OrderID              string   `json:"order_id"`
	RetailerProductID    string   `json:"retailer_product_id"`
	Quantity             float32  `json:"quantity"`
	UnitPrice            *float32 `json:"unit_price,omitempty"`
	TotalPriceMinor      *int64   `json:"total_price_minor,omitempty"`
	Currency             string   `json:"currency"`
	SubstitutedForItemID *string  `json:"substituted_for_item_id,omitempty"`
}

type listOrdersHandler struct{ svc OrderService }
type getOrderHandler struct{ svc OrderService }
type listOrderItemsHandler struct{ svc OrderService }

func (h *listOrdersHandler) listOrders(w http.ResponseWriter, r *http.Request) {
	var retailer *string
	if v := r.URL.Query().Get("retailer"); v != "" {
		retailer = &v
	}
	var cartID *string
	if v := r.URL.Query().Get("cartId"); v != "" {
		cartID = &v
	}
	out, err := h.svc.ListOrders(r.Context(), retailer, cartID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list orders: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *getOrderHandler) getOrder(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("orderId")
	out, err := h.svc.GetOrder(r.Context(), orderID)
	if errors.Is(err, ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorBody{Message: "order " + orderID + " not found"})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get order: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *listOrderItemsHandler) listOrderItems(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("orderId")
	out, err := h.svc.ListOrderItems(r.Context(), orderID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list order items: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}
