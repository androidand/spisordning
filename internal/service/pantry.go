// Package service — pantry inventory service.
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/androidand/spisordning/internal/httpapi"
	"github.com/androidand/spisordning/internal/persistence"
)

// PantryService is the surface the /pantry handlers need.
// Types are defined in httpapi; this package imports them for the interface.
type PantryService interface {
	ListLocations(ctx context.Context, householdID string) ([]httpapi.PantryLocation, error)
	CreateLocation(ctx context.Context, in httpapi.PantryLocationNew) (httpapi.PantryLocation, error)
	ListLots(ctx context.Context, locationID string) ([]httpapi.PantryLot, error)
	Purchase(ctx context.Context, in httpapi.PantryPurchaseInput) (httpapi.PantryLot, error)
	Consume(ctx context.Context, lotID int64, in httpapi.PantryConsumeInput) error
}



// Pantry implements PantryService.
type Pantry struct{ db Store }

// NewPantry returns a Pantry service backed by db.
func NewPantry(db Store) *Pantry { return &Pantry{db: db} }

func (s *Pantry) ListLocations(ctx context.Context, householdID string) ([]httpapi.PantryLocation, error) {
	locations, err := s.db.ListInventoryLocations(ctx, householdID)
	if err != nil {
		return nil, fmt.Errorf("service: list locations: %w", err)
	}
	out := make([]httpapi.PantryLocation, 0, len(locations))
	for _, l := range locations {
		out = append(out, httpapi.PantryLocation{
			ID: l.ID, HouseholdID: l.HouseholdID, Name: l.Name,
			LocationType: l.LocationType, ParentLocationID: l.ParentLocationID,
		})
	}
	return out, nil
}

func (s *Pantry) CreateLocation(ctx context.Context, in httpapi.PantryLocationNew) (httpapi.PantryLocation, error) {
	if in.Name == "" {
		return httpapi.PantryLocation{}, fmt.Errorf("service: create location: name is required")
	}
	l := persistence.InventoryLocation{
		ID:               generateID(),
		HouseholdID:      in.HouseholdID,
		Name:             in.Name,
		LocationType:     in.LocationType,
		ParentLocationID: in.ParentLocationID,
	}
	if err := s.db.CreateInventoryLocation(ctx, l); err != nil {
		return httpapi.PantryLocation{}, fmt.Errorf("service: create location: %w", err)
	}
	return httpapi.PantryLocation{
		ID: l.ID, HouseholdID: l.HouseholdID, Name: l.Name,
		LocationType: l.LocationType, ParentLocationID: l.ParentLocationID,
	}, nil
}

func (s *Pantry) ListLots(ctx context.Context, locationID string) ([]httpapi.PantryLot, error) {
	lots, err := s.db.ListLotsUnderLocation(ctx, locationID)
	if err != nil {
		return nil, fmt.Errorf("service: list lots: %w", err)
	}
	out := make([]httpapi.PantryLot, 0, len(lots))
	for _, l := range lots {
		out = append(out, httpapi.PantryLot{
			ID: l.ID, IngredientID: l.IngredientID, ProductID: l.ProductID,
			LocationID: l.LocationID, Quantity: l.Quantity, Unit: l.Unit,
			Confidence: string(l.Confidence), BestBefore: nilOrTime(l.BestBefore),
			OpenedAt: nilOrTime(l.OpenedAt), CreatedAt: l.CreatedAt, UpdatedAt: l.UpdatedAt,
		})
	}
	return out, nil
}

func (s *Pantry) Purchase(ctx context.Context, in httpapi.PantryPurchaseInput) (httpapi.PantryLot, error) {
	if in.Quantity <= 0 {
		return httpapi.PantryLot{}, fmt.Errorf("service: purchase: quantity must be > 0")
	}
	var bestBefore *time.Time
	if in.BestBefore != "" {
		t, err := time.Parse("2006-01-02", in.BestBefore)
		if err != nil {
			return httpapi.PantryLot{}, fmt.Errorf("service: purchase: invalid best_before %q: %w", in.BestBefore, err)
		}
		bestBefore = &t
	}
	lotID, err := s.db.RecordPurchase(ctx, in.IngredientID, in.ProductID, in.LocationID, in.Quantity, in.Unit, bestBefore, in.Source)
	if err != nil {
		return httpapi.PantryLot{}, fmt.Errorf("service: purchase: %w", err)
	}
	lot, err := s.db.GetInventoryLot(ctx, lotID)
	if err != nil {
		return httpapi.PantryLot{}, fmt.Errorf("service: purchase: read lot: %w", err)
	}
	return httpapi.PantryLot{
		ID: lot.ID, IngredientID: lot.IngredientID, ProductID: lot.ProductID,
		LocationID: lot.LocationID, Quantity: lot.Quantity, Unit: lot.Unit,
		Confidence: string(lot.Confidence), BestBefore: nilOrTime(lot.BestBefore),
		OpenedAt: nilOrTime(lot.OpenedAt), CreatedAt: lot.CreatedAt, UpdatedAt: lot.UpdatedAt,
	}, nil
}

func (s *Pantry) Consume(ctx context.Context, lotID int64, in httpapi.PantryConsumeInput) error {
	if in.Quantity <= 0 {
		return fmt.Errorf("service: consume: quantity must be > 0")
	}
	if err := s.db.RecordConsume(ctx, lotID, in.Quantity, in.Estimated, in.Source); err != nil {
		return fmt.Errorf("service: consume: %w", err)
	}
	return nil
}

func nilOrTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}
