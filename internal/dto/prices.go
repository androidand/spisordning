package dto

import (
	"context"
	"time"
)

// StorePrice is the current price of one retailer product at one store, for one
// price kind. It is a row from the current_store_product_price view.
type StorePrice struct {
	StoreID           string    `json:"store_id"`
	StoreName         string    `json:"store_name"`
	RetailerID        string    `json:"retailer_id"`
	RetailerName      string    `json:"retailer_name"`
	PriceKind         string    `json:"price_kind"`
	Price             float64   `json:"price"`
	ObservedAt        time.Time `json:"observed_at"`
	Source            string    `json:"source"`
}

// ProductPriceGroup is the current prices for one retailer product across all
// stores, with the cheapest store highlighted. This is the read model behind
// the "cheapest store per product" use case.
type ProductPriceGroup struct {
	RetailerProductID string       `json:"retailer_product_id"`
	ProductID         string       `json:"product_id,omitempty"`
	DisplayName       string       `json:"display_name,omitempty"`
	RetailerID        string       `json:"retailer_id"`
	RetailerName      string       `json:"retailer_name"`
	Prices            []StorePrice `json:"prices"`
	// Cheapest is the lowest-priced store for this product (nil when no prices).
	Cheapest *StorePrice `json:"cheapest,omitempty"`
}

// PriceIntelligenceService is the read surface for price intelligence: current
// prices per retailer product across stores, with the cheapest store computed.
type PriceIntelligenceService interface {
	// ListProductPrices returns every retailer product with its current prices
	// across all stores and the cheapest store per product.
	ListProductPrices(ctx context.Context) ([]ProductPriceGroup, error)
}
