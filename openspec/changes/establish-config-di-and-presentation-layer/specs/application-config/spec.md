## ADDED Requirements

### Requirement: A single Config struct is the source of truth for runtime configuration
Each binary (`food-brain`, `mcp-server`) SHALL build exactly one `internal/config.Config` at
startup from environment variables, and pass values from it to every client/service constructor —
no package outside `internal/config` SHALL call `os.Getenv` directly for application configuration.

#### Scenario: Both binaries load configuration the same way
- **WHEN** `cmd/food-brain` and `cmd/mcp-server` start
- **THEN** both call `internal/config.Load()` rather than reading environment variables directly
  in their own composition-root code

#### Scenario: A required variable is missing for the requested command
- **WHEN** `food-brain serve` starts without `DATABASE_URL` set
- **THEN** startup fails with a clear error naming the missing variable, before any client
  constructor is reached

#### Scenario: An optional integration stays optional
- **WHEN** `DABAS_ENABLED` is unset
- **THEN** `Config` reports the Dabas client as disabled and `internal/service/ingredients.go`
  behaves exactly as it does today (client left `nil`, feature unavailable) — no behavior change,
  only where the check lives
