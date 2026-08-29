package dto

import "context"

// GrocyProduct is the HTTP-side view of a Grocy product.
type GrocyProduct struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	Barcode      string  `json:"barcode"`
	LocationID   int     `json:"location_id"`
	QuIDStock    int     `json:"qu_id_stock"`
	QuIDPurchase int     `json:"qu_id_purchase"`
	MinStock     float64 `json:"min_stock_amount"`
}

// GrocyStockEntry is the HTTP-side view of one Grocy stock lot.
type GrocyStockEntry struct {
	ID         int     `json:"id"`
	ProductID  int     `json:"product_id"`
	ProductName string `json:"product_name"`
	Amount     float64 `json:"amount"`
	QuID       int     `json:"qu_id"`
	LocationID int     `json:"location_id"`
	BestBefore string  `json:"best_before,omitempty"`
}

// GrocyShoppingItem is the HTTP-side view of one Grocy shopping list line.
type GrocyShoppingItem struct {
	ID        int     `json:"id"`
	ProductID int     `json:"product_id"`
	Note      string  `json:"note"`
	Amount    float64 `json:"amount"`
	QuID      int     `json:"qu_id"`
	Done      bool    `json:"done"`
}

// GrocyStatus reports whether a Grocy instance is configured and reachable.
type GrocyStatus struct {
	Configured bool   `json:"configured"`
	Reachable  bool   `json:"reachable"`
	Version    string `json:"version,omitempty"`
	BaseURL    string `json:"base_url,omitempty"`
}

// GrocyService is the surface the /grocy handlers need.
type GrocyService interface {
	// Status reports configuration + reachability.
	Status(ctx context.Context) (GrocyStatus, error)
	// ListProducts returns the Grocy product catalog.
	ListProducts(ctx context.Context) ([]GrocyProduct, error)
	// ListStock returns current Grocy stock lots (zero-amount lots excluded).
	ListStock(ctx context.Context) ([]GrocyStockEntry, error)
	// ListShoppingList returns the Grocy shopping list.
	ListShoppingList(ctx context.Context) ([]GrocyShoppingItem, error)
	// AddStock adds amount to a product's stock on Grocy.
	AddStock(ctx context.Context, productID int, amount float64, bestBefore string) error
	// ConsumeStock consumes amount from a product on Grocy.
	ConsumeStock(ctx context.Context, productID int, amount float64) error
	// AddShoppingItem adds a line to the Grocy shopping list (free-text when
	// productID is 0).
	AddShoppingItem(ctx context.Context, productID int, note string, amount float64) error
}
