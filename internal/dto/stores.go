package dto

import (
	"context"
)

// StoresService is the surface the /stores and /products handlers need.
type StoresService interface {
	SearchProducts(ctx context.Context, query string) ([]IngredientProduct, error)
	SearchProductsByGTIN(ctx context.Context, gtin string) ([]IngredientProduct, error)
}
