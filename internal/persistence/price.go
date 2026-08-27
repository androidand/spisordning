package persistence

import (
	"context"
	"fmt"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/jackc/pgx/v5"
)

// CreateRetailer inserts a retailer.
func (s *Store) CreateRetailer(ctx context.Context, r domain.Retailer) error {
	const q = `INSERT INTO retailer (id, name) VALUES ($1, $2)`
	if _, err := s.db.Exec(ctx, q, r.ID, r.Name); err != nil {
		return fmt.Errorf("persistence: create retailer: %w", err)
	}
	return nil
}

// UpsertRetailer inserts a retailer or, if one with the same id already exists,
// refreshes its name. Idempotent, so sync commands can call it unconditionally.
func (s *Store) UpsertRetailer(ctx context.Context, r domain.Retailer) error {
	const q = `INSERT INTO retailer (id, name) VALUES ($1, $2)
		ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name`
	if _, err := s.db.Exec(ctx, q, r.ID, r.Name); err != nil {
		return fmt.Errorf("persistence: upsert retailer: %w", err)
	}
	return nil
}

// GetRetailer fetches one retailer by id.
func (s *Store) GetRetailer(ctx context.Context, id string) (domain.Retailer, error) {
	const q = `SELECT id, name, created_at FROM retailer WHERE id = $1`
	var r domain.Retailer
	if err := s.db.QueryRow(ctx, q, id).Scan(&r.ID, &r.Name, &r.CreatedAt); err != nil {
		return domain.Retailer{}, fmt.Errorf("persistence: get retailer: %w", err)
	}
	return r, nil
}

// ListRetailers returns all retailers, ordered by id.
func (s *Store) ListRetailers(ctx context.Context) ([]domain.Retailer, error) {
	const q = `SELECT id, name, created_at FROM retailer ORDER BY id`
	rows, err := s.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("persistence: list retailers: %w", err)
	}
	defer rows.Close()
	var out []domain.Retailer
	for rows.Next() {
		var r domain.Retailer
		if err := rows.Scan(&r.ID, &r.Name, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// CreateStore inserts a store.
func (s *Store) CreateStore(ctx context.Context, st domain.Store) error {
	const q = `INSERT INTO store (id, retailer_id, name) VALUES ($1, $2, $3)`
	if _, err := s.db.Exec(ctx, q, st.ID, st.RetailerID, st.Name); err != nil {
		return fmt.Errorf("persistence: create store: %w", err)
	}
	return nil
}

// GetStore fetches one store by id.
func (s *Store) GetStore(ctx context.Context, id string) (domain.Store, error) {
	const q = `SELECT id, retailer_id, name, created_at FROM store WHERE id = $1`
	var st domain.Store
	if err := s.db.QueryRow(ctx, q, id).Scan(&st.ID, &st.RetailerID, &st.Name, &st.CreatedAt); err != nil {
		return domain.Store{}, fmt.Errorf("persistence: get store: %w", err)
	}
	return st, nil
}

// ListStores returns all stores for a retailer, ordered by id.
func (s *Store) ListStores(ctx context.Context, retailerID string) ([]domain.Store, error) {
	const q = `SELECT id, retailer_id, name, created_at FROM store WHERE retailer_id = $1 ORDER BY id`
	rows, err := s.db.Query(ctx, q, retailerID)
	if err != nil {
		return nil, fmt.Errorf("persistence: list stores: %w", err)
	}
	defer rows.Close()
	var out []domain.Store
	for rows.Next() {
		var st domain.Store
		if err := rows.Scan(&st.ID, &st.RetailerID, &st.Name, &st.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// UpsertStore inserts a store or, if one with the same id already exists,
// refreshes its name and retailer. Idempotent for sync commands.
func (s *Store) UpsertStore(ctx context.Context, st domain.Store) error {
	const q = `INSERT INTO store (id, retailer_id, name) VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE SET
			retailer_id = EXCLUDED.retailer_id,
			name = EXCLUDED.name`
	if _, err := s.db.Exec(ctx, q, st.ID, st.RetailerID, st.Name); err != nil {
		return fmt.Errorf("persistence: upsert store: %w", err)
	}
	return nil
}

// ListAllStores returns every store across all retailers, ordered by id.
func (s *Store) ListAllStores(ctx context.Context) ([]domain.Store, error) {
	const q = `SELECT id, retailer_id, name, created_at FROM store ORDER BY id`
	rows, err := s.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("persistence: list all stores: %w", err)
	}
	defer rows.Close()
	var out []domain.Store
	for rows.Next() {
		var st domain.Store
		if err := rows.Scan(&st.ID, &st.RetailerID, &st.Name, &st.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// UpsertRetailerProduct inserts or updates a retailer product. If a row with the
// same (retailer_id, retailer_sku) already exists, the product_id and display_name
// are updated (a SKU may be unmapped at first and mapped later).
func (s *Store) UpsertRetailerProduct(ctx context.Context, rp domain.RetailerProduct) error {
	const q = `INSERT INTO retailer_product (id, retailer_id, product_id, retailer_sku, display_name)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (retailer_id, retailer_sku) DO UPDATE SET
			product_id = EXCLUDED.product_id,
			display_name = EXCLUDED.display_name`
	if _, err := s.db.Exec(ctx, q, rp.ID, rp.RetailerID, nullableText(rp.ProductID), rp.RetailerSKU, nullableText(rp.DisplayName)); err != nil {
		return fmt.Errorf("persistence: upsert retailer_product: %w", err)
	}
	return nil
}

// GetRetailerProduct fetches one retailer product by id.
func (s *Store) GetRetailerProduct(ctx context.Context, id string) (domain.RetailerProduct, error) {
	const q = `SELECT id, retailer_id, product_id, retailer_sku, display_name, created_at FROM retailer_product WHERE id = $1`
	var rp domain.RetailerProduct
	var productID, displayName *string
	if err := s.db.QueryRow(ctx, q, id).Scan(&rp.ID, &rp.RetailerID, &productID, &rp.RetailerSKU, &displayName, &rp.CreatedAt); err != nil {
		return domain.RetailerProduct{}, fmt.Errorf("persistence: get retailer_product: %w", err)
	}
	if productID != nil {
		rp.ProductID = *productID
	}
	if displayName != nil {
		rp.DisplayName = *displayName
	}
	return rp, nil
}

// ListRetailerProducts returns all retailer products for a retailer, ordered by id.
func (s *Store) ListRetailerProducts(ctx context.Context, retailerID string) ([]domain.RetailerProduct, error) {
	const q = `SELECT id, retailer_id, product_id, retailer_sku, display_name, created_at FROM retailer_product WHERE retailer_id = $1 ORDER BY id`
	rows, err := s.db.Query(ctx, q, retailerID)
	if err != nil {
		return nil, fmt.Errorf("persistence: list retailer_products: %w", err)
	}
	defer rows.Close()
	var out []domain.RetailerProduct
	for rows.Next() {
		var rp domain.RetailerProduct
		var productID, displayName *string
		if err := rows.Scan(&rp.ID, &rp.RetailerID, &productID, &rp.RetailerSKU, &displayName, &rp.CreatedAt); err != nil {
			return nil, err
		}
		if productID != nil {
			rp.ProductID = *productID
		}
		if displayName != nil {
			rp.DisplayName = *displayName
		}
		out = append(out, rp)
	}
	return out, rows.Err()
}

// UpsertStoreProductOffer inserts or updates a store_product_offer. If the row
// already exists, currently_carried and updated_at are refreshed. Returns the
// assigned BIGSERIAL id.
func (s *Store) UpsertStoreProductOffer(ctx context.Context, offer domain.StoreProductOffer) (int64, error) {
	const q = `INSERT INTO store_product_offer (store_id, retailer_product_id, currently_carried)
		VALUES ($1, $2, $3)
		ON CONFLICT (store_id, retailer_product_id) DO UPDATE SET
			currently_carried = EXCLUDED.currently_carried,
			updated_at = now()
		RETURNING id`
	var id int64
	if err := s.db.QueryRow(ctx, q, offer.StoreID, offer.RetailerProductID, offer.CurrentlyCarried).Scan(&id); err != nil {
		return 0, fmt.Errorf("persistence: upsert store_product_offer: %w", err)
	}
	return id, nil
}

// GetStoreProductOffer fetches one store_product_offer by id.
func (s *Store) GetStoreProductOffer(ctx context.Context, id int64) (domain.StoreProductOffer, error) {
	const q = `SELECT id, store_id, retailer_product_id, currently_carried, created_at, updated_at FROM store_product_offer WHERE id = $1`
	var offer domain.StoreProductOffer
	if err := s.db.QueryRow(ctx, q, id).Scan(&offer.ID, &offer.StoreID, &offer.RetailerProductID, &offer.CurrentlyCarried, &offer.CreatedAt, &offer.UpdatedAt); err != nil {
		return domain.StoreProductOffer{}, fmt.Errorf("persistence: get store_product_offer: %w", err)
	}
	return offer, nil
}

// ListStoreProductOffers returns all offers for a store, ordered by id.
func (s *Store) ListStoreProductOffers(ctx context.Context, storeID string) ([]domain.StoreProductOffer, error) {
	const q = `SELECT id, store_id, retailer_product_id, currently_carried, created_at, updated_at FROM store_product_offer WHERE store_id = $1 ORDER BY id`
	rows, err := s.db.Query(ctx, q, storeID)
	if err != nil {
		return nil, fmt.Errorf("persistence: list store_product_offers: %w", err)
	}
	defer rows.Close()
	var out []domain.StoreProductOffer
	for rows.Next() {
		var offer domain.StoreProductOffer
		if err := rows.Scan(&offer.ID, &offer.StoreID, &offer.RetailerProductID, &offer.CurrentlyCarried, &offer.CreatedAt, &offer.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, offer)
	}
	return out, rows.Err()
}

// ListRetailerProductOffers returns all offers for a retailer product, ordered by id.
func (s *Store) ListRetailerProductOffers(ctx context.Context, retailerProductID string) ([]domain.StoreProductOffer, error) {
	const q = `SELECT id, store_id, retailer_product_id, currently_carried, created_at, updated_at FROM store_product_offer WHERE retailer_product_id = $1 ORDER BY id`
	rows, err := s.db.Query(ctx, q, retailerProductID)
	if err != nil {
		return nil, fmt.Errorf("persistence: list retailer_product_offers: %w", err)
	}
	defer rows.Close()
	var out []domain.StoreProductOffer
	for rows.Next() {
		var offer domain.StoreProductOffer
		if err := rows.Scan(&offer.ID, &offer.StoreID, &offer.RetailerProductID, &offer.CurrentlyCarried, &offer.CreatedAt, &offer.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, offer)
	}
	return out, rows.Err()
}

// InsertPriceObservation appends a new price_observation row. Rows are never
// updated or deleted — this is an append-only ledger (design.md invariant: price
// is an observation series, not a mutable field).
func (s *Store) InsertPriceObservation(ctx context.Context, obs domain.PriceObservation) error {
	const q = `INSERT INTO price_observation (store_product_offer_id, observed_at, price, price_kind, source)
		VALUES ($1, $2, $3, $4, $5)`
	if _, err := s.db.Exec(ctx, q, obs.StoreProductOfferID, obs.ObservedAt, obs.Price, string(obs.PriceKind), obs.Source); err != nil {
		return fmt.Errorf("persistence: insert price_observation: %w", err)
	}
	return nil
}

// ListPriceObservations returns all observations for an offer, ordered by
// observed_at descending.
func (s *Store) ListPriceObservations(ctx context.Context, offerID int64) ([]domain.PriceObservation, error) {
	const q = `SELECT id, store_product_offer_id, observed_at, price, price_kind, source, created_at FROM price_observation WHERE store_product_offer_id = $1 ORDER BY observed_at DESC, id DESC`
	rows, err := s.db.Query(ctx, q, offerID)
	if err != nil {
		return nil, fmt.Errorf("persistence: list price_observations: %w", err)
	}
	return scanPriceObservations(rows)
}

// GetLatestPriceObservation returns the most recent observation for an offer and
// price_kind, or an error if none exists. This is the canonical way to read
// "the current price" for a specific kind — callers may also read from the
// current_store_product_price view for a broader query.
func (s *Store) GetLatestPriceObservation(ctx context.Context, offerID int64, kind domain.PriceKind) (domain.PriceObservation, error) {
	const q = `SELECT id, store_product_offer_id, observed_at, price, price_kind, source, created_at
		FROM price_observation WHERE store_product_offer_id = $1 AND price_kind = $2
		ORDER BY observed_at DESC, id DESC LIMIT 1`
	var obs domain.PriceObservation
	if err := s.db.QueryRow(ctx, q, offerID, string(kind)).Scan(&obs.ID, &obs.StoreProductOfferID, &obs.ObservedAt, &obs.Price,
		(*string)(&obs.PriceKind), &obs.Source, &obs.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return domain.PriceObservation{}, fmt.Errorf("persistence: no price observation found for offer %d kind %q", offerID, kind)
		}
		return domain.PriceObservation{}, fmt.Errorf("persistence: get latest price_observation: %w", err)
	}
	return obs, nil
}

// ListCurrentPrices returns all rows from the current_store_product_price view.
func (s *Store) ListCurrentPrices(ctx context.Context) ([]domain.CurrentStoreProductPrice, error) {
	const q = `SELECT offer_id, store_id, retailer_product_id, price_kind, price, observed_at, source FROM current_store_product_price ORDER BY offer_id, price_kind`
	rows, err := s.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("persistence: list current prices: %w", err)
	}
	return scanCurrentPrices(rows)
}

func scanPriceObservations(rows pgx.Rows) ([]domain.PriceObservation, error) {
	defer rows.Close()
	var out []domain.PriceObservation
	for rows.Next() {
		var obs domain.PriceObservation
		var kind string
		if err := rows.Scan(&obs.ID, &obs.StoreProductOfferID, &obs.ObservedAt, &obs.Price,
			&kind, &obs.Source, &obs.CreatedAt); err != nil {
			return nil, err
		}
		obs.PriceKind = domain.PriceKind(kind)
		out = append(out, obs)
	}
	return out, rows.Err()
}

func scanCurrentPrices(rows pgx.Rows) ([]domain.CurrentStoreProductPrice, error) {
	defer rows.Close()
	var out []domain.CurrentStoreProductPrice
	for rows.Next() {
		var p domain.CurrentStoreProductPrice
		var kind string
		if err := rows.Scan(&p.OfferID, &p.StoreID, &p.RetailerProductID, &kind, &p.Price, &p.ObservedAt, &p.Source); err != nil {
			return nil, err
		}
		p.PriceKind = domain.PriceKind(kind)
		out = append(out, p)
	}
	return out, rows.Err()
}

// PriceObservationForProduct returns all price observations across all offers for
// a given retailer product, ordered by observed_at descending. Useful for
// price-history queries over time.
func (s *Store) PriceObservationsForProduct(ctx context.Context, retailerProductID string) ([]domain.PriceObservation, error) {
	const q = `SELECT po.id, po.store_product_offer_id, po.observed_at, po.price, po.price_kind, po.source, po.created_at
		FROM price_observation po
		JOIN store_product_offer spo ON spo.id = po.store_product_offer_id
		WHERE spo.retailer_product_id = $1
		ORDER BY po.observed_at DESC`
	rows, err := s.db.Query(ctx, q, retailerProductID)
	if err != nil {
		return nil, fmt.Errorf("persistence: price observations for product: %w", err)
	}
	return scanPriceObservations(rows)
}

// PriceObservationsForStore returns all price observations for offers at a
// specific store, ordered by observed_at descending.
func (s *Store) PriceObservationsForStore(ctx context.Context, storeID string) ([]domain.PriceObservation, error) {
	const q = `SELECT po.id, po.store_product_offer_id, po.observed_at, po.price, po.price_kind, po.source, po.created_at
		FROM price_observation po
		JOIN store_product_offer spo ON spo.id = po.store_product_offer_id
		WHERE spo.store_id = $1
		ORDER BY po.observed_at DESC`
	rows, err := s.db.Query(ctx, q, storeID)
	if err != nil {
		return nil, fmt.Errorf("persistence: price observations for store: %w", err)
	}
	return scanPriceObservations(rows)
}
