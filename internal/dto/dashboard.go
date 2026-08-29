package dto

import (
	"context"
	"time"
)

// DashboardTonight is the dashboard's view of tonight's meal. Null-safe: the
// dashboard reports "nothing planned" rather than erroring when there is no
// approved decision for today.
type DashboardTonight struct {
	ServedOn string          `json:"served_on"`
	Recipe   RecipeRefResponse `json:"recipe"`
}

// DashboardExpiringLot is one pantry lot that is expired or expiring soon.
type DashboardExpiringLot struct {
	IngredientID string    `json:"ingredient_id"`
	Quantity     float64   `json:"quantity"`
	Unit         string    `json:"unit"`
	BestBefore   time.Time `json:"best_before,omitempty"`
}

// DashboardPantry is the dashboard's summary of pantry state.
type DashboardPantry struct {
	Locations int `json:"locations"`
	Lots      int `json:"lots"`
	Expiring  int `json:"expiring"`
}

// Dashboard is the aggregate read model behind the "widgets" / home-screen use
// case: tonight's meal, a pantry summary, and the most urgent expiring items —
// in a single round-trip instead of three separate page loads.
type Dashboard struct {
	Tonight  *DashboardTonight      `json:"tonight,omitempty"`
	Pantry   DashboardPantry        `json:"pantry"`
	Expiring []DashboardExpiringLot `json:"expiring"`
}

// DashboardService is the read surface for the dashboard / widgets endpoint.
type DashboardService interface {
	// Get returns the aggregate dashboard for the given household.
	Get(ctx context.Context, householdID string) (Dashboard, error)
}
