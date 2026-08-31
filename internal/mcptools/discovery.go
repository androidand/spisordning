package mcptools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ── Discovery tool input/output ─────────────────────────────────────────────

// DiscoverRecipeInput is the input for the discover_recipe tool.
type DiscoverRecipeInput struct {
	// URL is the absolute http(s) URL of the recipe page to fetch.
	URL string `json:"url"`
	// SourceID optionally selects the external recipe source; defaults to web-jsonld.
	SourceID string `json:"source_id,omitempty"`
}

// ListImportCandidatesInput is the input for the list_import_candidates tool.
type ListImportCandidatesInput struct {
	// Status optionally filters candidates by status (candidate, promoted, rejected).
	Status string `json:"status,omitempty"`
}

// GetImportCandidateInput is the input for the get_import_candidate tool.
type GetImportCandidateInput struct {
	// ID is the candidate id.
	ID string `json:"id"`
}

// RejectImportCandidateInput is the input for the reject_import_candidate tool.
type RejectImportCandidateInput struct {
	// ID is the candidate id to reject.
	ID string `json:"id"`
}

// PromoteImportCandidateInput is the input for the promote_import_candidate tool.
type PromoteImportCandidateInput struct {
	// ID is the candidate id to promote.
	ID string `json:"id"`
	// FamilyID optionally reuses an existing recipe family; a new family is
	// created when omitted.
	FamilyID string `json:"family_id,omitempty"`
}

// DiscoveryIngredient is one parsed ingredient line of a staged candidate.
type DiscoveryIngredient struct {
	LineNo       int      `json:"line_no"`
	RawText      string   `json:"raw_text"`
	Quantity     *float64 `json:"quantity,omitempty"`
	Unit         string   `json:"unit"`
	IngredientID *string  `json:"ingredient_id,omitempty"`
	NeedsReview  bool     `json:"needs_review"`
}

// ImportCandidate is the MCP-side view of a staged external recipe awaiting
// review or promotion.
type ImportCandidate struct {
	ID                string                `json:"id"`
	SourceID          string                `json:"source_id"`
	SourceURL         string                `json:"source_url"`
	ExternalID        *string               `json:"external_id,omitempty"`
	Title             string                `json:"title"`
	Description       *string               `json:"description,omitempty"`
	ImageURL          *string               `json:"image_url,omitempty"`
	Servings          *int                  `json:"servings,omitempty"`
	PrepTimeSec       *int                  `json:"prep_time_sec,omitempty"`
	CookTimeSec       *int                  `json:"cook_time_sec,omitempty"`
	TotalTimeSec      *int                  `json:"total_time_sec,omitempty"`
	Category          *string               `json:"category,omitempty"`
	Cuisine           *string               `json:"cuisine,omitempty"`
	Attribution       *string               `json:"attribution,omitempty"`
	Rating            *float64              `json:"rating,omitempty"`
	RatingCount       *int                  `json:"rating_count,omitempty"`
	LicenseNote       *string               `json:"license_note,omitempty"`
	ImportedAt        time.Time             `json:"imported_at"`
	Status            string                `json:"status"`
	PromotedVariantID *string               `json:"promoted_variant_id,omitempty"`
	Ingredients       []DiscoveryIngredient `json:"ingredients"`
}

// RejectCandidateResult is the output of the reject_import_candidate tool.
type RejectCandidateResult struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// PromoteCandidateResult is the output of the promote_import_candidate tool.
type PromoteCandidateResult struct {
	FamilyID        string `json:"family_id"`
	VariantID       string `json:"variant_id"`
	RevisionID      string `json:"revision_id"`
	CandidateStatus string `json:"candidate_status"`
}

// ── Discovery service interface ─────────────────────────────────────────────

// DiscoveryService fetches external recipe URLs, stages review candidates, and
// promotes/rejects them into the native recipe_family hierarchy. The composition
// root implements it by delegating to the application-layer discovery service.
type DiscoveryService interface {
	DiscoverFromURL(ctx context.Context, in DiscoverRecipeInput) (ImportCandidate, error)
	ListCandidates(ctx context.Context, status *string) ([]ImportCandidate, error)
	GetCandidate(ctx context.Context, id string) (ImportCandidate, error)
	RejectCandidate(ctx context.Context, id string) error
	PromoteCandidate(ctx context.Context, id string, familyID *string) (PromoteCandidateResult, error)
}

// ── Discovery tool handlers ─────────────────────────────────────────────────

func discoverRecipeHandler(s DiscoveryService) mcp.ToolHandlerFor[DiscoverRecipeInput, ImportCandidate] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in DiscoverRecipeInput) (*mcp.CallToolResult, ImportCandidate, error) {
		if strings.TrimSpace(in.URL) == "" {
			return nil, ImportCandidate{}, fmt.Errorf("discover_recipe: url is required")
		}
		res, err := s.DiscoverFromURL(ctx, in)
		if err != nil {
			return nil, ImportCandidate{}, err
		}
		return nil, res, nil
	}
}

func listImportCandidatesHandler(s DiscoveryService) mcp.ToolHandlerFor[ListImportCandidatesInput, []ImportCandidate] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListImportCandidatesInput) (*mcp.CallToolResult, []ImportCandidate, error) {
		var status *string
		if in.Status != "" {
			status = &in.Status
		}
		res, err := s.ListCandidates(ctx, status)
		if err != nil {
			return nil, nil, err
		}
		return nil, res, nil
	}
}

func getImportCandidateHandler(s DiscoveryService) mcp.ToolHandlerFor[GetImportCandidateInput, ImportCandidate] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in GetImportCandidateInput) (*mcp.CallToolResult, ImportCandidate, error) {
		if strings.TrimSpace(in.ID) == "" {
			return nil, ImportCandidate{}, fmt.Errorf("get_import_candidate: id is required")
		}
		res, err := s.GetCandidate(ctx, in.ID)
		if err != nil {
			return nil, ImportCandidate{}, err
		}
		return nil, res, nil
	}
}

func rejectImportCandidateHandler(s DiscoveryService) mcp.ToolHandlerFor[RejectImportCandidateInput, RejectCandidateResult] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in RejectImportCandidateInput) (*mcp.CallToolResult, RejectCandidateResult, error) {
		if strings.TrimSpace(in.ID) == "" {
			return nil, RejectCandidateResult{}, fmt.Errorf("reject_import_candidate: id is required")
		}
		if err := s.RejectCandidate(ctx, in.ID); err != nil {
			return nil, RejectCandidateResult{}, err
		}
		return nil, RejectCandidateResult{ID: in.ID, Status: "rejected"}, nil
	}
}

func promoteImportCandidateHandler(s DiscoveryService) mcp.ToolHandlerFor[PromoteImportCandidateInput, PromoteCandidateResult] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in PromoteImportCandidateInput) (*mcp.CallToolResult, PromoteCandidateResult, error) {
		if strings.TrimSpace(in.ID) == "" {
			return nil, PromoteCandidateResult{}, fmt.Errorf("promote_import_candidate: id is required")
		}
		var familyID *string
		if in.FamilyID != "" {
			familyID = &in.FamilyID
		}
		res, err := s.PromoteCandidate(ctx, in.ID, familyID)
		if err != nil {
			return nil, PromoteCandidateResult{}, err
		}
		return nil, res, nil
	}
}
