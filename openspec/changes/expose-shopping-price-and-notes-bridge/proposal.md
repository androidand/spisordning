## Why

`implement-shopping-and-commerce` already shipped a real, tested shopping pipeline —
`shopping_list` → resolve via retailer adapter → `retailer_list_binding` (durable wishlist) →
`shopping_cart` checkpoint → manual `order` confirmation — reachable over `cmd/food-brain`'s HTTP
API. But none of it is reachable from an AI chat session: `implement-mcp-server` deliberately scoped
`internal/mcptools` to exactly 3 tools (planning, reactions, requirements), so there is no MCP tool
that resolves a shopping list against a retailer or creates a wishlist. Separately, no code path
anywhere compares price across retailers — `internal/retailer.Resolution` (Willys) has no price
field at all (the willys-adapter tracks `priceValue`/`price` internally for its own review UI but
never returns it from `/resolve`), and every external Swedish price-comparison data source
(Matpriskollen, Matmoms, Comparator — `docs/research/swedish-price-data.md`) requires direct
business outreach before it's usable, so near-term price comparison has to come from the retailer
adapters themselves. Finally, the household's actual ingredient list lives in an Apple Note
("Köp Mat Andreas") on the user's Mac; the existing reader (`willys-client/apps/notes-sync`) uses
`osascript` against Notes.app, which only works run locally on macOS — it currently talks directly
and only to Willys, standalone, with no connection to spisordning's own API, and spisordning itself
is not deployed anywhere yet (intended target: the user's Proxmox cluster, see the companion
`deploy-food-brain-to-proxmox` change).

**Explicit scope boundary (user decision):** this change goes only as far as creating retailer
wishlists. It does not call the existing `to-cart` step, and does not touch checkout, payment, or
delivery-slot booking — those are explicitly deferred to a later change. It also treats ICA as
secondary: ICA has a second-factor auth step that expires and requires an occasional manual login to
refresh session cookies (user-reported operational constraint, not yet verified in `ica-adapter`
code), whereas Willys and Hemköp (both `axfood-client`-based) do not have this problem — so the
cheapest-price comparison and wishlist flow should degrade gracefully (report ICA as
stale/unavailable) rather than block on it.

## What Changes

- Add price to the Willys-adapter's `/resolve` response (cross-repo: `willys-client/apps/willys-
  adapter`) and to spisordning's `internal/retailer.Resolution` struct, so a resolved product
  carries a comparable price the same way `icaretailer.ProductDetail` already does for ICA.
- Add a price-comparison step in spisordning: given a canonical shopping requirement, resolve it
  against each configured retailer adapter (Willys, Hemköp once its adapter exists, ICA
  best-effort) and report the cheapest available match, degrading gracefully when a retailer's
  session is stale (surface that explicitly rather than silently dropping it).
- Add MCP tools exposing the existing shopping pipeline up to (and not past) wishlist creation:
  create/resolve a shopping list, compare price across retailers per item, and push to a chosen
  retailer's wishlist (`POST /shopping-lists` via the existing `internal/retailer`/`internal/
  icaretailer` clients). Explicitly do NOT expose the existing `to-cart` endpoint via MCP yet.
- Design (not necessarily fully implement in this change) the Mac-local companion path: the
  existing `osascript`-based note reader, adapted to parse the "Köp Mat Andreas" checklist and POST
  it to spisordning's HTTP API (once deployed — see `deploy-food-brain-to-proxmox`) instead of
  talking to Willys directly, so the household's real note becomes spisordning's actual shopping-
  list input rather than a parallel, disconnected sync path.

## Capabilities

### New Capabilities
- `retailer-price-comparison`: resolving a shopping requirement against multiple retailer adapters
  and reporting the cheapest available match, with graceful degradation for a stale/unavailable
  retailer session.
- `mcp-shopping-tools`: MCP tools for creating a shopping list, comparing price, and pushing to a
  retailer wishlist — stopping short of cart/checkout.
- `apple-notes-ingestion`: a Mac-local bridge that reads the household's named Apple Note and
  submits its items to spisordning's shopping-list API as the source of a shopping list.
- `apple-notes-outbound-sync` (task group 7, added 2026-08-29): the other half of the loop — once
  spisordning resolves a note-sourced item (priced, pushed to a retailer wishlist), the same
  Mac-local bridge writes that status back onto the actual checklist, checking off only the lines
  it itself resolved. Apple Notes has no push API, so this is poll-based, not event-driven; the
  match key is a normalized label tied to the originating `shopping_list_id`, and the bridge never
  rewrites text it did not ingest itself, to avoid ever destroying a hand-edit.

### Modified Capabilities
- `retailer-adapter` (merged in `openspec/specs/retailer-adapter/`): add price to the resolution
  response — check that spec file for the exact existing resolution-response requirement before
  writing the delta, since this changes an already-merged capability's behavior.

## Impact

- `internal/retailer`, `internal/icaretailer`: extend `Resolution`/`ProductDetail` types with price;
  add a price-comparison helper.
- `willys-client/apps/willys-adapter` (sibling repo): surface `priceValue`/`price` in `/resolve`
  responses — cross-repo change, sequence and coordinate accordingly.
- `internal/mcptools`, `cmd/mcp-server/adapters.go`: new tools for shopping-list creation, price
  comparison, and wishlist push.
- `willys-client/apps/notes-sync` (sibling repo) or a new spisordning-side ingestion endpoint: the
  Mac-companion bridge target.
- Does not touch `to-cart`, checkout, payment, or delivery-slot booking — explicitly deferred.
- Depends on `deploy-food-brain-to-proxmox` for the notes-bridge to have a real HTTP target to POST
  to; the price-comparison and MCP-tool work does not depend on deployment.
