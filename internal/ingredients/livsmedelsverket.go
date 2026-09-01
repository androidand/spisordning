// Package ingredients — Livsmedelsverket client.
//
// Livsmedelsverket (Swedish Food Agency) operates the national food database
// with ~2,600 food items and 50+ nutrients each. This client wraps the REST
// API at dataportal.livsmedelsverket.se/livsmedel.
//
// License: CC BY 4.0 — attribution required when using the data.
package ingredients

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/androidand/spisordning/internal/httpclient"
)

// Språk controls the language of returned names. 1=Swedish, 2=English.
// Sprak is the ASCII alias for Språk (avoids non-ASCII identifiers in some tooling).
type Språk int

const (
	SprakSwedish Språk = 1
	SprakEnglish Språk = 2
)

// Client talks to the Livsmedelsverket API.
type Client struct {
	http *httpclient.Client
	base string
}

// NewLivsmedelsverket returns a Client for the SLV API at the given base URL.
// Default: "https://dataportal.livsmedelsverket.se/livsmedel".
func NewLivsmedelsverket(baseURL string) *Client {
	if baseURL == "" {
		baseURL = "https://dataportal.livsmedelsverket.se/livsmedel"
	}
	return &Client{
		http: httpclient.New(baseURL, "livsmedelsverket", 30*time.Second),
		base: baseURL,
	}
}

// Food is a single item from the SLV database.
type Food struct {
	Nummer             int    `json:"nummer"`
	Namn               string `json:"namn"`
	VetenskapligtNamn  string `json:"vetenskapligtNamn"`
	LivsmedelsTypID    int    `json:"livsmedelsTypId"`
	LivsmedelsTyp      string `json:"livsmedelsTyp"`
	Projekt            string `json:"projekt"`
	Analys             string `json:"analys"`
	Tillagningsmetod   string `json:"tillagningsmetod"`
	Version            string `json:"version"`
	Links              []Link `json:"links"`

	naringsvardenHref  string
	klassificeringarHref string
	ravarorHref        string
}

// NutritionLink returns the HREF for the nutrition endpoint, if present.
func (f Food) NutritionLink() string { return f.naringsvardenHref }

// ClassificationLink returns the HREF for the classifications endpoint.
func (f Food) ClassificationLink() string { return f.klassificeringarHref }

// RawIngredientsLink returns the HREF for the raw commodities endpoint.
func (f Food) RawIngredientsLink() string { return f.ravarorHref }

func extractLinks(links []Link) (nutr, klass, raw string) {
	for _, l := range links {
		switch l.Rel {
		case "naringvarden":
			nutr = l.Href
		case "klassificeringar":
			klass = l.Href
		case "ravaror":
			raw = l.Href
		}
	}
	return
}

// Link is a HATEOAS relation from the SLV API.
type Link struct {
	Href   string `json:"href"`
	Rel    string `json:"rel"`
	Method string `json:"method"`
}

// FoodPage is a paginated response from the SLV list endpoint.
type FoodPage struct {
	Meta  Meta   `json:"_meta"`
	Foods []Food `json:"livsmedel"`
	Links []Link `json:"_links"`
}

// Meta holds pagination metadata.
type Meta struct {
	TotalRecords int `json:"totalRecords"`
	Offset       int `json:"offset"`
	Limit        int `json:"limit"`
	Count        int `json:"count"`
}

// Nutrient is one nutritional value for a food (per 100g edible portion).
type Nutrient struct {
	Namn           string  `json:"namn"`
	EuroFIRKod     string  `json:"euroFIRkod"`
	Forkortning    string  `json:"forkortning"`
	Värde          float64 `json:"varde"`
	Enhet          string  `json:"enhet"`
	ViktGram       int     `json:"viktGram"`
	Matrisenhet    string  `json:"matrisenhet"`
	MatrisenhetKod string  `json:"matrisenhetkod"`
	Metodtyp       string  `json:"metodtyp"`
	MetodtypKod    string  `json:"metodtypkod"`
	Ursprung       string  `json:"ursprung"`
	Kommentar      string  `json:"kommentar"`
}

// Classification is a LanguaL™ / FoodEx2 classification for a food.
type Classification struct {
	Typ       string `json:"typ"`
	Fasett    string `json:"fasett"`
	Fasettkod string `json:"fasettkod"`
	Kod       string `json:"kod"` // FoodEx2 code
	Namn      string `json:"namn"`
	LangualID string `json:"langualId"`
}

// RawCommodity describes the raw agricultural commodity a processed food is made from.
type RawCommodity struct {
	Namn           string  `json:"namn"`
	FoodEx2        string  `json:"foodEx2"`
	Tillagning     string  `json:"tillagning"`
	Andel          float64 `json:"andel"`
	Faktor         float64 `json:"faktor"`
	OmraknadTillRA float64 `json:"omraknadTillRa"`
}

// LookupFood returns a food by its SLV nummer.
func (c *Client) LookupFood(ctx context.Context, nummer int, lang Språk) (*Food, error) {
	path := fmt.Sprintf("/api/v1/livsmedel/%d?sprak=%d", nummer, lang)
	var food Food
	if err := c.http.GetJSON(ctx, path, &food, nil); err != nil {
		return nil, fmt.Errorf("livsmedelsverket: lookup food %d: %w", nummer, err)
	}
	food.naringsvardenHref, food.klassificeringarHref, food.ravarorHref = extractLinks(food.Links)
	return &food, nil
}

// SearchFood returns the first page of all foods from SLV.
// The SLV API does not support name-based filtering — callers must filter
// client-side. Use a local synced copy for efficient name searches.
func (c *Client) SearchFood(ctx context.Context, lang Språk, limit int) (*FoodPage, error) {
	if limit <= 0 {
		limit = 20
	}
	params := url.Values{}
	params.Set("sprak", fmt.Sprintf("%d", lang))
	params.Set("limit", fmt.Sprintf("%d", limit))
	path := "/api/v1/livsmedel?" + params.Encode()

	var page FoodPage
	if err := c.http.GetJSON(ctx, path, &page, nil); err != nil {
		return nil, fmt.Errorf("livsmedelsverket: search food: %w", err)
	}
	for i := range page.Foods {
		page.Foods[i].naringsvardenHref, page.Foods[i].klassificeringarHref, page.Foods[i].ravarorHref = extractLinks(page.Foods[i].Links)
	}
	return &page, nil
}

// LookupNutrition returns all nutrient values for a food.
func (c *Client) LookupNutrition(ctx context.Context, nummer int, lang Språk) ([]Nutrient, error) {
	path := fmt.Sprintf("/api/v1/livsmedel/%d/naringsvarden?sprak=%d", nummer, lang)
	var nutrients []Nutrient
	if err := c.http.GetJSON(ctx, path, &nutrients, nil); err != nil {
		return nil, fmt.Errorf("livsmedelsverket: lookup nutrition %d: %w", nummer, err)
	}
	return nutrients, nil
}

// LookupClassifications returns LanguaL™ and FoodEx2 classifications.
func (c *Client) LookupClassifications(ctx context.Context, nummer int, lang Språk) ([]Classification, error) {
	path := fmt.Sprintf("/api/v1/livsmedel/%d/klassificeringar?sprak=%d", nummer, lang)
	var klass []Classification
	if err := c.http.GetJSON(ctx, path, &klass, nil); err != nil {
		return nil, fmt.Errorf("livsmedelsverket: lookup classifications %d: %w", nummer, err)
	}
	return klass, nil
}

// LookupRawCommodities returns the raw agricultural commodities a food is made from.
func (c *Client) LookupRawCommodities(ctx context.Context, nummer int, lang Språk) ([]RawCommodity, error) {
	path := fmt.Sprintf("/api/v1/livsmedel/%d/ravaror?sprak=%d", nummer, lang)
	var raw []RawCommodity
	if err := c.http.GetJSON(ctx, path, &raw, nil); err != nil {
		return nil, fmt.Errorf("livsmedelsverket: lookup raw commodities %d: %w", nummer, err)
	}
	return raw, nil
}

// SyncAll fetches every food item and its nutrition data.
// Returns all foods with their nutrition prefetched. The SLV API supports
// pagination via offset/limit; this pages through the full dataset.
func (c *Client) SyncAll(ctx context.Context, lang Språk) ([]FoodWithNutrition, error) {
	const pageSize = 100
	var all []FoodWithNutrition
	offset := 0
	for {
		params := url.Values{}
		params.Set("sprak", fmt.Sprintf("%d", lang))
		params.Set("limit", fmt.Sprintf("%d", pageSize))
		params.Set("offset", fmt.Sprintf("%d", offset))
		path := "/api/v1/livsmedel?" + params.Encode()

		var page FoodPage
		if err := c.http.GetJSON(ctx, path, &page, nil); err != nil {
			return nil, fmt.Errorf("livsmedelsverket: sync page offset=%d: %w", offset, err)
		}
		for _, food := range page.Foods {
			nutr, _ := c.LookupNutrition(ctx, food.Nummer, lang)
			all = append(all, FoodWithNutrition{Food: food, Nutrition: nutr})
		}
		if offset+pageSize >= page.Meta.TotalRecords {
			break
		}
		offset += pageSize
	}
	return all, nil
}

// FoodWithNutrition is a food with its nutrition data prefetched.
type FoodWithNutrition struct {
	Food      Food       `json:"food"`
	Nutrition []Nutrient `json:"nutrition"`
}
