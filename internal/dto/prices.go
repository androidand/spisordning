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

// RetailerOut is the HTTP-side view of a retailer.
type RetailerOut struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// StoreOut is the HTTP-side view of a store.
type StoreOut struct {
	ID        string    `json:"id"`
	RetailerID string   `json:"retailer_id"`
	Name      string    `json:"name"`
	Latitude  *float64  `json:"latitude,omitempty"`
	Longitude *float64  `json:"longitude,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// RetailerProductOut is the HTTP-side view of a retailer product.
type RetailerProductOut struct {
	ID            string    `json:"id"`
	RetailerID    string    `json:"retailer_id"`
	ProductID     *string   `json:"product_id,omitempty"`
	RetailerSKU   string    `json:"retailer_sku"`
	DisplayName   string    `json:"display_name"`
	CreatedAt     time.Time `json:"created_at"`
}

// PriceObservationOut is the HTTP-side view of a price observation.
type PriceObservationOut struct {
	ID                  string    `json:"id"`
	StoreProductOfferID string    `json:"store_product_offer_id"`
	ObservedAt          time.Time `json:"observed_at"`
	Price               float64   `json:"price"`
	PriceKind           string    `json:"price_kind"`
	Source              string    `json:"source"`
	CreatedAt           time.Time `json:"created_at"`
}

// PriceIntelligenceService is the read surface for price intelligence: current
// prices per retailer product across stores, with the cheapest store computed.
type PriceIntelligenceService interface {
	// ListProductPrices returns every retailer product with its current prices
	// across all stores and the cheapest store per product.
	ListProductPrices(ctx context.Context) ([]ProductPriceGroup, error)
	// ListRetailers returns all retailers.
	ListRetailers(ctx context.Context) ([]RetailerOut, error)
	// ListRetailerStores returns all stores for a retailer.
	ListRetailerStores(ctx context.Context, retailerID string) ([]StoreOut, error)
	// ListRetailerProducts returns all products for a retailer.
	ListRetailerProducts(ctx context.Context, retailerID string) ([]RetailerProductOut, error)
	// PriceHistoryForProduct returns the price history for a retailer product
	// across all its offers, most recent first.
	PriceHistoryForProduct(ctx context.Context, retailerProductID string) ([]PriceObservationOut, error)
	// PriceHistoryForStore returns the price history for all offers at a store,
	// most recent first.
	PriceHistoryForStore(ctx context.Context, storeID string) ([]PriceObservationOut, error)
}
