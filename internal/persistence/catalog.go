package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Household mirrors migrations/0008_household_catalog_minimal.sql household.
// Deliberately minimal: no account/membership modeling — see that migration's
// header comment and establish-household-and-catalog's own, larger scope.
type Household struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

// CreateHousehold inserts a household.
func (s *Store) CreateHousehold(ctx context.Context, h Household) error {
	const q = `INSERT INTO household (id, name) VALUES ($1, $2)`
	if _, err := s.db.Exec(ctx, q, h.ID, h.Name); err != nil {
		return fmt.Errorf("persistence: create household: %w", err)
	}
	return nil
}

// GetHousehold fetches one household by id.
func (s *Store) GetHousehold(ctx context.Context, id string) (Household, error) {
	const q = `SELECT id, name, created_at FROM household WHERE id = $1`
	var h Household
	if err := s.db.QueryRow(ctx, q, id).Scan(&h.ID, &h.Name, &h.CreatedAt); err != nil {
		return Household{}, fmt.Errorf("persistence: get household: %w", err)
	}
	return h, nil
}

// Product mirrors migrations/0008_household_catalog_minimal.sql product — a
// concrete, purchasable good, distinct from the canonical Ingredient it may
// (optionally) map to.
type Product struct {
	ID          string
	Name        string
	Brand       string
	PackageSize string
	CreatedAt   time.Time
}

// CreateProduct inserts a product.
func (s *Store) CreateProduct(ctx context.Context, p Product) error {
	const q = `INSERT INTO product (id, name, brand, package_size) VALUES ($1, $2, $3, $4)`
	brand := nullableText(p.Brand)
	pkg := nullableText(p.PackageSize)
	if _, err := s.db.Exec(ctx, q, p.ID, p.Name, brand, pkg); err != nil {
		return fmt.Errorf("persistence: create product: %w", err)
	}
	return nil
}

// GetProduct fetches one product by id.
func (s *Store) GetProduct(ctx context.Context, id string) (Product, error) {
	const q = `SELECT id, name, brand, package_size, created_at FROM product WHERE id = $1`
	var p Product
	var brand, pkg *string
	if err := s.db.QueryRow(ctx, q, id).Scan(&p.ID, &p.Name, &brand, &pkg, &p.CreatedAt); err != nil {
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

// ListProducts returns every registered product, ordered by id. Used today
// only by ListCandidateProductsForIngredient's name-match fallback
// (internal/persistence/pantry.go) — there is no catalog-browse UI yet.
func (s *Store) ListProducts(ctx context.Context) ([]Product, error) {
	const q = `SELECT id, name, brand, package_size, created_at FROM product ORDER BY id`
	rows, err := s.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("persistence: list products: %w", err)
	}
	defer rows.Close()
	var out []Product
	for rows.Next() {
		var p Product
		var brand, pkg *string
		if err := rows.Scan(&p.ID, &p.Name, &brand, &pkg, &p.CreatedAt); err != nil {
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
func (s *Store) UpsertProductIdentifier(ctx context.Context, productID, gtin string) error {
	const q = `INSERT INTO product_identifier (product_id, gtin) VALUES ($1, $2)
		ON CONFLICT (gtin) DO UPDATE SET product_id = EXCLUDED.product_id`
	if _, err := s.db.Exec(ctx, q, productID, gtin); err != nil {
		return fmt.Errorf("persistence: upsert product_identifier: %w", err)
	}
	return nil
}

// LookupProductByGTIN resolves a normalized GTIN to a product id via
// product_identifier. Returns "", nil (not an error) when no identifier row
// matches — the caller (LookupBarcode) is responsible for falling through to
// the rest of D6's resolution chain.
func (s *Store) LookupProductByGTIN(ctx context.Context, gtin string) (string, error) {
	const q = `SELECT product_id FROM product_identifier WHERE gtin = $1`
	var productID string
	err := s.db.QueryRow(ctx, q, gtin).Scan(&productID)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("persistence: lookup product_identifier: %w", err)
	}
	return productID, nil
}

// ProductIngredientMapping mirrors migrations/0008_household_catalog_minimal.sql
// product_ingredient_mapping. A Product may map to more than one Ingredient
// (e.g. a spice mix); absence of any row for a product_id means "unmapped,
// flagged for review" (ingredient-catalog spec) — never invented.
type ProductIngredientMapping struct {
	ProductID    string
	IngredientID string
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
func (s *Store) ListProductsForIngredient(ctx context.Context, ingredientID string) ([]Product, error) {
	const q = `SELECT p.id, p.name, p.brand, p.package_size, p.created_at
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
		if err := rows.Scan(&p.ID, &p.Name, &brand, &pkg, &p.CreatedAt); err != nil {
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

func nullableText(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
