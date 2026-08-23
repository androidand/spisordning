// Package service — pantry inventory service.
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/androidand/spisordning/internal/persistence"
)

// PantryLocation is the HTTP-side view of an inventory location.
type PantryLocation struct {
	ID               string    `json:"id"`
	HouseholdID      string    `json:"household_id"`
	Name             string    `json:"name"`
	LocationType     string    `json:"location_type"`
	ParentLocationID string    `json:"parent_location_id"`
	ArchivedAt       time.Time `json:"archived_at,omitempty"`
}

// PantryLot is the HTTP-side view of an inventory lot.
type PantryLot struct {
	ID           int64     `json:"id"`
	IngredientID string    `json:"ingredient_id"`
	ProductID    string    `json:"product_id"`
	LocationID   string    `json:"location_id"`
	Quantity     float64   `json:"quantity"`
	Unit         string    `json:"unit"`
	Confidence   string    `json:"confidence"`
	BestBefore   time.Time `json:"best_before,omitempty"`
	OpenedAt     time.Time `json:"opened_at,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// PantryService is the surface the /pantry handlers need.
type PantryService interface {
	ListLocations(ctx context.Context, householdID string) ([]PantryLocation, error)
	CreateLocation(ctx context.Context, in PantryLocationNew) (PantryLocation, error)
	ListLots(ctx context.Context, locationID string) ([]PantryLot, error)
	Purchase(ctx context.Context, in PantryPurchaseInput) (PantryLot, error)
	Consume(ctx context.Context, lotID int64, in PantryConsumeInput) error
}

// PantryLocationNew is the POST /pantry/locations request body.
type PantryLocationNew struct {
	HouseholdID      string `json:"household_id"`
	Name             string `json:"name"`
	LocationType     string `json:"location_type"`
	ParentLocationID string `json:"parent_location_id"`
}

// PantryPurchaseInput is the POST /pantry/lots/purchase request body.
type PantryPurchaseInput struct {
	IngredientID string  `json:"ingredient_id"`
	ProductID    string  `json:"product_id"`
	LocationID   string  `json:"location_id"`
	Quantity     float64 `json:"quantity"`
	Unit         string  `json:"unit"`
	BestBefore   string  `json:"best_before,omitempty"`
	Source       string  `json:"source"`
}

// PantryConsumeInput is the POST /pantry/lots/{id}/consume request body.
type PantryConsumeInput struct {
	Quantity float64 `json:"quantity"`
	Estimated bool   `json:"estimated"`
	Source     string `json:"source"`
}

// Pantry implements PantryService.
type Pantry struct{ db Store }

// NewPantry returns a Pantry service backed by db.
func NewPantry(db Store) *Pantry { return &Pantry{db: db} }

func (s *Pantry) ListLocations(ctx context.Context, householdID string) ([]PantryLocation, error) {
	// ListLotsUnderLocation requires a location id; for listing all locations we
	// query directly. The Store interface currently exposes Get/List per location;
	// a full list is a persistence gap we fill here with a minimal query.
	// TODO: add ListInventoryLocations to persistence.Store when the endpoint is needed.
	return nil, fmt.Errorf("service: list locations: not yet implemented (needs ListInventoryLocations in persistence)")
}

func (s *Pantry) CreateLocation(ctx context.Context, in PantryLocationNew) (PantryLocation, error) {
	if in.Name == "" {
		return PantryLocation{}, fmt.Errorf("service: create location: name is required")
	}
	l := persistence.InventoryLocation{
		ID:               generateID(),
		HouseholdID:      in.HouseholdID,
		Name:             in.Name,
		LocationType:     in.LocationType,
		ParentLocationID: in.ParentLocationID,
	}
	if err := s.db.CreateInventoryLocation(ctx, l); err != nil {
		return PantryLocation{}, fmt.Errorf("service: create location: %w", err)
	}
	return PantryLocation{
		ID: l.ID, HouseholdID: l.HouseholdID, Name: l.Name,
		LocationType: l.LocationType, ParentLocationID: l.ParentLocationID,
	}, nil
}

func (s *Pantry) ListLots(ctx context.Context, locationID string) ([]PantryLot, error) {
	lots, err := s.db.ListLotsUnderLocation(ctx, locationID)
	if err != nil {
		return nil, fmt.Errorf("service: list lots: %w", err)
	}
	out := make([]PantryLot, 0, len(lots))
	for _, l := range lots {
		out = append(out, PantryLot{
			ID: l.ID, IngredientID: l.IngredientID, ProductID: l.ProductID,
			LocationID: l.LocationID, Quantity: l.Quantity, Unit: l.Unit,
			Confidence: string(l.Confidence), BestBefore: nilOrTime(l.BestBefore),
			OpenedAt: nilOrTime(l.OpenedAt), CreatedAt: l.CreatedAt, UpdatedAt: l.UpdatedAt,
		})
	}
	return out, nil
}

func (s *Pantry) Purchase(ctx context.Context, in PantryPurchaseInput) (PantryLot, error) {
	if in.Quantity <= 0 {
		return PantryLot{}, fmt.Errorf("service: purchase: quantity must be > 0")
	}
	var bestBefore *time.Time
	if in.BestBefore != "" {
		t, err := time.Parse("2006-01-02", in.BestBefore)
		if err != nil {
			return PantryLot{}, fmt.Errorf("service: purchase: invalid best_before %q: %w", in.BestBefore, err)
		}
		bestBefore = &t
	}
	lotID, err := s.db.RecordPurchase(ctx, in.IngredientID, in.ProductID, in.LocationID, in.Quantity, in.Unit, bestBefore, in.Source)
	if err != nil {
		return PantryLot{}, fmt.Errorf("service: purchase: %w", err)
	}
	lot, err := s.db.GetInventoryLot(ctx, lotID)
	if err != nil {
		return PantryLot{}, fmt.Errorf("service: purchase: read lot: %w", err)
	}
	return PantryLot{
		ID: lot.ID, IngredientID: lot.IngredientID, ProductID: lot.ProductID,
		LocationID: lot.LocationID, Quantity: lot.Quantity, Unit: lot.Unit,
		Confidence: string(lot.Confidence), BestBefore: nilOrTime(lot.BestBefore),
		OpenedAt: nilOrTime(lot.OpenedAt), CreatedAt: lot.CreatedAt, UpdatedAt: lot.UpdatedAt,
	}, nil
}

func (s *Pantry) Consume(ctx context.Context, lotID int64, in PantryConsumeInput) error {
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
