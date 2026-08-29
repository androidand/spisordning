package dto

import "errors"

// TonightView mirrors api/openapi.yaml components/schemas/TonightView. It is a
// wire type, so it lives in dto (the shared wire layer) rather than httpapi —
// this lets the service layer (e.g. the dashboard) depend on it without
// importing httpapi.
type TonightView struct {
	ServedOn  string                 `json:"served_on"`
	Recipe    RecipeRefResponse      `json:"recipe"`
	Reactions []MealReactionResponse `json:"reactions"`
}

// ErrNoMealTonight is returned when there is no approved plan with a decision
// for today. Handlers map this to HTTP 404.
var ErrNoMealTonight = errors.New("no meal tonight")
