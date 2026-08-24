// Package retailer is the Food Brain's client for retailer-adapter HTTP
// services (willys-adapter, ica-adapter). The adapters own all retailer session
// state (login, cookies, CSRF, store pinning); this package only exchanges
// domain data. Their terminal output is a durable wishlist id — never a cart,
// never a payment.
package retailer

import (
	"context"
	"fmt"
	"time"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/androidand/spisordning/internal/httpclient"
)

// RetailerKind identifies which retailer backend a Client talks to. Used by
// the plan command to dispatch between willys-adapter and ica-adapter via the
// same ResolveRequirements / CreateShoppingList interface.
type RetailerKind string

const (
	RetailerWillys RetailerKind = "willys"
	RetailerICA    RetailerKind = "ica"
)

// Client talks to a running retailer-adapter instance.
type Client struct {
	http *httpclient.Client
}

// New returns a Client for the willys-adapter at baseURL
// (e.g. "http://localhost:8402").
func New(baseURL string) *Client {
	return &Client{http: httpclient.New(baseURL, "adapter", 60*time.Second)}
}

// NewICA returns a Client for the ica-adapter at baseURL
// (e.g. "http://localhost:8403"). The ICA adapter shares the same HTTP shape
// as willys-adapter (same /resolve, /shopping-lists, /pins, /review/queue
// routes) but adds ICA-specific endpoints (/barcode, /bonus).
func NewICA(baseURL string) *Client {
	return &Client{http: httpclient.New(baseURL, "ica-adapter", 60*time.Second)}
}

// NewFromKind returns a Client for the given retailerKind at the corresponding
// baseURL. kind=WILLYS uses New (ADAPTER_URL); kind=ICA uses NewICA
// (ICA_ADAPTER_URL). The returned client's error prefix ("adapter" or
// "ica-adapter") matches the underlying HTTP client so failures stay
// attributable. Returns an error when kind is unknown.
func NewFromKind(kind RetailerKind, willysURL, icaURL string) (*Client, error) {
	switch kind {
	case RetailerWillys:
		return New(willysURL), nil
	case RetailerICA:
		return NewICA(icaURL), nil
	default:
		return nil, fmt.Errorf("retailer: unknown kind %q (want %q or %q)", kind, RetailerWillys, RetailerICA)
	}
}

// requirementPayload mirrors the adapter's Requirement JSON shape.
type requirementPayload struct {
	IngredientID    string   `json:"ingredientId"`
	SearchTerm      string   `json:"searchTerm,omitempty"`
	Quantity        float64  `json:"quantity"`
	Unit            string   `json:"unit"`
	AcceptableForms []string `json:"acceptableForms,omitempty"`
	PreferredForm   string   `json:"preferredForm,omitempty"`
}

// Resolution mirrors the adapter's resolution JSON shape.
type Resolution struct {
	IngredientID      string   `json:"ingredientId"`
	RetailerProductID *string  `json:"retailerProductId"`
	ProductName       string   `json:"productName"`
	Packages          int      `json:"packages"`
	ResolvedQuantity  *float64 `json:"resolvedQuantity"`
	MatchType         string   `json:"matchType"` // "pinned" | "pinned-backup" | "exact" | "fuzzy" | "none"
	// Confidence is name-match confidence only; quantity uncertainty is the
	// separate QuantityUncertain flag (packages defaults to a safe 1).
	Confidence        float64 `json:"confidence"`
	NeedsReview       bool    `json:"needsReview"`
	QuantityUncertain bool    `json:"quantityUncertain"`
}

// SearchTerms maps canonical ingredient ids to human search terms (Swedish).
// Sourced from the ingredient table; passing nil falls back to the ids.
type SearchTerms map[string]string

// ResolveRequirements sends canonical requirements to the adapter and returns
// its product resolutions, review flags intact.
func (c *Client) ResolveRequirements(
	ctx context.Context,
	reqs []domain.ShoppingRequirement,
	terms SearchTerms,
) ([]Resolution, error) {
	payload := struct {
		Requirements []requirementPayload `json:"requirements"`
	}{}
	for _, r := range reqs {
		payload.Requirements = append(payload.Requirements, requirementPayload{
			IngredientID:    r.IngredientID,
			SearchTerm:      terms[r.IngredientID],
			Quantity:        r.Quantity,
			Unit:            r.Unit,
			AcceptableForms: r.AcceptableForms,
			PreferredForm:   r.PreferredForm,
		})
	}

	var out struct {
		Resolutions []Resolution `json:"resolutions"`
	}
	if err := c.http.PostJSON(ctx, "/resolve", payload, &out, nil); err != nil {
		return nil, err
	}
	return out.Resolutions, nil
}

// ShoppingListItem is one line of the wishlist to create.
type ShoppingListItem struct {
	ProductCode string `json:"productCode"`
	Quantity    int    `json:"quantity"`
}

// CreatedList identifies the durable wishlist the adapter created.
type CreatedList struct {
	WishlistID string `json:"wishlistId"`
	Name       string `json:"name"`
}

// CreateShoppingList creates the per-week wishlist. It never fills a cart.
func (c *Client) CreateShoppingList(
	ctx context.Context,
	name string,
	items []ShoppingListItem,
) (*CreatedList, error) {
	payload := struct {
		Name  string             `json:"name"`
		Items []ShoppingListItem `json:"items"`
	}{Name: name, Items: items}

	var out CreatedList
	if err := c.http.PostJSON(ctx, "/shopping-lists", payload, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// ── Catalog wiring (barcode → product_identifier) ──────────────────────────

// ExtractRetailerEANs returns the EAN/barcode from each resolution that has
// one. Used by the plan command to wire resolved products into the catalog
// path after resolution.
func ExtractRetailerEANs(resolutions []Resolution) []string {
	eans := make([]string, 0, len(resolutions))
	for _, r := range resolutions {
		if r.RetailerProductID != nil && *r.RetailerProductID != "" {
			eans = append(eans, *r.RetailerProductID)
		}
	}
	return eans
}

// ── ICA-specific endpoints ──────────────────────────────────────────────────

// BarcodeLookupResult holds the result of an ICA barcode (EAN) lookup.
type BarcodeLookupResult struct {
	GTIN           *string `json:"gtin"`
	Name           *string `json:"name"`
	ArticleID      *int    `json:"articleId"`
	ArticleGroupID *int    `json:"articleGroupId"`
}

// LookupBarcode calls /barcode/:ean on an ICA adapter and returns the result.
func (c *Client) LookupBarcode(ctx context.Context, ean string) (*BarcodeLookupResult, error) {
	var out BarcodeLookupResult
	if err := c.http.GetJSON(ctx, "/barcode/"+ean, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// BonusBalance holds the result of an ICA bonus balance query.
type BonusBalance struct {
	Balance           float64 `json:"balance"`
	Vouchers          int     `json:"vouchers"`
	DiscountSummary   string  `json:"discountSummary"`
}

// GetBonusBalance calls /bonus on an ICA adapter and returns the result.
func (c *Client) GetBonusBalance(ctx context.Context) (*BonusBalance, error) {
	var out BonusBalance
	if err := c.http.GetJSON(ctx, "/bonus", &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// ShoppingListSyncDelta describes one row to sync on an ICA shopping list.
type ShoppingListSyncDelta struct {
	OfflineID      string `json:"offlineId"`
	ProductName    string `json:"productName"`
	ProductEan     string `json:"productEan"`
	Quantity       float64 `json:"quantity"`
	Unit           string `json:"unit"`
	IsStrikedOver  bool   `json:"isStrikedOver"`
}

// ShoppingListSyncPayload is the body sent to /shopping-lists/:id/sync.
type ShoppingListSyncPayload struct {
	OfflineID string                      `json:"offlineId"`
	Created   []ShoppingListSyncDelta     `json:"createdRows,omitempty"`
	Changed   []ShoppingListSyncDelta     `json:"changedRows,omitempty"`
	Deleted   []string                    `json:"deletedRows,omitempty"`
}

// SyncedList is the response from /shopping-lists/:id/sync.
type SyncedList struct {
	OfflineID    string                  `json:"offlineId"`
	Rows         []ShoppingListSyncDelta `json:"rows"`
	LatestChange *string                 `json:"latestChange"`
}

// SyncShoppingList sends a MERGE sync delta to an ICA adapter and returns the
// server-confirmed list state.
func (c *Client) SyncShoppingList(
	ctx context.Context,
	listID string,
	delta ShoppingListSyncPayload,
) (*SyncedList, error) {
	var out SyncedList
	if err := c.http.PostJSON(ctx, "/shopping-lists/"+listID+"/sync", delta, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// IcaOffer mirrors the ICA offer data returned by /offers.
type IcaOffer struct {
	ArticleID        int     `json:"articleId"`
	ArticleGroupID   int     `json:"articleGroupId"`
	Name             string  `json:"name"`
	Price            float64 `json:"price"`
	OriginalPrice    float64 `json:"originalPrice"`
	DiscountPercent  float64 `json:"discountPercent"`
	ValidFrom        string  `json:"validFrom"`
	ValidTo          string  `json:"validTo"`
	StoreID          string  `json:"storeId"`
}

// SyncOffers fetches current offers from an ICA adapter and returns them.
// Callers (e.g. a future `food-brain sync-offers` command) are responsible for
// persisting results into the price-intelligence tables.
func (c *Client) SyncOffers(ctx context.Context, storeID string) ([]IcaOffer, error) {
	var out struct {
		Offers []IcaOffer `json:"offers"`
		Data   []IcaOffer `json:"data"`
	}
	if err := c.http.GetJSON(ctx, "/offers?storeId="+storeID, &out, nil); err != nil {
		// Also try without storeId param (adapter may ignore it).
		if err2 := c.http.GetJSON(ctx, "/offers", &out, nil); err2 != nil {
			return nil, err
		}
	}
	if len(out.Offers) > 0 {
		return out.Offers, nil
	}
	return out.Data, nil
}
