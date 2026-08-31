package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/androidand/spisordning/internal/dto"
	"github.com/androidand/spisordning/internal/persistence"
	"github.com/androidand/spisordning/internal/recipeimport"
)

// Discovery implements dto.DiscoveryService. It fetches external recipes,
// stages them as review candidates, and promotes them into the native
// recipe_family hierarchy.
type Discovery struct {
	db      Store
	family  dto.RecipeFamilyService
	httpClient *http.Client
}

// NewDiscovery returns a Discovery service backed by db and family.
// httpClient may be nil; a default HTTP client is used when nil.
func NewDiscovery(db Store, family dto.RecipeFamilyService, httpClient *http.Client) *Discovery {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Discovery{db: db, family: family, httpClient: httpClient}
}

// DiscoverFromURL fetches a recipe URL, extracts schema.org/Recipe JSON-LD,
// parses it, and stages a review candidate.
func (s *Discovery) DiscoverFromURL(ctx context.Context, in dto.DiscoverRecipeInput) (dto.ImportCandidateResponse, error) {
	if in.URL == "" {
		return dto.ImportCandidateResponse{}, fmt.Errorf("service: discover: url is required")
	}
	u := strings.TrimSpace(in.URL)
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		return dto.ImportCandidateResponse{}, fmt.Errorf("service: discover: url must start with http:// or https://")
	}

	// Resolve the source.
	sourceID := in.SourceID
	if sourceID == "" {
		sourceID = "web-jsonld"
	}
	src, err := s.db.GetExternalRecipeSource(ctx, sourceID)
	if err != nil {
		return dto.ImportCandidateResponse{}, fmt.Errorf("service: discover: resolve source %q: %w", sourceID, err)
	}
	if !src.Enabled {
		return dto.ImportCandidateResponse{}, fmt.Errorf("service: discover: source %q is disabled", sourceID)
	}

	// Fetch the page.
	resp, err := s.httpClient.Get(u)
	if err != nil {
		return dto.ImportCandidateResponse{}, fmt.Errorf("service: discover: fetch %s: %w", u, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return dto.ImportCandidateResponse{}, fmt.Errorf("service: discover: fetch %s: status %d", u, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return dto.ImportCandidateResponse{}, fmt.Errorf("service: discover: read %s: %w", u, err)
	}
	buf := body

	// Extract and parse JSON-LD.
	node, err := recipeimport.ExtractRecipeJSONLD(string(buf))
	if err != nil {
		return dto.ImportCandidateResponse{}, fmt.Errorf("service: discover: extract JSON-LD from %s: %w", u, err)
	}
	parsed, err := recipeimport.ParseRecipe(node)
	if err != nil {
		return dto.ImportCandidateResponse{}, fmt.Errorf("service: discover: parse recipe from %s: %w", u, err)
	}

	// Build the candidate.
	raw := recipeimport.CandidateFromParsed(recipeimport.Source{
		ID:          src.ID,
		Name:        src.Name,
		Kind:        src.Kind,
		BaseURL:     src.BaseURL,
		LicenseNote: src.LicenseNote,
		Decision:    src.Decision,
		Enabled:     src.Enabled,
	}, u, parsed)

	// Map to persistence type.
	c := persistence.ImportCandidate{
		SourceID:      raw.SourceID,
		SourceURL:     raw.SourceURL,
		Title:         parsed.Title,
		Description:   parsed.Description,
		ImageURL:      ptrToStr(parsed.ImageURL),
		Servings:      ptrToInt(parsed.Servings),
		PrepTimeSec:   ptrToInt(parsed.PrepSec),
		CookTimeSec:   ptrToInt(parsed.CookSec),
		TotalTimeSec:  ptrToInt(parsed.TotalSec),
		Category:      ptrToStr(parsed.Category),
		Cuisine:       ptrToStr(parsed.Cuisine),
		Attribution:   ptrToStr(parsed.Attribution),
		Rating:        ptrToFloat(parsed.Rating),
		RatingCount:   ptrToInt(parsed.RatingCount),
		Nutrition:     parsed.Nutrition,
		RawJSONLD:     raw.Parsed.RawJSONLD,
		LicenseNote:   ptrToStr(raw.LicenseNote),
		Status:        string(recipeimport.StatusCandidate),
	}
	cid, err := s.db.SaveImportCandidate(ctx, c)
	if err != nil {
		return dto.ImportCandidateResponse{}, fmt.Errorf("service: discover: save candidate: %w", err)
	}

	// Persist ingredient lines.
	lines := make([]persistence.ImportCandidateIngredient, 0, len(parsed.Ingredients))
	for _, ing := range parsed.Ingredients {
		lines = append(lines, persistence.ImportCandidateIngredient{
			LineNo:      ing.LineNo,
			RawText:     ing.RawText,
			Quantity:    ptrToFloat(ing.Quantity),
			Unit:        ing.Unit,
			NeedsReview: ing.NeedsReview,
		})
	}
	if err := s.db.SaveCandidateIngredients(ctx, cid, lines); err != nil {
		return dto.ImportCandidateResponse{}, fmt.Errorf("service: discover: save ingredients: %w", err)
	}

	return s.get_candidate(ctx, cid)
}

// ListCandidates returns staged candidates, optionally filtered by status.
func (s *Discovery) ListCandidates(ctx context.Context, status *string) ([]dto.ImportCandidateResponse, error) {
	cands, err := s.db.ListImportCandidates(ctx, status)
	if err != nil {
		return nil, fmt.Errorf("service: list candidates: %w", err)
	}
	out := make([]dto.ImportCandidateResponse, 0, len(cands))
	for _, c := range cands {
		ings, ierr := s.db.ListCandidateIngredients(ctx, c.ID)
		if ierr != nil {
			return nil, fmt.Errorf("service: list candidates: ingredients for %s: %w", c.ID, ierr)
		}
		out = append(out, candidate_to_dto(c, ings))
	}
	return out, nil
}

// GetCandidate returns one candidate with its ingredient lines.
func (s *Discovery) GetCandidate(ctx context.Context, id string) (dto.ImportCandidateResponse, error) {
	cid, err := domain.ParseRecipeImportCandidateID(id)
	if err != nil {
		return dto.ImportCandidateResponse{}, fmt.Errorf("service: get candidate: %w", err)
	}
	c, err := s.db.GetImportCandidate(ctx, cid)
	if err != nil {
		if errors.Is(err, persistence.ErrNoRows) {
			return dto.ImportCandidateResponse{}, fmt.Errorf("service: get candidate: %w", dto.ErrNotFound)
		}
		return dto.ImportCandidateResponse{}, fmt.Errorf("service: get candidate: %w", err)
	}
	ings, ierr := s.db.ListCandidateIngredients(ctx, cid)
	if ierr != nil {
		return dto.ImportCandidateResponse{}, fmt.Errorf("service: get candidate: ingredients: %w", ierr)
	}
	return candidate_to_dto(c, ings), nil
}

// RejectCandidate marks a non-promoted candidate as rejected.
func (s *Discovery) RejectCandidate(ctx context.Context, id string) error {
	cid, err := domain.ParseRecipeImportCandidateID(id)
	if err != nil {
		return fmt.Errorf("service: reject candidate: %w", err)
	}
	c, err := s.db.GetImportCandidate(ctx, cid)
	if err != nil {
		if errors.Is(err, persistence.ErrNoRows) {
			return fmt.Errorf("service: reject candidate: %w", dto.ErrNotFound)
		}
		return fmt.Errorf("service: reject candidate: %w", err)
	}
	if c.Status == string(recipeimport.StatusPromoted) {
		return fmt.Errorf("service: reject candidate: already promoted")
	}
	if err := s.db.SetCandidateStatus(ctx, cid, string(recipeimport.StatusRejected)); err != nil {
		return fmt.Errorf("service: reject candidate: %w", err)
	}
	return nil
}

// PromoteCandidate creates native recipe content from a candidate. If the
// candidate is already promoted, returns the existing linked content.
func (s *Discovery) PromoteCandidate(ctx context.Context, id string, in dto.PromoteCandidateInput) (dto.PromoteCandidateResponse, error) {
	cid, err := domain.ParseRecipeImportCandidateID(id)
	if err != nil {
		return dto.PromoteCandidateResponse{}, fmt.Errorf("service: promote candidate: %w", err)
	}
	c, err := s.db.GetImportCandidate(ctx, cid)
	if err != nil {
		if errors.Is(err, persistence.ErrNoRows) {
			return dto.PromoteCandidateResponse{}, fmt.Errorf("service: promote candidate: %w", dto.ErrNotFound)
		}
		return dto.PromoteCandidateResponse{}, fmt.Errorf("service: promote candidate: %w", err)
	}
	if c.Status == string(recipeimport.StatusPromoted) {
		// Idempotent: return existing linked content.
		if c.PromotedVariantID == nil {
			return dto.PromoteCandidateResponse{}, fmt.Errorf("service: promote candidate: already promoted but no variant linked")
		}
		variant, verr := s.db.GetRecipeVariant(ctx, *c.PromotedVariantID)
		if verr != nil {
			return dto.PromoteCandidateResponse{}, fmt.Errorf("service: promote candidate: get variant: %w", verr)
		}
		fam, fer := s.db.GetRecipeFamily(ctx, variant.FamilyID)
		if fer != nil {
			return dto.PromoteCandidateResponse{}, fmt.Errorf("service: promote candidate: get family: %w", fer)
		}
		// Find the latest revision for this variant.
		revs, rerr := s.db.ListRecipeRevisions(ctx, *c.PromotedVariantID)
		if rerr != nil {
			return dto.PromoteCandidateResponse{}, fmt.Errorf("service: promote candidate: list revisions: %w", rerr)
		}
		var latestRev domain.RecipeRevisionID
		if len(revs) > 0 {
			latestRev = revs[0].ID
		}
		return dto.PromoteCandidateResponse{
			FamilyID:        fam.ID.String(),
			VariantID:       variant.ID.String(),
			RevisionID:      latestRev.String(),
			CandidateStatus: c.Status,
		}, nil
	}

	// Create or reuse the target family.
	var famID domain.RecipeFamilyID
	var newFamily bool
	if in.FamilyID != nil && *in.FamilyID != "" {
		famID, err = domain.ParseRecipeFamilyID(*in.FamilyID)
		if err != nil {
			return dto.PromoteCandidateResponse{}, fmt.Errorf("service: promote candidate: invalid family_id: %w", err)
		}
	} else {
		// Create a new family from the candidate's title.
		slug := domain.CanonicalIngredientID(c.Title)
		f := persistence.RecipeFamily{
			ID:        domain.NewRecipeFamilyID(),
			Slug:      slug,
			Name:      c.Title,
			Archived:  false,
			CreatedAt: time.Now(),
		}
		if err := s.db.CreateRecipeFamily(ctx, f); err != nil {
			return dto.PromoteCandidateResponse{}, fmt.Errorf("service: promote candidate: create family: %w", err)
		}
		famID = f.ID
		newFamily = true
	}

	// Create the variant.
	v := persistence.RecipeVariant{
		ID:                domain.NewRecipeVariantID(),
		Slug:              domain.CanonicalIngredientID(c.Title),
		FamilyID:          famID,
		Title:             c.Title,
		SourceAttribution: c.SourceURL,
		Archived:          false,
		CreatedAt:         time.Now(),
	}
	if err := s.db.CreateRecipeVariant(ctx, v); err != nil {
		return dto.PromoteCandidateResponse{}, fmt.Errorf("service: promote candidate: create variant: %w", err)
	}
	varID := v.ID

	// Re-parse from raw_jsonld to get instructions and ingredients.
	parsedForPromo, err := recipeimport.ParseRecipe(c.RawJSONLD)
	if err != nil {
		return dto.PromoteCandidateResponse{}, fmt.Errorf("service: promote candidate: re-parse raw jsonld: %w", err)
	}
	ingredients := make([]domain.Ingredient, 0, len(parsedForPromo.Ingredients))
	for _, ing := range parsedForPromo.Ingredients {
		ingID := domain.IngredientIDForName(domain.CanonicalIngredientID(ing.Food))
		ingredients = append(ingredients, domain.Ingredient{
			IngredientID: ingID.String(),
			Quantity:     ing.Quantity,
			Unit:         ing.Unit,
			RawText:      ing.RawText,
		})
	}
	rev := persistence.RecipeRevision{
		VariantID:   varID,
		Servings:    ptrDerefOr(c.Servings, 0),
		Description: c.Description,
		Ingredients: ingredients,
		Steps:       parsedForPromo.Instructions,
		CreatedAt:   time.Now(),
	}
	revID, err := s.db.CreateRecipeRevision(ctx, rev)
	if err != nil {
		return dto.PromoteCandidateResponse{}, fmt.Errorf("service: promote candidate: create revision: %w", err)
	}

	// Set the new variant as default when a new family was created.
	if newFamily {
		if err := s.db.SetRecipeFamilyDefaultVariant(ctx, famID, varID); err != nil {
			return dto.PromoteCandidateResponse{}, fmt.Errorf("service: promote candidate: set default variant: %w", err)
		}
	}

	// Mark the candidate promoted.
	if err := s.db.SetCandidatePromoted(ctx, cid, varID); err != nil {
		return dto.PromoteCandidateResponse{}, fmt.Errorf("service: promote candidate: mark promoted: %w", err)
	}

	return dto.PromoteCandidateResponse{
		FamilyID:        famID.String(),
		VariantID:       varID.String(),
		RevisionID:      revID.String(),
		CandidateStatus: string(recipeimport.StatusPromoted),
	}, nil
}

// get_candidate is an internal helper that fetches a candidate and its
// ingredient lines, returning the DTO.
func (s *Discovery) get_candidate(ctx context.Context, id domain.RecipeImportCandidateID) (dto.ImportCandidateResponse, error) {
	c, err := s.db.GetImportCandidate(ctx, id)
	if err != nil {
		return dto.ImportCandidateResponse{}, err
	}
	ings, ierr := s.db.ListCandidateIngredients(ctx, id)
	if ierr != nil {
		return dto.ImportCandidateResponse{}, fmt.Errorf("service: get candidate: ingredients: %w", ierr)
	}
	return candidate_to_dto(c, ings), nil
}

// candidate_to_dto maps a persistence.ImportCandidate plus its ingredient
// lines to the wire DTO.
func candidate_to_dto(c persistence.ImportCandidate, ings []persistence.ImportCandidateIngredient) dto.ImportCandidateResponse {
	out := dto.ImportCandidateResponse{
		ID:                c.ID.String(),
		SourceID:          c.SourceID,
		SourceURL:         c.SourceURL,
		Title:             c.Title,
		Status:            c.Status,
		ImportedAt:        c.ImportedAt,
		PromotedVariantID: nilOrString(c.PromotedVariantID),
	}
	if c.ExternalID != nil {
		out.ExternalID = c.ExternalID
	}
	if c.Description != "" {
		out.Description = &c.Description
	}
	if c.ImageURL != nil {
		out.ImageURL = c.ImageURL
	}
	if c.Servings != nil {
		out.Servings = c.Servings
	}
	if c.PrepTimeSec != nil {
		out.PrepTimeSec = c.PrepTimeSec
	}
	if c.CookTimeSec != nil {
		out.CookTimeSec = c.CookTimeSec
	}
	if c.TotalTimeSec != nil {
		out.TotalTimeSec = c.TotalTimeSec
	}
	if c.Category != nil {
		out.Category = c.Category
	}
	if c.Cuisine != nil {
		out.Cuisine = c.Cuisine
	}
	if c.Attribution != nil {
		out.Attribution = c.Attribution
	}
	if c.Rating != nil {
		out.Rating = c.Rating
	}
	if c.RatingCount != nil {
		out.RatingCount = c.RatingCount
	}
	if c.LicenseNote != nil {
		out.LicenseNote = c.LicenseNote
	}
	out.Ingredients = make([]dto.ImportCandidateIngredientResponse, 0, len(ings))
	for _, ing := range ings {
		resp := dto.ImportCandidateIngredientResponse{
			LineNo:      ing.LineNo,
			RawText:     ing.RawText,
			Unit:        ing.Unit,
			NeedsReview: ing.NeedsReview,
		}
		if ing.Quantity != nil {
			resp.Quantity = ing.Quantity
		}
		if ing.IngredientID != (domain.IngredientID{}) {
			s := ing.IngredientID.String()
			resp.IngredientID = &s
		}
		out.Ingredients = append(out.Ingredients, resp)
	}
	return out
}

// ── helpers ──────────────────────────────────────────────────────────────────

func ptrToStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func ptrToInt(n int) *int {
	if n == 0 {
		return nil
	}
	return &n
}

func ptrToFloat(f float64) *float64 {
	if f == 0 {
		return nil
	}
	return &f
}

func ptrDerefOr[T comparable](p *T, zero T) T {
	if p == nil {
		return zero
	}
	return *p
}

func ptrDerefOrStr(p *string, zero string) string {
	if p == nil {
		return zero
	}
	return *p
}

func nilOrString(u *domain.RecipeVariantID) *string {
	if u == nil || u.String() == "00000000-0000-0000-0000-000000000000" {
		return nil
	}
	s := u.String()
	return &s
}
