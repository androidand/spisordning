package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/androidand/spisordning/internal/domain"
)

// ExternalRecipeSource mirrors migrations/000002_recipe_discovery.sql
// external_recipe_source.
type ExternalRecipeSource struct {
	ID          string
	Name        string
	Kind        string
	BaseURL     string
	LicenseNote string
	Decision    string
	Enabled     bool
	CreatedAt   time.Time
}

// GetExternalRecipeSource fetches one source by its stable-code id.
func (s *Store) GetExternalRecipeSource(ctx context.Context, id string) (ExternalRecipeSource, error) {
	const q = `SELECT id, name, kind, COALESCE(base_url, ''), COALESCE(license_note, ''),
		decision, enabled, created_at
		FROM external_recipe_source WHERE id = $1`
	var src ExternalRecipeSource
	err := s.db.QueryRow(ctx, q, id).Scan(
		&src.ID, &src.Name, &src.Kind, &src.BaseURL, &src.LicenseNote,
		&src.Decision, &src.Enabled, &src.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ExternalRecipeSource{}, fmt.Errorf("persistence: get external recipe source %q: %w", id, pgx.ErrNoRows)
		}
		return ExternalRecipeSource{}, fmt.Errorf("persistence: get external recipe source: %w", err)
	}
	return src, nil
}

// UpsertExternalRecipeSource inserts or refreshes a source. The id is the
// stable-code primary key; name, kind, etc. are refreshed on upsert.
func (s *Store) UpsertExternalRecipeSource(ctx context.Context, src ExternalRecipeSource) error {
	const q = `INSERT INTO external_recipe_source (id, name, kind, base_url, license_note, decision, enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			kind = EXCLUDED.kind,
			base_url = EXCLUDED.base_url,
			license_note = EXCLUDED.license_note,
			decision = EXCLUDED.decision,
			enabled = EXCLUDED.enabled`
	if _, err := s.db.Exec(ctx, q, src.ID, src.Name, src.Kind, src.BaseURL, src.LicenseNote, src.Decision, src.Enabled); err != nil {
		return fmt.Errorf("persistence: upsert external recipe source: %w", err)
	}
	return nil
}

// ImportCandidate mirrors migrations/000002_recipe_discovery.sql
// recipe_import_candidate.
type ImportCandidate struct {
	ID                domain.RecipeImportCandidateID
	SourceID          string
	SourceURL         string
	ExternalID        *string
	Title             string
	Description       string
	ImageURL          *string
	Servings          *int
	PrepTimeSec       *int
	CookTimeSec       *int
	TotalTimeSec      *int
	Category          *string
	Cuisine           *string
	Attribution       *string
	Rating            *float64
	RatingCount       *int
	Nutrition         []byte
	RawJSONLD         []byte
	LicenseNote       *string
	ImportedAt        time.Time
	FirstServedAt     *time.Time
	Status            string
	PromotedVariantID *domain.RecipeVariantID
}

// SaveImportCandidate upserts a candidate using the existing unique indexes:
// (source_id, external_id) when external_id is set, else (source_url).
func (s *Store) SaveImportCandidate(ctx context.Context, c ImportCandidate) error {
	if c.ID == (domain.RecipeImportCandidateID{}) {
		c.ID = domain.NewRecipeImportCandidateID()
	}
	const baseQ = `INSERT INTO recipe_import_candidate
		(id, source_id, source_url, external_id, title, description, image_url,
		 servings, prep_time_sec, cook_time_sec, total_time_sec,
		 category, cuisine, attribution, rating, rating_count,
		 nutrition, raw_jsonld, license_note, imported_at, first_served_at,
		 status, promoted_variant_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, now(), $19, $20, $21)
		ON CONFLICT DO UPDATE SET
			title = EXCLUDED.title,
			description = EXCLUDED.description,
			image_url = EXCLUDED.image_url,
			servings = EXCLUDED.servings,
			prep_time_sec = EXCLUDED.prep_time_sec,
			cook_time_sec = EXCLUDED.cook_time_sec,
			total_time_sec = EXCLUDED.total_time_sec,
			category = EXCLUDED.category,
			cuisine = EXCLUDED.cuisine,
			attribution = EXCLUDED.attribution,
			rating = EXCLUDED.rating,
			rating_count = EXCLUDED.rating_count,
			nutrition = EXCLUDED.nutrition,
			raw_jsonld = EXCLUDED.raw_jsonld,
			license_note = EXCLUDED.license_note`

	if c.ExternalID != nil && *c.ExternalID != "" {
		const q = baseQ + ` WHERE (source_id, external_id) = ($2, $4)`
		if _, err := s.db.Exec(ctx, q,
			c.ID, c.SourceID, c.SourceURL, *c.ExternalID, c.Title, c.Description,
			nullableTextStr(c.ImageURL), c.Servings, c.PrepTimeSec, c.CookTimeSec,
			c.TotalTimeSec, c.Category, c.Cuisine, c.Attribution, c.Rating,
			c.RatingCount, c.Nutrition, c.RawJSONLD, c.LicenseNote,
			c.Status, c.PromotedVariantID,
		); err != nil {
			return fmt.Errorf("persistence: save import candidate (by external_id): %w", err)
		}
		return nil
	}
	const q = baseQ + ` WHERE source_url = $3`
	if _, err := s.db.Exec(ctx, q,
		c.ID, c.SourceID, c.SourceURL, nil, c.Title, c.Description,
		nullableTextStr(c.ImageURL), c.Servings, c.PrepTimeSec, c.CookTimeSec,
		c.TotalTimeSec, c.Category, c.Cuisine, c.Attribution, c.Rating,
		c.RatingCount, c.Nutrition, c.RawJSONLD, c.LicenseNote,
		c.Status, c.PromotedVariantID,
	); err != nil {
		return fmt.Errorf("persistence: save import candidate (by url): %w", err)
	}
	return nil
}

// nullableTextStr returns a *string pointing at s, or nil if s is empty.
func nullableTextStr(s *string) *string {
	if s == nil || *s == "" {
		return nil
	}
	return s
}

// SaveCandidateIngredients replaces all ingredient lines for a candidate.
// Lines are ordered by line_no; existing lines for this candidate are deleted
// and re-inserted so the final set matches `lines` exactly.
func (s *Store) SaveCandidateIngredients(ctx context.Context, candidateID domain.RecipeImportCandidateID, lines []ImportCandidateIngredient) error {
	const deleteQ = `DELETE FROM recipe_import_candidate_ingredient WHERE candidate_id = $1`
	if _, err := s.db.Exec(ctx, deleteQ, candidateID); err != nil {
		return fmt.Errorf("persistence: delete candidate ingredients: %w", err)
	}
	const insertQ = `INSERT INTO recipe_import_candidate_ingredient
		(candidate_id, line_no, raw_text, ingredient_id, quantity, unit, needs_review)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	for _, line := range lines {
		var ingID *domain.IngredientID
		if line.IngredientID != (domain.IngredientID{}) {
			ingID = &line.IngredientID
		}
		if _, err := s.db.Exec(ctx, insertQ,
			candidateID, line.LineNo, line.RawText, ingID,
			line.Quantity, line.Unit, line.NeedsReview,
		); err != nil {
			return fmt.Errorf("persistence: insert candidate ingredient line %d: %w", line.LineNo, err)
		}
	}
	return nil
}

// ImportCandidateIngredient mirrors migrations/000002_recipe_discovery.sql
// recipe_import_candidate_ingredient.
type ImportCandidateIngredient struct {
	CandidateID  domain.RecipeImportCandidateID
	LineNo       int
	RawText      string
	IngredientID domain.IngredientID
	Quantity     *float64
	Unit         string
	NeedsReview  bool
}

// GetImportCandidate fetches one candidate.
func (s *Store) GetImportCandidate(ctx context.Context, id domain.RecipeImportCandidateID) (ImportCandidate, error) {
	const q = `SELECT id, source_id, source_url, external_id, title, description,
		image_url, servings, prep_time_sec, cook_time_sec, total_time_sec,
		category, cuisine, attribution, rating, rating_count, nutrition,
		raw_jsonld, license_note, imported_at, first_served_at, status,
		promoted_variant_id
		FROM recipe_import_candidate WHERE id = $1`
	var c ImportCandidate
	var (
		extID          *string
		imageURL       *string
		servings       *int
		prepTime       *int
		cookTime       *int
		totalTime      *int
		category       *string
		cuisine        *string
		attribution    *string
		rating         *float64
		ratingCount    *int
		nutrition      []byte
		rawJSONLD      []byte
		licenseNote    *string
		firstServedAt  *time.Time
		promotedVarID  *domain.RecipeVariantID
	)
	err := s.db.QueryRow(ctx, q, id).Scan(
		&c.ID, &c.SourceID, &c.SourceURL, &extID, &c.Title, &c.Description,
		&imageURL, &servings, &prepTime, &cookTime, &totalTime,
		&category, &cuisine, &attribution, &rating, &ratingCount,
		&nutrition, &rawJSONLD, &licenseNote, &c.ImportedAt, &firstServedAt,
		&c.Status, &promotedVarID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ImportCandidate{}, fmt.Errorf("persistence: get import candidate %s: %w", id, pgx.ErrNoRows)
		}
		return ImportCandidate{}, fmt.Errorf("persistence: get import candidate: %w", err)
	}
	c.ExternalID = extID
	c.ImageURL = imageURL
	c.Servings = servings
	c.PrepTimeSec = prepTime
	c.CookTimeSec = cookTime
	c.TotalTimeSec = totalTime
	c.Category = category
	c.Cuisine = cuisine
	c.Attribution = attribution
	c.Rating = rating
	c.RatingCount = ratingCount
	c.Nutrition = nutrition
	c.RawJSONLD = rawJSONLD
	c.LicenseNote = licenseNote
	c.FirstServedAt = firstServedAt
	c.PromotedVariantID = promotedVarID
	return c, nil
}

// ListImportCandidates returns candidates ordered by imported_at DESC,
// optionally filtered by status.
func (s *Store) ListImportCandidates(ctx context.Context, status *string) ([]ImportCandidate, error) {
	var q string
	var args []interface{}
	if status != nil && *status != "" {
		q = `SELECT id, source_id, source_url, external_id, title, description,
			image_url, servings, prep_time_sec, cook_time_sec, total_time_sec,
			category, cuisine, attribution, rating, rating_count, nutrition,
			raw_jsonld, license_note, imported_at, first_served_at, status,
			promoted_variant_id
			FROM recipe_import_candidate WHERE status = $1
			ORDER BY imported_at DESC`
		args = append(args, *status)
	} else {
		q = `SELECT id, source_id, source_url, external_id, title, description,
			image_url, servings, prep_time_sec, cook_time_sec, total_time_sec,
			category, cuisine, attribution, rating, rating_count, nutrition,
			raw_jsonld, license_note, imported_at, first_served_at, status,
			promoted_variant_id
			FROM recipe_import_candidate
			ORDER BY imported_at DESC`
	}
	rows, err := s.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("persistence: list import candidates: %w", err)
	}
	defer rows.Close()
	var out []ImportCandidate
	for rows.Next() {
		var c ImportCandidate
		var (
			extID          *string
			imageURL       *string
			servings       *int
			prepTime       *int
			cookTime       *int
			totalTime      *int
			category       *string
			cuisine        *string
			attribution    *string
			rating         *float64
			ratingCount    *int
			nutrition      []byte
			rawJSONLD      []byte
			licenseNote    *string
			firstServedAt  *time.Time
			promotedVarID  *domain.RecipeVariantID
		)
		if err := rows.Scan(
			&c.ID, &c.SourceID, &c.SourceURL, &extID, &c.Title, &c.Description,
			&imageURL, &servings, &prepTime, &cookTime, &totalTime,
			&category, &cuisine, &attribution, &rating, &ratingCount,
			&nutrition, &rawJSONLD, &licenseNote, &c.ImportedAt, &firstServedAt,
			&c.Status, &promotedVarID,
		); err != nil {
			return nil, err
		}
		c.ExternalID = extID
		c.ImageURL = imageURL
		c.Servings = servings
		c.PrepTimeSec = prepTime
		c.CookTimeSec = cookTime
		c.TotalTimeSec = totalTime
		c.Category = category
		c.Cuisine = cuisine
		c.Attribution = attribution
		c.Rating = rating
		c.RatingCount = ratingCount
		c.Nutrition = nutrition
		c.RawJSONLD = rawJSONLD
		c.LicenseNote = licenseNote
		c.FirstServedAt = firstServedAt
		c.PromotedVariantID = promotedVarID
		out = append(out, c)
	}
	return out, rows.Err()
}

// SetCandidateStatus updates a candidate's status.
func (s *Store) SetCandidateStatus(ctx context.Context, id domain.RecipeImportCandidateID, status string) error {
	const q = `UPDATE recipe_import_candidate SET status = $2 WHERE id = $1`
	if _, err := s.db.Exec(ctx, q, id, status); err != nil {
		return fmt.Errorf("persistence: set candidate status: %w", err)
	}
	return nil
}

// SetCandidatePromoted marks a candidate as promoted and links the variant.
func (s *Store) SetCandidatePromoted(ctx context.Context, id domain.RecipeImportCandidateID, variantID domain.RecipeVariantID) error {
	const q = `UPDATE recipe_import_candidate
		SET status = 'promoted', promoted_variant_id = $2
		WHERE id = $1`
	if _, err := s.db.Exec(ctx, q, id, variantID); err != nil {
		return fmt.Errorf("persistence: set candidate promoted: %w", err)
	}
	return nil
}

// ListCandidateIngredients returns the ingredient lines for a candidate,
// ordered by line_no.
func (s *Store) ListCandidateIngredients(ctx context.Context, candidateID domain.RecipeImportCandidateID) ([]ImportCandidateIngredient, error) {
	const q = `SELECT candidate_id, line_no, raw_text, ingredient_id, quantity, unit, needs_review
		FROM recipe_import_candidate_ingredient
		WHERE candidate_id = $1 ORDER BY line_no`
	rows, err := s.db.Query(ctx, q, candidateID)
	if err != nil {
		return nil, fmt.Errorf("persistence: list candidate ingredients: %w", err)
	}
	defer rows.Close()
	var out []ImportCandidateIngredient
	for rows.Next() {
		var line ImportCandidateIngredient
		var qty *float64
		if err := rows.Scan(&line.CandidateID, &line.LineNo, &line.RawText, &line.IngredientID, &qty, &line.Unit, &line.NeedsReview); err != nil {
			return nil, err
		}
		line.Quantity = qty
		out = append(out, line)
	}
	return out, rows.Err()
}
