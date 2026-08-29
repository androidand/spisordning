package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/androidand/spisordning/internal/dto"
	"github.com/androidand/spisordning/internal/grocy"
)

// ErrGrocyNotConfigured is returned when no Grocy instance is configured.
var ErrGrocyNotConfigured = errors.New("grocy is not configured")

// Grocy bridges a running Grocy instance into Spisordning's HTTP surface.
// client is nil when no GROCY_BASE_URL is set; every method then returns
// ErrGrocyNotConfigured so the API degrades gracefully.
type Grocy struct {
	client  *grocy.Client
	baseURL string
}

// NewGrocy returns a Grocy service. client may be nil (not configured); baseURL
// is reported in Status for diagnostics.
func NewGrocy(client *grocy.Client, baseURL string) *Grocy {
	return &Grocy{client: client, baseURL: baseURL}
}

func (s *Grocy) requireClient() error {
	if s.client == nil {
		return ErrGrocyNotConfigured
	}
	return nil
}

// Status reports whether Grocy is configured and reachable.
func (s *Grocy) Status(ctx context.Context) (dto.GrocyStatus, error) {
	status := dto.GrocyStatus{Configured: s.client != nil, BaseURL: s.baseURL}
	if s.client == nil {
		return status, nil
	}
	var info struct {
		Version string `json:"version"`
	}
	if err := s.client.Ping(ctx); err != nil {
		// Reachable=false but not an error: a down instance is a status, not a 500.
		return status, nil
	}
	status.Reachable = true
	_ = info
	return status, nil
}

// ListProducts returns the Grocy product catalog.
func (s *Grocy) ListProducts(ctx context.Context) ([]dto.GrocyProduct, error) {
	if err := s.requireClient(); err != nil {
		return nil, err
	}
	products, err := s.client.ListProducts(ctx)
	if err != nil {
		return nil, fmt.Errorf("service: grocy list products: %w", err)
	}
	out := make([]dto.GrocyProduct, 0, len(products))
	for _, p := range products {
		out = append(out, dto.GrocyProduct{
			ID: p.ID, Name: p.Name, Barcode: p.Barcode,
			LocationID: p.LocationID, QuIDStock: p.QuIDStock,
			QuIDPurchase: p.QuIDPurchase, MinStock: p.MinStock,
		})
	}
	return out, nil
}

// ListStock returns current Grocy stock lots, enriched with product names and
// with zero-amount lots filtered out.
func (s *Grocy) ListStock(ctx context.Context) ([]dto.GrocyStockEntry, error) {
	if err := s.requireClient(); err != nil {
		return nil, err
	}
	products, err := s.client.ListProducts(ctx)
	if err != nil {
		return nil, fmt.Errorf("service: grocy list stock: %w", err)
	}
	nameByID := make(map[int]string, len(products))
	for _, p := range products {
		nameByID[p.ID] = p.Name
	}
	entries, err := s.client.ListStock(ctx)
	if err != nil {
		return nil, fmt.Errorf("service: grocy list stock: %w", err)
	}
	out := make([]dto.GrocyStockEntry, 0, len(entries))
	for _, e := range entries {
		if e.Amount <= 0 {
			continue
		}
		out = append(out, dto.GrocyStockEntry{
			ID: e.ID, ProductID: e.ProductID, ProductName: nameByID[e.ProductID],
			Amount: e.Amount, QuID: e.QuID, LocationID: e.LocationID, BestBefore: e.BestBefore,
		})
	}
	return out, nil
}

// ListShoppingList returns the Grocy shopping list.
func (s *Grocy) ListShoppingList(ctx context.Context) ([]dto.GrocyShoppingItem, error) {
	if err := s.requireClient(); err != nil {
		return nil, err
	}
	items, err := s.client.ListShoppingList(ctx)
	if err != nil {
		return nil, fmt.Errorf("service: grocy list shopping list: %w", err)
	}
	out := make([]dto.GrocyShoppingItem, 0, len(items))
	for _, it := range items {
		out = append(out, dto.GrocyShoppingItem{
			ID: it.ID, ProductID: it.ProductID, Note: it.Note,
			Amount: it.Amount, QuID: it.QuID, Done: it.Done,
		})
	}
	return out, nil
}

// AddStock adds amount to a product's stock on Grocy.
func (s *Grocy) AddStock(ctx context.Context, productID int, amount float64, bestBefore string) error {
	if err := s.requireClient(); err != nil {
		return err
	}
	if amount <= 0 {
		return fmt.Errorf("%w: amount must be > 0", ErrGrocyNotConfigured)
	}
	req := grocy.AddStockRequest{Amount: amount}
	if bestBefore != "" {
		if _, perr := time.Parse("2006-01-02", bestBefore); perr != nil {
			return fmt.Errorf("service: grocy add stock: best_before must be YYYY-MM-DD: %w", perr)
		}
		req.BestBefore = bestBefore
	}
	return s.client.AddStock(ctx, productID, req)
}

// ConsumeStock consumes amount from a product on Grocy.
func (s *Grocy) ConsumeStock(ctx context.Context, productID int, amount float64) error {
	if err := s.requireClient(); err != nil {
		return err
	}
	if amount <= 0 {
		return fmt.Errorf("%w: amount must be > 0", ErrGrocyNotConfigured)
	}
	return s.client.ConsumeStock(ctx, productID, grocy.ConsumeStockRequest{Amount: amount})
}

// AddShoppingItem adds a line to the Grocy shopping list.
func (s *Grocy) AddShoppingItem(ctx context.Context, productID int, note string, amount float64) error {
	if err := s.requireClient(); err != nil {
		return err
	}
	req := grocy.AddShoppingItemRequest{ProductID: productID, Note: note, Amount: amount}
	return s.client.AddShoppingItem(ctx, req)
}
