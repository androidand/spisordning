// Package icaretailer is the Food Brain's client for the ica-adapter HTTP
// service. The adapter owns all ICA session state (OAuth2 tokens, store pinning);
// this package only exchanges domain data. Its terminal output is a durable
// wishlist id — never a cart, never a payment.
package icaretailer

import (
	"context"
	"time"

	"github.com/androidand/spisordning/internal/httpclient"
)

// Client talks to a running ica-adapter instance.
type Client struct {
	http *httpclient.Client
}

// New returns a Client for the adapter at baseURL (e.g. "http://localhost:8403").
func New(baseURL string) *Client {
	return &Client{http: httpclient.New(baseURL, "ica-adapter", 60*time.Second)}
}

// Resolution mirrors the adapter's resolution JSON shape.
type Resolution struct {
	MatchType         string  `json:"matchType"` // "pinned" | "pinned-backup" | "search" | "barcode" | "none"
	ProductCode       string  `json:"productCode"`
	ProductName       string  `json:"productName"`
	Packages          int     `json:"packages"`
	Confidence        float64 `json:"confidence"`
	NeedsReview       bool    `json:"needsReview"`
	QuantityUncertain bool    `json:"quantityUncertain"`
	Retailer          string  `json:"retailer"` // "ica"
	// PriceValue is the numeric SEK price of the resolved product (per package),
	// when the adapter knows it. Nil when the product has no price. Field names
	// and types match internal/retailer.Resolution (Willys) for cross-retailer
	// price comparison.
	PriceValue *float64 `json:"priceValue"`
	// Price is the formatted display price (e.g. "29.90 kr"), when available.
	Price *string `json:"price"`
}

// ResolveRequest is the body for POST /resolve.
type ResolveRequest struct {
	Terms []string `json:"terms"`
}

// Resolve sends search terms to the adapter and returns product resolutions.
func (c *Client) Resolve(ctx context.Context, terms []string) ([]Resolution, error) {
	payload := ResolveRequest{Terms: terms}
	var out struct {
		Resolutions []Resolution `json:"resolutions"`
	}
	if err := c.http.PostJSON(ctx, "/resolve", payload, &out, nil); err != nil {
		return nil, err
	}
	return out.Resolutions, nil
}

// BarcodeLookup sends a barcode to the adapter and returns the product info.
func (c *Client) BarcodeLookup(ctx context.Context, barcode string) (*Resolution, error) {
	var out Resolution
	if err := c.http.GetJSON(ctx, "/barcode/"+barcode, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// ShoppingListItem is one line of the wishlist to create or sync.
type ShoppingListItem struct {
	Label       string  `json:"label"`
	Quantity    float64 `json:"quantity"`
	Unit        string  `json:"unit"`
	ProductCode string  `json:"productCode,omitempty"`
}

// CreateShoppingListRequest is the body for POST /shopping-lists.
type CreateShoppingListRequest struct {
	Name  string             `json:"name"`
	Items []ShoppingListItem `json:"items"`
}

// CreatedList identifies the durable wishlist the adapter created on ICA.
type CreatedList struct {
	ExternalListID string `json:"externalListId"`
	Name           string `json:"name"`
}

// CreateShoppingList creates a new ICA shopping list via the adapter.
func (c *Client) CreateShoppingList(ctx context.Context, name string, items []ShoppingListItem) (*CreatedList, error) {
	payload := CreateShoppingListRequest{Name: name, Items: items}
	var out CreatedList
	if err := c.http.PostJSON(ctx, "/shopping-lists", payload, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// SyncShoppingListRequest is the body for POST /shopping-lists/:id/sync.
type SyncShoppingListRequest struct {
	ExternalListID string             `json:"externalListId"`
	Name           string             `json:"name"`
	Items          []ShoppingListItem `json:"items"`
}

// SyncShoppingList syncs (MERGE) a shopping list on ICA via the adapter.
func (c *Client) SyncShoppingList(ctx context.Context, externalListID string, name string, items []ShoppingListItem) (*CreatedList, error) {
	payload := SyncShoppingListRequest{ExternalListID: externalListID, Name: name, Items: items}
	var out CreatedList
	if err := c.http.PostJSON(ctx, "/shopping-lists/"+externalListID+"/sync", payload, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// BonusBalance is the body returned by GET /bonus.
type BonusBalance struct {
	Balance         float64  `json:"balance"`
	Vouchers        []string `json:"vouchers"`
	DiscountSummary string   `json:"discountSummary,omitempty"`
}

// GetBonusBalance returns the current ICA bonus balance via the adapter.
func (c *Client) GetBonusBalance(ctx context.Context) (*BonusBalance, error) {
	var out BonusBalance
	if err := c.http.GetJSON(ctx, "/bonus", &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// ProductSearchRequest is the body for POST /search.
type ProductSearchRequest struct {
	Query string `json:"query"`
}

// ProductHit is a single search result.
type ProductHit struct {
	ProductCode string  `json:"productCode"`
	ProductName string  `json:"productName"`
	Price       float64 `json:"price,omitempty"`
	Available   bool    `json:"available"`
}

// ProductSearchResponse is the body returned by POST /search.
type ProductSearchResponse struct {
	Hits []ProductHit `json:"hits"`
}

// SearchProducts searches for products via the adapter (anonymous surface).
func (c *Client) SearchProducts(ctx context.Context, query string) (*ProductSearchResponse, error) {
	payload := ProductSearchRequest{Query: query}
	var out ProductSearchResponse
	if err := c.http.PostJSON(ctx, "/search", payload, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// ProductDetailRequest is the path param for GET /products/:code.
type ProductDetail struct {
	ProductCode string  `json:"productCode"`
	ProductName string  `json:"productName"`
	Price       float64 `json:"price,omitempty"`
	Available   bool    `json:"available"`
}

// GetProduct returns product detail by barcode/EAN via the adapter.
func (c *Client) GetProduct(ctx context.Context, code string) (*ProductDetail, error) {
	var out ProductDetail
	if err := c.http.GetJSON(ctx, "/products/"+code, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}
