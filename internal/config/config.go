// Package config is the single source of truth for spisordning's runtime
// configuration. It replaces the os.Getenv calls that used to be scattered
// across cmd/food-brain and cmd/mcp-server's own composition-root files with
// one Load() call per binary.
//
// It deliberately does not parse or hold internal/persistence's DATABASE_URL
// config — persistence.FromEnv already owns that (env-based, injectable
// getenv, its own validation) and duplicating it here would just be two
// copies of the same parsing logic. Composition roots call both: config.Load()
// for everything else, persistence.FromEnv(os.Getenv) for the database.
//
// This package is cmd-only by design (enforced by internal/architecturetest):
// clients keep receiving plain typed constructor arguments exactly as they do
// today (e.g. mealie.New(baseURL, token)) — only the composition root reads
// this struct and extracts the fields each constructor needs. No internal/
// layer other than cmd imports this package.
package config

import (
	"os"
	"strconv"
)

// Config holds every environment-derived value spisordning's composition
// roots need, gathered in one place instead of read ad hoc across files.
type Config struct {
	// HTTP server addresses.
	HTTPAddr string // SPISORNING_ADDR, food-brain's HTTP API. Default ":8080".
	MCPAddr  string // SPISORNING_MCP_ADDR, the MCP server. Default ":8081".

	// Mealie (recipe source).
	MealieBaseURL  string // MEALIE_BASE_URL
	MealieAPIToken string // MEALIE_API_TOKEN

	// Skolmaten (school lunch menu, for dinner dedup).
	SkolmatenBaseURL     string // SKOLMATEN_BASE_URL. Default "http://192.168.1.120:8787".
	SkolmatenClientToken string // SKOLMATEN_CLIENT_TOKEN
	SkolmatenSchool      string // SKOLMATEN_SCHOOL (empty = skip school-lunch dedup)

	// Retailer adapters.
	WillysAdapterURL string // ADAPTER_URL. Default "http://localhost:8402".
	ICAAdapterURL    string // ICA_ADAPTER_URL. Default "http://localhost:8403".

	// ICAElevatedCredentialPath is where ica-adapter's manually-refreshed
	// elevated (OAuth2 mobile session) credential is expected to live on disk —
	// see internal/retailer's AuthTier doc comment. Go never reads this file's
	// contents (ica-adapter owns the session entirely); the path exists so a
	// future health check can report the credential's age, and so the manual
	// refresh handoff has one documented, discoverable location instead of
	// none (see docs/infrastructure/ica-elevated-auth.md). The 401/403
	// staleness detection in internal/retailer works independently of this —
	// it reads the adapter's live HTTP response, not this file.
	ICAElevatedCredentialPath string // ICA_ELEVATED_CREDENTIAL_PATH

	// Ingredient/product data sources — each independently optional.
	SLVBaseURL   string // SLV_BASE_URL (Livsmedelsverket nutrition data)
	DabasEnabled bool   // DABAS_ENABLED
	MPKEnabled   bool   // MPK_ENABLED (Matpriskollen)

	// LLM (optional recipe-ranking explanations via Olla).
	OllaBaseURL string // OLLA_OPENAI_BASE_URL
	OllaModel   string // OLLA_MODEL
}

// Load builds a Config from the current environment. It never fails — every
// field has either a sane default or is legitimately optional (empty means
// "that integration is disabled"), matching today's behavior. Commands that
// need a value to be present (e.g. DATABASE_URL for `serve`/`migrate`, still
// validated by persistence.FromEnv) fail at the point they try to use it, not
// here — Load's job is gathering values, not deciding which command needs
// which.
func Load() Config {
	return Config{
		HTTPAddr: envDefault("SPISORNING_ADDR", ":8080"),
		MCPAddr:  envDefault("SPISORNING_MCP_ADDR", ":8081"),

		MealieBaseURL:  os.Getenv("MEALIE_BASE_URL"),
		MealieAPIToken: os.Getenv("MEALIE_API_TOKEN"),

		SkolmatenBaseURL:     envDefault("SKOLMATEN_BASE_URL", "http://192.168.1.120:8787"),
		SkolmatenClientToken: os.Getenv("SKOLMATEN_CLIENT_TOKEN"),
		SkolmatenSchool:      os.Getenv("SKOLMATEN_SCHOOL"),

		WillysAdapterURL:          envDefault("ADAPTER_URL", "http://localhost:8402"),
		ICAAdapterURL:             envDefault("ICA_ADAPTER_URL", "http://localhost:8403"),
		ICAElevatedCredentialPath: os.Getenv("ICA_ELEVATED_CREDENTIAL_PATH"),

		SLVBaseURL:   os.Getenv("SLV_BASE_URL"),
		DabasEnabled: envBool("DABAS_ENABLED"),
		MPKEnabled:   envBool("MPK_ENABLED"),

		OllaBaseURL: os.Getenv("OLLA_OPENAI_BASE_URL"),
		OllaModel:   os.Getenv("OLLA_MODEL"),
	}
}

// MealieEnabled reports whether both Mealie values needed to construct a
// client are present — mirrors the `mealieURL != "" && token != ""` checks
// that used to be duplicated at each call site.
func (c Config) MealieEnabled() bool {
	return c.MealieBaseURL != "" && c.MealieAPIToken != ""
}

// OllaEnabled reports whether both Olla values needed to construct a client
// are present.
func (c Config) OllaEnabled() bool {
	return c.OllaBaseURL != "" && c.OllaModel != ""
}

func envDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envBool treats any non-empty value as true, matching the existing
// `os.Getenv("DABAS_ENABLED") != ""` convention (a presence flag, not a
// parsed boolean) — but also accepts an explicit "false"/"0" so a compose
// file can set the var to a falsy string without enabling the feature.
func envBool(key string) bool {
	v := os.Getenv(key)
	if v == "" {
		return false
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return true // any other non-empty value keeps the old "presence = true" behavior
	}
	return b
}
