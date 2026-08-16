# Spisordning — Lead Planning and Implementation Agent Mission

## Mission

You are the lead architect, research coordinator and eventual implementation agent for **Spisordning**.

Spisordning is a self-hosted household food knowledge system combining:

* recipe management,
* Git-like recipe evolution,
* household profiles and preferences,
* actual meal history and reviews,
* weekly meal planning,
* recommendations,
* pantry/inventory,
* ingredient/product knowledge,
* grocery shopping,
* retailer integrations,
* price intelligence,
* external recipe discovery,
* optional AI interaction.

Your immediate task is NOT broad implementation.

Your immediate task is to:

1. investigate the reference systems,
2. investigate external data/integration opportunities,
3. design the relational domain model,
4. challenge and refine the proposed entities,
5. establish mechanically enforced architecture,
6. produce coherent OpenSpec changes and ADRs,
7. only then implement the first vertical slices.

---

# Selected Initial Strategy

Run **Mealie and Grocy** as reference systems on Proxmox through Tengil.

Use them as a live reference laboratory.

Study:

```text
behavior
APIs
databases
migrations
source
tests
edge cases
```

Do not make them permanent runtime dependencies.

Build Spisordning independently using:

```text
Go
PostgreSQL
REST
OpenAPI
Docker Compose
```

Use **Directus initially as an OPTIONAL admin/development workbench**.

Directus must remain removable.

---

# First Principle

Do not mechanically port Mealie or Grocy.

Use this process:

```text
observe mature implementation
            ↓
understand behavior
            ↓
understand data model
            ↓
understand edge cases
            ↓
extract domain knowledge
            ↓
design Spisordning concept
            ↓
OpenSpec
            ↓
implementation
```

---

# Phase 0 — Inspect Existing Spisordning

Before changing anything:

* inspect the repository,
* inspect current OpenSpec/specsync state,
* inspect existing code,
* identify existing decisions,
* locate the existing Willys client or references to it,
* inspect Tengil deployment conventions where locally available.

Do not duplicate existing work.

Produce:

```text
docs/research/current-state.md
```

---

# Phase 1 — Build the Reference Laboratory

Deploy:

```text
Mealie
Grocy
```

to Proxmox through Tengil / Docker Compose.

Use isolated persistence.

Record:

```text
version
commit/tag if applicable
container image
license
deployment config
database/storage
```

Create comparable representative data.

Exercise important workflows.

---

# Mealie Investigation

Use sub-agents.

Investigate at least:

```text
recipe model
recipe editing
recipe import
recipe parsing
structured ingredients
foods
units
servings
scaling
images
tags
categories
cookbooks
search
meal plans
shopping
households
users
ratings if any
API
database
migrations
tests
provenance
```

For each important feature capture:

```text
user behavior
API behavior
DB mutation
source implementation
tests
strengths
weaknesses
Spisordning lesson
```

---

# Grocy Investigation

Investigate at least:

```text
products
barcodes
locations
stock
stock journal
lots
expiry
purchase
consume
discard
transfer
adjust
mark empty
units
unit conversion
product-specific conversion
shopping
recipes
meal planning
cost tracking
API
database
migrations
tests
```

Pay special attention to edge cases accumulated through years of inventory use.

---

# Feature Overlap Matrix

For every overlapping capability decide:

```text
MEALIE
GROCY
MERGE
REDESIGN
DEFER
OMIT
```

No vague:

> take the best parts.

State exactly what is useful and why.

---

# Phase 2 — Database Archaeology

Treat database migrations as historical architecture documentation.

For both reference systems determine:

* tables,
* relationships,
* foreign keys,
* uniqueness,
* nullable relationships,
* deletion behavior,
* quantity representations,
* audit/history structures,
* problematic schemas later migrated away from.

Produce ER diagrams.

---

# Phase 3 — Design Spisordning's Domain Model

Use the supplied OpenSpec as the starting hypothesis.

Do not blindly accept it.

Challenge every proposed table.

The following concepts require explicit analysis.

---

## Household

Candidate:

```text
households
accounts
persons
household_memberships
```

Do not conflate login identity with household Person.

---

## Preferences and Constraints

Separate:

```text
LIKES / DISLIKES
```

from:

```text
ALLERGIES / HARD RESTRICTIONS
```

Never convert an allergy into a recommendation score.

---

## Recipe Hierarchy

The current preferred model is:

```text
RecipeFamily
    │
    ├── RecipeVariant
    │       │
    │       ├── RecipeRevision
    │       ├── RecipeRevision
    │       └── RecipeRevision
    │
    └── RecipeVariant
            │
            └── RecipeRevision
```

A RecipeFamily represents a conceptual dish.

A RecipeVariant represents one recognizable fork/style/source.

A RecipeRevision represents immutable evolution of that variant.

Validate this model against real recipe workflows.

---

# Git-Like Recipe Design

Investigate PostgreSQL DAG representation.

Current candidate:

```text
recipe_families
recipe_variants
recipe_revisions
recipe_revision_parents
```

Support future semantics:

```text
fork
history
diff
branch-like variant
tag
merge
```

Do not implement literal Git unless compelling evidence appears.

Prefer simple immutable revisions.

Recipes are small; optimize semantics before storage efficiency.

---

# Visual Recipe Family Requirement

The future UI should be able to display:

```text
Korvstroganoff
★★★★★ Default household variant
       ▼ expand
       ├── Andreas version
       ├── ICA version
       ├── Köket version
       ├── Child-friendly
       └── Find more variants...
```

The domain model must make this straightforward.

Determine whether:

```text
default_variant_id
```

should be:

* stored,
* manually pinned,
* computed from ratings,
* or computed with optional override.

---

# Meals

Distinguish planned food from actual food.

Candidate:

```text
meal_plans
meal_plan_entries

meals
meal_participants
meal_reviews
```

A planned dinner may produce an actual Meal.

Actual meal history drives learning.

---

# Favorites

Do not use a global recipe boolean unless there is strong reason.

Favorites should normally be person/household-specific.

Investigate whether favorites are:

```text
explicit preferences
```

while ratings are:

```text
observations from actual meals
```

This distinction is desirable.

---

# Ratings

A person should be able to review the actual meal instance.

Example:

```text
Andreas   5/5
Vera      4/5
Valdemar  2/5
```

Use reviews to derive recipe-level aggregates.

---

# Ingredient Model

Canonical semantic Ingredient:

```text
cucumber
tomato
chicken breast
cream
basil
```

is distinct from Product:

```text
Arla Vispgrädde 500ml
Garant Kycklingfilé 900g
```

This distinction is NON-NEGOTIABLE unless research provides overwhelming contrary evidence.

---

# Ingredient Forms

Investigate how to represent:

```text
fresh basil
dried basil

fresh tomatoes
canned tomatoes
crushed tomatoes

fresh pasta
dried pasta

fresh vegetables
frozen vegetables
```

Candidate model:

```text
ingredients
ingredient_forms
```

Alternative:

related ingredient graph.

Research external taxonomies and mature implementations.

---

# Ingredient Substitution

Substitution must be explicit and capable of being directional.

Potential categories:

```text
EQUIVALENT
GOOD
ACCEPTABLE
FORM
DIETARY
EMERGENCY
```

Example:

```text
fresh basil
   ↓
dried basil
conversion != 1:1
```

Research quantity conversion semantics.

---

# Unit System

Study both Mealie and Grocy deeply.

The system must support:

```text
g
kg
ml
dl
l
piece
tbsp
tsp
pinch
package
can
```

Universal dimensions must remain distinct from ingredient-specific conversions.

Do not invent density values universally.

---

# Product

Candidate:

```text
products
product_identifiers
product_ingredient_mappings
```

Products may be:

```text
commercial packaged
commercial unpackaged
manual/generic
```

Barcode is optional.

---

# Retailer Identity

Keep:

```text
Ingredient
Product
RetailerProduct
StoreOffer
```

separate.

Expected relationship:

```text
Ingredient
    ▲
    │
 Product
    ▲
    │
RetailerProduct
    │
    ▼
 StoreOffer
```

A retailer SKU does not define the canonical food ontology.

---

# Retailers and Stores

Candidate:

```text
retailers
stores
```

Retailer:

```text
ICA
Willys
Coop
```

Store:

```text
ICA Maxi Lindhagen
specific Willys store
```

Assortment and prices may be store-specific.

---

# Price Model

Research whether current price should be mutable or represented as observations.

Likely model:

```text
retailer_products
store_product_offers
price_observations
```

Price history may later support:

```text
basket optimization
offer detection
price trends
```

---

# Pantry

Candidate:

```text
inventory_locations
inventory_lots
inventory_events
```

A lot represents physical household inventory.

Do not use:

```text
products.current_quantity
```

as the complete inventory model.

---

# Inventory Uncertainty

Support:

```text
EXACT
LIKELY
ESTIMATED
UNKNOWN
```

Investigate whether confidence belongs:

* directly on current lot state,
* on observations,
* derived from event history,
* or some combination.

---

# Inventory Events

Investigate:

```text
PURCHASE
CONSUME
DISCARD
ADJUST
TRANSFER
MARK_EMPTY
OPEN
```

Use Grocy behavior as the primary reference.

---

# Barcode

Barcode scanning should provide convenient identification.

It must not define identity.

Research:

```text
GTIN/EAN normalization
Open Food Facts
retailer lookup
manual fallback
```

---

# Local Shopping Intent

Spisordning should own the canonical shopping requirement/list.

Distinguish:

```text
Need 500g chicken breast
```

from:

```text
Willys Garant Chicken 900g
```

Candidate:

```text
shopping_lists
shopping_list_items
```

---

# Retailer Lists

Retailer lists are external projections/synchronizations.

Candidate:

```text
retailer_list_bindings
```

Study two-way synchronization and conflicts.

---

# Carts

Do not conflate:

```text
shopping list
cart
order
```

Candidate:

```text
shopping_carts
shopping_cart_items
```

---

# Orders

Candidate:

```text
orders
order_items
```

A completed order may create inventory PURCHASE events.

Preserve actual:

```text
quantity
price
retailer product
substitutions
```

---

# Receipts

Research receipt import.

Do not prioritize it initially.

Potential sources:

```text
retailer API
PDF
Kivra export
email
manual
```

A receipt may be useful where no retailer order API exists.

---

# External Research — ICA

Investigate current ICA access.

Seed repositories:

`https://github.com/LazyTarget/ha-ica-todo`

`https://github.com/svendahlstrand/ica-api`

Current initial observations to verify:

* the older `ica-api` documentation became inaccurate after ICA API changes in April 2024;
* `ha-ica-todo` contains much newer work, including 2026 commits;
* it implements or investigates shopping lists, offers, article grouping, auth refresh and synchronization;
* the repository contains an `ICA+Grocy.md` describing an inventory lifecycle similar to Spisordning.

Inspect that document carefully.

Extract useful ideas.

Do not inherit Home Assistant-specific design unless it makes sense.

---

# Existing Willys Client

Treat the existing Willys client as important prior art.

Create a capability map.

Determine current support for:

```text
authentication
store selection
product search
GTIN
prices
campaigns
shopping lists
cart
checkout
orders
purchase history
```

Do not design the retailer interface until this real capability map exists.

---

# External Product Data

Evaluate:

## Livsmedelsverket

Research official public food database API.

Potential uses:

```text
canonical ingredient vocabulary
nutrition
classification
raw ingredients
```

Do not blindly adopt its ontology.

---

## Open Food Facts

Evaluate:

```text
barcode lookup
product name
brand
ingredients
allergens
nutrition
images
categories
```

Evaluate current API version and licensing requirements.

---

## Open Prices

Evaluate location-specific food price data.

Determine Swedish coverage before relying on it.

---

# Swedish Price Intelligence

Dedicated research workstream:

```text
Primat
Matpriskollen
Matmoms
Matpriser.nu
Comparator
Open Prices
```

For each determine:

```text
API availability
license
rate limits
number of retailers
store-level granularity
EAN identity
campaigns
member prices
history
update interval
commercial use
```

Primat deserves particularly deep evaluation because current initial research indicates that it exposes retailer/store price data through a REST API.

Reverify current terms.

---

# External Recipe Sources

Evaluate:

```text
TheMealDB
Edamam
Spoonacular
Foodie
Swedish recipe sources
```

For each record:

```text
license
cost
rate limits
commercial rights
Swedish content
ingredient structure
images
quality
API stability
```

---

# Generic Web Recipe Import

Prioritize `schema.org/Recipe` / JSON-LD extraction.

Many recipe sites expose structured data intended for search engines.

Investigate a generic pipeline:

```text
Fetch URL
   ↓
find Recipe JSON-LD
   ↓
parse structured recipe
   ↓
parse ingredient strings
   ↓
canonicalize ingredients
   ↓
review unresolved mappings
   ↓
import
```

Add per-site parsers only where necessary.

---

# Inspiration Site

Investigate:

`https://vadfanskajaglagatillmiddag.nu/`

Use network inspection and source analysis.

Determine:

```text
candidate recipe list
selection mechanism
server vs client randomness
source sites
categories
possible hidden API
legal/terms constraints
```

Do not assume scraping is appropriate.

Even if it cannot be integrated, study the extremely simple inspiration UX.

---

# Recommendation Domain

Do not model Recommendation as merely:

```text
LLM response
```

Create deterministic candidate/ranking logic.

Inputs:

```text
people eating
allergies
preferences
ratings
meal history
recent meals
pantry availability
expiry
substitutions
effort
time
price
shopping requirements
```

Candidate scoring should be explainable.

---

# Recommendation Inspiration

Recommendations should include both:

```text
KNOWN FAVORITES
```

and:

```text
DISCOVERY / NOVELTY
```

Balance familiarity against inspiration.

Potential future user controls:

```text
safe choice
something similar
surprise me
something completely new
```

---

# Automatic Cookbook Growth

Desired long-term behavior:

```text
external recipe
      ↓
planned/cooked
      ↓
save local version
      ↓
review
      ↓
household cookbook
```

Recipes actually cooked should become easy to retain locally.

Do not necessarily auto-import everything without user review.

---

# Home Assistant

Design future integration boundary.

Possible capabilities:

```text
household identity
Todo/shopping lists
notifications
voice commands
dashboards
scanner events
presence
calendar
```

Keep Home Assistant optional.

---

# AI Provider

Implement provider abstraction later.

Primary initial target:

```text
OpenAI-compatible API
```

for local llama-skein.

AI SHALL call application-layer tools.

Never expose unrestricted SQL.

---

# Directus Research Spike

Do not just install Directus and start using it.

Answer:

1. Can Spisordning remain sole migration owner?

2. What Directus metadata does it add?

3. Can Directus safely expose read-only PostgreSQL views?

4. Can database permissions limit Directus writes?

5. Which tables should be SAFE_DIRECT_CRUD?

6. Which must be DOMAIN_CONTROLLED?

7. How does media handling affect portability?

8. What are current licensing implications?

9. How painful are upgrades?

10. Would custom Go admin endpoints actually be simpler?

Classify every exposed collection:

```text
SAFE_DIRECT_CRUD
READ_ONLY
DOMAIN_CONTROLLED
HIDDEN
```

---

# Directus Exit Gate

After Recipe + Catalog + Inventory exist, write an ADR:

```text
keep-directus
```

or:

```text
remove-directus
```

based on observed development experience.

Do not retain it by inertia.

---

# Database Design Process

This is critical.

DO NOT begin by creating all proposed tables.

Perform the following sequence.

## Step 1 — Vocabulary

Produce canonical glossary.

## Step 2 — Aggregates

Identify domain aggregates and boundaries.

## Step 3 — Relationships

Create conceptual ER model.

## Step 4 — Lifecycle

Define mutable vs immutable entities.

## Step 5 — Commands

List important operations.

## Step 6 — Invariants

Define what must always be true.

## Step 7 — Persistence

Only now propose tables and constraints.

## Step 8 — Review

Use a separate architecture/data-model review sub-agent.

## Step 9 — Migrations

Create migrations only for the first implementation slice.

---

# Database Review Questions

For every proposed table ask:

```text
What domain concept does this represent?

Who owns it?

Who may mutate it?

Is it mutable?

Does it require history?

What is its lifecycle?

What is the deletion behavior?

What are its uniqueness constraints?

What external IDs exist?

What should be indexed?

Can the relationship be represented with a real FK?

Are we using JSON because it is correct or because modeling is difficult?
```

---

# Do Not Use Generic Polymorphism Carelessly

Avoid tables such as:

```text
entity_type
entity_id
value
```

unless the loss of foreign-key integrity is consciously accepted.

Prefer real relational relationships.

---

# Expected Domain Documents

Produce:

```text
docs/domain/
├── glossary.md
├── context-map.md
├── aggregates.md
├── lifecycle-and-events.md
├── recipe-model.md
├── ingredient-product-model.md
├── inventory-model.md
├── household-preferences.md
├── meal-planning-model.md
├── shopping-retailer-model.md
└── recommendation-model.md
```

---

# Expected Database Documents

Produce:

```text
docs/database/
├── conceptual-er-model.md
├── logical-schema.md
├── constraints.md
├── indexes.md
├── history-and-versioning.md
├── external-identifiers.md
└── migration-strategy.md
```

Create diagrams where useful.

---

# Research Documents

At minimum:

```text
docs/research/
├── mealie-*.md
├── grocy-*.md
├── ica-current-api.md
├── willys-capabilities.md
├── external-product-data.md
├── swedish-price-data.md
├── recipe-data-sources.md
├── recipe-web-import.md
├── directus-evaluation.md
└── inspiration-sites.md
```

---

# ADRs

At minimum plan ADRs for:

```text
Go
PostgreSQL
modular Clean Architecture
architecture enforcement
REST/OpenAPI

Directus optional workbench

RecipeFamily / Variant / Revision
recipe revision DAG

Ingredient vs Product
ingredient form representation
substitution model

inventory ledger
inventory confidence

Meal vs MealPlanEntry
rating/favorite semantics

retailer abstraction
Product vs RetailerProduct
price history

external recipe provenance
AI provider abstraction
```

---

# OpenSpec Changes

After research, split implementation into coherent changes.

Likely sequence:

```text
establish-enforced-go-architecture

establish-household-and-catalog

implement-recipe-family-and-revisions

integrate-directus-workbench

implement-meals-and-preferences

implement-pantry-inventory

implement-recipe-availability

implement-meal-planning

implement-recommendations

integrate-willys

research-and-integrate-ica

implement-price-intelligence

implement-recipe-discovery

integrate-ai
```

Adjust after research.

---

# Implementation Rule

Each change should ideally deliver a complete vertical capability:

```text
domain
application
persistence
REST
OpenAPI
tests
```

Avoid implementing all entities first and all HTTP later.

---

# Testing

Use:

```text
domain unit tests
PostgreSQL integration tests
API integration tests
architecture tests
migration tests
```

When reimplementing semantics learned from Mealie or Grocy, create reference-behavior tests for useful edge cases.

Do not preserve reference-system bugs merely because they exist.

---

# Initial Definition of Done

The first implementation milestone should NOT attempt the entire vision.

It succeeds when:

```text
Go backend boots
PostgreSQL boots
migrations work
OpenAPI exists

architecture is enforced by CI

Household exists
Person exists

Ingredient exists
Unit exists
Product exists
Product maps to Ingredient

RecipeFamily exists
RecipeVariant exists
RecipeRevision exists
revision parentage works

recipe revision has structured ingredients and steps

REST API exposes these capabilities

Docker image builds
Compose works

Directus can optionally inspect the database

stopping Directus does not affect Spisordning
```

---

# Stop Gate Before Broad Coding

Do not begin large-scale implementation until the lead agent can clearly answer:

1. What are Spisordning's canonical entities?

2. Why does each proposed table exist?

3. What did Mealie teach us?

4. What did Grocy teach us?

5. What existing mistakes are we avoiding?

6. What is RecipeFamily vs Variant vs Revision?

7. How will recipe lineage work?

8. What is Ingredient vs Product vs RetailerProduct?

9. How are ingredient forms represented?

10. How are substitutions represented?

11. How does inventory uncertainty work?

12. How does inventory history work?

13. How does Meal differ from MealPlanEntry?

14. Where do favorites and ratings belong?

15. How are shopping intent and retailer products separated?

16. How does Willys fit behind an adapter?

17. What does current ICA integration actually permit?

18. Which external product databases are useful?

19. Which recipe sources are legally and technically useful?

20. Is Directus accelerating or distorting the architecture?

If these answers are weak, continue research.

---

# Operating Principle

Agents are encouraged to challenge this specification.

They are NOT encouraged to silently ignore it.

If evidence suggests a better domain model:

1. document the evidence,
2. propose the alternative,
3. record tradeoffs,
4. update OpenSpec/ADR,
5. then implement.

---

# Final Goal

Build a coherent food knowledge graph expressed through a strongly relational domain:

```text
People
  │
  ▼
Preferences ─────────────┐
                         │
Recipe Family            │
  │                      │
  ▼                      │
Variant                  │
  │                      │
  ▼                      │
Revision ──► Ingredient ◄┘
                │
         ┌──────┴───────┐
         ▼              ▼
   Substitutes        Product
                        │
                   ┌────┴─────┐
                   ▼          ▼
                Pantry    RetailerProduct
                              │
                              ▼
                            Store
                              │
                              ▼
                            Price
```

and close the operational loop:

```text
Discover
   ↓
Cookbook
   ↓
Plan
   ↓
Shop
   ↓
Pantry
   ↓
Cook
   ↓
Review
   ↓
Learn
   ↓
Recommend
   ↓
Plan again
```

The system should become more useful every time the household shops, cooks or reviews a meal.
