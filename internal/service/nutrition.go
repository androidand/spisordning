// Package service — nutrition sync service (research-nutrition-data-sources
// task 6: `food-brain sync nutrition`). Persists SLV foods + nutrients, Dabas
// products, and the cross-reference mappings into the nutrition tables, tracking
// the last full sync per source for incremental updates.
package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/androidand/spisordning/internal/ingredients"
	"github.com/androidand/spisordning/internal/persistence"
)

// NutritionSync is the nutrition sync job. It fetches full datasets from SLV and
// Dabas and persists them idempotently into the nutrition tables, recording a
// sync-status row per source so later runs can be incremental.
type NutritionSync struct {
	db    Store
	slv   *ingredients.Client
	dabas *ingredients.DabasClient
}

// NewNutritionSync builds a nutrition sync service. Pass nil for any client that
// is not configured (e.g. no SLV_BASE_URL → nil); the corresponding sync is then
// skipped with an informative error.
func NewNutritionSync(db Store, slv *ingredients.Client, dabas *ingredients.DabasClient) *NutritionSync {
	return &NutritionSync{db: db, slv: slv, dabas: dabas}
}

// SyncResult summarises one source's sync: how many records were written and when.
type SyncResult struct {
	Source     string
	Written    int
	SyncedAt   time.Time
	Skipped    bool
}

// SyncAll runs a full sync of every configured source (SLV first, then Dabas)
// and returns a result per source. A nil client for a source is reported as a
// skipped result (not an error) so one missing API key does not abort the rest.
func (s *NutritionSync) SyncAll(ctx context.Context) ([]SyncResult, error) {
	var results []SyncResult
	if s.slv != nil {
		r, err := s.SyncSLV(ctx)
		if err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	if s.dabas != nil {
		r, err := s.SyncDabas(ctx)
		if err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("service: nutrition sync: no sources configured (set SLV_BASE_URL and/or DABAS_BASE_URL)")
	}
	return results, nil
}

// SyncSLV runs the Livsmedelsverket full sync: paginates the complete food
// dataset with nutrition prefetched, persists each food + its nutrients, and
// records a sync-status row.
func (s *NutritionSync) SyncSLV(ctx context.Context) (SyncResult, error) {
	if s.slv == nil {
		return SyncResult{Source: "slv", Skipped: true}, nil
	}
	all, err := s.slv.SyncAll(ctx, ingredients.SprakSwedish)
	if err != nil {
		return SyncResult{Source: "slv"}, fmt.Errorf("service: sync SLV: %w", err)
	}

	written := 0
	for _, item := range all {
		f := persistence.Food{
			SlvNummer:        item.Food.Nummer,
			Namn:             item.Food.Namn,
			VetenskapligtNamn: nullableStr(item.Food.VetenskapligtNamn),
			LivsmedelsTyp:    nullableStr(item.Food.LivsmedelsTyp),
			Projekt:          nullableStr(item.Food.Projekt),
			Version:          nullableStr(item.Food.Version),
		}
		if err := s.db.UpsertFood(ctx, f); err != nil {
			return SyncResult{Source: "slv"}, err
		}
		nutr := make([]persistence.Nutrient, 0, len(item.Nutrition))
		for _, n := range item.Nutrition {
			nutr = append(nutr, persistence.Nutrient{
				FoodNummer:  item.Food.Nummer,
				EuroFIRKod:  nullableStr(n.EuroFIRKod),
				Name:        n.Namn,
				Värde:       n.Värde,
				Enhet:       n.Enhet,
				Metodtyp:    nullableStr(n.Metodtyp),
			})
		}
		if err := s.db.UpsertNutrients(ctx, item.Food.Nummer, nutr); err != nil {
			return SyncResult{Source: "slv"}, err
		}
		written++
	}

	if err := s.db.UpsertNutritionSyncStatus(ctx, persistence.NutritionSyncStatus{
		Source:      "slv",
		LastSynced:  time.Now().UTC(),
		RecordCount: written,
	}); err != nil {
		return SyncResult{Source: "slv"}, err
	}

	return SyncResult{Source: "slv", Written: written, SyncedAt: time.Now().UTC()}, nil
}

// SyncDabas persists the Dabas products found for the given queries as
// product_mappings keyed on GTIN and Arident. Dabas has no documented "list all"
// endpoint, so the caller supplies the search terms to enumerate the namespace
// (empty query returns the first page only); full-dataset Dabas sync is tracked
// as a follow-up. Each resolved mapping links a Dabas product to the canonical
// SLV food once resolveProductToSLV has run.
func (s *NutritionSync) SyncDabas(ctx context.Context, queries ...string) (SyncResult, error) {
	if s.dabas == nil {
		return SyncResult{Source: "dabas", Skipped: true}, nil
	}

	written := 0
	for _, q := range queries {
		err := s.dabas.SearchAll(ctx, q, func(page *ingredients.DabasSearchResult) error {
			for _, p := range page.Results {
				mp := persistence.ProductMapping{
					GTIN:         nullableStrPtr(p.GTIN),
					DabasARIdent: nullableStrPtr(p.ArticleID),
				}
				if err := s.db.UpsertProductMapping(ctx, mp); err != nil {
					return err
				}
				written++
			}
			return nil
		})
		if err != nil {
			return SyncResult{Source: "dabas"}, fmt.Errorf("service: sync Dabas query %q: %w", q, err)
		}
	}

	if err := s.db.UpsertNutritionSyncStatus(ctx, persistence.NutritionSyncStatus{
		Source:      "dabas",
		LastSynced:  time.Now().UTC(),
		RecordCount: written,
	}); err != nil {
		return SyncResult{Source: "dabas"}, err
	}

	return SyncResult{Source: "dabas", Written: written, SyncedAt: time.Now().UTC()}, nil
}

// SyncStatusFor returns the last full-sync status for a source ("slv" / "dabas"),
// or a zero result with ok=false if no sync has run yet.
func (s *NutritionSync) SyncStatusFor(ctx context.Context, source string) (persistence.NutritionSyncStatus, bool, error) {
	st, err := s.db.GetNutritionSyncStatus(ctx, source)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") || strings.Contains(err.Error(), "ErrNoRows") {
			return persistence.NutritionSyncStatus{}, false, nil
		}
		return persistence.NutritionSyncStatus{}, false, err
	}
	return st, true, nil
}

// nullableStr copies a string pointer, returning nil for nil or empty.
func nullableStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// nullableStrPtr copies a string pointer through, returning nil for nil or empty.
func nullableStrPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
