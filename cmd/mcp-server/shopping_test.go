package main

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/androidand/spisordning/internal/mcptools"
)

// Fakes for the shopping services. They implement the mcptools service
// interfaces so the round-trip test can drive the real composition root over
// Streamable HTTP without a database or live retailer adapters.

type fakeShoppingList struct {
	calls  int
	last   mcptools.CreateShoppingListInput
	nextID int64
}

func (f *fakeShoppingList) CreateShoppingList(_ context.Context, in mcptools.CreateShoppingListInput) (mcptools.CreateShoppingListResult, error) {
	f.calls++
	f.last = in
	f.nextID++
	return mcptools.CreateShoppingListResult{
		ListID: f.nextID,
		Name:   in.Name,
		Status: "active",
		Items:  len(in.Items),
	}, nil
}

type fakeCompare struct {
	calls int
	last  []mcptools.ShoppingRequirement
}

func (f *fakeCompare) ComparePrices(_ context.Context, reqs []mcptools.ShoppingRequirement) (mcptools.PriceComparison, error) {
	f.calls++
	f.last = reqs
	ing := "mjolk"
	if len(reqs) > 0 {
		ing = reqs[0].Ingredient
	}
	willysPrice := 29.9
	willysID := "101233931_ST"
	willysName := "Mjölk 3%"
	willysDisplay := "29,90 kr"
	return mcptools.PriceComparison{
		Items: []mcptools.ItemComparison{
			{
				Ingredient: ing,
				Results: []mcptools.RetailerPriceResult{
					{Retailer: "willys", Available: true, ProductID: &willysID, ProductName: &willysName, PriceValue: &willysPrice, Price: &willysDisplay},
					{Retailer: "ica", Available: false},
				},
				Cheapest:   &mcptools.RetailerPriceResult{Retailer: "willys", Available: true, PriceValue: &willysPrice},
				Unresolved: false,
			},
		},
	}, nil
}

type fakeWishlist struct {
	calls int
	last  mcptools.PushWishlistInput
}

func (f *fakeWishlist) PushToWishlist(_ context.Context, in mcptools.PushWishlistInput) (mcptools.PushWishlistResult, error) {
	f.calls++
	f.last = in
	return mcptools.PushWishlistResult{
		Retailer:       in.Retailer,
		WishlistID:     "wl-123",
		ListName:       in.ListName,
		Items:          len(in.Items),
		ShoppingListID: in.ShoppingListID,
	}, nil
}

// TestIntegration_ShoppingRoundTrip drives the real MCP server end-to-end over
// Streamable HTTP: create a shopping list, compare prices across retailers, and
// push the chosen resolutions to a Willys wishlist bound to the created list.
// It asserts both the structured responses and the side effects on the fakes.
func TestIntegration_ShoppingRoundTrip(t *testing.T) {
	shoppingList := &fakeShoppingList{}
	compare := &fakeCompare{}
	wishlist := &fakeWishlist{}
	deps := mcptools.Dependencies{
		ShoppingList: shoppingList,
		Compare:      compare,
		Wishlist:     wishlist,
	}

	cs := connectClient(t, startServer(t, deps))

	// 1. Create a shopping list from requirements.
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "create_shopping_list",
		Arguments: map[string]any{
			"name": "Vecka 34",
			"items": []map[string]any{
				{"ingredient": "mjolk", "quantity": 2, "unit": "pcs"},
			},
		},
	})
	if err != nil {
		t.Fatalf("call create_shopping_list: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}
	created := structured[mcptools.CreateShoppingListResult](t, res)
	if created.ListID != 1 || created.Name != "Vecka 34" || created.Items != 1 {
		t.Fatalf("unexpected create result: %+v", created)
	}
	if shoppingList.calls != 1 || shoppingList.last.Name != "Vecka 34" || len(shoppingList.last.Items) != 1 {
		t.Fatalf("unexpected create side effect: %+v", shoppingList.last)
	}
	if shoppingList.last.Items[0].Ingredient != "mjolk" || shoppingList.last.Items[0].Quantity != 2 {
		t.Fatalf("unexpected create item: %+v", shoppingList.last.Items[0])
	}

	// 2. Compare prices across retailers.
	res2, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "compare_shopping_prices",
		Arguments: map[string]any{
			"requirements": []map[string]any{
				{"ingredient": "mjolk", "quantity": 2, "unit": "pcs"},
			},
		},
	})
	if err != nil {
		t.Fatalf("call compare_shopping_prices: %v", err)
	}
	if res2.IsError {
		t.Fatalf("unexpected tool error: %+v", res2)
	}
	cmp := structured[mcptools.PriceComparison](t, res2)
	if len(cmp.Items) != 1 {
		t.Fatalf("expected 1 compared item, got %d", len(cmp.Items))
	}
	if cmp.Items[0].Cheapest == nil || cmp.Items[0].Cheapest.Retailer != "willys" {
		t.Fatalf("unexpected cheapest: %+v", cmp.Items[0].Cheapest)
	}
	// ICA must degrade to available:false rather than failing the call.
	icaSeen, icaAvailable := false, false
	for _, r := range cmp.Items[0].Results {
		if r.Retailer == "ica" {
			icaSeen = true
			icaAvailable = r.Available
		}
	}
	if !icaSeen {
		t.Fatalf("expected an ICA result in the comparison")
	}
	if icaAvailable {
		t.Fatalf("expected ICA to be unavailable (degraded), got available")
	}
	if compare.calls != 1 || len(compare.last) != 1 || compare.last[0].Ingredient != "mjolk" {
		t.Fatalf("unexpected compare side effect: %+v", compare.last)
	}

	// 3. Push the chosen resolutions to a Willys wishlist, bound to the list.
	listID := created.ListID
	res3, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "push_shopping_wishlist",
		Arguments: map[string]any{
			"retailer": "willys",
			"list_name": "Vecka 34",
			"items": []map[string]any{
				{"product_code": "101233931_ST", "quantity": 2},
			},
			"shopping_list_id": listID,
		},
	})
	if err != nil {
		t.Fatalf("call push_shopping_wishlist: %v", err)
	}
	if res3.IsError {
		t.Fatalf("unexpected tool error: %+v", res3)
	}
	pushed := structured[mcptools.PushWishlistResult](t, res3)
	if pushed.Retailer != "willys" || pushed.WishlistID != "wl-123" || pushed.Items != 1 {
		t.Fatalf("unexpected push result: %+v", pushed)
	}
	if wishlist.calls != 1 || wishlist.last.Retailer != "willys" {
		t.Fatalf("unexpected push side effect: %+v", wishlist.last)
	}
	if wishlist.last.ShoppingListID == nil || *wishlist.last.ShoppingListID != listID {
		t.Fatalf("expected push to bind to list %d, got %+v", listID, wishlist.last.ShoppingListID)
	}
	if len(wishlist.last.Items) != 1 || wishlist.last.Items[0].ProductCode != "101233931_ST" || wishlist.last.Items[0].Quantity != 2 {
		t.Fatalf("unexpected push item: %+v", wishlist.last.Items[0])
	}
}

// TestIntegration_PushWishlistRejectsUnknownRetailer verifies the tool rejects a
// retailer that is not willys or ica before reaching the application layer.
func TestIntegration_PushWishlistRejectsUnknownRetailer(t *testing.T) {
	wishlist := &fakeWishlist{}
	cs := connectClient(t, startServer(t, mcptools.Dependencies{Wishlist: wishlist}))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "push_shopping_wishlist",
		Arguments: map[string]any{
			"retailer":  "systembolaget",
			"list_name": "Vecka 34",
			"items": []map[string]any{
				{"product_code": "123", "quantity": 1},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected protocol error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected tool error for unknown retailer, got %+v", res)
	}
	if wishlist.calls != 0 {
		t.Fatalf("fake wishlist called %d times, want 0 (unknown retailer must not reach the application layer)", wishlist.calls)
	}
}
