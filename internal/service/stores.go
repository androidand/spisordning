// Package service — stores / pricing service.
package service

import (
	"context"
	"fmt"

	"github.com/androidand/spisordning/internal/httpapi"
	"github.com/androidand/spisordning/internal/ingredients"
)

// StoresService is the surface the /stores and /products handlers need.
type StoresService interface {
	SearchProducts(ctx context.Context, query string) ([]httpapi.IngredientProduct, error)
	SearchProductsByGTIN(ctx context.Context, gtin string) ([]httpapi.IngredientProduct, error)
}

// Stores implements StoresService.
type Stores struct {
	mpk *ingredients.MPKClient
}

// NewStores returns a Stores service. Pass nil mpk to skip.
func NewStores(mpk *ingredients.MPKClient) *Stores {
	return &Stores{mpk: mpk}
}

func (s *Stores) SearchProducts(ctx context.Context, query string) ([]httpapi.IngredientProduct, error) {
	if s.mpk == nil {
		return nil, fmt.Errorf("service: search products: Matpriskollen client not configured")
	}
	products, err := s.mpk.Search(ctx, query, 50)
	if err != nil {
		return nil, fmt.Errorf("service: search products: %w", err)
	}
	out := make([]httpapi.IngredientProduct, 0, len(products))
	for _, p := range products {
		out = append(out, httpapi.IngredientProduct{
			Key:  p.Key,
			GTIN: p.GTIN,
			Name: p.Name,
			Brand: p.Brand,
		})
	}
	return out, nil
}

func (s *Stores) SearchProductsByGTIN(ctx context.Context, gtin string) ([]httpapi.IngredientProduct, error) {
	if s.mpk == nil {
		return nil, fmt.Errorf("service: search by gtin: Matpriskollen client not configured")
	}
	products, err := s.mpk.SearchByGTIN(ctx, gtin)
	if err != nil {
		return nil, fmt.Errorf("service: search by gtin: %w", err)
	}
	out := make([]httpapi.IngredientProduct, 0, len(products))
	for _, p := range products {
		out = append(out, httpapi.IngredientProduct{
			Key:  p.Key,
			GTIN: p.GTIN,
			Name: p.Name,
			Brand: p.Brand,
		})
	}
	return out, nil
}
