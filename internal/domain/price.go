package domain

import "time"

// PriceKind is one of the three price kinds that a store_product_offer may have.
// See implement-price-intelligence design.md — price_kind is a CHECK-constrained
// TEXT column, not a free-form value (invariant: only these three are valid).
type PriceKind string

const (
	PriceKindRegular PriceKind = "regular"
	PriceKindMember  PriceKind = "member"
	PriceKindCampaign PriceKind = "campaign"
)

// Retailer is a retail chain (ICA, Willys, Coop, ...). One row per chain.
type Retailer struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

// Store is a specific location within a retailer. Assortment and prices may
// differ per store.
type Store struct {
	ID          string
	RetailerID  string
	Name        string
	CreatedAt   time.Time
}

// RetailerProduct is one retailer's SKU for a canonical Product. Distinct from
// Product — it is the retailer's view of the product, not the product itself.
// ProductID is nullable: a retailer may list a SKU before the canonical mapping
// is resolved (the row is flagged for review until mapped).
type RetailerProduct struct {
	ID           string
	RetailerID   string
	ProductID    string // "" when unmapped
	RetailerSKU  string
	DisplayName  string
	CreatedAt    time.Time
}

// StoreProductOffer is whether a specific store currently carries a specific
// retailer product. Mutable: assortment genuinely changes. The price history
// for this offer lives in price_observation, not here.
type StoreProductOffer struct {
	ID                 int64
	StoreID            string
	RetailerProductID  string
	CurrentlyCarried   bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// PriceObservation is one timestamped, sourced reading of a store_product_offer's
// price. Rows are append-only — never UPDATEd or DELETEd. Reading "the current
// price" is a query over the latest observation(s), never a stored value.
type PriceObservation struct {
	ID                   int64
	StoreProductOfferID  int64
	ObservedAt           time.Time
	Price                float64
	PriceKind            PriceKind
	Source               string
	CreatedAt            time.Time
}

// CurrentStoreProductPrice is the read-optimized shape exposed by the
// current_store_product_price view. Each row is the latest observation for one
// (offer, price_kind) pair.
type CurrentStoreProductPrice struct {
	OfferID             int64
	StoreID             string
	RetailerProductID   string
	PriceKind           PriceKind
	Price               float64
	ObservedAt          time.Time
	Source              string
}
