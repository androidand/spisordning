# Testing policy — spisordning

This is a **Go** project (see `go.mod`). There is **no** JavaScript/TypeScript code
and **no** JS/TS test files in this repository.

## Test command

Run from the repository root:

    go test ./...

Verification passes only when this command exits 0.

## Do NOT use

- `bun test`, `npm test`, `jest`, `vitest`, `playwright test`, or any other
  JS/TS test runner — this repo has no JS/TS code, so they will always fail with
  "0 test files". The only test command is `go test ./...`.
