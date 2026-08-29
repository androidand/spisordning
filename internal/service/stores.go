// Package service — stores / pricing service.
package service

import (
	"context"
	"fmt"
	"math"
	"sort"

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

// ListStores returns every store across all retailers, with position and
// retailer name.
func (s *Stores) ListStores(ctx context.Context) ([]dto.Store, error) {
	return s.LocateStores(ctx, dto.LocateStoresInput{})
}

// LocateStores returns every store with its position and, when an origin is
// supplied, its distance from that origin. When an origin is given, geo-mapped
// stores are ranked nearest-first (geo-less stores last); otherwise stores are
// ordered by retailer then name.
func (s *Stores) LocateStores(ctx context.Context, input dto.LocateStoresInput) ([]dto.Store, error) {
	rows, err := s.db.ListAllStores(ctx)
	if err != nil {
		return nil, fmt.Errorf("service: list stores: %w", err)
	}
	retailers, err := s.db.ListRetailers(ctx)
	if err != nil {
		return nil, fmt.Errorf("service: list retailers: %w", err)
	}
	retailerNames := make(map[string]string, len(retailers))
	for _, r := range retailers {
		retailerNames[r.ID] = r.Name
	}

	hasOrigin := input.Latitude != nil && input.Longitude != nil

	out := make([]dto.Store, 0, len(rows))
	for _, st := range rows {
		loc := dto.Store{
			ID:           st.ID,
			RetailerID:   st.RetailerID,
			RetailerName: retailerNames[st.RetailerID],
			Name:         st.Name,
			Latitude:     st.Latitude,
			Longitude:    st.Longitude,
		}
		if hasOrigin && st.Latitude != nil && st.Longitude != nil {
			d := haversineKm(*input.Latitude, *input.Longitude, *st.Latitude, *st.Longitude)
			loc.DistanceKm = &d
		}
		out = append(out, loc)
	}

	if hasOrigin {
		sort.SliceStable(out, func(i, j int) bool {
			di, dj := out[i].DistanceKm, out[j].DistanceKm
			if di == nil && dj == nil {
				return false
			}
			if di == nil {
				return false // geo-less stores sort last
			}
			if dj == nil {
				return true
			}
			return *di < *dj
		})
	} else {
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].RetailerName != out[j].RetailerName {
				return out[i].RetailerName < out[j].RetailerName
			}
			return out[i].Name < out[j].Name
		})
	}
	return out, nil
}

// earthRadiusKm is the mean Earth radius in kilometres (IUGG).
const earthRadiusKm = 6371.0088

// haversineKm returns the great-circle distance between two WGS84 points in
// kilometres.
func haversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	const rad = math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLon := (lon2 - lon1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusKm * c
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
