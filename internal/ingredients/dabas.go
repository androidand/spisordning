// Package ingredients — Dabas client.
//
// Dabas is a Swedish food product database operated by food industry associations.
// It contains ~30,000 branded products with nutrition info, allergens, and
// ingredient lists. The API is undocumented; this client is reverse-engineered
// from live responses.
package ingredients

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

)

// DabasClient talks to the Dabas API.
type DabasClient struct {
	base string
	http *http.Client
}

// DabasProduct is a product from the Dabas database.
type DabasProduct struct {
	GTIN              string          `json:"GTIN"`
	ArticleName       string          `json:"ArtikelBenanmning"`
	Category          string          `json:"ArtikelKategori"`
	Manufacturer      string          `json:"Tillverkare"`
	Brand             string          `json:"Varumarke"`
	Country           string          `json:"Tillverkningsland"`
	Package           string          `json:"Forpackning"`
	IngredientText    string          `json:"Ingredient"`
	ProductGroupCode  string          `json:"Produktgruppskod"`
	ProductArea       string          `json:"Varuomradebenamning"`
	ProductGroup      string          `json:"Varugruppbenamning"`
	MainGroup         string          `json:"Huvudgruppbenamning"`
	SubGroup          string          `json:"Varuundergruppbenamning"`
	ArticleID         string          `json:"Arident"`
	URL               string          `json:"Url"`
	PreparationStatus string          `json:"Tillagningsstatus"`
	Origin            string          `json:"Ursprung"`
	Allergens         []string        `json:"Allergener"`
	Nutrition         []DabasNutrient `json:"Naringsinfo"`
	Size              string          `json:"Storlek"`
	ImageThumb        string          `json:"ProduktbildThumb"`
	ImageMedium       string          `json:"ProduktbildMedium"`
	ImageFull         string          `json:"Produktbild"`
	SupplierLogo      string          `json:"UppgiftslamnareBild"`
	MarketingMessages []string        `json:"Marknadsbudskap"`
	UHMKriteria       string          `json:"Uhmkriterier"`
}

// DabasNutrient is one entry from the Naringsinfo array.
type DabasNutrient struct {
	Name  string `json:"Benamning"`
	Value string `json:"Value"`
}

// DabasSearchResult is the top-level response from the Dabas search endpoint.
type DabasSearchResult struct {
	TotalRecords int              `json:"TotalRecords"`
	Results      []DabasProduct   `json:"SearchResults"`
}

// NewDabas returns a Client for the Dabas API.
func NewDabas() *DabasClient {
	return &DabasClient{
		base: "https://www.dabas.com",
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

// Search searches the Dabas product database.
// Returns up to 100 results per call; use Page for pagination.
func (c *DabasClient) Search(ctx context.Context, query string, page int) (*DabasSearchResult, error) {
	if page < 0 {
		page = 0
	}
	params := url.Values{}
	params.Set("FromSearch", "true")
	params.Set("SearchText", query)
	params.Set("FromRange", fmt.Sprintf("%d", page*100))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/api/v2/search?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("dabas: create request: %w", err)
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", "spisordning-food-brain/1.0")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dabas: search %q: %w", query, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("dabas: search %q: HTTP %d: %s", query, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result DabasSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("dabas: decode search results: %w", err)
	}
	return &result, nil
}

// SearchAll paginates through all results for a query.
// Calls the callback for each page (up to 100 products).
func (c *DabasClient) SearchAll(ctx context.Context, query string, fn func(page *DabasSearchResult) error) error {
	for page := 0; ; page++ {
		result, err := c.Search(ctx, query, page)
		if err != nil {
			return err
		}
		if err := fn(result); err != nil {
			return err
		}
		if len(result.Results) < 100 || page*100 >= result.TotalRecords {
			break
		}
	}
	return nil
}

// ParseNutrientValue attempts to parse a Dabas nutrient value string into
// a float64 and unit. Returns (value, unit, ok).
// Examples: " 282 kcal" → (282, "kcal", true); "< 0.5 g" → (0.5, "g", true)
func ParseNutrientValue(v string) (float64, string, bool) {
	v = strings.TrimSpace(v)
	parts := strings.Fields(v)
	if len(parts) == 0 {
		return 0, "", false
	}
	valStr := parts[len(parts)-1]
	lessThan := false
	if len(parts) > 1 && parts[0] == "<" {
		valStr = parts[1]
		lessThan = true
	}
	var val float64
	fmt.Sscanf(valStr, "%f", &val)
	unit := ""
	if len(parts) > 1 {
		if lessThan {
			unit = parts[len(parts)-1]
		} else {
			unit = parts[len(parts)-1]
		}
	}
	return val, unit, valStr != "0" || len(parts) > 1
}
