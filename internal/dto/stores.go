package dto

import (
	"context"
	"time"
)

// Store is an HTTP-side view of a store location.
type Store struct {
	ID         string `json:"id"`
	RetailerID string `json:"retailer_id"`
	Name       string `json:"name"`
}

// StoreOffer is an HTTP-side view of a store_product_offer row.
type StoreOffer struct {
	ID                int64     `json:"id"`
	StoreID           string    `json:"store_id"`
	RetailerProductID string    `json:"retailer_product_id"`
	CurrentlyCarried  bool      `json:"currently_carried"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// StoresService is the surface the /stores and /products handlers need.
type StoresService interface {
	ListStores(ctx context.Context) ([]Store, error)
	ListStoreOffers(ctx context.Context, storeID string) ([]StoreOffer, error)
	SearchProducts(ctx context.Context, query string) ([]IngredientProduct, error)
	SearchProductsByGTIN(ctx context.Context, gtin string) ([]IngredientProduct, error)
}
