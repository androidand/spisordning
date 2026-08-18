// Package retailer is the Food Brain's client for the willys-adapter HTTP
// service. The adapter owns all Willys session state (login, cookies, CSRF,
// store pinning); this package only exchanges domain data. Its terminal output
// is a durable wishlist id — never a cart, never a payment.
package retailer

import (
	"context"
	"time"

	"github.com/androidand/spisordning/internal/httpclient"
	"github.com/androidand/spisordning/internal/planning"
)

// Client talks to a running willys-adapter instance.
type Client struct {
	http *httpclient.Client
}

// New returns a Client for the adapter at baseURL (e.g. "http://localhost:8402").
func New(baseURL string) *Client {
	return &Client{http: httpclient.New(baseURL, "adapter", 60*time.Second)}
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
	reqs []planning.ShoppingRequirement,
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
