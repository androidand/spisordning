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
		retailerNames[r.ID.String()] = r.Name
	}
	stores, err := s.db.ListAllStores(ctx)
	if err != nil {
		return nil, fmt.Errorf("service: list stores: %w", err)
	}
	storeNames := make(map[string]string, len(stores))
	storeRetailers := make(map[string]string, len(stores))
	for _, st := range stores {
		storeNames[st.ID.String()] = st.Name
		storeRetailers[st.ID.String()] = st.RetailerID.String()
	}

	// Load retailer products for display names and retailer ids.
	productByRP := make(map[string]domain.RetailerProduct)
	for _, r := range retailers {
		rps, err := s.db.ListRetailerProducts(ctx, r.ID)
		if err != nil {
			return nil, fmt.Errorf("service: list retailer products: %w", err)
		}
		for _, rp := range rps {
			productByRP[rp.ID.String()] = rp
		}
	}

	// Group prices by retailer product.
	groups := make(map[string]*dto.ProductPriceGroup)
	var order []string
	for _, p := range prices {
		rpKey := p.RetailerProductID.String()
		g, ok := groups[rpKey]
		if !ok {
			rp := productByRP[rpKey]
			productIDStr := ""
			if rp.ProductID != nil {
				productIDStr = rp.ProductID.String()
			}
			g = &dto.ProductPriceGroup{
				RetailerProductID: rpKey,
				ProductID:         productIDStr,
				DisplayName:       rp.DisplayName,
				RetailerID:        rp.RetailerID.String(),
				RetailerName:      retailerNames[rp.RetailerID.String()],
				Prices:            []dto.StorePrice{},
			}
			groups[rpKey] = g
			order = append(order, rpKey)
		}
		storeIDStr := p.StoreID.String()
		sp := dto.StorePrice{
			StoreID:      storeIDStr,
			StoreName:    storeNames[storeIDStr],
			RetailerID:   storeRetailers[storeIDStr],
			RetailerName: retailerNames[storeRetailers[storeIDStr]],
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

func (s *PriceIntelligence) ListRetailers(ctx context.Context) ([]dto.RetailerOut, error) {
	retailers, err := s.db.ListRetailers(ctx)
	if err != nil {
		return nil, fmt.Errorf("service: list retailers: %w", err)
	}
	out := make([]dto.RetailerOut, 0, len(retailers))
	for _, r := range retailers {
		out = append(out, dto.RetailerOut{ID: r.ID.String(), Name: r.Name, CreatedAt: r.CreatedAt})
	}
	return out, nil
}

func (s *PriceIntelligence) ListRetailerStores(ctx context.Context, retailerID string) ([]dto.StoreOut, error) {
	rid, err := domain.ParseRetailerID(retailerID)
	if err != nil {
		return nil, fmt.Errorf("service: list retailer stores: %w", err)
	}
	stores, err := s.db.ListStores(ctx, rid)
	if err != nil {
		return nil, fmt.Errorf("service: list retailer stores: %w", err)
	}
	out := make([]dto.StoreOut, 0, len(stores))
	for _, st := range stores {
		out = append(out, dto.StoreOut{
			ID: st.ID.String(), RetailerID: st.RetailerID.String(), Name: st.Name,
			Latitude: st.Latitude, Longitude: st.Longitude, CreatedAt: st.CreatedAt,
		})
	}
	return out, nil
}

func (s *PriceIntelligence) ListRetailerProducts(ctx context.Context, retailerID string) ([]dto.RetailerProductOut, error) {
	rid, err := domain.ParseRetailerID(retailerID)
	if err != nil {
		return nil, fmt.Errorf("service: list retailer products: %w", err)
	}
	rps, err := s.db.ListRetailerProducts(ctx, rid)
	if err != nil {
		return nil, fmt.Errorf("service: list retailer products: %w", err)
	}
	out := make([]dto.RetailerProductOut, 0, len(rps))
	for _, rp := range rps {
		var productID *string
		if rp.ProductID != nil {
			p := rp.ProductID.String()
			productID = &p
		}
		out = append(out, dto.RetailerProductOut{
			ID: rp.ID.String(), RetailerID: rp.RetailerID.String(), ProductID: productID,
			RetailerSKU: rp.RetailerSKU, DisplayName: rp.DisplayName, CreatedAt: rp.CreatedAt,
		})
	}
	return out, nil
}

func (s *PriceIntelligence) PriceHistoryForProduct(ctx context.Context, retailerProductID string) ([]dto.PriceObservationOut, error) {
	rpid, err := domain.ParseRetailerProductID(retailerProductID)
	if err != nil {
		return nil, fmt.Errorf("service: price history for product: %w", err)
	}
	obs, err := s.db.PriceObservationsForProduct(ctx, rpid)
	if err != nil {
		return nil, fmt.Errorf("service: price history for product: %w", err)
	}
	return toPriceObservationOuts(obs), nil
}

func (s *PriceIntelligence) PriceHistoryForStore(ctx context.Context, storeID string) ([]dto.PriceObservationOut, error) {
	sid, err := domain.ParseStoreID(storeID)
	if err != nil {
		return nil, fmt.Errorf("service: price history for store: %w", err)
	}
	obs, err := s.db.PriceObservationsForStore(ctx, sid)
	if err != nil {
		return nil, fmt.Errorf("service: price history for store: %w", err)
	}
	return toPriceObservationOuts(obs), nil
}

func toPriceObservationOuts(obs []domain.PriceObservation) []dto.PriceObservationOut {
	out := make([]dto.PriceObservationOut, 0, len(obs))
	for _, o := range obs {
		out = append(out, dto.PriceObservationOut{
			ID: o.ID.String(), StoreProductOfferID: o.StoreProductOfferID.String(),
			ObservedAt: o.ObservedAt, Price: o.Price, PriceKind: string(o.PriceKind),
			Source: o.Source, CreatedAt: o.CreatedAt,
		})
	}
	return out
}
