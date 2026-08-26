package persistence

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// DefineUnitConversion is the ONLY write path to unit_conversion
// (design.md invariant 11). It upserts a universal same-dimension conversion;
// factor is to = from * factor. The same-dimension rule is enforced in the
// domain layer (domain.NewUnitConversion), not here — this is a thin
// repository method. No trigger or product-registration path ever writes a
// conversion row (the Grocy regression, invariant 11).
func (s *Store) DefineUnitConversion(ctx context.Context, fromUnit, toUnit string, factor float64) error {
	const q = `INSERT INTO unit_conversion (from_unit, to_unit, factor)
		VALUES ($1, $2, $3)
		ON CONFLICT (from_unit, to_unit) DO UPDATE SET factor = EXCLUDED.factor`
	if _, err := s.db.Exec(ctx, q, fromUnit, toUnit, factor); err != nil {
		return fmt.Errorf("persistence: define unit_conversion: %w", err)
	}
	return nil
}

// DefineIngredientUnitConversion is the ONLY write path to
// ingredient_unit_conversion (design.md invariant 11). It inserts an
// ingredient-specific (possibly cross-dimension) conversion. A duplicate
// (ingredient, from, to) is a hard error — the caller must not silently
// overwrite a curated factor.
func (s *Store) DefineIngredientUnitConversion(ctx context.Context, ingredientID, fromUnit, toUnit string, factor float64) error {
	const q = `INSERT INTO ingredient_unit_conversion (ingredient_id, from_unit, to_unit, factor)
		VALUES ($1, $2, $3, $4)`
	if _, err := s.db.Exec(ctx, q, ingredientID, fromUnit, toUnit, factor); err != nil {
		return fmt.Errorf("persistence: define ingredient_unit_conversion: %w", err)
	}
	return nil
}

// CountUnitConversions returns the number of universal conversion rows. Used by
// the invariant-11 regression test to assert product registration auto-creates
// none.
func (s *Store) CountUnitConversions(ctx context.Context) (int, error) {
	var n int
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM unit_conversion`).Scan(&n); err != nil {
		return 0, fmt.Errorf("persistence: count unit_conversion: %w", err)
	}
	return n, nil
}

// CountIngredientUnitConversions returns the number of ingredient-specific
// conversion rows for one ingredient.
func (s *Store) CountIngredientUnitConversions(ctx context.Context, ingredientID string) (int, error) {
	var n int
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM ingredient_unit_conversion WHERE ingredient_id = $1`, ingredientID).Scan(&n); err != nil {
		return 0, fmt.Errorf("persistence: count ingredient_unit_conversion: %w", err)
	}
	return n, nil
}

// GetIngredientUnitConversion returns the factor for an ingredient-specific
// conversion, or found=false when none is defined. Absence is a valid,
// queryable state per invariant 11 — "no conversion defined yet", never a
// silent 1:1 default.
func (s *Store) GetIngredientUnitConversion(ctx context.Context, ingredientID, fromUnit, toUnit string) (factor float64, found bool, err error) {
	const q = `SELECT factor FROM ingredient_unit_conversion
		WHERE ingredient_id = $1 AND from_unit = $2 AND to_unit = $3`
	err = s.db.QueryRow(ctx, q, ingredientID, fromUnit, toUnit).Scan(&factor)
	if err == pgx.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("persistence: get ingredient_unit_conversion: %w", err)
	}
	return factor, true, nil
}
