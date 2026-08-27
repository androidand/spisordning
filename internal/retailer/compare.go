package retailer

import (
	"context"
	"sync"

	"github.com/androidand/spisordning/internal/domain"
)

// RetailerOrder is the fixed order retailers appear in each ItemComparison's
// Results. Keeping it a named var lets callers and tests rely on a stable
// ordering.
var RetailerOrder = []RetailerKind{RetailerWillys, RetailerICA}

// RetailerResult is one retailer's outcome for a single requirement.
type RetailerResult struct {
	// Retailer identifies which backend produced this result.
	Retailer RetailerKind
	// Available is true when this retailer resolved the requirement to a
	// concrete product. It is false when the retailer is unreachable (its
	// resolve call errored) or when it could not match the requirement.
	Available bool
	// Resolution is the retailer's resolution when it returned one. Zero value
	// when the retailer errored or returned nothing for this item.
	Resolution Resolution
	// PriceValue is the comparable numeric SEK price (per package) when the
	// retailer both resolved the item and knows its price. Nil otherwise.
	PriceValue *float64
	// Error is set when the retailer's resolve call failed entirely (e.g. a
	// stale session) rather than when it simply failed to match this item.
	Error string
}

// ItemComparison is the cross-retailer comparison for one requirement.
type ItemComparison struct {
	// Requirement is the canonical requirement being compared.
	Requirement domain.ShoppingRequirement
	// Results has one entry per retailer, in RetailerOrder.
	Results []RetailerResult
	// Cheapest is the available result with the lowest PriceValue, or nil when
	// no retailer has a comparable price.
	Cheapest *RetailerResult
	// Unresolved is true when no retailer resolved the requirement to a product.
	Unresolved bool
}

// Comparison is the full cross-retailer price comparison for a set of
// requirements.
type Comparison struct {
	Items []ItemComparison
}

// Compare resolves each requirement against every retailer and returns, per
// item, each retailer's resolution + price, the cheapest, and per-retailer
// availability. It never fails on a single retailer: a retailer whose resolve
// call errors (e.g. a stale ICA session) is marked unavailable for every item
// while the others still report — see the retailer-price-comparison spec's
// graceful-degradation requirement.
func Compare(
	ctx context.Context,
	reqs []domain.ShoppingRequirement,
	terms SearchTerms,
	willysURL, icaURL string,
) *Comparison {
	// Resolve all requirements against each retailer in a single call, in
	// parallel. A retailer whose call errors (e.g. a stale ICA session)
	// contributes no resolutions and is marked unavailable for every item,
	// rather than failing the whole comparison.
	n := len(RetailerOrder)
	resByKind := make([][]Resolution, n)
	errByKind := make([]error, n)

	var wg sync.WaitGroup
	for pos, kind := range RetailerOrder {
		wg.Add(1)
		go func(pos int, kind RetailerKind) {
			defer wg.Done()
			c, err := NewFromKind(kind, willysURL, icaURL)
			if err != nil {
				errByKind[pos] = err
				return
			}
			res, err := c.ResolveRequirements(ctx, reqs, terms)
			if err != nil {
				errByKind[pos] = err
				return
			}
			resByKind[pos] = res
		}(pos, kind)
	}
	wg.Wait()

	// Index each retailer's resolutions by ingredient id so a requirement maps
	// to its own resolution even if an adapter reorders or drops entries.
	idxByKind := make([]map[string]Resolution, n)
	for pos := range RetailerOrder {
		m := make(map[string]Resolution, len(resByKind[pos]))
		for _, r := range resByKind[pos] {
			if r.IngredientID != "" {
				m[r.IngredientID] = r
			}
		}
		idxByKind[pos] = m
	}

	cmp := &Comparison{Items: make([]ItemComparison, 0, len(reqs))}
	for i, req := range reqs {
		item := ItemComparison{
			Requirement: req,
			Results:     make([]RetailerResult, 0, n),
		}
		anyResolved := false
		for pos, kind := range RetailerOrder {
			rr := RetailerResult{Retailer: kind}
			if errByKind[pos] != nil {
				rr.Error = errByKind[pos].Error()
			} else if r, ok := idxByKind[pos][req.IngredientID]; ok {
				rr.Resolution = r
			} else if i < len(resByKind[pos]) {
				// Positional fallback for an adapter that did not echo the id.
				rr.Resolution = resByKind[pos][i]
			}
			if rr.Resolution.RetailerProductID != nil && *rr.Resolution.RetailerProductID != "" {
				rr.Available = true
				rr.PriceValue = rr.Resolution.PriceValue
				anyResolved = true
			}
			item.Results = append(item.Results, rr)
		}
		item.Unresolved = !anyResolved
		for i := range item.Results {
			r := &item.Results[i]
			if !r.Available || r.PriceValue == nil {
				continue
			}
			if item.Cheapest == nil || *r.PriceValue < *item.Cheapest.PriceValue {
				item.Cheapest = r
			}
		}
		cmp.Items = append(cmp.Items, item)
	}
	return cmp
}
