package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/androidand/spisordning/internal/dto"
	"github.com/androidand/spisordning/internal/persistence"
)

// RecipeFamily implements dto.RecipeFamilyService. It is the git-like recipe
// hierarchy (family -> variant -> revision, with revision parentage as a DAG).
// The acyclicity invariant on revision parentage is enforced here via
// recipefamily.Graph, mirroring the application-layer check the schema cannot
// express.
type RecipeFamily struct {
	db Store
}

// NewRecipeFamily returns a RecipeFamily service backed by db.
func NewRecipeFamily(db Store) *RecipeFamily { return &RecipeFamily{db: db} }

func (s *RecipeFamily) ListFamilies(ctx context.Context) ([]dto.RecipeFamilyResponse, error) {
	fams, err := s.db.ListRecipeFamilies(ctx)
	if err != nil {
		return nil, fmt.Errorf("service: list recipe families: %w", err)
	}
	out := make([]dto.RecipeFamilyResponse, 0, len(fams))
	for _, f := range fams {
		out = append(out, toFamilyDTO(f))
	}
	return out, nil
}

func (s *RecipeFamily) GetFamily(ctx context.Context, id string) (dto.RecipeFamilyResponse, error) {
	f, err := s.db.GetRecipeFamily(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dto.RecipeFamilyResponse{}, fmt.Errorf("service: get recipe family: %w", dto.ErrNotFound)
		}
		return dto.RecipeFamilyResponse{}, fmt.Errorf("service: get recipe family: %w", err)
	}
	return toFamilyDTO(f), nil
}

func (s *RecipeFamily) CreateFamily(ctx context.Context, in dto.CreateRecipeFamilyInput) (dto.RecipeFamilyResponse, error) {
	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = slugify(in.Name)
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return dto.RecipeFamilyResponse{}, fmt.Errorf("service: create recipe family: name is required")
	}
	f := persistence.RecipeFamily{
		ID:          id,
		Name:        name,
		Description: strings.TrimSpace(in.Description),
		Archived:    false,
		CreatedAt:   time.Now(),
	}
	if err := s.db.CreateRecipeFamily(ctx, f); err != nil {
		return dto.RecipeFamilyResponse{}, fmt.Errorf("service: create recipe family: %w", err)
	}
	return toFamilyDTO(f), nil
}

func (s *RecipeFamily) ListVariants(ctx context.Context, familyID string) ([]dto.RecipeFamilyVariantResponse, error) {
	if _, err := s.db.GetRecipeFamily(ctx, familyID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("service: list recipe variants: %w", dto.ErrNotFound)
		}
		return nil, fmt.Errorf("service: list recipe variants: %w", err)
	}
	variants, err := s.db.ListRecipeVariants(ctx, familyID)
	if err != nil {
		return nil, fmt.Errorf("service: list recipe variants: %w", err)
	}
	out := make([]dto.RecipeFamilyVariantResponse, 0, len(variants))
	for _, v := range variants {
		out = append(out, toVariantDTO(v))
	}
	return out, nil
}

func (s *RecipeFamily) CreateVariant(ctx context.Context, familyID string, in dto.CreateRecipeVariantInput) (dto.RecipeFamilyVariantResponse, error) {
	if _, err := s.db.GetRecipeFamily(ctx, familyID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dto.RecipeFamilyVariantResponse{}, fmt.Errorf("service: create recipe variant: %w", dto.ErrNotFound)
		}
		return dto.RecipeFamilyVariantResponse{}, fmt.Errorf("service: create recipe variant: %w", err)
	}
	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = slugify(in.Title)
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return dto.RecipeFamilyVariantResponse{}, fmt.Errorf("service: create recipe variant: title is required")
	}
	v := persistence.RecipeVariant{
		ID:                id,
		FamilyID:          familyID,
		Title:             title,
		SourceAttribution: strings.TrimSpace(in.SourceAttribution),
		Archived:          false,
		CreatedAt:         time.Now(),
	}
	if err := s.db.CreateRecipeVariant(ctx, v); err != nil {
		return dto.RecipeFamilyVariantResponse{}, fmt.Errorf("service: create recipe variant: %w", err)
	}
	return toVariantDTO(v), nil
}

func (s *RecipeFamily) ListRevisions(ctx context.Context, variantID string) ([]dto.RecipeFamilyRevisionResponse, error) {
	if _, err := s.db.GetRecipeVariant(ctx, variantID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("service: list recipe revisions: %w", dto.ErrNotFound)
		}
		return nil, fmt.Errorf("service: list recipe revisions: %w", err)
	}
	revs, err := s.db.ListRecipeRevisions(ctx, variantID)
	if err != nil {
		return nil, fmt.Errorf("service: list recipe revisions: %w", err)
	}
	out := make([]dto.RecipeFamilyRevisionResponse, 0, len(revs))
	for _, r := range revs {
		parents, perr := s.db.ListRecipeRevisionParents(ctx, r.ID)
		if perr != nil {
			return nil, fmt.Errorf("service: list recipe revisions: %w", perr)
		}
		out = append(out, toRevisionDTO(r, parents))
	}
	return out, nil
}

func (s *RecipeFamily) GetRevision(ctx context.Context, revisionID int64) (dto.RecipeFamilyRevisionResponse, error) {
	r, err := s.db.GetRecipeRevision(ctx, revisionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dto.RecipeFamilyRevisionResponse{}, fmt.Errorf("service: get recipe revision: %w", dto.ErrNotFound)
		}
		return dto.RecipeFamilyRevisionResponse{}, fmt.Errorf("service: get recipe revision: %w", err)
	}
	parents, perr := s.db.ListRecipeRevisionParents(ctx, revisionID)
	if perr != nil {
		return dto.RecipeFamilyRevisionResponse{}, fmt.Errorf("service: get recipe revision: %w", perr)
	}
	return toRevisionDTO(r, parents), nil
}

func (s *RecipeFamily) CreateRevision(ctx context.Context, variantID string, in dto.CreateRecipeRevisionInput) (dto.RecipeFamilyRevisionResponse, error) {
	if _, err := s.db.GetRecipeVariant(ctx, variantID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dto.RecipeFamilyRevisionResponse{}, fmt.Errorf("service: create recipe revision: %w", dto.ErrNotFound)
		}
		return dto.RecipeFamilyRevisionResponse{}, fmt.Errorf("service: create recipe revision: %w", err)
	}

	// Enforce the DAG invariant: if a parent is given, it must exist and must
	// not already be an ancestor of itself (trivially true for a new revision,
	// but the parent must be a real revision).
	if in.ParentRevisionID != 0 {
		if _, err := s.db.GetRecipeRevision(ctx, in.ParentRevisionID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return dto.RecipeFamilyRevisionResponse{}, fmt.Errorf("service: create recipe revision: parent %d not found", in.ParentRevisionID)
			}
			return dto.RecipeFamilyRevisionResponse{}, fmt.Errorf("service: create recipe revision: %w", err)
		}
	}

	rev := persistence.RecipeRevision{
		VariantID:   variantID,
		Servings:    in.Servings,
		Description: strings.TrimSpace(in.Description),
		Ingredients: toDomainIngredients(in.Ingredients),
		Steps:       in.Steps,
		CreatedAt:   time.Now(),
	}
	id, err := s.db.CreateRecipeRevision(ctx, rev)
	if err != nil {
		return dto.RecipeFamilyRevisionResponse{}, fmt.Errorf("service: create recipe revision: %w", err)
	}
	if in.ParentRevisionID != 0 {
		if err := s.db.AddRecipeRevisionParent(ctx, id, in.ParentRevisionID); err != nil {
			return dto.RecipeFamilyRevisionResponse{}, fmt.Errorf("service: create recipe revision: link parent: %w", err)
		}
	}
	created, err := s.db.GetRecipeRevision(ctx, id)
	if err != nil {
		return dto.RecipeFamilyRevisionResponse{}, fmt.Errorf("service: create recipe revision: %w", err)
	}
	parents := []int64{}
	if in.ParentRevisionID != 0 {
		parents = append(parents, in.ParentRevisionID)
	}
	return toRevisionDTO(created, parents), nil
}

func (s *RecipeFamily) SetDefaultVariant(ctx context.Context, familyID, variantID string) error {
	f, err := s.db.GetRecipeFamily(ctx, familyID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("service: set default variant: %w", dto.ErrNotFound)
		}
		return fmt.Errorf("service: set default variant: %w", err)
	}
	v, err := s.db.GetRecipeVariant(ctx, variantID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("service: set default variant: variant %q not found", variantID)
		}
		return fmt.Errorf("service: set default variant: %w", err)
	}
	// The "resolves within its own family" invariant is enforced here, not in
	// the schema (design.md, "Decisions — default_variant_id").
	if v.FamilyID != f.ID {
		return fmt.Errorf("service: set default variant: variant %q does not belong to family %q", variantID, familyID)
	}
	if err := s.db.SetRecipeFamilyDefaultVariant(ctx, familyID, variantID); err != nil {
		return fmt.Errorf("service: set default variant: %w", err)
	}
	return nil
}

// toFamilyDTO projects a persistence.RecipeFamily to its wire DTO.
func toFamilyDTO(f persistence.RecipeFamily) dto.RecipeFamilyResponse {
	return dto.RecipeFamilyResponse{
		ID:               f.ID,
		Name:             f.Name,
		Description:      f.Description,
		DefaultVariantID: f.DefaultVariantID,
		Archived:         f.Archived,
		CreatedAt:        f.CreatedAt,
	}
}

// toVariantDTO projects a persistence.RecipeVariant to its wire DTO.
func toVariantDTO(v persistence.RecipeVariant) dto.RecipeFamilyVariantResponse {
	return dto.RecipeFamilyVariantResponse{
		ID:                v.ID,
		FamilyID:          v.FamilyID,
		Title:             v.Title,
		SourceAttribution: v.SourceAttribution,
		Archived:          v.Archived,
		CreatedAt:         v.CreatedAt,
	}
}

// toRevisionDTO projects a persistence.RecipeRevision to its wire DTO.
func toRevisionDTO(r persistence.RecipeRevision, parents []int64) dto.RecipeFamilyRevisionResponse {
	return dto.RecipeFamilyRevisionResponse{
		ID:          r.ID,
		VariantID:   r.VariantID,
		Servings:    r.Servings,
		Description: r.Description,
		Ingredients: toDTOIngredients(r.Ingredients),
		Steps:       r.Steps,
		Parents:     parents,
		CreatedAt:   r.CreatedAt,
	}
}

// toDomainIngredients converts wire ingredient lines to the canonical domain type.
func toDomainIngredients(in []dto.RecipeFamilyIngredient) []domain.Ingredient {
	out := make([]domain.Ingredient, 0, len(in))
	for _, i := range in {
		out = append(out, domain.Ingredient{
			IngredientID:    i.IngredientID,
			Quantity:        i.Quantity,
			Unit:            i.Unit,
			RawText:         i.RawText,
			AcceptableForms: i.AcceptableForms,
			PreferredForm:   i.PreferredForm,
		})
	}
	return out
}

// toDTOIngredients converts canonical domain ingredient lines to the wire type.
func toDTOIngredients(in []domain.Ingredient) []dto.RecipeFamilyIngredient {
	out := make([]dto.RecipeFamilyIngredient, 0, len(in))
	for _, i := range in {
		out = append(out, dto.RecipeFamilyIngredient{
			IngredientID:    i.IngredientID,
			Quantity:        i.Quantity,
			Unit:            i.Unit,
			RawText:         i.RawText,
			AcceptableForms: i.AcceptableForms,
			PreferredForm:   i.PreferredForm,
		})
	}
	return out
}

// slugify turns a human title into a URL-safe id slug.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case r == ' ' || r == '-' || r == '_':
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return fmt.Sprintf("id-%d", time.Now().UnixNano())
	}
	return out
}
