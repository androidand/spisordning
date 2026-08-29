package main

import (
	"context"
	"fmt"
	"time"

	"github.com/androidand/spisordning/internal/config"
	"github.com/androidand/spisordning/internal/domain"
	"github.com/androidand/spisordning/internal/persistence"
	"github.com/androidand/spisordning/internal/retailer"
)

// PushShoppingList projects a shopping_list onto a retailer's own list (v1,
// outbound-only — see openspec/changes/implement-shopping-and-commerce/design.md
// "Retailer list binding"). It resolves each item's canonical ingredient to a
// retailer product, creates (or re-creates) the retailer wishlist, and
// records the outcome on the shopping_list's retailer_list_binding row. It
// never fills a cart or checks out — see storeAdapter.ToCart for that step.
func PushShoppingList(ctx context.Context, db *persistence.Store, listID int64, adapterURL, retailerName string) (*retailer.CreatedList, error) {
	list, err := db.GetShoppingList(ctx, listID)
	if err != nil {
		return nil, fmt.Errorf("push shopping list: get list: %w", err)
	}
	items, err := db.ListShoppingListItems(ctx, listID)
	if err != nil {
		return nil, fmt.Errorf("push shopping list: list items: %w", err)
	}

	kind := retailer.RetailerKind(retailerName)
	rc, err := retailer.NewFromKind(kind, adapterURL, adapterURL, adapterURL)
	if err != nil {
		recordPushFailure(ctx, db, listID, retailerName)
		return nil, fmt.Errorf("push shopping list: retailer: %w", err)
	}
	rc.WithAuthFile(config.Load().ICAAuthFile)

	var reqs []domain.ShoppingRequirement
	terms := retailer.SearchTerms{}
	for _, item := range items {
		id := ""
		if item.IngredientID != nil {
			id = *item.IngredientID
		} else if item.Label != nil {
			id = domain.CanonicalIngredientID(*item.Label)
		}
		if id == "" {
			continue // nothing to resolve against
		}
		reqs = append(reqs, domain.ShoppingRequirement{IngredientID: id, Quantity: item.Quantity, Unit: item.Unit})
		if item.Label != nil {
			terms[id] = *item.Label
		}
	}

	resolutions, err := rc.ResolveRequirements(ctx, reqs, terms)
	if err != nil {
		recordPushFailure(ctx, db, listID, retailerName)
		return nil, fmt.Errorf("push shopping list: resolve: %w", err)
	}

	var retItems []retailer.ShoppingListItem
	for _, res := range resolutions {
		if res.NeedsReview || res.RetailerProductID == nil {
			continue // never silently push a review-flagged item
		}
		retItems = append(retItems, retailer.ShoppingListItem{ProductCode: *res.RetailerProductID, Quantity: res.Packages})
	}
	if len(retItems) == 0 {
		recordPushFailure(ctx, db, listID, retailerName)
		return nil, fmt.Errorf("push shopping list: nothing confidently resolved")
	}

	created, err := rc.CreateShoppingList(ctx, list.Name, retItems)
	if err != nil {
		recordPushFailure(ctx, db, listID, retailerName)
		return nil, fmt.Errorf("push shopping list: create: %w", err)
	}

	now := time.Now()
	status := "success"
	if err := db.CreateOrUpdateRetailerListBinding(ctx, persistence.RetailerListBinding{
		ShoppingListID: listID, Retailer: retailerName, ExternalListID: created.WishlistID,
		SyncDirection: "outbound", LastPushedAt: &now, LastPushStatus: &status,
	}); err != nil {
		return nil, fmt.Errorf("push shopping list: record binding: %w", err)
	}
	return created, nil
}

// recordPushFailure marks the binding as failed so GET responses surface the
// error state rather than silently reusing stale success data. Errors here
// are swallowed (best-effort) — the caller already has the real error to report.
func recordPushFailure(ctx context.Context, db *persistence.Store, listID int64, retailerName string) {
	now := time.Now()
	status := "error"
	existing, err := db.GetRetailerListBinding(ctx, listID, retailerName)
	external := ""
	if err == nil {
		external = existing.ExternalListID
	}
	_ = db.CreateOrUpdateRetailerListBinding(ctx, persistence.RetailerListBinding{
		ShoppingListID: listID, Retailer: retailerName, ExternalListID: external,
		SyncDirection: "outbound", LastPushedAt: &now, LastPushStatus: &status,
	})
}
