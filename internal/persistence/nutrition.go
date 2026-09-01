// Package persistence — nutrition repositories (research-nutrition-data-sources
// task 5/6): `foods`, `nutrients`, `product_mappings`, and the nutrition sync
// status row that tracks the last successful full sync per source.
package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/jackc/pgx/v5"
)

// Food mirrors foods (migration 000021). The SLV nummer is the canonical
// nutrition key — the rest of the nutrition namespace resolves through it.
type Food struct {
	SlvNummer        int
	Namn             string
	VetenskapligtNamn  *string
	LivsmedelsTyp    *string
	Projekt          *string
	Version          *string
	SyncedAt         time.Time
}

// Nutrient mirrors nutrients (migration 000021), one row per (food, nutrient).
type Nutrient struct {
	FoodNummer  int
	EuroFIRKod *string
	Name        string
	Värde       float64
	Enhet       string
	Metodtyp    *string
	SyncedAt    time.Time
}

// ProductMapping mirrors product_mappings (migration 000021): the cross-reference
// glue that resolves a GTIN / Dabas Arident to a canonical SLV food (and, via
// Foods.SlvNummer, to a nutrition profile).
type ProductMapping struct {
	ID                    int
	GTIN                  *string
	DabasARIdent          *string
	FoodsSLVNummer        *int
	CanonicalIngredientID *domain.IngredientID
	MappedAt              time.Time
}

// NutritionSyncStatus captures the last successful full sync per source
// (one row per source key). It powers incremental updates and observability.
type NutritionSyncStatus struct {
	Source      string
	LastSynced  time.Time
	RecordCount int
}

// UpsertFood inserts a food or refreshes its nullable fields on conflict.
// Keyed on the SLV nummer (PK). Idempotent for syncs.
func (s *Store) UpsertFood(ctx context.Context, f Food) error {
	const q = `INSERT INTO foods (slv_nummer, namn, venskapligtNamn, livsmedels_typ, projekt, version, synced_at)
		VALUES ($1, $2, $3, $4, $5, $6, now())
		ON CONFLICT (slv_nummer) DO UPDATE SET
			namn = EXCLUDED.namn,
			venskapligtNamn = EXCLUDED.venskapligtNamn,
			livsmedels_typ = EXCLUDED.livsmedels_typ,
			projekt = EXCLUDED.projekt,
			version = EXCLUDED.version,
			synced_at = now()`
	if _, err := s.db.Exec(ctx, q, f.SlvNummer, f.Namn, f.VetenskapligtNamn, f.LivsmedelsTyp, f.Projekt, f.Version); err != nil {
		return fmt.Errorf("persistence: upsert food %d: %w", f.SlvNummer, err)
	}
	return nil
}

// UpsertNutrients replaces all nutrients for a food. It deletes the food's
// existing nutrient rows first (so a re-sync drops removed nutrients) then
// inserts the fresh set.
func (s *Store) UpsertNutrients(ctx context.Context, foodNummer int, nutrients []Nutrient) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("persistence: begin nutrients: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback is a no-op if committed

	if _, err := tx.Exec(ctx, `DELETE FROM nutrients WHERE food_nummer = $1`, foodNummer); err != nil {
		return fmt.Errorf("persistence: delete nutrients for food %d: %w", foodNummer, err)
	}
	upsert := `INSERT INTO nutrients (food_nummer, eurofir_kod, namn, varde, enhet, metodtyp, synced_at)
		VALUES ($1, $2, $3, $4, $5, $6, now())
		ON CONFLICT (food_nummer, namn, enhet) DO UPDATE SET
			eurofir_kod = EXCLUDED.eurofir_kod,
			varde = EXCLUDED.varde,
			metodtyp = EXCLUDED.metodtyp,
			synced_at = now()`
	for _, n := range nutrients {
		if _, err := tx.Exec(ctx, upsert, n.FoodNummer, n.EuroFIRKod, n.Name, n.Värde, n.Enhet, n.Metodtyp); err != nil {
			return fmt.Errorf("persistence: upsert nutrient %q for food %d: %w", n.Name, foodNummer, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("persistence: commit nutrients for food %d: %w", foodNummer, err)
	}
	return nil
}

// GetFood returns one food by SLV nummer, or an error wrapping pgx.ErrNoRows.
func (s *Store) GetFood(ctx context.Context, nummer int) (Food, error) {
	const q = `SELECT slv_nummer, namn, venskapligtNamn, livsmedels_typ, projekt, version, synced_at
		FROM foods WHERE slv_nummer = $1`
	var f Food
	if err := s.db.QueryRow(ctx, q, nummer).Scan(
		&f.SlvNummer, &f.Namn, &f.VetenskapligtNamn, &f.LivsmedelsTyp, &f.Projekt, &f.Version, &f.SyncedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return Food{}, fmt.Errorf("persistence: no food %d: %w", nummer, pgx.ErrNoRows)
		}
		return Food{}, fmt.Errorf("persistence: get food %d: %w", nummer, err)
	}
	return f, nil
}

// ListFoods returns all foods, ordered by SLV nummer.
func (s *Store) ListFoods(ctx context.Context) ([]Food, error) {
	const q = `SELECT slv_nummer, namn, venskapligtNamn, livsmedels_typ, projekt, version, synced_at
		FROM foods ORDER BY slv_nummer`
	rows, err := s.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("persistence: list foods: %w", err)
	}
	defer rows.Close()
	var out []Food
	for rows.Next() {
		var f Food
		if err := rows.Scan(&f.SlvNummer, &f.Namn, &f.VetenskapligtNamn, &f.LivsmedelsTyp, &f.Projekt, &f.Version, &f.SyncedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// CountFoods returns the number of foods in the table (sync progress reporting).
func (s *Store) CountFoods(ctx context.Context) (int, error) {
	const q = `SELECT count(*) FROM foods`
	var n int
	if err := s.db.QueryRow(ctx, q).Scan(&n); err != nil {
		return 0, fmt.Errorf("persistence: count foods: %w", err)
	}
	return n, nil
}

// UpsertProductMapping inserts or updates a product mapping. Exactly one of
// GTIN / DabasARIdent / FoodsSLVNummer is populated by the caller; the row is
// the "this product = this SLV food" statement. Keyed on (gtin, arident).
func (s *Store) UpsertProductMapping(ctx context.Context, m ProductMapping) error {
	var canonical *domain.IngredientID
	if m.CanonicalIngredientID != nil {
		c := *m.CanonicalIngredientID
		canonical = &c
	}
	const q = `INSERT INTO product_mappings (gtin, dabas_arident, slv_nummer, canonical_ingredient_id, mapped_at)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (gtin, dabas_arident) DO UPDATE SET
			slv_nummer = EXCLUDED.slv_nummer,
			canonical_ingredient_id = EXCLUDED.canonical_ingredient_id,
			mapped_at = now()`
	if _, err := s.db.Exec(ctx, q, m.GTIN, m.DabasARIdent, m.FoodsSLVNummer, canonical); err != nil {
		return fmt.Errorf("persistence: upsert product mapping: %w", err)
	}
	return nil
}

// GetProductMappingByGTIN returns the mapping for a GTIN, or an error wrapping
// pgx.ErrNoRows.
func (s *Store) GetProductMappingByGTIN(ctx context.Context, gtin string) (ProductMapping, error) {
	const q = `SELECT id, gtin, dabas_arident, slv_nummer, canonical_ingredient_id, mapped_at
		FROM product_mappings WHERE gtin = $1`
	var m ProductMapping
	var canonical *domain.IngredientID
	if err := s.db.QueryRow(ctx, q, gtin).Scan(
		&m.ID, &m.GTIN, &m.DabasARIdent, &m.FoodsSLVNummer, &canonical, &m.MappedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return ProductMapping{}, fmt.Errorf("persistence: no mapping for gtin %q: %w", gtin, pgx.ErrNoRows)
		}
		return ProductMapping{}, fmt.Errorf("persistence: get mapping by gtin %q: %w", gtin, err)
	}
	m.CanonicalIngredientID = canonical
	return m, nil
}

// GetProductMappingByDabasARIdent returns the mapping for a Dabas Arident, or an
// error wrapping pgx.ErrNoRows.
func (s *Store) GetProductMappingByDabasARIdent(ctx context.Context, arident string) (ProductMapping, error) {
	const q = `SELECT id, gtin, dabas_arident, slv_nummer, canonical_ingredient_id, mapped_at
		FROM product_mappings WHERE dabas_arident = $1`
	var m ProductMapping
	var canonical *domain.IngredientID
	if err := s.db.QueryRow(ctx, q, arident).Scan(
		&m.ID, &m.GTIN, &m.DabasARIdent, &m.FoodsSLVNummer, &canonical, &m.MappedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return ProductMapping{}, fmt.Errorf("persistence: no mapping for arident %q: %w", arident, pgx.ErrNoRows)
		}
		return ProductMapping{}, fmt.Errorf("persistence: get mapping by arident %q: %w", arident, err)
	}
	m.CanonicalIngredientID = canonical
	return m, nil
}

// GetNutritionForFood returns all nutrients for a food, ordered by name.
func (s *Store) GetNutritionForFood(ctx context.Context, foodNummer int) ([]Nutrient, error) {
	const q = `SELECT food_nummer, eurofir_kod, namn, varde, enhet, metodtyp, synced_at
		FROM nutrients WHERE food_nummer = $1 ORDER BY namn`
	rows, err := s.db.Query(ctx, q, foodNummer)
	if err != nil {
		return nil, fmt.Errorf("persistence: get nutrition for food %d: %w", foodNummer, err)
	}
	defer rows.Close()
	var out []Nutrient
	for rows.Next() {
		var n Nutrient
		if err := rows.Scan(&n.FoodNummer, &n.EuroFIRKod, &n.Name, &n.Värde, &n.Enhet, &n.Metodtyp, &n.SyncedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// UpsertNutritionSyncStatus records a successful full sync for a source, keyed
// on (source). Idempotent for sync commands.
func (s *Store) UpsertNutritionSyncStatus(ctx context.Context, status NutritionSyncStatus) error {
	const q = `INSERT INTO nutrition_sync_status (source, last_synced, record_count)
		VALUES ($1, now(), $2)
		ON CONFLICT (source) DO UPDATE SET
			last_synced = now(),
			record_count = EXCLUDED.record_count`
	if _, err := s.db.Exec(ctx, q, status.Source, status.RecordCount); err != nil {
		return fmt.Errorf("persistence: upsert nutrition sync status %q: %w", status.Source, err)
	}
	return nil
}

// GetNutritionSyncStatus returns the last full-sync status for a source, or an
// error wrapping pgx.ErrNoRows if none exists.
func (s *Store) GetNutritionSyncStatus(ctx context.Context, source string) (NutritionSyncStatus, error) {
	const q = `SELECT source, last_synced, record_count FROM nutrition_sync_status WHERE source = $1`
	var st NutritionSyncStatus
	if err := s.db.QueryRow(ctx, q, source).Scan(&st.Source, &st.LastSynced, &st.RecordCount); err != nil {
		if err == pgx.ErrNoRows {
			return NutritionSyncStatus{}, fmt.Errorf("persistence: no sync status for %q: %w", source, pgx.ErrNoRows)
		}
		return NutritionSyncStatus{}, fmt.Errorf("persistence: get nutrition sync status %q: %w", source, err)
	}
	return st, nil
}
