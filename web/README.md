# Spisordning web

Full-featured frontend for the Spisordning household food brain. It talks to
the real `food-brain serve` REST API (no mock data) and covers:

- **Planner** — list/create weekly plans, run the planner, view the week, pick
  candidates per day, approve, commit decisions, and see the resulting shopping
  requirements.
- **Shopping** — manage lists, add items (manually or via ingredient search),
  check items off, delete, and push a list to a retailer (Willys/ICA).
- **Compare** — cheapest grocery bag across Willys and ICA. Compares a plan's
  shopping requirements (or manual items) via `POST /compare` and shows
  per-item prices, cheapest retailer, and bag totals.
- **Recipes** — browse and filter the recipe library (Mealie-backed).
- **Preferences** — read learned per-person preferences.
- **Pantry** — locations, stock lots, record purchases, consume.
- **Orders** — browse placed orders and their items.
- **Tonight** — tonight's meal + one-tap reactions.
- **Sync** — status of external sync sources (Mealie, offers, Apple Notes).

Features that still lack a backend endpoint are shown as clear "not available
yet" states rather than faked: recipe **variants**, **preference editing**, and
triggering **Mealie/offer sync** from the UI.

## Stack

- React 19 + Vite 8 + TypeScript 7 (native/Go compiler)
- TanStack Query 5 for data fetching
- openapi-fetch for the typed API client
- ESLint 9 (flat config)

## TypeScript 7 note

This project is **compiled with TypeScript 7** (`tsc` from the `typescript7`
alias). TS 7.0 does not ship a compiler API, so tooling that needs one
(typescript-eslint, and `openapi-typescript` for codegen) cannot run against it
directly. Two consequences:

1. **Lint** uses the TS 6 API via the `typescript` → `@typescript/typescript6`
   npm alias (the official "run side-by-side with TypeScript 6.0" approach from
   the TS 7.0 announcement). `npm run build` still type-checks with TS 7.
2. **API types are hand-written**, not codegen'd. `src/generated/spisordning.ts`
   is a manual mirror of `api/openapi.yaml` (the same contract the Go server
   generates from into `internal/openapi/types.gen.go`). When the spec changes,
   re-transcribe the affected schemas there — see the file header for details.

## Run locally

Prereq: a running `food-brain serve` instance on `http://localhost:8080`
(start it from the repo root, e.g. via `docker-compose up` or
`go run ./cmd/food-brain serve`).

```sh
cd web
npm install
npm run dev          # http://localhost:5173, proxies /api -> :8080
```

To point at a different backend:

```sh
VITE_API_URL=http://localhost:9000 npm run dev
```

## Scripts

- `npm run dev` — Vite dev server (proxies `/api` to the backend).
- `npm run build` — type-check with TS 7 (`tsc -b`) + Vite production build.
- `npm run preview` — serve the production build.
- `npm run lint` — ESLint (typescript-eslint on the TS 6 API).
