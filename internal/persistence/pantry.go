package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/jackc/pgx/v5"
)

// InventoryLocation mirrors migrations/0009_pantry_inventory.sql
// inventory_location. LocationType and ParentLocationID are both optional
// (design.md D9) — most locations are expected to stay flat
// (ParentLocationID == "").
type InventoryLocation struct {
	ID               string
	HouseholdID      string
	Name             string
	LocationType     string // "" when unset; one of domain's location-type strings otherwise
	ParentLocationID string // "" when this location has no parent
	ArchivedAt       *time.Time
}

// CreateInventoryLocation inserts a location, rejecting a parent assignment
// that would create a cycle (design.md D9, invariant 8). The cycle check
// walks the candidate parent's ancestor chain with a recursive query and
// hands it to domain.WouldCreateLocationCycle for the actual decision.
func (s *Store) CreateInventoryLocation(ctx context.Context, l InventoryLocation) error {
	if l.ParentLocationID != "" {
		ancestors, err := s.locationAncestors(ctx, l.ParentLocationID)
		if err != nil {
			return err
		}
		if domain.WouldCreateLocationCycle(l.ID, l.ParentLocationID, ancestors) {
			return fmt.Errorf("persistence: creating inventory_location %q under %q would create a cycle", l.ID, l.ParentLocationID)
		}
	}
	const q = `INSERT INTO inventory_location (id, household_id, name, location_type, parent_location_id)
		VALUES ($1, $2, $3, $4, $5)`
	locType := nullableText(l.LocationType)
	parent := nullableText(l.ParentLocationID)
	if _, err := s.db.Exec(ctx, q, l.ID, l.HouseholdID, l.Name, locType, parent); err != nil {
		return fmt.Errorf("persistence: create inventory_location: %w", err)
	}
	return nil
}

// locationAncestors returns id's full ancestor chain (parent, grandparent,
// ... up to the root), via a recursive query over parent_location_id.
func (s *Store) locationAncestors(ctx context.Context, id string) ([]string, error) {
	const q = `WITH RECURSIVE ancestors AS (
			SELECT id, parent_location_id FROM inventory_location WHERE id = $1
		UNION ALL
			SELECT l.id, l.parent_location_id
			FROM inventory_location l
			JOIN ancestors a ON l.id = a.parent_location_id
		)
		SELECT id FROM ancestors`
	rows, err := s.db.Query(ctx, q, id)
	if err != nil {
		return nil, fmt.Errorf("persistence: walk inventory_location ancestors: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var locID string
		if err := rows.Scan(&locID); err != nil {
			return nil, err
		}
		out = append(out, locID)
	}
	return out, rows.Err()
}

// GetInventoryLocation fetches one location by id.
func (s *Store) GetInventoryLocation(ctx context.Context, id string) (InventoryLocation, error) {
	const q = `SELECT id, household_id, name, location_type, parent_location_id, archived_at
		FROM inventory_location WHERE id = $1`
	var l InventoryLocation
	var locType, parent *string
	if err := s.db.QueryRow(ctx, q, id).Scan(&l.ID, &l.HouseholdID, &l.Name, &locType, &parent, &l.ArchivedAt); err != nil {
		return InventoryLocation{}, fmt.Errorf("persistence: get inventory_location: %w", err)
	}
	if locType != nil {
		l.LocationType = *locType
	}
	if parent != nil {
		l.ParentLocationID = *parent
	}
	return l, nil
}

// ListLotsUnderLocation returns every lot recorded directly against id or any
// of its descendant locations (design.md D9: "everything under this
// location" is a recursive walk down, not a requirement that lots be
// recorded at the coarsest ancestor).
func (s *Store) ListLotsUnderLocation(ctx context.Context, id string) ([]InventoryLot, error) {
	const q = `WITH RECURSIVE descendants AS (
			SELECT id FROM inventory_location WHERE id = $1
		UNION ALL
			SELECT l.id
			FROM inventory_location l
			JOIN descendants d ON l.parent_location_id = d.id
		)
		SELECT lot.id, lot.ingredient_id, lot.product_id, lot.location_id, lot.quantity, lot.unit,
			lot.confidence, lot.best_before, lot.opened_at, lot.created_at, lot.updated_at
		FROM inventory_lot lot
		JOIN descendants d ON lot.location_id = d.id
		ORDER BY lot.id`
	rows, err := s.db.Query(ctx, q, id)
	if err != nil {
		return nil, fmt.Errorf("persistence: list lots under location: %w", err)
	}
	return scanLots(rows)
}

// InventoryLot mirrors migrations/0009_pantry_inventory.sql inventory_lot.
// ProductID is "" when the lot exists at ingredient-only specificity
// (design.md D8).
type InventoryLot struct {
	ID           int64
	IngredientID string
	ProductID    string
	LocationID   string
	Quantity     float64
	Unit         string
	Confidence   domain.Confidence
	BestBefore   *time.Time
	OpenedAt     *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func scanLots(rows pgx.Rows) ([]InventoryLot, error) {
	defer rows.Close()
	var out []InventoryLot
	for rows.Next() {
		var l InventoryLot
		var productID *string
		var confidence string
		if err := rows.Scan(&l.ID, &l.IngredientID, &productID, &l.LocationID, &l.Quantity, &l.Unit,
			&confidence, &l.BestBefore, &l.OpenedAt, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, err
		}
		if productID != nil {
			l.ProductID = *productID
		}
		l.Confidence = domain.Confidence(confidence)
		out = append(out, l)
	}
	return out, rows.Err()
}

// GetInventoryLot fetches one lot by id.
func (s *Store) GetInventoryLot(ctx context.Context, id int64) (InventoryLot, error) {
	const q = `SELECT id, ingredient_id, product_id, location_id, quantity, unit, confidence,
		best_before, opened_at, created_at, updated_at FROM inventory_lot WHERE id = $1`
	rows, err := s.db.Query(ctx, q, id)
	if err != nil {
		return InventoryLot{}, fmt.Errorf("persistence: get inventory_lot: %w", err)
	}
	lots, err := scanLots(rows)
	if err != nil {
		return InventoryLot{}, err
	}
	if len(lots) == 0 {
		return InventoryLot{}, fmt.Errorf("persistence: inventory_lot %d not found", id)
	}
	return lots[0], nil
}

const sourceShoppingOrder = "shopping_order"

// RecordPurchase creates a new InventoryLot and appends a PURCHASE event, in
// one transaction (design.md D2: ledger-plus-projection, invariant 3).
// productID may be "" — the lot is created at ingredient-only specificity —
// except when source is "shopping_order", where a productID is required
// (design.md D8, invariant 7): the retailer resolution that produced the
// order already determined the exact product, so there is no "quick" version
// of this path.
func (s *Store) RecordPurchase(ctx context.Context, ingredientID, productID, locationID string, quantity float64, unit string, bestBefore *time.Time, source string) (int64, error) {
	if source == sourceShoppingOrder && productID == "" {
		return 0, errors.New("persistence: RecordPurchase: productId is required when source is shopping_order")
	}
	if quantity <= 0 {
		return 0, errors.New("persistence: RecordPurchase: quantity must be positive")
	}
	confidence := domain.ConfidenceForEvent(domain.EventPurchase, source, false)

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("persistence: begin RecordPurchase: %w", err)
	}
	defer tx.Rollback(ctx)

	var lotID int64
	const insertLot = `INSERT INTO inventory_lot
		(ingredient_id, product_id, location_id, quantity, unit, confidence, best_before)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`
	if err := tx.QueryRow(ctx, insertLot, ingredientID, nullableText(productID), locationID, quantity, unit, string(confidence), bestBefore).Scan(&lotID); err != nil {
		return 0, fmt.Errorf("persistence: RecordPurchase insert lot: %w", err)
	}

	const insertEvent = `INSERT INTO inventory_event
		(kind, lot_id, ingredient_id, product_id, quantity_delta, source)
		VALUES ($1, $2, $3, $4, $5, $6)`
	if _, err := tx.Exec(ctx, insertEvent, string(domain.EventPurchase), lotID, ingredientID, nullableText(productID), quantity, source); err != nil {
		return 0, fmt.Errorf("persistence: RecordPurchase insert event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("persistence: commit RecordPurchase: %w", err)
	}
	return lotID, nil
}

// RefineLotProduct attaches a specific Product to a previously
// ingredient-only lot. It does not change quantity, location, or confidence
// (design.md D8) — it is not itself an inventory_event; it corrects the
// lot's own identity field, the one exception to "lots only change via an
// event" the design allows for this specific field (D8's own text: "at any
// later time without altering the lot's quantity, location, or confidence").
func (s *Store) RefineLotProduct(ctx context.Context, lotID int64, productID string) error {
	if productID == "" {
		return errors.New("persistence: RefineLotProduct: productID is required")
	}
	const q = `UPDATE inventory_lot SET product_id = $1, updated_at = now() WHERE id = $2`
	c, err := s.db.Exec(ctx, q, productID, lotID)
	if err != nil {
		return fmt.Errorf("persistence: RefineLotProduct: %w", err)
	}
	if c.RowsAffected() == 0 {
		return fmt.Errorf("persistence: inventory_lot %d not found", lotID)
	}
	return nil
}

// ListCandidateProductsForIngredient powers the RefineLotProduct picker
// (design.md D8): products already mapped to ingredientID first, falling
// back to a name-match search against the ingredient's own display name when
// no mapping exists yet. Never an unscoped catalog-wide search.
func (s *Store) ListCandidateProductsForIngredient(ctx context.Context, ingredientID string) ([]Product, error) {
	mapped, err := s.ListProductsForIngredient(ctx, ingredientID)
	if err != nil {
		return nil, err
	}
	if len(mapped) > 0 {
		return mapped, nil
	}
	ing, err := s.GetIngredient(ctx, ingredientID)
	if err != nil {
		return nil, err
	}
	const q = `SELECT id, name, brand, package_size, created_at FROM product
		WHERE name ILIKE '%' || $1 || '%' ORDER BY id`
	rows, err := s.db.Query(ctx, q, ing.Display)
	if err != nil {
		return nil, fmt.Errorf("persistence: name-match candidate products: %w", err)
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

// applyLotDelta is the shared write path RecordConsume/RecordDiscard use:
// atomically move the lot's quantity by delta (negative for consumption) and
// append the causing event, in one transaction (design.md D2, invariant 3).
// The quantity change is expressed as a single SQL arithmetic UPDATE
// (`quantity = quantity + delta`) rather than a Go-side read-then-write, so
// no separate row lock or pre-read is needed and no lost-update race exists
// against a concurrent writer of the same lot: the `quantity + $1 >= 0`
// guard makes an over-consuming/over-discarding write fail atomically
// (RowsAffected 0) instead of driving the lot negative.
func (s *Store) applyLotDelta(ctx context.Context, lotID int64, kind domain.EventKind, delta float64, confidence domain.Confidence, reason, source string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("persistence: begin %s: %w", kind, err)
	}
	defer tx.Rollback(ctx)

	const updateLot = `UPDATE inventory_lot SET quantity = quantity + $1, confidence = $2, updated_at = now()
		WHERE id = $3 AND quantity + $1 >= 0`
	c, err := tx.Exec(ctx, updateLot, delta, string(confidence), lotID)
	if err != nil {
		return fmt.Errorf("persistence: %s update lot: %w", kind, err)
	}
	if c.RowsAffected() == 0 {
		return fmt.Errorf("persistence: %s: inventory_lot %d not found, or delta %v would make quantity negative", kind, lotID, delta)
	}

	const insertEvent = `INSERT INTO inventory_event (kind, lot_id, quantity_delta, reason, source)
		VALUES ($1, $2, $3, $4, $5)`
	if _, err := tx.Exec(ctx, insertEvent, string(kind), lotID, delta, nullableText(reason), source); err != nil {
		return fmt.Errorf("persistence: %s insert event: %w", kind, err)
	}

	return tx.Commit(ctx)
}

// RecordConsume decrements lotID by quantity and appends a CONSUME event.
// estimated controls the resulting confidence (design.md D3): false for an
// explicit counted amount, true for a household estimate like "about half".
func (s *Store) RecordConsume(ctx context.Context, lotID int64, quantity float64, estimated bool, source string) error {
	if quantity <= 0 {
		return errors.New("persistence: RecordConsume: quantity must be positive")
	}
	confidence := domain.ConfidenceForEvent(domain.EventConsume, source, estimated)
	return s.applyLotDelta(ctx, lotID, domain.EventConsume, -quantity, confidence, "", source)
}

// RecordDiscard decrements lotID by quantity and appends a DISCARD event with
// reason (expired/spoiled/other).
func (s *Store) RecordDiscard(ctx context.Context, lotID int64, quantity float64, estimated bool, reason, source string) error {
	if quantity <= 0 {
		return errors.New("persistence: RecordDiscard: quantity must be positive")
	}
	confidence := domain.ConfidenceForEvent(domain.EventDiscard, source, estimated)
	return s.applyLotDelta(ctx, lotID, domain.EventDiscard, -quantity, confidence, reason, source)
}

// RecordAdjust corrects lotID to newQuantity (an observed count) and appends
// an ADJUST event whose quantity_delta is newQuantity minus the lot's
// quantity immediately before this call. Unlike RecordConsume/RecordDiscard,
// this command sets an absolute target rather than applying a known delta,
// so it locks the row (SELECT ... FOR UPDATE) inside its own transaction to
// read the prior quantity safely, rather than reading it in a separate,
// racy query beforehand.
func (s *Store) RecordAdjust(ctx context.Context, lotID int64, newQuantity float64, estimated bool, reason, source string) error {
	if newQuantity < 0 {
		return errors.New("persistence: RecordAdjust: newQuantity must be non-negative")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("persistence: begin RecordAdjust: %w", err)
	}
	defer tx.Rollback(ctx)

	var oldQuantity float64
	if err := tx.QueryRow(ctx, `SELECT quantity FROM inventory_lot WHERE id = $1 FOR UPDATE`, lotID).Scan(&oldQuantity); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("persistence: inventory_lot %d not found", lotID)
		}
		return fmt.Errorf("persistence: RecordAdjust lock lot: %w", err)
	}
	confidence := domain.ConfidenceForEvent(domain.EventAdjust, source, estimated)
	delta := newQuantity - oldQuantity

	const updateLot = `UPDATE inventory_lot SET quantity = $1, confidence = $2, updated_at = now() WHERE id = $3`
	if _, err := tx.Exec(ctx, updateLot, newQuantity, string(confidence), lotID); err != nil {
		return fmt.Errorf("persistence: RecordAdjust update lot: %w", err)
	}

	const insertEvent = `INSERT INTO inventory_event (kind, lot_id, quantity_delta, reason, source)
		VALUES ($1, $2, $3, $4, $5)`
	if _, err := tx.Exec(ctx, insertEvent, string(domain.EventAdjust), lotID, delta, nullableText(reason), source); err != nil {
		return fmt.Errorf("persistence: RecordAdjust insert event: %w", err)
	}

	return tx.Commit(ctx)
}

// RecordMarkEmpty sets lotID's quantity to zero. Per design.md D7 this writes
// a CONSUME event (not its own event kind) with source "mark_empty", which
// domain.ConfidenceForEvent always resolves to EXACT ("zero is a certain
// observation") regardless of any prior confidence. Locks the row (SELECT
// ... FOR UPDATE) to read the prior quantity safely for the event's
// quantity_delta, same reasoning as RecordAdjust.
func (s *Store) RecordMarkEmpty(ctx context.Context, lotID int64) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("persistence: begin RecordMarkEmpty: %w", err)
	}
	defer tx.Rollback(ctx)

	var oldQuantity float64
	if err := tx.QueryRow(ctx, `SELECT quantity FROM inventory_lot WHERE id = $1 FOR UPDATE`, lotID).Scan(&oldQuantity); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("persistence: inventory_lot %d not found", lotID)
		}
		return fmt.Errorf("persistence: RecordMarkEmpty lock lot: %w", err)
	}
	confidence := domain.ConfidenceForEvent(domain.EventConsume, domain.SourceMarkEmpty, false)

	const updateLot = `UPDATE inventory_lot SET quantity = 0, confidence = $1, updated_at = now() WHERE id = $2`
	if _, err := tx.Exec(ctx, updateLot, string(confidence), lotID); err != nil {
		return fmt.Errorf("persistence: RecordMarkEmpty update lot: %w", err)
	}

	const insertEvent = `INSERT INTO inventory_event (kind, lot_id, quantity_delta, source)
		VALUES ($1, $2, $3, $4)`
	if _, err := tx.Exec(ctx, insertEvent, string(domain.EventConsume), lotID, -oldQuantity, domain.SourceMarkEmpty); err != nil {
		return fmt.Errorf("persistence: RecordMarkEmpty insert event: %w", err)
	}

	return tx.Commit(ctx)
}

// RecordOpen marks a sealed lot opened. It does not by itself change
// quantity or confidence (design.md D3) — it only records the OPEN event and
// sets opened_at, making the lot eligible for future time-based confidence
// decay (a separate, not-yet-implemented mechanism per design.md D3/D4).
func (s *Store) RecordOpen(ctx context.Context, lotID int64, source string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("persistence: begin RecordOpen: %w", err)
	}
	defer tx.Rollback(ctx)

	const updateLot = `UPDATE inventory_lot SET opened_at = now(), updated_at = now() WHERE id = $1`
	c, err := tx.Exec(ctx, updateLot, lotID)
	if err != nil {
		return fmt.Errorf("persistence: RecordOpen update lot: %w", err)
	}
	if c.RowsAffected() == 0 {
		return fmt.Errorf("persistence: inventory_lot %d not found", lotID)
	}

	const insertEvent = `INSERT INTO inventory_event (kind, lot_id, quantity_delta, source)
		VALUES ($1, $2, 0, $3)`
	if _, err := tx.Exec(ctx, insertEvent, string(domain.EventOpen), lotID, source); err != nil {
		return fmt.Errorf("persistence: RecordOpen insert event: %w", err)
	}

	return tx.Commit(ctx)
}

// RecordTransfer moves quantity (all or part of a lot) from its current
// location to toLocationID and appends a TRANSFER event. A full-quantity
// transfer moves the lot itself (updates its location_id in place); a
// partial transfer decrements the source lot and creates a new lot at the
// destination, per design.md's "a TRANSFER that moves only part of a lot's
// quantity leaves the source lot's remaining quantity correct and the
// destination as a distinct lot, not a merge" (tasks.md 9.3). Confidence is
// preserved unchanged on both sides (design.md D3: "moving inventory doesn't
// change how sure we are of the amount").
func (s *Store) RecordTransfer(ctx context.Context, lotID int64, toLocationID string, quantity float64, source string) (newLotID int64, err error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("persistence: begin RecordTransfer: %w", err)
	}
	defer tx.Rollback(ctx)

	// Lock the source lot inside this transaction rather than reading it in a
	// separate query beforehand, so the quantity this function acts on can't
	// go stale against a concurrent writer of the same lot (same reasoning as
	// RecordAdjust/RecordMarkEmpty).
	const lockLot = `SELECT ingredient_id, product_id, location_id, quantity, unit, confidence, best_before
		FROM inventory_lot WHERE id = $1 FOR UPDATE`
	var lot InventoryLot
	var productID *string
	var confidence string
	row := tx.QueryRow(ctx, lockLot, lotID)
	if err := row.Scan(&lot.IngredientID, &productID, &lot.LocationID, &lot.Quantity, &lot.Unit, &confidence, &lot.BestBefore); err != nil {
		if err == pgx.ErrNoRows {
			return 0, fmt.Errorf("persistence: inventory_lot %d not found", lotID)
		}
		return 0, fmt.Errorf("persistence: RecordTransfer lock lot: %w", err)
	}
	if productID != nil {
		lot.ProductID = *productID
	}
	lot.Confidence = domain.Confidence(confidence)

	if quantity <= 0 || quantity > lot.Quantity {
		return 0, fmt.Errorf("persistence: RecordTransfer: invalid quantity %v for lot %d holding %v", quantity, lotID, lot.Quantity)
	}
	fromLocationID := lot.LocationID

	if quantity == lot.Quantity {
		// Full transfer: move the lot itself, no new row.
		const updateLot = `UPDATE inventory_lot SET location_id = $1, updated_at = now() WHERE id = $2`
		if _, err := tx.Exec(ctx, updateLot, toLocationID, lotID); err != nil {
			return 0, fmt.Errorf("persistence: RecordTransfer move lot: %w", err)
		}
		newLotID = lotID
	} else {
		// Partial transfer: decrement the source, create a distinct destination lot.
		const decrementLot = `UPDATE inventory_lot SET quantity = quantity - $1, updated_at = now() WHERE id = $2`
		if _, err := tx.Exec(ctx, decrementLot, quantity, lotID); err != nil {
			return 0, fmt.Errorf("persistence: RecordTransfer decrement source lot: %w", err)
		}
		const insertLot = `INSERT INTO inventory_lot
			(ingredient_id, product_id, location_id, quantity, unit, confidence, best_before)
			VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`
		if err := tx.QueryRow(ctx, insertLot, lot.IngredientID, nullableText(lot.ProductID), toLocationID, quantity, lot.Unit, string(lot.Confidence), lot.BestBefore).Scan(&newLotID); err != nil {
			return 0, fmt.Errorf("persistence: RecordTransfer create destination lot: %w", err)
		}
	}

	const insertEvent = `INSERT INTO inventory_event
		(kind, lot_id, from_location_id, to_location_id, quantity_delta, source)
		VALUES ($1, $2, $3, $4, $5, $6)`
	if _, err := tx.Exec(ctx, insertEvent, string(domain.EventTransfer), lotID, fromLocationID, toLocationID, quantity, source); err != nil {
		return 0, fmt.Errorf("persistence: RecordTransfer insert event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("persistence: commit RecordTransfer: %w", err)
	}
	return newLotID, nil
}

// LookupBarcode normalizes gtin and resolves it against ProductIdentifier —
// the first step of design.md D6's fallback chain. Open Food Facts and
// retailer-lookup fallback (D6's remaining steps) are intentionally not
// implemented here: they are live external-API integrations, scoped
// separately (see tasks.md 7.3/7.4, left unchecked). Returns "", nil (not an
// error) when normalization succeeds but no identifier row matches yet — the
// same "not found, not fabricated" shape LookupProductByGTIN already uses.
func (s *Store) LookupBarcode(ctx context.Context, rawGTIN string) (productID string, err error) {
	normalized, err := domain.NormalizeGTIN(rawGTIN)
	if err != nil {
		return "", err
	}
	return s.LookupProductByGTIN(ctx, normalized)
}
