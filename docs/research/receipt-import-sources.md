# Receipt import sources — evaluation

`implement-shopping-and-commerce` section 5 is **research-only**: evaluate the plausible sources
for populating `order` records and record findings. No receipt parser is implemented in this
change. The design decision (D4) is that `order.source` is an explicit enum
(`'manual' | 'retailer_api' | 'receipt_import'`) because no retailer in scope has a real
order-history API today; this document evaluates which sources are plausible and in what priority.

## 5.1 Retailer API as a receipt source

- **Willys: currently infeasible.** Confirmed from `willys-capabilities.md`: the
  `src/personal/service.ts` `personalElementList`/`personalElement` endpoints exist but are an
  **untyped stub** (`{ order: any; digitalReceipt: any }`), unused, with no real order-history
  retrieval or receipt parsing. There is no typed, working order-history API. Flag as currently
  infeasible; re-evaluate if Willys ships a real order-history/receipt API.
- **ICA: pending `research-and-integrate-ica` findings.** Unknown whether ICA exposes an
  order-history or receipt API; this is tracked in the separate `research-and-integrate-ica`
  change. Not evaluated here.

## 5.2 PDF receipt import

Swedish retailers (Willys, ICA, Coop) typically offer a PDF receipt via email or their app.

- **Format variability:** layouts, fonts, field names, and structure differ across retailers and
  change over time. A per-retailer parser is feasible but requires maintenance; a generic parser
  is harder due to the variability.
- **Text layer vs. scan:** most modern retailer PDFs have a digital text layer, so
  **structured-text extraction is more feasible than OCR**. A scanned-image PDF would require OCR
  (more complex, less reliable).
- **Extraction targets:** line items (product, quantity, price), totals, tax, payment method,
  date, store. The main challenge is mapping the variable layout to these fields.
- **Feasibility: moderate.** A per-retailer PDF parser (text-layer extraction + layout
  heuristics) is tractable; a generic parser is not. This is the most plausible *automated*
  source for retailers that offer PDF receipts.

## 5.3 Kivra digital-mailbox export

Kivra (kivra.se) is a Swedish secure digital-mailbox service for receiving digital mail.

- **What retailers deliver to Kivra:** Kivra is primarily used for bills and official mail (banks,
  telecom, public agencies). Whether grocery retailers (Willys, ICA) deliver receipts to Kivra is
  **uncertain and needs verification**. If they do not, Kivra is not a useful receipt source for
  this project.
- **Export/API access:** Kivra provides an API for listing and downloading documents from a
  mailbox, plus the Kivra app for manual access. API access requires authentication and is subject
  to Kivra's terms. The exact scope of the API (which document types, rate limits, retention)
  needs verification before it can be treated as a source.
- **Feasibility: uncertain, pending verification.** The key open question is whether the
  retailers in scope deliver receipts to Kivra. If yes, Kivra is a plausible structured source
  (PDFs in a mailbox, accessible via API). If no, it is not useful. Flag as a verification task,
  not a confirmed source.

## 5.4 Email receipt parsing

Swedish retailers typically send receipts via email (PDF attachment or inline HTML).

- **PDF attachment:** same as 5.2 (PDF receipt import).
- **Inline HTML:** structured-text parsing (easier than PDF); the main challenge is the
  variability in format across retailers.
- **Feasibility: moderate.** A per-retailer email parser (extract the PDF attachment or parse the
  inline HTML) is tractable. Email is a plausible delivery channel for PDF receipts, so this
  overlaps with 5.2.

## 5.5 Manual receipt entry (fallback baseline)

Already covered by `order.source = 'manual'` in section 4 (the manual order-confirmation flow,
task 4.2). A person confirms what they actually bought, pre-filled from the most recent
`shopping_cart` checkpoint. This is the fallback baseline and the default path until an automated
source (retailer API or receipt import) exists.

## 5.6 Findings summary

| Source | Feasibility | Notes |
|---|---|---|
| Retailer API (Willys) | Infeasible today | Untyped stub only; no real order-history/receipt API. |
| Retailer API (ICA) | Pending | Tracked in `research-and-integrate-ica`. |
| PDF receipt import | Moderate | Per-retailer parser tractable; text-layer extraction > OCR. |
| Kivra export | Uncertain | Depends on whether retailers deliver receipts to Kivra (verify). |
| Email receipt parsing | Moderate | Overlaps with PDF import; inline HTML is easier. |
| Manual entry | Baseline | `order.source = 'manual'`; the default path. |

## 5.7 Re-prioritization call-out

Because Willys has **no real order/purchase-history API** (5.1), receipt import (PDF or email) is
a plausible **primary** path to populate `orders` for Willys, not merely a fallback. If the
PDF/email receipt research (5.2, 5.4) confirms a per-retailer parser is tractable, a future change
should re-prioritize receipt import as the primary automated source for Willys orders, ahead of
waiting for a Willys order-history API that may not come.
