// Package ingredients — Matpriskollen client.
//
// Matpriskollen is Sveriges största oberoende matpristjänst. It aggregates prices
// from ICA, Coop, Willys, Hemköp, Lidl, and City Gross. The search endpoint is
// accessible without authentication; price/offer endpoints require further
// reverse-engineering (JS bundles reference offerKey, comprice, requireAuthForOffer).
//
// This client covers the product search surface. Price fetching is tracked as
// a follow-up task.
package ingredients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// MPKProduct is a product from the Matpriskollen database.
type MPKProduct struct {
	Key          string         `json:"key"`
	GTIN         string         `json:"gtin"`
	Name         string         `json:"name"`
	Brand        string         `json:"brand"`
	Description  string         `json:"description"`
	Amount       string         `json:"amount"`
	BaseUnit     int            `json:"baseUnit"` // 1=gram/kg, 3=piece/stycken
	ImageURL     string         `json:"imageUrl"`
	ThumbnailURL string         `json:"thumbnailUrl"`
	Category     MPKCategory    `json:"category"`
	SubCategory  MPKCategory    `json:"subCategory"`
	ProductGroup MPKCategory    `json:"productGroup"`
}

// MPKCategory is a category/subcategory/productGroup entry.
type MPKCategory struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

// baseUnitName returns a human-readable name for the baseUnit enum.
func (p MPKProduct) BaseUnitName() string {
	switch p.BaseUnit {
	case 1:
		return "weight" // gram/kg
	case 3:
		return "piece" // stycken
	default:
		return fmt.Sprintf("unknown(%d)", p.BaseUnit)
	}
}

// MPKSearchResult is a plain array of products (no wrapper object).
type MPKSearchResult []MPKProduct

// MPKClient talks to the Matpriskollen API.
type MPKClient struct {
	base string
	http *http.Client
}

// NewMatpriskollen returns a Client for the Matpriskollen API.
func NewMatpriskollen() *MPKClient {
	return &MPKClient{
		base: "https://matpriskollen.se",
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

// Search searches products by name. Returns up to `limit` results.
func (c *MPKClient) Search(ctx context.Context, query string, limit int) ([]MPKProduct, error) {
	if limit <= 0 {
		limit = 50
	}
	params := url.Values{}
	params.Set("q", query)
	params.Set("limit", fmt.Sprintf("%d", limit))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/api/v1/proxy/search?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("matpriskollen: create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "spisordning-food-brain/1.0")
	req.Header.Set("Referer", c.base+"/")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("matpriskollen: search %q: %w", query, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("matpriskollen: search %q: HTTP %d", query, resp.StatusCode)
	}

	var products MPKSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&products); err != nil {
		return nil, fmt.Errorf("matpriskollen: decode search results: %w", err)
	}
	return products, nil
}

// SearchByGTIN looks up a product by its GTIN (barcode).
func (c *MPKClient) SearchByGTIN(ctx context.Context, gtin string) ([]MPKProduct, error) {
	params := url.Values{}
	params.Set("gtin", gtin)
	params.Set("limit", "10")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/api/v1/proxy/search?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("matpriskollen: GTIN request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "spisordning-food-brain/1.0")
	req.Header.Set("Referer", c.base+"/")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("matpriskollen: search GTIN %q: %w", gtin, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("matpriskollen: search GTIN %q: HTTP %d", gtin, resp.StatusCode)
	}

	var products MPKSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&products); err != nil {
		return nil, fmt.Errorf("matpriskollen: decode GTIN results: %w", err)
	}
	return products, nil
}
