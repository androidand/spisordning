// Package config is the single source of truth for runtime configuration. Each
// binary (food-brain, mcp-server) builds exactly one Config at startup via
// Load() and passes its fields to every client/service constructor. No package
// outside this one calls os.Getenv directly for application configuration —
// the composition roots are the only place env vars are read, and they do so
// through Load() rather than ad hoc os.Getenv calls.
//
// Config is env-only (no YAML/TOML file support) by design — see
// openspec/changes/establish-config-di-and-presentation-layer/design.md D1.
// The same env var names already used today are preserved, so no .env
// migration is needed.
package config

import (
	"fmt"
	"os"
	"strings"
)

// Config holds every runtime configuration value the two composition roots
// (cmd/food-brain, cmd/mcp-server) currently read from the environment.
//
// Optional integrations are represented by empty-string / false zero values
// when their env vars are unset; callers check for presence before
// constructing a client (the existing "optional-client convention"). Load()
// never fails on a missing optional var — it only fails when a var that is
// required for the requested command is missing (see Require).
type Config struct {
	// ── Database ──────────────────────────────────────────────────────────
	// DatabaseURL is the full Postgres DSN (takes precedence over the
	// individual POSTGRES_* fields when set).
	DatabaseURL string
	// PostgresHost, PostgresPort, PostgresDB, PostgresUser, PostgresPassword,
	// PostgresSSLMode are the individual Postgres connection fields, used when
	// DatabaseURL is unset (matching docker-compose's convention).
	PostgresHost     string
	PostgresPort     string
	PostgresDB       string
	PostgresUser     string
	PostgresPassword string
	PostgresSSLMode  string

	// ── Mealie (recipe sync) ──────────────────────────────────────────────
	MealieBaseURL string
	MealieAPIToken string

	// ── Grocy (self-hosted inventory) ─────────────────────────────────────
	GrocyBaseURL string
	GrocyAPIKey  string

	// ── Ingredient data sources ───────────────────────────────────────────
	// SLVBaseURL is the Livsmedelsverket (Swedish Food Agency) nutrition API.
	SLVBaseURL string
	// DabasEnabled enables the Dabas (Swedish food composition) client.
	DabasEnabled bool
	// MPKEnabled enables the Matpriskollen (Swedish price comparison) client.
	MPKEnabled bool

	// ── Retailer adapters ─────────────────────────────────────────────────
	// AdapterURL is the willys-adapter base URL.
	AdapterURL string
	// ICAAdapterURL is the ica-adapter base URL.
	ICAAdapterURL string
	// HemkopAdapterURL is the hemkop-adapter base URL.
	HemkopAdapterURL string
	// ICAAuthFile is the path to ICA's elevated-auth credential file — the
	// artifact a manual (Playwright-driven) web login drops where Go can find
	// it. ICA is the first tiered retailer: its anonymous ecom surface (search)
	// needs no credential, but the wishlist push needs this OAuth2 session.
	// Empty when unset; the manual login step itself stays on the ica-adapter
	// (TS) side. See design.md D2.
	ICAAuthFile string

	// ── Skolmaten (school meals) ──────────────────────────────────────────
	SkolmatenBaseURL     string
	SkolmatenClientToken string
	SkolmatenSchool      string

	// ── LLM (Ollama) ──────────────────────────────────────────────────────
	OllamaBaseURL string
	OllamaModel   string

	// ── Server addresses ──────────────────────────────────────────────────
	// SpisordningAddr is the REST HTTP server listen address (food-brain serve).
	SpisordningAddr string
	// SpisordningMCPAddr is the MCP server listen address (mcp-server).
	SpisordningMCPAddr string
}

// Load reads every configuration value from the environment (via os.Getenv)
// into a single Config. Optional integrations are left at their zero values
// when unset; Load does not validate required combinations — that is the
// caller's job (see Require).
func Load() Config {
	return Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		PostgresHost:     os.Getenv("POSTGRES_HOST"),
		PostgresPort:     os.Getenv("POSTGRES_PORT"),
		PostgresDB:       os.Getenv("POSTGRES_DB"),
		PostgresUser:     os.Getenv("POSTGRES_USER"),
		PostgresPassword: os.Getenv("POSTGRES_PASSWORD"),
		PostgresSSLMode:  os.Getenv("POSTGRES_SSLMODE"),

		MealieBaseURL:  os.Getenv("MEALIE_BASE_URL"),
		MealieAPIToken: os.Getenv("MEALIE_API_TOKEN"),

		GrocyBaseURL: os.Getenv("GROCY_BASE_URL"),
		GrocyAPIKey:  os.Getenv("GROCY_API_KEY"),

		SLVBaseURL:   os.Getenv("SLV_BASE_URL"),
		DabasEnabled: os.Getenv("DABAS_ENABLED") != "",
		MPKEnabled:   os.Getenv("MPK_ENABLED") != "",

		AdapterURL:       envOr("ADAPTER_URL", "http://localhost:8402"),
		ICAAdapterURL:    envOr("ICA_ADAPTER_URL", "http://localhost:8403"),
		HemkopAdapterURL: envOr("HEMKOP_ADAPTER_URL", "http://localhost:8404"),
		ICAAuthFile:      os.Getenv("ICA_AUTH_FILE"),

		SkolmatenBaseURL:     os.Getenv("SKOLMATEN_BASE_URL"),
		SkolmatenClientToken: os.Getenv("SKOLMATEN_CLIENT_TOKEN"),
		SkolmatenSchool:      os.Getenv("SKOLMATEN_SCHOOL"),

		OllamaBaseURL: os.Getenv("OLLA_OPENAI_BASE_URL"),
		OllamaModel:   os.Getenv("OLLA_MODEL"),

		SpisordningAddr:    envOr("SPISORNING_ADDR", ":8080"),
		SpisordningMCPAddr: envOr("SPISORNING_MCP_ADDR", ":8081"),
	}
}

// envOr returns the value of the env var key, or fallback when unset/empty.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// HasDatabase reports whether a database is configured (either via
// DATABASE_URL or the individual POSTGRES_* fields).
func (c Config) HasDatabase() bool {
	if c.DatabaseURL != "" {
		return true
	}
	return c.PostgresPassword != "" || c.PostgresUser != ""
}

// HasMealie reports whether the Mealie client is configured.
func (c Config) HasMealie() bool {
	return c.MealieBaseURL != "" && c.MealieAPIToken != ""
}

// HasGrocy reports whether the Grocy client is configured.
func (c Config) HasGrocy() bool {
	return c.GrocyBaseURL != ""
}

// HasSLV reports whether the Livsmedelsverket client is configured.
func (c Config) HasSLV() bool {
	return c.SLVBaseURL != ""
}

// HasSkolmaten reports whether the Skolmaten client is configured.
func (c Config) HasSkolmaten() bool {
	return c.SkolmatenBaseURL != "" && c.SkolmatenClientToken != ""
}

// HasOllama reports whether the Ollama LLM client is configured.
func (c Config) HasOllama() bool {
	return c.OllamaBaseURL != "" && c.OllamaModel != ""
}

// HasWillys reports whether the Willys adapter is configured.
func (c Config) HasWillys() bool {
	return c.AdapterURL != ""
}

// HasICAAuth reports whether ICA's elevated-auth credential file is configured.
// When false, ICA's elevated-tier operations (wishlist push) cannot run; its
// basic-tier operations (search) are unaffected.
func (c Config) HasICAAuth() bool {
	return c.ICAAuthFile != ""
}

// Validate checks that the configuration satisfies the requirements of the
// given command and returns a clear error naming the first missing variable.
// It is called by the composition root after Load() and before any client
// constructor runs, so a misconfigured startup fails fast with an actionable
// message rather than deep inside a client.
//
// Commands and their requirements:
//
//	serve, migrate, tonight, ingredients — require a database
//	plan                                  — require Mealie (base URL + token)
//	sync                                  — require a database (recipes) or SLV (nutrition)
//	sync-offers                           — require at least one retailer adapter
//	demo, migrate (status)                — require nothing
func (c Config) Validate(command string) error {
	switch command {
	case "serve", "migrate", "tonight", "ingredients":
		if !c.HasDatabase() {
			return fmt.Errorf("config: %q requires a database — set DATABASE_URL or POSTGRES_PASSWORD (and POSTGRES_HOST/PORT/DB/USER)", command)
		}
	case "plan":
		if c.MealieBaseURL == "" {
			return fmt.Errorf("config: %q requires MEALIE_BASE_URL", command)
		}
		if c.MealieAPIToken == "" {
			return fmt.Errorf("config: %q requires MEALIE_API_TOKEN", command)
		}
	case "sync":
		// sync recipes needs a database; sync nutrition needs SLV.
		// We can't tell which sub-command from here, so require at least one.
		if !c.HasDatabase() && !c.HasSLV() {
			return fmt.Errorf("config: %q requires a database (for recipes) or SLV_BASE_URL (for nutrition)", command)
		}
	case "sync-offers":
		if !c.HasWillys() && c.ICAAdapterURL == "" && c.HemkopAdapterURL == "" {
			return fmt.Errorf("config: %q requires at least one retailer adapter — set ADAPTER_URL, ICA_ADAPTER_URL, or HEMKOP_ADAPTER_URL", command)
		}
	case "demo":
		// demo requires nothing.
	default:
		// Unknown commands are not validated here; the CLI dispatcher handles
		// them. We don't fail on unknown commands so that new commands can be
		// added without updating this switch.
	}
	return nil
}

// ValidateForCommand is an alias for Validate, provided for readability at
// call sites that want to be explicit about the command name.
func (c Config) ValidateForCommand(command string) error {
	return c.Validate(command)
}

// MissingVars returns a human-readable list of the env var names that are
// required for the given command but not set. Used by tests and by the
// composition root to produce actionable error messages.
func (c Config) MissingVars(command string) []string {
	var missing []string
	switch command {
	case "serve", "migrate", "tonight", "ingredients":
		if !c.HasDatabase() {
			missing = append(missing, "DATABASE_URL (or POSTGRES_PASSWORD+POSTGRES_HOST+POSTGRES_PORT+POSTGRES_DB+POSTGRES_USER)")
		}
	case "plan":
		if c.MealieBaseURL == "" {
			missing = append(missing, "MEALIE_BASE_URL")
		}
		if c.MealieAPIToken == "" {
			missing = append(missing, "MEALIE_API_TOKEN")
		}
	case "sync":
		if !c.HasDatabase() && !c.HasSLV() {
			missing = append(missing, "DATABASE_URL (or SLV_BASE_URL)")
		}
	case "sync-offers":
		if !c.HasWillys() && c.ICAAdapterURL == "" && c.HemkopAdapterURL == "" {
			missing = append(missing, "ADAPTER_URL (or ICA_ADAPTER_URL or HEMKOP_ADAPTER_URL)")
		}
	}
	return missing
}

// FormatMissing returns a single-line string of the missing vars for the
// given command, or "" when nothing is missing.
func (c Config) FormatMissing(command string) string {
	m := c.MissingVars(command)
	if len(m) == 0 {
		return ""
	}
	return strings.Join(m, ", ")
}
