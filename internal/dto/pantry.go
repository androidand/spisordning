package dto

import (
	"context"
	"time"
)

// PantryLocation is the HTTP-side view of an inventory location.
type PantryLocation struct {
	ID               string     `json:"id"`
	HouseholdID      string     `json:"household_id"`
	Name             string     `json:"name"`
	LocationType     string     `json:"location_type"`
	ParentLocationID *string    `json:"parent_location_id,omitempty"`
	ArchivedAt       time.Time  `json:"archived_at,omitempty"`
}

// PantryLot is the HTTP-side view of an inventory lot.
type PantryLot struct {
	ID           string    `json:"id"`
	IngredientID string    `json:"ingredient_id"`
	ProductID    *string   `json:"product_id,omitempty"`
	LocationID   string    `json:"location_id"`
	Quantity     float64   `json:"quantity"`
	Unit         string    `json:"unit"`
	Confidence   string    `json:"confidence"`
	BestBefore   time.Time `json:"best_before,omitempty"`
	OpenedAt     time.Time `json:"opened_at,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// PantryLocationNew is the POST /pantry/locations request body.
type PantryLocationNew struct {
	HouseholdID      string  `json:"household_id"`
	Name             string  `json:"name"`
	LocationType     string  `json:"location_type"`
	ParentLocationID *string `json:"parent_location_id,omitempty"`
}

// PantryPurchaseInput is the POST /pantry/lots/purchase request body.
type PantryPurchaseInput struct {
	IngredientID string  `json:"ingredient_id"`
	ProductID    *string `json:"product_id,omitempty"`
	LocationID   string  `json:"location_id"`
	Quantity     float64 `json:"quantity"`
	Unit         string  `json:"unit"`
	BestBefore   string  `json:"best_before,omitempty"`
	Source       string  `json:"source"`
}

// PantryConsumeInput is the POST /pantry/lots/{id}/consume request body.
type PantryConsumeInput struct {
	Quantity  float64 `json:"quantity"`
	Estimated bool    `json:"estimated"`
	Source    string  `json:"source"`
}

// PantryDiscardInput is the POST /pantry/lots/{id}/discard request body.
type PantryDiscardInput struct {
	Quantity  float64 `json:"quantity"`
	Estimated bool    `json:"estimated"`
	Reason    string  `json:"reason"`
	Source    string  `json:"source"`
}

// PantryAdjustInput is the POST /pantry/lots/{id}/adjust request body.
type PantryAdjustInput struct {
	Quantity  float64 `json:"quantity"`
	Estimated bool    `json:"estimated"`
	Reason    string  `json:"reason"`
	Source    string  `json:"source"`
}

// PantryOpenInput is the POST /pantry/lots/{id}/open request body.
type PantryOpenInput struct {
	Source string `json:"source"`
}

// PantryTransferInput is the POST /pantry/lots/{id}/transfer request body.
type PantryTransferInput struct {
	LocationID string  `json:"location_id"`
	Quantity   float64 `json:"quantity"`
	Source     string  `json:"source"`
}

// PantryService is the surface the /pantry handlers need.
type PantryService interface {
	ListLocations(ctx context.Context, householdID string) ([]PantryLocation, error)
	CreateLocation(ctx context.Context, in PantryLocationNew) (PantryLocation, error)
	ListLots(ctx context.Context, locationID string) ([]PantryLot, error)
	Purchase(ctx context.Context, in PantryPurchaseInput) (PantryLot, error)
	Consume(ctx context.Context, lotID string, in PantryConsumeInput) error
	Discard(ctx context.Context, lotID string, in PantryDiscardInput) (PantryLot, error)
	Adjust(ctx context.Context, lotID string, in PantryAdjustInput) (PantryLot, error)
	MarkEmpty(ctx context.Context, lotID string) (PantryLot, error)
	Open(ctx context.Context, lotID string, in PantryOpenInput) (PantryLot, error)
	Transfer(ctx context.Context, lotID string, in PantryTransferInput) (PantryLot, error)
	// ListExpiring returns non-empty lots whose best_before is within the given
	// window (already expired or expiring soon), most urgent first.
	ListExpiring(ctx context.Context, within time.Duration) ([]PantryLot, error)
}
