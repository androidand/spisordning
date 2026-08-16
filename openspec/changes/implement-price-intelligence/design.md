## Context

`PLAN.md`'s "Price Model" section asks explicitly: "Research whether current price should be
mutable or represented as observations," and lists a likely model of `retailer_products` /
`store_product_offers` / `price_observations` — three nouns, which already hints at an answer, but
`PLAN.md` treats it as an open question rather than a given. This design resolves it before any
migration is written, per `PLAN.md`'s own "Database Design Process" (vocabulary → aggregates →
relationships → lifecycle → commands → invariants → persistence, in that order).

## Goals / Non-Goals

**Goals:**
- Decide, with reasoning, whether price is a mutable field or an append-only observation series.
- Define `retailer`, `store`, `retailer_product`, `store_product_offer`, `price_observation` and
  how they relate to the existing `Ingredient`/`Product` chain (owned elsewhere) and to
  `retailer-adapter`'s existing resolved-product output.
- Set up (without building) the foundation basket optimization, offer detection, and price trends
  would need.

**Non-Goals:**
- Not implementing basket optimization, offer detection, or trend UI.
- Not deciding Ingredient-vs-Product modeling (already decided elsewhere, non-negotiable per
  `PLAN.md`).
- Not committing to any specific Swedish price-intelligence source as an ingestion pipeline before
  its research task (`tasks.md` §3) confirms current terms.

## Decision: price is an observation series, not a mutable field

**Conclusion: `price_observations` is append-only; there is no mutable "current price" column
anywhere in the schema. "Current price" is always a derived read (latest observation per offer),
never a field that gets UPDATEd in place.**

### Reasoning

1. **`PLAN.md` says why it wants price data at all**: "Price history may later support basket
   optimization, offer detection, price trends." All three of those are impossible to build later
   if only the current price was ever kept — offer detection specifically requires knowing what a
   price *was* to notice it changed, and a trend is definitionally a time series. Choosing a
   mutable field would foreclose these stated future features; choosing an observation series
   keeps them possible at zero extra cost today (an INSERT costs the same as an UPDATE).

2. **Real-world grocery prices already behave as a series, not a fact**: the same
   `retailer_product` at the same `store` legitimately has different prices depending on *when*
   observed — regular price, member price, campaign price, price after a competitor-matching
   adjustment. `willys-capabilities.md` confirms the Willys search-result `Product` already
   exposes three price-shaped fields (`price`, `priceValue`, `comparePrice`) at once for a single
   product — that's already evidence a single scalar "the price" is insufficient; a single ingest
   naturally produces multiple observations (e.g. one row tagged `kind='regular'`, one
   `kind='member'`, one `kind='campaign'`), not one mutable value overwritten by whichever field
   happened to be read last.

3. **Multiple independent ingestion sources will observe the same offer at different times**: the
   retailer-adapter's own search results, and (pending their research tasks) any of Primat,
   Matpriskollen, Matmoms, Matpriser.nu, Comparator, or Open Prices. An observation series lets
   each source's reading stand on its own, timestamped and sourced, rather than requiring a
   reconciliation policy for "whose write wins" on a single mutable column — which
   `PLAN.md`'s own "Do not use generic polymorphism carelessly" / prefer-real-relations guidance
   argues against solving with a fragile last-write-wins field.

4. **Cost is acceptable**: grocery price observations are low-cardinality and low-frequency
   relative to, say, sensor telemetry — one row per (offer, source, timestamp) tuple, expected at
   most a few times a day per tracked product. This is not a case where append-only history is
   disproportionately expensive; `PLAN.md`'s own caution against inventing complexity ("Are we
   using JSON because it is correct or because modeling is difficult?") does not apply here — a
   plain relational history table is the *simpler* correct model, not a complex one adopted for
   its own sake.

### What this looks like

```
retailer            store                  retailer_product         store_product_offer          price_observation
(ICA, Willys, ...)  (specific location,    (a retailer's SKU for    (that SKU, offered at         (one reading of that
                     belongs to a retailer)  a Product — owned        a specific store — this       offer's price, at a
                                              elsewhere)               is what varies by store)      point in time, from
                                                                                                       a named source)
     │                    │                        │                          │                            │
     └──── has many ─────▶│                        │                          │                            │
                            └──── offers a ────────▶│ ◀── references Product   │                            │
                                                      └──── priced via ───────▶│                            │
                                                                                 └──── observed as ─────────▶│
```

`store_product_offer` is the *current assortment fact* — "this store currently carries this SKU"
— and is itself allowed to be mutable (assortment genuinely changes: a store stops carrying an
item). What must never be mutable is the **price** attached to it; that always lives in
`price_observation`, keyed to the offer, with `observed_at`, `price`, `price_kind`
(`'regular'|'member'|'campaign'`), and `source` (`'willys_adapter'|'primat'|'matpriskollen'|...`).
"Current price for an offer" is a query (`ORDER BY observed_at DESC LIMIT 1` per `price_kind`), not
a stored column — optionally materialized later as a read-optimized view once real query patterns
exist, but never as the system of record.

## Risks / Trade-offs

- **Query complexity**: reading "the current price" requires a latest-per-group query instead of a
  flat column read; mitigated by a view (`current_store_product_price`) so callers don't hand-roll
  it repeatedly.
- **Storage growth**: unbounded append growth over years; acceptable at expected volume (see
  Reasoning §4), revisit with retention/rollup policy only if it becomes a real problem.
- **Multi-source disagreement**: two sources may report different prices for the same offer at
  overlapping times; the observation series preserves both rather than forcing a false single
  truth — reconciliation (if ever needed for display) happens at read time, not write time.
