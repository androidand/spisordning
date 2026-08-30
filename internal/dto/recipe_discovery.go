package dto

import (
	"context"
	"time"
)

// DiscoverRecipeInput is the POST /recipes/discover request body.
type DiscoverRecipeInput struct {
	URL      string `json:"url"`
	SourceID string `json:"source_id,omitempty"` // defaults to 'web-jsonld'
}

// ImportCandidateIngredientResponse is one parsed ingredient line of a candidate.
type ImportCandidateIngredientResponse struct {
	LineNo       int     `json:"line_no"`
	RawText      string  `json:"raw_text"`
	Quantity     *float64 `json:"quantity,omitempty"`
	Unit         string  `json:"unit"`
	IngredientID *string `json:"ingredient_id,omitempty"`
	NeedsReview  bool    `json:"needs_review"`
}

// ImportCandidateResponse is the HTTP-side view of a recipe_import_candidate.
type ImportCandidateResponse struct {
	ID                string                             `json:"id"`
	SourceID          string                             `json:"source_id"`
	SourceURL         string                             `json:"source_url"`
	ExternalID        *string                            `json:"external_id,omitempty"`
	Title             string                             `json:"title"`
	Description       *string                            `json:"description,omitempty"`
	ImageURL          *string                            `json:"image_url,omitempty"`
	Servings          *int                               `json:"servings,omitempty"`
	PrepTimeSec       *int                               `json:"prep_time_sec,omitempty"`
	CookTimeSec       *int                               `json:"cook_time_sec,omitempty"`
	TotalTimeSec      *int                               `json:"total_time_sec,omitempty"`
	Category          *string                            `json:"category,omitempty"`
	Cuisine           *string                            `json:"cuisine,omitempty"`
	Attribution       *string                            `json:"attribution,omitempty"`
	Rating            *float64                           `json:"rating,omitempty"`
	RatingCount       *int                               `json:"rating_count,omitempty"`
	LicenseNote       *string                            `json:"license_note,omitempty"`
	ImportedAt        time.Time                          `json:"imported_at"`
	Status            string                             `json:"status"`
	PromotedVariantID *string                            `json:"promoted_variant_id,omitempty"`
	Ingredients       []ImportCandidateIngredientResponse `json:"ingredients"`
}

// PromoteCandidateInput is the POST /recipes/discovery/candidates/{id}/promote request body.
type PromoteCandidateInput struct {
	FamilyID *string `json:"family_id,omitempty"` // reuse existing family; creates new if unset
}

// PromoteCandidateResponse is the response from promoting a candidate.
type PromoteCandidateResponse struct {
	FamilyID     string `json:"family_id"`
	VariantID    string `json:"variant_id"`
	RevisionID   string `json:"revision_id"`
	CandidateStatus string `json:"candidate_status"`
}

// DiscoveryService is the surface the /recipes/discover handlers need.
type DiscoveryService interface {
	// DiscoverFromURL fetches a recipe URL, extracts JSON-LD, and stages a candidate.
	DiscoverFromURL(ctx context.Context, in DiscoverRecipeInput) (ImportCandidateResponse, error)
	// ListCandidates returns staged candidates, optionally filtered by status.
	ListCandidates(ctx context.Context, status *string) ([]ImportCandidateResponse, error)
	// GetCandidate returns one candidate with its ingredient lines.
	GetCandidate(ctx context.Context, id string) (ImportCandidateResponse, error)
	// RejectCandidate marks a non-promoted candidate as rejected.
	RejectCandidate(ctx context.Context, id string) error
	// PromoteCandidate creates native recipe content from a candidate.
	PromoteCandidate(ctx context.Context, id string, in PromoteCandidateInput) (PromoteCandidateResponse, error)
}
