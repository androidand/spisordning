# shopping-and-commerce (delta)

## ADDED Requirements

### Requirement: Shopping list is retailer-independent

The system SHALL persist a `shopping_list` owned by spisordning, distinct from any retailer's own
list representation. A `shopping_list_item` SHALL identify what is needed canonically (via an
`ingredient_id`, a `shopping_requirement_id`, or a free-text `label` for non-ingredient items) and
SHALL NOT require a retailer product identifier to exist.

#### Scenario: A shopping list item exists before any retailer resolution

- **WHEN** a person adds "500g chicken breast" to a shopping list
- **THEN** the `shopping_list_item` is persisted with an ingredient reference and quantity
- **AND** no retailer product id is required for the item to exist or be displayed

#### Scenario: Manual items are supported alongside plan-derived items

- **WHEN** a person adds "paper towels" to a shopping list with no meal plan or ingredient behind
  it
- **THEN** the item is persisted using its free-text `label`
- **AND** it appears in the same list as ingredient-derived items

### Requirement: Shopping list, cart, and order are distinct, non-conflated stages

The system SHALL model `shopping_list`, `shopping_cart`, and `order` as separate entities with
separate lifecycles. A `shopping_cart` SHALL only be created from a resolved, retailer-bound
list (via the existing retailer adapter's cart-fill step), and an `order` SHALL only be created
from a cart or a manually confirmed purchase. The system SHALL NOT merge these three concepts
into a single table or a single mutable state field.

#### Scenario: Editing a shopping list does not affect an existing cart or order

- **WHEN** a person edits a `shopping_list` after a `shopping_cart` was created from it
- **THEN** the existing `shopping_cart` and any `order` derived from it are unaffected
- **AND** the edited list requires a new resolution/cart cycle to reflect the change

#### Scenario: A shopping list can exist with no cart or order

- **WHEN** a shopping list has never been pushed to a retailer or checked out
- **THEN** it is a valid, persisted `shopping_list` with no associated `shopping_cart` or `order`

### Requirement: The system SHALL NOT perform automated checkout, payment, or slot booking

No code path introduced by this capability SHALL place a retailer order, charge payment, or book
a delivery/pickup slot without a human action in the retailer's own app or site. This mirrors the
existing, deliberate omission of checkout in the `retailer-adapter` capability.

#### Scenario: Cart creation stops short of purchase

- **WHEN** a `shopping_cart` is created via the adapter's to-cart step
- **THEN** no payment, checkout, or slot-booking call is made as part of that operation
- **AND** completing the purchase remains a manual action outside spisordning

### Requirement: Orders preserve actual purchase fidelity

An `order_item` SHALL record the actual quantity purchased, the actual price paid, the resolved
`retailer_product_id`, and — when the retailer substituted a product — a reference to what was
substituted. `order.source` SHALL explicitly record whether the order was entered manually,
retrieved from a retailer API, or derived from an imported receipt.

#### Scenario: A manually confirmed order records actual quantities and prices

- **WHEN** a person confirms a completed Willys purchase from the corresponding `shopping_cart`
  checkpoint
- **THEN** each `order_item` records the actual quantity and price, defaulting from the cart
  checkpoint but editable to reflect what was truly bought
- **AND** `order.source` is `'manual'`

#### Scenario: A substitution is preserved, not silently overwritten

- **WHEN** an order item reflects a retailer substitution of the originally resolved product
- **THEN** the `order_item` records the substituted product actually received
- **AND** retains a reference to the originally resolved item it replaced

### Requirement: Retailer list bindings are projections, not authoritative state

A `retailer_list_binding` SHALL record that a `shopping_list` has been projected onto a specific
external retailer list (e.g. a Willys wishlist id), including the last successful push time. The
system SHALL treat spisordning's `shopping_list` as authoritative for intent; the retailer's list
is a synchronized projection, and the binding SHALL make projection staleness (time since last
successful push) inspectable.

#### Scenario: A binding records which external list a shopping list maps to

- **WHEN** a shopping list is pushed to Willys for the first time
- **THEN** a `retailer_list_binding` is created recording the returned external wishlist id
- **AND** subsequent pushes for the same shopping list update the same binding's
  `last_pushed_at`

#### Scenario: Staleness is inspectable

- **WHEN** a shopping list has been edited since its binding's `last_pushed_at`
- **THEN** the binding can be queried to show it is stale relative to the current list content
