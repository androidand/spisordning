# Research pharmacy/supplement integration

## Why

Spisordning's shopping pipeline is scoped to groceries, but the household's actual
needs list isn't purely food — Andreas takes ADHD medication and wants Omega-3, iron,
and zinc supplements resolved and ordered the same way a recipe ingredient is (source
idea, 2026-08-28 session: "Spisordning is focused on food, so, it could perhaps also
tackle the latter, but those are typically bought from pharmacies"). Supplements and
OTC health items fit the same shape as a grocery shopping requirement — a named need,
a quantity, a preference (brand/price), a retailer resolution, a wishlist push — so
extending the resolver to a pharmacy retailer is a natural widening of scope, not a
new subsystem.

ICA Gruppen owns Apotek Hjärtat, which raises the question of whether it can piggyback
on the existing `ica-client` integration. It almost certainly cannot: "ICA" in this
codebase talks to ICA's own grocery retail platform (already treated as a separate,
harder integration than Willys/Hemköp's shared SAP Commerce — see
`docs/infrastructure/ica-elevated-auth.md` and the axfood-platform-investigation doc).
A pharmacy storefront is a different regulated domain (prescription items, age-gated
categories, pharmacist-consultation flows) and is very likely a third, unrelated
platform — same shape of effort as building `ica-client` was from scratch, not a
shortcut through it.

**This change is research/scoping only.** No client code, no integration, until the
findings below are in and reviewed.

## What to research

- Does apotekhjartat.se (or apoteket.se, kronans.se as fallback comparisons) expose
  any public/discoverable product search API, or does browsing require a full
  browser session (JS-rendered SPA, no clean REST surface)?
- What platform is it built on — is it SAP Commerce (unlikely, but would mean the
  existing `axfood-client` patterns transfer), a pharmacy-specific commerce platform,
  or fully custom?
- Auth model: can OTC/supplement categories (no prescription required) be browsed
  and added to a cart/wishlist anonymously or with a lightweight account, or does
  every action require a BankID-verified pharmacy account (common in Swedish
  pharmacy e-commerce for liability/regulatory reasons)?
- Terms of service check: does automated browsing/ordering violate the site's terms?
  Pharmacy retailers sometimes have stricter automation restrictions than grocery
  retailers given the regulated-goods context.
- Product scope check: confirm Omega-3, iron, and zinc supplements are unrestricted
  OTC items there (expected, but verify) — this change should never touch anything
  prescription-gated or age-restricted.
- If a usable API exists: sketch (don't build) how it would plug into the
  ingredient-resolver design from `implement-ingredient-quantity-resolver`
  (or whatever that change ends up named) — a "supplement need" is structurally the
  same as a "grocery ingredient need," just resolved against a different retailer
  client.

## Impact

- No code changes in this pass — findings only, written up as this change's tasks.md
  gets worked through.
- If findings are favorable, follow-up work would add a new `store-clients/apotek-
  hjartat-client` (or similar) TypeScript package, mirroring `ica-client`'s shape,
  plus a corresponding adapter and Go-side retailer registration — scoped as a
  separate change once this research lands.
