package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/jackc/pgx/v5"
)

// Order mirrors migrations/000007_order.sql. It is a persisted record of a
// completed purchase, preserving actual quantity, price, retailer product,
// and substitutions per design D4.
type Order struct {
	ID              domain.OrderID
	ShoppingCartID  *domain.ShoppingCartID
	Retailer        string
	Source          string // 'manual' | 'retailer_api' | 'receipt_import'
	OrderedAt       time.Time
	TotalPriceMinor *int64
	Currency        string
}

// CreateOrder inserts a new order and returns its id.
func (s *Store) CreateOrder(ctx context.Context, o Order) (domain.OrderID, error) {
	const q = `INSERT INTO "order"
		(id, shopping_cart_id, retailer, source, ordered_at, total_price_minor, currency)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`
	id := domain.NewOrderID()
	var returnedID domain.OrderID
	err := s.db.QueryRow(ctx, q, id, o.ShoppingCartID, o.Retailer, o.Source, o.OrderedAt, o.TotalPriceMinor, o.Currency).Scan(&returnedID)
	if err != nil {
		return domain.OrderID{}, fmt.Errorf("persistence: create order: %w", err)
	}
	return returnedID, nil
}

// GetOrder fetches one order by id.
func (s *Store) GetOrder(ctx context.Context, id domain.OrderID) (Order, error) {
	const q = `SELECT id, shopping_cart_id, retailer, source, ordered_at, total_price_minor, currency
		FROM "order" WHERE id = $1`
	var o Order
	if err := s.db.QueryRow(ctx, q, id).Scan(&o.ID, &o.ShoppingCartID, &o.Retailer, &o.Source, &o.OrderedAt, &o.TotalPriceMinor, &o.Currency); err != nil {
		return Order{}, fmt.Errorf("persistence: get order: %w", err)
	}
	return o, nil
}

// ListOrders returns orders optionally filtered by cart or retailer.
func (s *Store) ListOrders(ctx context.Context, cartID *domain.ShoppingCartID, retailer *string) ([]Order, error) {
	q := `SELECT id, shopping_cart_id, retailer, source, ordered_at, total_price_minor, currency FROM "order" WHERE 1=1`
	var args []any
	idx := 1
	if cartID != nil {
		q += fmt.Sprintf(" AND shopping_cart_id = $%d", idx)
		args = append(args, *cartID)
		idx++
	}
	if retailer != nil {
		q += fmt.Sprintf(" AND retailer = $%d", idx)
		args = append(args, *retailer)
		idx++
	}
	q += " ORDER BY ordered_at DESC"

	rows, err := s.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("persistence: list orders: %w", err)
	}
	defer rows.Close()
	return scanOrders(rows)
}

func scanOrders(rows pgx.Rows) ([]Order, error) {
	defer rows.Close()
	var out []Order
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.ShoppingCartID, &o.Retailer, &o.Source, &o.OrderedAt, &o.TotalPriceMinor, &o.Currency); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// ── Order items ──────────────────────────────────────────────────────────────

// OrderItem mirrors migrations/000007_order.sql order_item.
type OrderItem struct {
	ID                   domain.OrderItemID
	OrderID              domain.OrderID
	RetailerProductID    domain.RetailerProductID
	Quantity             float64
	UnitPrice            *float64
	TotalPriceMinor      *int64
	Currency             string
	SubstitutedForItemID *domain.OrderItemID
}

// CreateOrderItem inserts a line item into an order and returns its id.
func (s *Store) CreateOrderItem(ctx context.Context, item OrderItem) (domain.OrderItemID, error) {
	const q = `INSERT INTO order_item
		(id, order_id, retailer_product_id, quantity, unit_price, total_price_minor, currency, substituted_for_item_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`
	id := domain.NewOrderItemID()
	var returnedID domain.OrderItemID
	err := s.db.QueryRow(ctx, q, id, item.OrderID, item.RetailerProductID,
		item.Quantity, item.UnitPrice, item.TotalPriceMinor, item.Currency, item.SubstitutedForItemID).Scan(&returnedID)
	if err != nil {
		return domain.OrderItemID{}, fmt.Errorf("persistence: create order_item: %w", err)
	}
	return returnedID, nil
}

// ListOrderItems returns all items for an order.
func (s *Store) ListOrderItems(ctx context.Context, orderID domain.OrderID) ([]OrderItem, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, order_id, retailer_product_id, quantity, unit_price, total_price_minor, currency, substituted_for_item_id
		FROM order_item WHERE order_id = $1`, orderID)
	if err != nil {
		return nil, fmt.Errorf("persistence: list order_items: %w", err)
	}
	defer rows.Close()
	return scanOrderItems(rows)
}

func scanOrderItems(rows pgx.Rows) ([]OrderItem, error) {
	defer rows.Close()
	var out []OrderItem
	for rows.Next() {
		var item OrderItem
		if err := rows.Scan(&item.ID, &item.OrderID, &item.RetailerProductID,
			&item.Quantity, &item.UnitPrice, &item.TotalPriceMinor, &item.Currency, &item.SubstitutedForItemID); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
