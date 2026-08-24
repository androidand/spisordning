package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ShoppingCart mirrors migrations/0006_shopping_cart.sql.
// It is a checkpoint record of a to-cart call, not a mirror of the retailer's
// live cart state (design D3).
type ShoppingCart struct {
	ID                     int64
	RetailerListBindingID  int64
	CreatedAt              time.Time
	Status                 string // 'created' | 'confirmed' | 'abandoned'
}

// CreateShoppingCart inserts a new cart (status='created' by default) and returns its id.
func (s *Store) CreateShoppingCart(ctx context.Context, c ShoppingCart) (int64, error) {
	if c.Status == "" {
		c.Status = "created"
	}
	const q = `INSERT INTO shopping_cart (retailer_list_binding_id, status)
		VALUES ($1, $2) RETURNING id`
	var id int64
	err := s.db.QueryRow(ctx, q, c.RetailerListBindingID, c.Status).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("persistence: create shopping_cart: %w", err)
	}
	return id, nil
}

// GetShoppingCart fetches one cart by id.
func (s *Store) GetShoppingCart(ctx context.Context, id int64) (ShoppingCart, error) {
	const q = `SELECT id, retailer_list_binding_id, created_at, status FROM shopping_cart WHERE id = $1`
	var c ShoppingCart
	if err := s.db.QueryRow(ctx, q, id).Scan(&c.ID, &c.RetailerListBindingID, &c.CreatedAt, &c.Status); err != nil {
		return ShoppingCart{}, fmt.Errorf("persistence: get shopping_cart: %w", err)
	}
	return c, nil
}

// ListShoppingCarts returns all carts for a binding, ordered by created_at descending.
func (s *Store) ListShoppingCarts(ctx context.Context, bindingID int64) ([]ShoppingCart, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, retailer_list_binding_id, created_at, status
		FROM shopping_cart WHERE retailer_list_binding_id = $1 ORDER BY created_at DESC`, bindingID)
	if err != nil {
		return nil, fmt.Errorf("persistence: list shopping_carts: %w", err)
	}
	defer rows.Close()
	return scanShoppingCarts(rows)
}

func scanShoppingCarts(rows pgx.Rows) ([]ShoppingCart, error) {
	defer rows.Close()
	var out []ShoppingCart
	for rows.Next() {
		var c ShoppingCart
		if err := rows.Scan(&c.ID, &c.RetailerListBindingID, &c.CreatedAt, &c.Status); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// UpdateShoppingCartStatus updates a cart's status (e.g. 'created' → 'confirmed').
func (s *Store) UpdateShoppingCartStatus(ctx context.Context, id int64, status string) error {
	const q = `UPDATE shopping_cart SET status = $1 WHERE id = $2`
	if _, err := s.db.Exec(ctx, q, status, id); err != nil {
		return fmt.Errorf("persistence: update shopping_cart status: %w", err)
	}
	return nil
}

// ── Shopping cart items ──────────────────────────────────────────────────────

// ShoppingCartItem mirrors migrations/0006_shopping_cart.sql shopping_cart_item.
type ShoppingCartItem struct {
	ID                int64
	ShoppingCartID    int64
	RetailerProductID string
	Quantity          float64
	Unit              string
	ResolvedPrice     *float64
}

// CreateShoppingCartItem inserts a line item into a cart and returns its id.
func (s *Store) CreateShoppingCartItem(ctx context.Context, item ShoppingCartItem) (int64, error) {
	const q = `INSERT INTO shopping_cart_item
		(shopping_cart_id, retailer_product_id, quantity, unit, resolved_price)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`
	var id int64
	err := s.db.QueryRow(ctx, q, item.ShoppingCartID, item.RetailerProductID,
		item.Quantity, item.Unit, item.ResolvedPrice).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("persistence: create shopping_cart_item: %w", err)
	}
	return id, nil
}

// ListShoppingCartItems returns all items for a cart.
func (s *Store) ListShoppingCartItems(ctx context.Context, cartID int64) ([]ShoppingCartItem, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, shopping_cart_id, retailer_product_id, quantity, unit, resolved_price
		FROM shopping_cart_item WHERE shopping_cart_id = $1`, cartID)
	if err != nil {
		return nil, fmt.Errorf("persistence: list shopping_cart_items: %w", err)
	}
	defer rows.Close()
	return scanShoppingCartItems(rows)
}

func scanShoppingCartItems(rows pgx.Rows) ([]ShoppingCartItem, error) {
	defer rows.Close()
	var out []ShoppingCartItem
	for rows.Next() {
		var item ShoppingCartItem
		if err := rows.Scan(&item.ID, &item.ShoppingCartID, &item.RetailerProductID,
			&item.Quantity, &item.Unit, &item.ResolvedPrice); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
