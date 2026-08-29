package dto

import (
	"context"
	"time"
)

// Store is an HTTP-side view of a store location.
//
// Latitude/Longitude are WGS84 decimal degrees and are optional: a store may
// be known (and carry prices) without a mapped position. DistanceKm is the
// great-circle distance from the request origin, in kilometres, and is nil when
// the store has no position or no origin was supplied.
type Store struct {
	ID         string   `json:"id"`
	RetailerID string   `json:"retailer_id"`
	RetailerName string `json:"retailer_name,omitempty"`
	Name       string   `json:"name"`
	Latitude   *float64 `json:"latitude,omitempty"`
	Longitude  *float64 `json:"longitude,omitempty"`
	// DistanceKm is the great-circle distance from the request origin, in
	// kilometres. Nil when the store has no position or no origin was given.
	DistanceKm *float64 `json:"distance_km,omitempty"`
}

// StoreOffer is an HTTP-side view of a store_product_offer row.
type StoreOffer struct {
	ID                int64     `json:"id"`
	StoreID           string    `json:"store_id"`
	RetailerProductID string    `json:"retailer_product_id"`
	CurrentlyCarried  bool      `json:"currently_carried"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// LocateStoresInput is the optional origin for a store-locator query. When
// Latitude and Longitude are both nil, distances are not computed and stores
// are returned ordered by retailer then name.
type LocateStoresInput struct {
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
}

// StoresService is the surface the /stores and /products handlers need.
type StoresService interface {
	ListStores(ctx context.Context) ([]Store, error)
	// LocateStores returns every store with its position, optionally ranked by
	// distance from an origin (nearest first; geo-less stores last).
	LocateStores(ctx context.Context, input LocateStoresInput) ([]Store, error)
	ListStoreOffers(ctx context.Context, storeID string) ([]StoreOffer, error)
	SearchProducts(ctx context.Context, query string) ([]IngredientProduct, error)
	SearchProductsByGTIN(ctx context.Context, gtin string) ([]IngredientProduct, error)
}
