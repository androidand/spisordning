package dto

import (
	"context"
	"time"
)

// RecipeFamilyIngredient is one structured ingredient line of a recipe revision.
type RecipeFamilyIngredient struct {
	IngredientID    string   `json:"ingredient_id"`
	Quantity        float64  `json:"quantity"`
	Unit            string   `json:"unit"`
	RawText         string   `json:"raw_text"`
	AcceptableForms []string `json:"acceptable_forms,omitempty"`
	PreferredForm   string   `json:"preferred_form,omitempty"`
}

// RecipeFamilyRevisionResponse is one immutable snapshot of a variant's content.
type RecipeFamilyRevisionResponse struct {
	ID          string                     `json:"id"`
	VariantID   string                     `json:"variant_id"`
	Servings    int                        `json:"servings,omitempty"`
	Description string                     `json:"description,omitempty"`
	Ingredients []RecipeFamilyIngredient   `json:"ingredients"`
	Steps       []string                   `json:"steps"`
	Parents     []string                   `json:"parents,omitempty"`
	CreatedAt   time.Time                  `json:"created_at"`
}

// RecipeFamilyVariantResponse is one recognizable fork of a family.
type RecipeFamilyVariantResponse struct {
	ID                string   `json:"id"`
	FamilyID          string   `json:"family_id"`
	Title             string   `json:"title"`
	SourceAttribution string   `json:"source_attribution,omitempty"`
	Archived          bool     `json:"archived"`
	CreatedAt         time.Time `json:"created_at"`
}

// RecipeFamilyResponse is a conceptual dish with its default variant pinned.
type RecipeFamilyResponse struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Description      string   `json:"description,omitempty"`
	DefaultVariantID string   `json:"default_variant_id,omitempty"`
	Archived         bool     `json:"archived"`
	CreatedAt        time.Time `json:"created_at"`
}

// CreateRecipeFamilyInput is the body for POST /recipe-families.
type CreateRecipeFamilyInput struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// CreateRecipeVariantInput is the body for POST /recipe-families/{id}/variants.
type CreateRecipeVariantInput struct {
	ID                string `json:"id"`
	Title             string `json:"title"`
	SourceAttribution string `json:"source_attribution"`
}

// CreateRecipeRevisionInput is the body for
// POST /recipe-families/{id}/variants/{variantId}/revisions.
type CreateRecipeRevisionInput struct {
	Servings    int                      `json:"servings"`
	Description string                   `json:"description"`
	Ingredients []RecipeFamilyIngredient `json:"ingredients"`
	Steps       []string                 `json:"steps"`
	// ParentRevisionID, when set, records that this revision derives from an
	// existing revision (the git-like "commit on top of" relationship).
	ParentRevisionID string `json:"parent_revision_id,omitempty"`
}

// RecipeFamilyService is the read/write surface the /recipe-families handlers
// need. It is the git-like recipe hierarchy: family -> variant -> revision, with
// revision parentage held as a DAG.
type RecipeFamilyService interface {
	ListFamilies(ctx context.Context) ([]RecipeFamilyResponse, error)
	GetFamily(ctx context.Context, id string) (RecipeFamilyResponse, error)
	CreateFamily(ctx context.Context, in CreateRecipeFamilyInput) (RecipeFamilyResponse, error)
	ListVariants(ctx context.Context, familyID string) ([]RecipeFamilyVariantResponse, error)
	CreateVariant(ctx context.Context, familyID string, in CreateRecipeVariantInput) (RecipeFamilyVariantResponse, error)
	ListRevisions(ctx context.Context, variantID string) ([]RecipeFamilyRevisionResponse, error)
	GetRevision(ctx context.Context, revisionID string) (RecipeFamilyRevisionResponse, error)
	CreateRevision(ctx context.Context, variantID string, in CreateRecipeRevisionInput) (RecipeFamilyRevisionResponse, error)
	SetDefaultVariant(ctx context.Context, familyID, variantID string) error
}
