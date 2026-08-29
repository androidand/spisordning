package service

import (
	"context"
	"fmt"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/androidand/spisordning/internal/dto"
)

// PriceIntelligence implements dto.PriceIntelligenceService. It reads the
// current_store_product_price view and joins store/retailer names, grouping
// prices by retailer product and computing the cheapest store per product.
type PriceIntelligence struct {
	db Store
}

// NewPriceIntelligence returns a PriceIntelligence service backed by db.
func NewPriceIntelligence(db Store) *PriceIntelligence { return &PriceIntelligence{db: db} }

func (s *PriceIntelligence) ListProductPrices(ctx context.Context) ([]dto.ProductPriceGroup, error) {
	prices, err := s.db.ListCurrentPrices(ctx)
	if err != nil {
		return nil, fmt.Errorf("service: list current prices: %w", err)
	}
	if len(prices) == 0 {
		return []dto.ProductPriceGroup{}, nil
	}

	// Load retailers and stores for name lookups.
	retailers, err := s.db.ListRetailers(ctx)
	if err != nil {
		return nil, fmt.Errorf("service: list retailers: %w", err)
	}
	retailerNames := make(map[string]string, len(retailers))
	for _, r := range retailers {
		retailerNames[r.ID] = r.Name
	}
	stores, err := s.db.ListAllStores(ctx)
	if err != nil {
		return nil, fmt.Errorf("service: list stores: %w", err)
	}
	storeNames := make(map[string]string, len(stores))
	storeRetailers := make(map[string]string, len(stores))
	for _, st := range stores {
		storeNames[st.ID] = st.Name
		storeRetailers[st.ID] = st.RetailerID
	}

	// Load retailer products for display names and retailer ids.
	productByRP := make(map[string]domain.RetailerProduct)
	for _, r := range retailers {
		rps, err := s.db.ListRetailerProducts(ctx, r.ID)
		if err != nil {
			return nil, fmt.Errorf("service: list retailer products: %w", err)
		}
		for _, rp := range rps {
			productByRP[rp.ID] = rp
		}
	}

	// Group prices by retailer product.
	groups := make(map[string]*dto.ProductPriceGroup)
	var order []string
	for _, p := range prices {
		g, ok := groups[p.RetailerProductID]
		if !ok {
			rp := productByRP[p.RetailerProductID]
			g = &dto.ProductPriceGroup{
				RetailerProductID: p.RetailerProductID,
				ProductID:         rp.ProductID,
				DisplayName:       rp.DisplayName,
				RetailerID:        rp.RetailerID,
				RetailerName:      retailerNames[rp.RetailerID],
				Prices:            []dto.StorePrice{},
			}
			groups[p.RetailerProductID] = g
			order = append(order, p.RetailerProductID)
		}
		sp := dto.StorePrice{
			StoreID:      p.StoreID,
			StoreName:    storeNames[p.StoreID],
			RetailerID:   storeRetailers[p.StoreID],
			RetailerName: retailerNames[storeRetailers[p.StoreID]],
			PriceKind:    string(p.PriceKind),
			Price:        p.Price,
			ObservedAt:   p.ObservedAt,
			Source:       p.Source,
		}
		g.Prices = append(g.Prices, sp)
		if g.Cheapest == nil || sp.Price < g.Cheapest.Price {
			c := sp
			g.Cheapest = &c
		}
	}

	out := make([]dto.ProductPriceGroup, 0, len(order))
	for _, id := range order {
		out = append(out, *groups[id])
	}
	return out, nil
}
