package domain

import "time"

// ShoppingList is a durable, retailer-independent shopping list owned by
// spisordning. It is distinct from any retailer's own list representation.
// See migrations/0004_shopping_list.sql and design.md §1.
type ShoppingList struct {
	ID             int64
	OwnerPersonID  *string
	Name           string
	Status         string // 'active' | 'archived'
	CreatedAt      time.Time
}

// ShoppingListItem is one line on a shopping_list. At most one of the three
// identifiers below is the "what is this" for the row: the shopping_requirement
// it was seeded from, a direct ingredient reference, or a free-text label for
// non-ingredient items (e.g. "paper towels"). No retailer product id —
// resolution is the adapter's job (design D1).
type ShoppingListItem struct {
	ID                      int64
	ShoppingListID          int64
	ShoppingRequirementID   *int64
	IngredientID            *string
	Label                   *string
	Quantity                float64
	Unit                    string
	Checked                 bool
	AddedAt                 time.Time
}

// RetailerListBinding records that a shopping_list has been projected onto a
// specific external retailer list (e.g. a Willys wishlist id). The spisordning
// shopping_list is authoritative for intent; the retailer's list is a
// synchronized projection. v1 is outbound-only (design D2).
type RetailerListBinding struct {
	ID              int64
	ShoppingListID  int64
	Retailer        string // e.g. 'willys'
	ExternalListID  string // the adapter's wishlistId string
	SyncDirection   string // 'outbound' in v1
	LastPushedAt    *time.Time
	LastPushStatus  *string // 'success' | 'error'
}
