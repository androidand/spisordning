// Package grocy is the Food Brain's client for a running Grocy instance's
// REST API. Grocy is a self-hosted household inventory + recipe app; this
// client reads its stock and shopping list so Spisordning can mirror them into
// its own pantry, and writes stock changes back. Auth is the GROCY-API-KEY
// header (Grocy's documented API-key mechanism).
package grocy

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/androidand/spisordning/internal/httpclient"
)

// Client talks to a running Grocy instance.
type Client struct {
	http *httpclient.Client
	key  string
}

// New returns a Client for the Grocy instance at baseURL (e.g.
// "http://localhost:8081"). apiKey is sent as the GROCY-API-KEY header on every
// request; an empty key is allowed for instances with auth disabled.
func New(baseURL, apiKey string) *Client {
	return &Client{
		http: httpclient.New(baseURL, "grocy", 30*time.Second),
		key:  apiKey,
	}
}

// authHeaders returns the per-request header setter that attaches the API key.
func (c *Client) authHeaders(req *http.Request) {
	if c.key != "" {
		req.Header.Set("GROCY-API-KEY", c.key)
	}
}

// Product is a Grocy product (a sellable/storable item).
type Product struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Barcode     string  `json:"barcode"`
	LocationID  int     `json:"location_id"`
	QuIDStock   int     `json:"qu_id_stock"`
	QuIDPurchase int    `json:"qu_id_purchase"`
	MinStock    float64 `json:"min_stock_amount"`
}

// ProductList is the body returned by GET /api/objects/products.
type ProductList struct {
	Objects []Product `json:"objects"`
}

// ListProducts returns every Grocy product.
func (c *Client) ListProducts(ctx context.Context) ([]Product, error) {
	var out ProductList
	if err := c.http.GetJSON(ctx, "/api/objects/products", &out, c.authHeaders); err != nil {
		return nil, err
	}
	return out.Objects, nil
}

// StockEntry is one row of Grocy's stock ledger (a lot with a best-before).
type StockEntry struct {
	ID          int     `json:"id"`
	ProductID   int     `json:"product_id"`
	Amount      float64 `json:"amount"`
	QuID        int     `json:"qu_id"`
	LocationID  int     `json:"location_id"`
	BestBefore  string  `json:"best_before"` // "" when unset
	PurchasedAt string  `json:"purchased_date"`
}

// StockList is the body returned by GET /api/objects/stock.
type StockList struct {
	Objects []StockEntry `json:"objects"`
}

// ListStock returns every Grocy stock entry (all lots, including zero-amount).
func (c *Client) ListStock(ctx context.Context) ([]StockEntry, error) {
	var out StockList
	if err := c.http.GetJSON(ctx, "/api/objects/stock", &out, c.authHeaders); err != nil {
		return nil, err
	}
	return out.Objects, nil
}

// ShoppingItem is one line of a Grocy shopping list. product_id is 0 for
// free-text items (Grocy allows product-less lines).
type ShoppingItem struct {
	ID        int     `json:"id"`
	ProductID int     `json:"product_id"`
	Note      string  `json:"note"`
	Amount    float64 `json:"amount"`
	QuID      int     `json:"qu_id"`
	Done      bool    `json:"done"`
	ListID    int     `json:"shopping_list_id"`
}

// ShoppingList is the body returned by GET /api/objects/shopping_list.
type ShoppingList struct {
	Objects []ShoppingItem `json:"objects"`
}

// ListShoppingList returns every line of Grocy's shopping list.
func (c *Client) ListShoppingList(ctx context.Context) ([]ShoppingItem, error) {
	var out ShoppingList
	if err := c.http.GetJSON(ctx, "/api/objects/shopping_list", &out, c.authHeaders); err != nil {
		return nil, err
	}
	return out.Objects, nil
}

// AddStockRequest is the body for POST /api/stock/products/{id}/add.
type AddStockRequest struct {
	Amount      float64 `json:"amount"`
	LocationID  int     `json:"location_id,omitempty"`
	BestBefore  string  `json:"best_before_date,omitempty"`
}

// AddStock adds amount (in the product's stock unit) to a product's stock.
func (c *Client) AddStock(ctx context.Context, productID int, req AddStockRequest) error {
	path := "/api/stock/products/" + strconv.Itoa(productID) + "/add"
	var out struct {
		Success bool `json:"success"`
	}
	if err := c.http.PostJSON(ctx, path, req, &out, c.authHeaders); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("grocy: add stock for product %d reported success=false", productID)
	}
	return nil
}

// ConsumeStockRequest is the body for POST /api/stock/products/{id}/consume.
type ConsumeStockRequest struct {
	Amount  float64 `json:"amount"`
	Spoiled bool    `json:"spoiled,omitempty"`
}

// ConsumeStock consumes amount (in the product's stock unit) from a product.
func (c *Client) ConsumeStock(ctx context.Context, productID int, req ConsumeStockRequest) error {
	path := "/api/stock/products/" + strconv.Itoa(productID) + "/consume"
	var out struct {
		Success bool `json:"success"`
	}
	if err := c.http.PostJSON(ctx, path, req, &out, c.authHeaders); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("grocy: consume stock for product %d reported success=false", productID)
	}
	return nil
}

// AddShoppingItemRequest is the body for POST /api/stock/shoppinglist/add-product.
type AddShoppingItemRequest struct {
	ProductID int     `json:"product_id,omitempty"`
	Note      string  `json:"note,omitempty"`
	Amount    float64 `json:"product_amount"`
	QuID      int     `json:"qu_id,omitempty"`
	ListID    int     `json:"list_id,omitempty"`
}

// AddShoppingItem adds a line to Grocy's shopping list. ProductID 0 + Note
// creates a free-text item.
func (c *Client) AddShoppingItem(ctx context.Context, req AddShoppingItemRequest) error {
	var out struct {
		Success bool `json:"success"`
	}
	if err := c.http.PostJSON(ctx, "/api/stock/shoppinglist/add-product", req, &out, c.authHeaders); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("grocy: add shopping item reported success=false")
	}
	return nil
}

// RemoveShoppingItemRequest is the body for POST /api/stock/shoppinglist/remove-product.
type RemoveShoppingItemRequest struct {
	ID int `json:"id"`
}

// RemoveShoppingItem removes a line from Grocy's shopping list by id.
func (c *Client) RemoveShoppingItem(ctx context.Context, id int) error {
	var out struct {
		Success bool `json:"success"`
	}
	if err := c.http.PostJSON(ctx, "/api/stock/shoppinglist/remove-product", RemoveShoppingItemRequest{ID: id}, &out, c.authHeaders); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("grocy: remove shopping item %d reported success=false", id)
	}
	return nil
}

// Ping checks that the instance is reachable and (when a key is set) that the
// key is accepted. It hits the lightweight /api/system/info endpoint.
func (c *Client) Ping(ctx context.Context) error {
	var out struct {
		Version string `json:"version"`
	}
	return c.http.GetJSON(ctx, "/api/system/info", &out, c.authHeaders)
}
