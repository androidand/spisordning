package persistence

import (
	"context"
	"testing"
	"time"
)

func TestOrder_CreateAndGet(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "order_item", "order", "shopping_cart_item", "shopping_cart", "retailer_list_binding", "shopping_list_item", "shopping_list")

	cartID := int64(1)
	order, err := s.CreateOrder(ctx, Order{
		ShoppingCartID: &cartID,
		Retailer:       "willys",
		Source:         "manual",
		OrderedAt:      time.Date(2026, 7, 20, 18, 30, 0, 0, time.UTC),
		TotalPrice:     ptrFloat64(149.5),
	})
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}
	got, err := s.GetOrder(ctx, order)
	if err != nil {
		t.Fatalf("GetOrder: %v", err)
	}
	if got.Retailer != "willys" || got.Source != "manual" {
		t.Errorf("got %+v", got)
	}
	if got.TotalPrice == nil || *got.TotalPrice != 149.5 {
		t.Errorf("total_price = %v, want 149.5", got.TotalPrice)
	}
}

func TestOrder_NullCartReference(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "order_item", "order")

	// Manual order without a cart reference (source='manual', no cart).
	order, err := s.CreateOrder(ctx, Order{
		Retailer: "willys",
		Source:   "manual",
		OrderedAt: time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}
	got, err := s.GetOrder(ctx, order)
	if err != nil {
		t.Fatalf("GetOrder: %v", err)
	}
	if got.ShoppingCartID != nil {
		t.Errorf("expected nil shopping_cart_id, got %d", *got.ShoppingCartID)
	}
}

func TestOrder_ListFilters(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "order_item", "order")

	cart1 := int64(1)
	cart2 := int64(2)
	_, err := s.CreateOrder(ctx, Order{ShoppingCartID: &cart1, Retailer: "willys", Source: "manual", OrderedAt: time.Now()})
	if err != nil {
		t.Fatalf("CreateOrder 1: %v", err)
	}
	_, err = s.CreateOrder(ctx, Order{ShoppingCartID: &cart2, Retailer: "willys", Source: "manual", OrderedAt: time.Now()})
	if err != nil {
		t.Fatalf("CreateOrder 2: %v", err)
	}
	_, err = s.CreateOrder(ctx, Order{ShoppingCartID: &cart1, Retailer: "ica", Source: "manual", OrderedAt: time.Now()})
	if err != nil {
		t.Fatalf("CreateOrder 3: %v", err)
	}

	// Filter by retailer.
	willys, err := s.ListOrders(ctx, nil, ptrString("willys"))
	if err != nil {
		t.Fatalf("ListOrders willys: %v", err)
	}
	if len(willys) != 2 {
		t.Errorf("expected 2 willys orders, got %d", len(willys))
	}

	// Filter by cart.
	cart1Orders, err := s.ListOrders(ctx, &cart1, nil)
	if err != nil {
		t.Fatalf("ListOrders cart1: %v", err)
	}
	if len(cart1Orders) != 2 {
		t.Errorf("expected 2 cart1 orders, got %d", len(cart1Orders))
	}
}

func TestOrderItem_RoundTrip(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "order_item", "order")

	orderID, err := s.CreateOrder(ctx, Order{
		Retailer: "willys",
		Source:   "manual",
		OrderedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}

	unitPrice := 49.5
	totalPrice := 99.0
	subFor := int64(1)
	item, err := s.CreateOrderItem(ctx, OrderItem{
		OrderID:              orderID,
		RetailerProductID:    "willys-123",
		Quantity:             2,
		UnitPrice:            &unitPrice,
		TotalPrice:           &totalPrice,
		SubstitutedForItemID: &subFor,
	})
	if err != nil {
		t.Fatalf("CreateOrderItem: %v", err)
	}

	items, err := s.ListOrderItems(ctx, orderID)
	if err != nil {
		t.Fatalf("ListOrderItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].RetailerProductID != "willys-123" || items[0].Quantity != 2 {
		t.Errorf("item = %+v", items[0])
	}
	if items[0].SubstitutedForItemID == nil || *items[0].SubstitutedForItemID != 1 {
		t.Errorf("substituted_for_item_id = %v, want 1", items[0].SubstitutedForItemID)
	}
	_ = item
}

func TestOrderItem_NullPrices(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "order_item", "order")

	orderID, err := s.CreateOrder(ctx, Order{
		Retailer: "willys",
		Source:   "manual",
		OrderedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}

	// No unit_price or total_price — manual entry may not have exact prices.
	_, err = s.CreateOrderItem(ctx, OrderItem{
		OrderID:           orderID,
		RetailerProductID: "willys-456",
		Quantity:          1,
		UnitPrice:         nil,
		TotalPrice:        nil,
	})
	if err != nil {
		t.Fatalf("CreateOrderItem (no prices): %v", err)
	}
	items, err := s.ListOrderItems(ctx, orderID)
	if err != nil {
		t.Fatalf("ListOrderItems: %v", err)
	}
	if len(items) != 1 || items[0].UnitPrice != nil || items[0].TotalPrice != nil {
		t.Errorf("expected null prices, got %+v", items[0])
	}
}

func ptrString(s string) *string { return &s }
func ptrFloat64(f float64) *float64 { return &f }
