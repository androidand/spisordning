package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Household mirrors migrations/0008_household_catalog_minimal.sql household.
// Deliberately minimal: no account/membership modeling — see that migration's
// header comment and establish-household-and-catalog's own, larger scope.
type Household struct {
	ID        domain.HouseholdID
	Slug      string
	Name      string
	CreatedAt time.Time
}

// CreateHousehold inserts a household.
func (s *Store) CreateHousehold(ctx context.Context, h Household) error {
	const q = `INSERT INTO household (id, slug, name) VALUES ($1, $2, $3)`
	if _, err := s.db.Exec(ctx, q, h.ID, h.Slug, h.Name); err != nil {
		return fmt.Errorf("persistence: create household: %w", err)
	}
	return nil
}

// GetHousehold fetches one household by id.
func (s *Store) GetHousehold(ctx context.Context, id domain.HouseholdID) (Household, error) {
	const q = `SELECT id, slug, name, created_at FROM household WHERE id = $1`
	var h Household
	if err := s.db.QueryRow(ctx, q, id).Scan(&h.ID, &h.Slug, &h.Name, &h.CreatedAt); err != nil {
		return Household{}, fmt.Errorf("persistence: get household: %w", err)
	}
	return h, nil
}

// Product mirrors migrations/0008_household_catalog_minimal.sql product — a
// concrete, purchasable good, distinct from the canonical Ingredient it may
// (optionally) map to.
type Product struct {
	ID          domain.ProductID
	Slug        string
	Name        string
	Brand       string
	PackageSize string
	CreatedAt   time.Time
}

// CreateProduct inserts a product.
func (s *Store) CreateProduct(ctx context.Context, p Product) error {
	const q = `INSERT INTO product (id, slug, name, brand, package_size) VALUES ($1, $2, $3, $4, $5)`
	brand := nullableText(p.Brand)
	pkg := nullableText(p.PackageSize)
	if _, err := s.db.Exec(ctx, q, p.ID, p.Slug, p.Name, brand, pkg); err != nil {
		return fmt.Errorf("persistence: create product: %w", err)
	}
	return nil
}

// GetProduct fetches one product by id.
func (s *Store) GetProduct(ctx context.Context, id domain.ProductID) (Product, error) {
	const q = `SELECT id, slug, name, brand, package_size, created_at FROM product WHERE id = $1`
	var p Product
	var brand, pkg *string
	if err := s.db.QueryRow(ctx, q, id).Scan(&p.ID, &p.Slug, &p.Name, &brand, &pkg, &p.CreatedAt); err != nil {
		return Product{}, fmt.Errorf("persistence: get product: %w", err)
	}
	if brand != nil {
		p.Brand = *brand
	}
	if pkg != nil {
		p.PackageSize = *pkg
	}
	return p, nil
}

// ListProducts returns every registered product, ordered by id. Not used
// today — ListCandidateProductsForIngredient runs its own SQL for the
// name-match fallback rather than calling this. Exposed for potential
// future catalog-browse UI.
func (s *Store) ListProducts(ctx context.Context) ([]Product, error) {
	const q = `SELECT id, slug, name, brand, package_size, created_at FROM product ORDER BY id`
	rows, err := s.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("persistence: list products: %w", err)
	}
	defer rows.Close()
	var out []Product
	for rows.Next() {
		var p Product
		var brand, pkg *string
		if err := rows.Scan(&p.ID, &p.Slug, &p.Name, &brand, &pkg, &p.CreatedAt); err != nil {
			return nil, err
		}
		if brand != nil {
			p.Brand = *brand
		}
		if pkg != nil {
			p.PackageSize = *pkg
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// UpsertProductIdentifier links a normalized GTIN to a product. Re-linking an
// already-known GTIN to a different product overwrites the link — barcode
// resolution (implement-pantry-inventory design.md D6) always wants the
// latest confirmed match, never a stale one.
func (s *Store) UpsertProductIdentifier(ctx context.Context, productID domain.ProductID, gtin string) error {
	const q = `INSERT INTO product_identifier (scheme, value, product_id) VALUES ('GTIN', $2, $1)
		ON CONFLICT (scheme, value) DO UPDATE SET product_id = EXCLUDED.product_id`
	if _, err := s.db.Exec(ctx, q, productID, gtin); err != nil {
		return fmt.Errorf("persistence: upsert product_identifier: %w", err)
	}
	return nil
}

// LookupProductByGTIN resolves a normalized GTIN to a product id via
// product_identifier. Returns "", nil (not an error) when no identifier row
// matches — the caller (LookupBarcode) is responsible for falling through to
// the rest of D6's resolution chain.
func (s *Store) LookupProductByGTIN(ctx context.Context, gtin string) (domain.ProductID, error) {
	const q = `SELECT product_id FROM product_identifier WHERE scheme = 'GTIN' AND value = $1`
	var productID domain.ProductID
	err := s.db.QueryRow(ctx, q, gtin).Scan(&productID)
	if err == pgx.ErrNoRows {
		return domain.ProductID{}, nil
	}
	if err != nil {
		return domain.ProductID{}, fmt.Errorf("persistence: lookup product_identifier: %w", err)
	}
	return productID, nil
}

// ProductIngredientMapping mirrors migrations/0008_household_catalog_minimal.sql
// product_ingredient_mapping. A Product may map to more than one Ingredient
// (e.g. a spice mix); absence of any row for a product_id means "unmapped,
// flagged for review" (ingredient-catalog spec) — never invented.
type ProductIngredientMapping struct {
	ProductID    domain.ProductID
	IngredientID domain.IngredientID
	Quantity     *float64
}

// SetProductIngredientMapping records (or re-confirms) that productID maps to
// ingredientID, optionally with a yield quantity. Idempotent on the
// (product_id, ingredient_id) primary key.
func (s *Store) SetProductIngredientMapping(ctx context.Context, m ProductIngredientMapping) error {
	const q = `INSERT INTO product_ingredient_mapping (product_id, ingredient_id, quantity)
		VALUES ($1, $2, $3)
		ON CONFLICT (product_id, ingredient_id) DO UPDATE SET quantity = EXCLUDED.quantity`
	if _, err := s.db.Exec(ctx, q, m.ProductID, m.IngredientID, m.Quantity); err != nil {
		return fmt.Errorf("persistence: set product_ingredient_mapping: %w", err)
	}
	return nil
}

// ListProductsForIngredient returns every product already mapped to
// ingredientID, ordered by product id. This is the primary (mapped) source
// ListCandidateProductsForIngredient (internal/persistence/pantry.go) reads
// from before falling back to a name match.
func (s *Store) ListProductsForIngredient(ctx context.Context, ingredientID domain.IngredientID) ([]Product, error) {
	const q = `SELECT p.id, p.slug, p.name, p.brand, p.package_size, p.created_at
		FROM product p
		JOIN product_ingredient_mapping m ON m.product_id = p.id
		WHERE m.ingredient_id = $1
		ORDER BY p.id`
	rows, err := s.db.Query(ctx, q, ingredientID)
	if err != nil {
		return nil, fmt.Errorf("persistence: list products for ingredient: %w", err)
	}
	defer rows.Close()
	var out []Product
	for rows.Next() {
		var p Product
		var brand, pkg *string
		if err := rows.Scan(&p.ID, &p.Slug, &p.Name, &brand, &pkg, &p.CreatedAt); err != nil {
			return nil, err
		}
		if brand != nil {
			p.Brand = *brand
		}
		if pkg != nil {
			p.PackageSize = *pkg
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// IngredientAlias mirrors migrations/000016_ingredient_alias.sql. It maps a
// household's nickname for an ingredient to the canonical ingredient id.
// HouseholdID is "" for a global alias (NULL in the DB).
type IngredientAlias struct {
	ID           string
	HouseholdID  string // "" = global
	Alias        string
	IngredientID string
	CreatedAt    time.Time
}

// UpsertIngredientAlias inserts or re-asserts a nickname → canonical
// ingredient mapping. The unique (household_id, alias) key makes this
// idempotent: re-adding the same alias updates the ingredient it points at.
func (s *Store) UpsertIngredientAlias(ctx context.Context, a IngredientAlias) error {
	id := uuid.New().String()
	const q = `INSERT INTO ingredient_alias (id, household_id, alias, ingredient_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (household_id, alias) DO UPDATE SET ingredient_id = EXCLUDED.ingredient_id`
	_, err := s.db.Exec(ctx, q, id, nullableText(a.HouseholdID), a.Alias, a.IngredientID)
	if err != nil {
		return fmt.Errorf("persistence: upsert ingredient_alias: %w", err)
	}
	return nil
}

// GetIngredientAlias fetches one alias by (household, alias). householdID ""
// matches the global-alias row (NULL household_id).
func (s *Store) GetIngredientAlias(ctx context.Context, householdID, alias string) (IngredientAlias, error) {
	const q = `SELECT id, household_id, alias, ingredient_id, created_at
		FROM ingredient_alias WHERE household_id IS NOT DISTINCT FROM $1 AND alias = $2`
	var a IngredientAlias
	err := s.db.QueryRow(ctx, q, nullableText(householdID), alias).
		Scan(&a.ID, &a.HouseholdID, &a.Alias, &a.IngredientID, &a.CreatedAt)
	if err != nil {
		return IngredientAlias{}, fmt.Errorf("persistence: get ingredient_alias: %w", err)
	}
	return a, nil
}

// ListIngredientAliases returns every alias for a household, including global
// aliases (household_id IS NULL). householdID "" returns only global aliases.
func (s *Store) ListIngredientAliases(ctx context.Context, householdID string) ([]IngredientAlias, error) {
	const q = `SELECT id, household_id, alias, ingredient_id, created_at
		FROM ingredient_alias
		WHERE household_id IS NULL OR household_id = $1
		ORDER BY alias`
	rows, err := s.db.Query(ctx, q, householdID)
	if err != nil {
		return nil, fmt.Errorf("persistence: list ingredient_alias: %w", err)
	}
	defer rows.Close()
	var out []IngredientAlias
	for rows.Next() {
		var a IngredientAlias
		var hh *string
		if err := rows.Scan(&a.ID, &hh, &a.Alias, &a.IngredientID, &a.CreatedAt); err != nil {
			return nil, err
		}
		if hh != nil {
			a.HouseholdID = *hh
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// DeleteIngredientAlias removes one alias by (household, alias). householdID ""
// targets the global-alias row.
func (s *Store) DeleteIngredientAlias(ctx context.Context, householdID, alias string) error {
	const q = `DELETE FROM ingredient_alias WHERE household_id IS NOT DISTINCT FROM $1 AND alias = $2`
	if _, err := s.db.Exec(ctx, q, nullableText(householdID), alias); err != nil {
		return fmt.Errorf("persistence: delete ingredient_alias: %w", err)
	}
	return nil
}

// ResolveIngredientAlias returns the canonical ingredient id for a nickname,
// or "" when no alias matches. It checks the household's aliases first, then
// global aliases.
func (s *Store) ResolveIngredientAlias(ctx context.Context, householdID, alias string) (string, error) {
	const q = `SELECT ingredient_id FROM ingredient_alias
		WHERE alias = $2 AND (household_id = $1 OR household_id IS NULL)
		ORDER BY (household_id IS NULL) ASC
		LIMIT 1`
	var ingredientID string
	err := s.db.QueryRow(ctx, q, householdID, alias).Scan(&ingredientID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("persistence: resolve ingredient_alias: %w", err)
	}
	return ingredientID, nil
}

func nullableText(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
