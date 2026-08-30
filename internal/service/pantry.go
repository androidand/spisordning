// Package service — pantry inventory service.
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/androidand/spisordning/internal/dto"
	"github.com/androidand/spisordning/internal/persistence"
)

// Pantry implements dto.PantryService.
type Pantry struct{ db Store }

// NewPantry returns a Pantry service backed by db.
func NewPantry(db Store) *Pantry { return &Pantry{db: db} }

func (s *Pantry) ListLocations(ctx context.Context, householdID string) ([]dto.PantryLocation, error) {
	locations, err := s.db.ListInventoryLocations(ctx, householdID)
	if err != nil {
		return nil, fmt.Errorf("service: list locations: %w", err)
	}
	out := make([]dto.PantryLocation, 0, len(locations))
	for _, l := range locations {
		var parent *string
		if l.ParentLocationID != nil {
			p := l.ParentLocationID.String()
			parent = &p
		}
		out = append(out, dto.PantryLocation{
			ID: l.ID.String(), HouseholdID: l.HouseholdID.String(), Name: l.Name,
			LocationType: l.LocationType, ParentLocationID: parent,
		})
	}
	return out, nil
}

func (s *Pantry) CreateLocation(ctx context.Context, in dto.PantryLocationNew) (dto.PantryLocation, error) {
	if in.Name == "" {
		return dto.PantryLocation{}, fmt.Errorf("service: create location: name is required")
	}
	hhID, err := domain.ParseHouseholdID(in.HouseholdID)
	if err != nil {
		return dto.PantryLocation{}, fmt.Errorf("service: create location: %w", err)
	}
	var parent *domain.InventoryLocationID
	if in.ParentLocationID != nil && *in.ParentLocationID != "" {
		p, perr := domain.ParseInventoryLocationID(*in.ParentLocationID)
		if perr != nil {
			return dto.PantryLocation{}, fmt.Errorf("service: create location: %w", perr)
		}
		parent = &p
	}
	l := persistence.InventoryLocation{
		ID:               domain.NewInventoryLocationID(),
		HouseholdID:      hhID,
		Name:             in.Name,
		LocationType:     in.LocationType,
		ParentLocationID: parent,
	}
	if err := s.db.CreateInventoryLocation(ctx, l); err != nil {
		return dto.PantryLocation{}, fmt.Errorf("service: create location: %w", err)
	}
	return dto.PantryLocation{
		ID: l.ID.String(), HouseholdID: l.HouseholdID.String(), Name: l.Name,
		LocationType: l.LocationType, ParentLocationID: in.ParentLocationID,
	}, nil
}

func (s *Pantry) ListLots(ctx context.Context, locationID string) ([]dto.PantryLot, error) {
	locID, err := domain.ParseInventoryLocationID(locationID)
	if err != nil {
		return nil, fmt.Errorf("service: list lots: %w", err)
	}
	lots, err := s.db.ListLotsUnderLocation(ctx, locID)
	if err != nil {
		return nil, fmt.Errorf("service: list lots: %w", err)
	}
	out := make([]dto.PantryLot, 0, len(lots))
	for _, l := range lots {
		var productID *string
		if l.ProductID != nil {
			p := l.ProductID.String()
			productID = &p
		}
		out = append(out, dto.PantryLot{
			ID: l.ID.String(), IngredientID: l.IngredientID.String(), ProductID: productID,
			LocationID: l.LocationID.String(), Quantity: l.Quantity, Unit: l.Unit,
			Confidence: string(l.Confidence), BestBefore: nilOrTime(l.BestBefore),
			OpenedAt: nilOrTime(l.OpenedAt), CreatedAt: l.CreatedAt, UpdatedAt: l.UpdatedAt,
		})
	}
	return out, nil
}

func (s *Pantry) Purchase(ctx context.Context, in dto.PantryPurchaseInput) (dto.PantryLot, error) {
	if in.Quantity <= 0 {
		return dto.PantryLot{}, fmt.Errorf("service: purchase: quantity must be > 0")
	}
	var bestBefore *time.Time
	if in.BestBefore != "" {
		t, err := time.Parse("2006-01-02", in.BestBefore)
		if err != nil {
			return dto.PantryLot{}, fmt.Errorf("service: purchase: invalid best_before %q: %w", in.BestBefore, err)
		}
		bestBefore = &t
	}
	ingredientID := domain.IngredientIDForName(in.IngredientID)
	locID, err := domain.ParseInventoryLocationID(in.LocationID)
	if err != nil {
		return dto.PantryLot{}, fmt.Errorf("service: purchase: %w", err)
	}
	var productID *domain.ProductID
	if in.ProductID != nil && *in.ProductID != "" {
		p, perr := domain.ParseProductID(*in.ProductID)
		if perr != nil {
			return dto.PantryLot{}, fmt.Errorf("service: purchase: %w", perr)
		}
		productID = &p
	}
	lotID, err := s.db.RecordPurchase(ctx, ingredientID, productID, locID, in.Quantity, in.Unit, bestBefore, in.Source)
	if err != nil {
		return dto.PantryLot{}, fmt.Errorf("service: purchase: %w", err)
	}
	lot, err := s.db.GetInventoryLot(ctx, lotID)
	if err != nil {
		return dto.PantryLot{}, fmt.Errorf("service: purchase: read lot: %w", err)
	}
	var lotProductID *string
	if lot.ProductID != nil {
		p := lot.ProductID.String()
		lotProductID = &p
	}
	return dto.PantryLot{
		ID: lot.ID.String(), IngredientID: in.IngredientID, ProductID: lotProductID,
		LocationID: lot.LocationID.String(), Quantity: lot.Quantity, Unit: lot.Unit,
		Confidence: string(lot.Confidence), BestBefore: nilOrTime(lot.BestBefore),
		OpenedAt: nilOrTime(lot.OpenedAt), CreatedAt: lot.CreatedAt, UpdatedAt: lot.UpdatedAt,
	}, nil
}

func (s *Pantry) Consume(ctx context.Context, lotID string, in dto.PantryConsumeInput) error {
	if in.Quantity <= 0 {
		return fmt.Errorf("service: consume: quantity must be > 0")
	}
	lid, err := domain.ParseInventoryLotID(lotID)
	if err != nil {
		return fmt.Errorf("service: consume: %w", err)
	}
	if err := s.db.RecordConsume(ctx, lid, in.Quantity, in.Estimated, in.Source); err != nil {
		return fmt.Errorf("service: consume: %w", err)
	}
	return nil
}

func (s *Pantry) ListExpiring(ctx context.Context, within time.Duration) ([]dto.PantryLot, error) {
	lots, err := s.db.ListExpiringLots(ctx, within)
	if err != nil {
		return nil, fmt.Errorf("service: list expiring lots: %w", err)
	}
	out := make([]dto.PantryLot, 0, len(lots))
	for _, l := range lots {
		var productID *string
		if l.ProductID != nil {
			p := l.ProductID.String()
			productID = &p
		}
		out = append(out, dto.PantryLot{
			ID: l.ID.String(), IngredientID: l.IngredientID.String(), ProductID: productID,
			LocationID: l.LocationID.String(), Quantity: l.Quantity, Unit: l.Unit,
			Confidence: string(l.Confidence), BestBefore: nilOrTime(l.BestBefore),
			OpenedAt: nilOrTime(l.OpenedAt), CreatedAt: l.CreatedAt, UpdatedAt: l.UpdatedAt,
		})
	}
	return out, nil
}

func nilOrTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}
