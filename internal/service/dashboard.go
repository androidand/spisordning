package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/androidand/spisordning/internal/dto"
)

// expiringWindow is how far ahead the dashboard looks for expiring pantry lots.
const expiringWindow = 7 * 24 * time.Hour

// expiringLimit caps how many expiring lots the dashboard surfaces.
const expiringLimit = 5

// TonightProvider is the subset of the tonight surface the dashboard needs.
// Providers return dto.ErrNoMealTonight when nothing is planned for today; the
// dashboard treats that as "no tonight" rather than an error.
type TonightProvider interface {
	GetTonight(ctx context.Context) (dto.TonightView, error)
}

// PantryProvider is the subset of the pantry service the dashboard needs.
type PantryProvider interface {
	ListLocations(ctx context.Context, householdID string) ([]dto.PantryLocation, error)
	ListExpiring(ctx context.Context, within time.Duration) ([]dto.PantryLot, error)
}

// Dashboard implements dto.DashboardService. It aggregates tonight's meal, a
// pantry summary, and the most urgent expiring items into a single read model.
type Dashboard struct {
	db      Store
	tonight TonightProvider
	pantry  PantryProvider
}

// NewDashboard returns a Dashboard service. tonight and pantry may be nil, in
// which case the corresponding dashboard sections are empty.
func NewDashboard(db Store, tonight TonightProvider, pantry PantryProvider) *Dashboard {
	return &Dashboard{db: db, tonight: tonight, pantry: pantry}
}

// Get returns the aggregate dashboard for the given household.
func (s *Dashboard) Get(ctx context.Context, householdID string) (dto.Dashboard, error) {
	out := dto.Dashboard{Expiring: []dto.DashboardExpiringLot{}}

	// Tonight's meal (optional).
	if s.tonight != nil {
		view, err := s.tonight.GetTonight(ctx)
		if err != nil {
			if !errors.Is(err, dto.ErrNoMealTonight) {
				return dto.Dashboard{}, fmt.Errorf("service: get tonight: %w", err)
			}
			// No meal planned for today — leave Tonight nil.
		} else {
			out.Tonight = &dto.DashboardTonight{ServedOn: view.ServedOn, Recipe: view.Recipe}
		}
	}

	// Pantry summary (optional).
	if s.pantry != nil {
		locations, err := s.pantry.ListLocations(ctx, householdID)
		if err != nil {
			return dto.Dashboard{}, fmt.Errorf("service: list pantry locations: %w", err)
		}
		out.Pantry.Locations = len(locations)

		lots, err := s.countLots(ctx, locations)
		if err != nil {
			return dto.Dashboard{}, err
		}
		out.Pantry.Lots = lots

		expiring, err := s.pantry.ListExpiring(ctx, expiringWindow)
		if err != nil {
			return dto.Dashboard{}, fmt.Errorf("service: list expiring: %w", err)
		}
		out.Pantry.Expiring = len(expiring)
		for i, lot := range expiring {
			if i >= expiringLimit {
				break
			}
			out.Expiring = append(out.Expiring, dto.DashboardExpiringLot{
				IngredientID: lot.IngredientID,
				Quantity:     lot.Quantity,
				Unit:         lot.Unit,
				BestBefore:   lot.BestBefore,
			})
		}
	}

	return out, nil
}

// countLots sums the lots across every location in the household.
func (s *Dashboard) countLots(ctx context.Context, locations []dto.PantryLocation) (int, error) {
	total := 0
	for _, loc := range locations {
		lots, err := s.db.ListLotsUnderLocation(ctx, loc.ID)
		if err != nil {
			return 0, fmt.Errorf("service: count lots: %w", err)
		}
		total += len(lots)
	}
	return total, nil
}
