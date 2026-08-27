// Package service — stores / pricing service.
package service

import (
	"context"
	"fmt"

	"github.com/androidand/spisordning/internal/dto"
	"github.com/androidand/spisordning/internal/ingredients"
)

// Stores implements dto.StoresService. Product search is backed by the
// Matpriskollen client; store and offer reads are backed by the
// price-intelligence tables (deviation: Matpriskollen exposes no store or
// offer endpoints, so those come from persistence, not the MPK client).
type Stores struct {
	db  Store
	mpk *ingredients.MPKClient
}

// NewStores returns a Stores service. Pass nil mpk to skip product search.
func NewStores(db Store, mpk *ingredients.MPKClient) *Stores {
	return &Stores{db: db, mpk: mpk}
}

// ListStores returns every store across all retailers.
func (s *Stores) ListStores(ctx context.Context) ([]dto.Store, error) {
	rows, err := s.db.ListAllStores(ctx)
	if err != nil {
		return nil, fmt.Errorf("service: list stores: %w", err)
	}
	out := make([]dto.Store, 0, len(rows))
	for _, st := range rows {
		out = append(out, dto.Store{ID: st.ID, RetailerID: st.RetailerID, Name: st.Name})
	}
	return out, nil
}

// ListStoreOffers returns the store_product_offer rows for one store.
func (s *Stores) ListStoreOffers(ctx context.Context, storeID string) ([]dto.StoreOffer, error) {
	rows, err := s.db.ListStoreProductOffers(ctx, storeID)
	if err != nil {
		return nil, fmt.Errorf("service: list store offers: %w", err)
	}
	out := make([]dto.StoreOffer, 0, len(rows))
	for _, o := range rows {
		out = append(out, dto.StoreOffer{
			ID:                o.ID,
			StoreID:           o.StoreID,
			RetailerProductID: o.RetailerProductID,
			CurrentlyCarried:  o.CurrentlyCarried,
			UpdatedAt:         o.UpdatedAt,
		})
	}
	return out, nil
}

func (s *Stores) SearchProducts(ctx context.Context, query string) ([]dto.IngredientProduct, error) {
	if s.mpk == nil {
		return nil, fmt.Errorf("service: search products: Matpriskollen client not configured")
	}
	products, err := s.mpk.Search(ctx, query, 50)
	if err != nil {
		return nil, fmt.Errorf("service: search products: %w", err)
	}
	out := make([]dto.IngredientProduct, 0, len(products))
	for _, p := range products {
		out = append(out, dto.IngredientProduct{
			Key:  p.Key,
			GTIN: p.GTIN,
			Name: p.Name,
			Brand: p.Brand,
		})
	}
	return out, nil
}

func (s *Stores) SearchProductsByGTIN(ctx context.Context, gtin string) ([]dto.IngredientProduct, error) {
	if s.mpk == nil {
		return nil, fmt.Errorf("service: search by gtin: Matpriskollen client not configured")
	}
	products, err := s.mpk.SearchByGTIN(ctx, gtin)
	if err != nil {
		return nil, fmt.Errorf("service: search by gtin: %w", err)
	}
	out := make([]dto.IngredientProduct, 0, len(products))
	for _, p := range products {
		out = append(out, dto.IngredientProduct{
			Key:  p.Key,
			GTIN: p.GTIN,
			Name: p.Name,
			Brand: p.Brand,
		})
	}
	return out, nil
}
