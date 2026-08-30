package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/jackc/pgx/v5"
)

// ShoppingCart mirrors migrations/000006_shopping_cart.sql.
// It is a checkpoint record of a to-cart call, not a mirror of the retailer's
// live cart state (design D3).
type ShoppingCart struct {
	ID             domain.ShoppingCartID
	ShoppingListID domain.ShoppingListID // composite FK reference together with Retailer
	Retailer       string // composite FK reference to retailer_list_binding(shopping_list_id, retailer)
	CreatedAt      time.Time
	Status         string // 'created' | 'confirmed' | 'abandoned'
}

// CreateShoppingCart inserts a new cart (status='created' by default) and returns its UUID.
func (s *Store) CreateShoppingCart(ctx context.Context, c ShoppingCart) (domain.ShoppingCartID, error) {
	if c.Status == "" {
		c.Status = "created"
	}
	id := domain.NewShoppingCartID()
	const q = `INSERT INTO shopping_cart (id, shopping_list_id, retailer, status)
		VALUES ($1, $2, $3, $4) RETURNING id`
	var returnedID domain.ShoppingCartID
	err := s.db.QueryRow(ctx, q, id, c.ShoppingListID, c.Retailer, c.Status).Scan(&returnedID)
	if err != nil {
		return domain.ShoppingCartID{}, fmt.Errorf("persistence: create shopping_cart: %w", err)
	}
	return returnedID, nil
}

// GetShoppingCart fetches one cart by id.
func (s *Store) GetShoppingCart(ctx context.Context, id domain.ShoppingCartID) (ShoppingCart, error) {
	const q = `SELECT id, shopping_list_id, retailer, created_at, status FROM shopping_cart WHERE id = $1`
	var c ShoppingCart
	if err := s.db.QueryRow(ctx, q, id).Scan(&c.ID, &c.ShoppingListID, &c.Retailer, &c.CreatedAt, &c.Status); err != nil {
		return ShoppingCart{}, fmt.Errorf("persistence: get shopping_cart: %w", err)
	}
	return c, nil
}

// ListShoppingCarts returns all carts for a (shopping_list, retailer) binding,
// ordered by created_at descending.
func (s *Store) ListShoppingCarts(ctx context.Context, shoppingListID domain.ShoppingListID, retailer string) ([]ShoppingCart, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, shopping_list_id, retailer, created_at, status
		FROM shopping_cart WHERE shopping_list_id = $1 AND retailer = $2 ORDER BY created_at DESC`,
		shoppingListID, retailer)
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
		if err := rows.Scan(&c.ID, &c.ShoppingListID, &c.Retailer, &c.CreatedAt, &c.Status); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// UpdateShoppingCartStatus updates a cart's status (e.g. 'created' → 'confirmed').
func (s *Store) UpdateShoppingCartStatus(ctx context.Context, id domain.ShoppingCartID, status string) error {
	const q = `UPDATE shopping_cart SET status = $1 WHERE id = $2`
	if _, err := s.db.Exec(ctx, q, status, id); err != nil {
		return fmt.Errorf("persistence: update shopping_cart status: %w", err)
	}
	return nil
}

// ── Shopping cart items ──────────────────────────────────────────────────────

// ShoppingCartItem mirrors migrations/000006_shopping_cart.sql shopping_cart_item.
// The PK is composite (shopping_cart_id, line_no) — there is no surrogate id.
type ShoppingCartItem struct {
	ShoppingCartID     domain.ShoppingCartID
	LineNo             int
	RetailerProductID  domain.RetailerProductID
	Quantity           float64
	Unit               string
	ResolvedPriceMinor *int64
	Currency           string // CHAR(3), default 'SEK'
}

// CreateShoppingCartItem inserts a line item into a cart and returns its line_no.
func (s *Store) CreateShoppingCartItem(ctx context.Context, item ShoppingCartItem) (int, error) {
	const q = `INSERT INTO shopping_cart_item
		(shopping_cart_id, line_no, retailer_product_id, quantity, unit, resolved_price_minor, currency)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := s.db.Exec(ctx, q, item.ShoppingCartID, item.LineNo, item.RetailerProductID,
		item.Quantity, item.Unit, item.ResolvedPriceMinor, item.Currency)
	if err != nil {
		return 0, fmt.Errorf("persistence: create shopping_cart_item: %w", err)
	}
	return item.LineNo, nil
}

// ListShoppingCartItems returns all items for a cart.
func (s *Store) ListShoppingCartItems(ctx context.Context, cartID domain.ShoppingCartID) ([]ShoppingCartItem, error) {
	rows, err := s.db.Query(ctx, `
		SELECT shopping_cart_id, line_no, retailer_product_id, quantity, unit, resolved_price_minor, currency
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
		if err := rows.Scan(&item.ShoppingCartID, &item.LineNo, &item.RetailerProductID,
			&item.Quantity, &item.Unit, &item.ResolvedPriceMinor, &item.Currency); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
