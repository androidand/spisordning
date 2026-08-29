## ADDED Requirements

### Requirement: A read-only web frontend shows the current week's plan and shopping list
A Vite + React + TypeScript frontend, generated against `api/openapi.yaml`, SHALL display the
current week's dinner plan and its shopping list by calling spisordning's existing REST API,
proving the contract serves a real browser client.

#### Scenario: The frontend renders a real plan from a running backend
- **WHEN** the frontend is loaded against a running `food-brain serve` instance with an existing
  weekly plan
- **THEN** it displays that week's planned dinners and the associated shopping list, sourced from
  the REST API, not mock data

#### Scenario: The first slice does not attempt write actions
- **WHEN** a user views the frontend
- **THEN** no UI element triggers a write (recording a reaction, pushing a wishlist, editing the
  plan) — the first slice is read-only by design, not by omission
