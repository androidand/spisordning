// Command food-brain sync-offers fetches current offers from a retailer adapter
// and prints them. Persistence into price-intelligence tables is gated on
// implement-price-intelligence (tasks 1.5 + 2.5) — this command is the fetch
// edge today; the store-pinning + persistence path is wired once that schema
// lands.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/androidand/spisordning/internal/config"
	"github.com/androidand/spisordning/internal/retailer"
)

func runSyncOffers(args []string) error {
	fs := flag.NewFlagSet("sync-offers", flag.ExitOnError)
	retailerFlag := fs.String("retailer", "willys", "retailer backend: willys (default) or ica")
	storeID := fs.String("store", "", "store id to filter offers (ica-adapter: ICA_STORE_ID env)")
	output := fs.String("output", "", "write offers to this file as JSON (default: stdout)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg := config.Load()
	ctx := context.Background()
	kind := retailer.RetailerKind(*retailerFlag)
	rc, err := retailer.NewFromKind(kind, cfg.AdapterURL, cfg.ICAAdapterURL, cfg.HemkopAdapterURL)
	if err != nil {
		return fmt.Errorf("sync-offers: %w", err)
	}
	rc.WithAuthFile(cfg.ICAAuthFile)

	fmt.Printf("Fetching offers from %s-adapter", kind)
	if *storeID != "" {
		fmt.Printf(" (store=%s)", *storeID)
	}
	fmt.Println("...")

	offers, err := rc.SyncOffers(ctx, *storeID)
	if err != nil {
		return fmt.Errorf("sync-offers: %w", err)
	}

	out, err := json.MarshalIndent(offers, "", "  ")
	if err != nil {
		return fmt.Errorf("sync-offers: marshal: %w", err)
	}
	out = append(out, '\n')

	if *output != "" {
		if err := os.WriteFile(*output, out, 0o644); err != nil {
			return fmt.Errorf("sync-offers: write: %w", err)
		}
		fmt.Printf("✅ wrote %d offer(s) to %s\n", len(offers), *output)
	} else {
		fmt.Printf("✅ %d offer(s):\n%s\n", len(offers), string(out))
	}
	return nil
}
