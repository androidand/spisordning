package config

import (
	"os"
	"strings"
	"testing"
)

// setEnv sets the given env vars for the duration of the test, restoring
// their original values on cleanup.
func setEnv(t *testing.T, vars map[string]string) {
	t.Helper()
	for k, v := range vars {
		old, existed := os.LookupEnv(k)
		t.Cleanup(func() {
			if existed {
				os.Setenv(k, old)
			} else {
				os.Unsetenv(k)
			}
		})
		os.Setenv(k, v)
	}
}

// clearEnv unsets the given env vars for the duration of the test.
func clearEnv(t *testing.T, keys ...string) {
	t.Helper()
	for _, k := range keys {
		old, existed := os.LookupEnv(k)
		t.Cleanup(func() {
			if existed {
				os.Setenv(k, old)
			} else {
				os.Unsetenv(k)
			}
		})
		os.Unsetenv(k)
	}
}

// allConfigEnvVars lists every env var that Load() reads, so tests can
// clear them all at once for a clean slate.
var allConfigEnvVars = []string{
	"DATABASE_URL", "POSTGRES_HOST", "POSTGRES_PORT", "POSTGRES_DB",
	"POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_SSLMODE",
	"MEALIE_BASE_URL", "MEALIE_API_TOKEN",
	"GROCY_BASE_URL", "GROCY_API_KEY",
	"SLV_BASE_URL", "DABAS_ENABLED", "MPK_ENABLED",
	"ADAPTER_URL", "ICA_ADAPTER_URL", "HEMKOP_ADAPTER_URL", "ICA_AUTH_FILE",
	"ICA_ELEVATED_CREDENTIAL_PATH",
	"SKOLMATEN_BASE_URL", "SKOLMATEN_CLIENT_TOKEN", "SKOLMATEN_SCHOOL",
	"OLLA_OPENAI_BASE_URL", "OLLA_MODEL",
	"SPISORNING_ADDR", "SPISORNING_MCP_ADDR",
}

func clearAllConfigEnv(t *testing.T) {
	t.Helper()
	clearEnv(t, allConfigEnvVars...)
}

func TestLoad_FullValidEnv(t *testing.T) {
	clearAllConfigEnv(t)
	setEnv(t, map[string]string{
		"DATABASE_URL":          "postgres://user:pass@localhost:5432/db",
		"MEALIE_BASE_URL":       "http://mealie:9000",
		"MEALIE_API_TOKEN":      "secret-token",
		"GROCY_BASE_URL":        "http://grocy:80",
		"GROCY_API_KEY":         "grocy-key",
		"SLV_BASE_URL":          "http://slv.example.com",
		"DABAS_ENABLED":         "true",
		"MPK_ENABLED":           "true",
		"ADAPTER_URL":           "http://willys:8402",
		"ICA_ADAPTER_URL":       "http://ica:8403",
		"HEMKOP_ADAPTER_URL":    "http://hemkop:8404",
		"SKOLMATEN_BASE_URL":    "http://skolmaten:8787",
		"SKOLMATEN_CLIENT_TOKEN": "skol-token",
		"SKOLMATEN_SCHOOL":      "grinda",
		"OLLA_OPENAI_BASE_URL":  "http://ollama:11434/v1",
		"OLLA_MODEL":            "llama3",
		"SPISORNING_ADDR":       ":9999",
		"SPISORNING_MCP_ADDR":   ":9998",
	})

	cfg := Load()

	if cfg.DatabaseURL != "postgres://user:pass@localhost:5432/db" {
		t.Errorf("DatabaseURL = %q", cfg.DatabaseURL)
	}
	if cfg.MealieBaseURL != "http://mealie:9000" {
		t.Errorf("MealieBaseURL = %q", cfg.MealieBaseURL)
	}
	if cfg.MealieAPIToken != "secret-token" {
		t.Errorf("MealieAPIToken = %q", cfg.MealieAPIToken)
	}
	if cfg.GrocyBaseURL != "http://grocy:80" {
		t.Errorf("GrocyBaseURL = %q", cfg.GrocyBaseURL)
	}
	if cfg.GrocyAPIKey != "grocy-key" {
		t.Errorf("GrocyAPIKey = %q", cfg.GrocyAPIKey)
	}
	if cfg.SLVBaseURL != "http://slv.example.com" {
		t.Errorf("SLVBaseURL = %q", cfg.SLVBaseURL)
	}
	if !cfg.DabasEnabled {
		t.Errorf("DabasEnabled = false, want true")
	}
	if !cfg.MPKEnabled {
		t.Errorf("MPKEnabled = false, want true")
	}
	if cfg.AdapterURL != "http://willys:8402" {
		t.Errorf("AdapterURL = %q", cfg.AdapterURL)
	}
	if cfg.ICAAdapterURL != "http://ica:8403" {
		t.Errorf("ICAAdapterURL = %q", cfg.ICAAdapterURL)
	}
	if cfg.HemkopAdapterURL != "http://hemkop:8404" {
		t.Errorf("HemkopAdapterURL = %q", cfg.HemkopAdapterURL)
	}
	if cfg.SkolmatenBaseURL != "http://skolmaten:8787" {
		t.Errorf("SkolmatenBaseURL = %q", cfg.SkolmatenBaseURL)
	}
	if cfg.SkolmatenClientToken != "skol-token" {
		t.Errorf("SkolmatenClientToken = %q", cfg.SkolmatenClientToken)
	}
	if cfg.SkolmatenSchool != "grinda" {
		t.Errorf("SkolmatenSchool = %q", cfg.SkolmatenSchool)
	}
	if cfg.OllamaBaseURL != "http://ollama:11434/v1" {
		t.Errorf("OllamaBaseURL = %q", cfg.OllamaBaseURL)
	}
	if cfg.OllamaModel != "llama3" {
		t.Errorf("OllamaModel = %q", cfg.OllamaModel)
	}
	if cfg.SpisordningAddr != ":9999" {
		t.Errorf("SpisordningAddr = %q", cfg.SpisordningAddr)
	}
	if cfg.SpisordningMCPAddr != ":9998" {
		t.Errorf("SpisordningMCPAddr = %q", cfg.SpisordningMCPAddr)
	}

	// Predicates should all report true.
	if !cfg.HasDatabase() {
		t.Errorf("HasDatabase = false, want true")
	}
	if !cfg.HasMealie() {
		t.Errorf("HasMealie = false, want true")
	}
	if !cfg.HasGrocy() {
		t.Errorf("HasGrocy = false, want true")
	}
	if !cfg.HasSLV() {
		t.Errorf("HasSLV = false, want true")
	}
	if !cfg.HasSkolmaten() {
		t.Errorf("HasSkolmaten = false, want true")
	}
	if !cfg.HasOllama() {
		t.Errorf("HasOllama = false, want true")
	}
	if !cfg.HasWillys() {
		t.Errorf("HasWillys = false, want true")
	}

	// Validate should pass for all commands.
	for _, cmd := range []string{"serve", "migrate", "tonight", "ingredients", "plan", "sync", "sync-offers", "demo"} {
		if err := cfg.Validate(cmd); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", cmd, err)
		}
	}
}

func TestLoad_OptionalIntegrationsUnset(t *testing.T) {
	clearAllConfigEnv(t)
	// Only set the database; leave all optional integrations unset.
	setEnv(t, map[string]string{
		"DATABASE_URL": "postgres://user:pass@localhost:5432/db",
	})

	cfg := Load()

	// Optional integrations should be at zero values.
	if cfg.MealieBaseURL != "" {
		t.Errorf("MealieBaseURL = %q, want empty", cfg.MealieBaseURL)
	}
	if cfg.MealieAPIToken != "" {
		t.Errorf("MealieAPIToken = %q, want empty", cfg.MealieAPIToken)
	}
	if cfg.GrocyBaseURL != "" {
		t.Errorf("GrocyBaseURL = %q, want empty", cfg.GrocyBaseURL)
	}
	if cfg.SLVBaseURL != "" {
		t.Errorf("SLVBaseURL = %q, want empty", cfg.SLVBaseURL)
	}
	if cfg.DabasEnabled {
		t.Errorf("DabasEnabled = true, want false")
	}
	if cfg.MPKEnabled {
		t.Errorf("MPKEnabled = true, want false")
	}
	if cfg.SkolmatenBaseURL != "" {
		t.Errorf("SkolmatenBaseURL = %q, want empty", cfg.SkolmatenBaseURL)
	}
	if cfg.OllamaBaseURL != "" {
		t.Errorf("OllamaBaseURL = %q, want empty", cfg.OllamaBaseURL)
	}

	// Predicates should report false for unset optional integrations.
	if cfg.HasMealie() {
		t.Errorf("HasMealie = true, want false")
	}
	if cfg.HasGrocy() {
		t.Errorf("HasGrocy = true, want false")
	}
	if cfg.HasSLV() {
		t.Errorf("HasSLV = true, want false")
	}
	if cfg.HasSkolmaten() {
		t.Errorf("HasSkolmaten = true, want false")
	}
	if cfg.HasOllama() {
		t.Errorf("HasOllama = true, want false")
	}

	// Database should still be configured.
	if !cfg.HasDatabase() {
		t.Errorf("HasDatabase = false, want true")
	}

	// Validate should pass for serve (database is set).
	if err := cfg.Validate("serve"); err != nil {
		t.Errorf("Validate(serve) = %v, want nil", err)
	}

	// Validate should fail for plan (Mealie not set).
	err := cfg.Validate("plan")
	if err == nil {
		t.Fatal("Validate(plan) = nil, want error")
	}
	if !strings.Contains(err.Error(), "MEALIE_BASE_URL") {
		t.Errorf("Validate(plan) error = %q, want it to mention MEALIE_BASE_URL", err)
	}
}

func TestLoad_DefaultsForRetailerAdapters(t *testing.T) {
	clearAllConfigEnv(t)
	// Don't set any adapter URLs; they should get defaults.
	cfg := Load()

	if cfg.AdapterURL != "http://localhost:8402" {
		t.Errorf("AdapterURL = %q, want default http://localhost:8402", cfg.AdapterURL)
	}
	if cfg.ICAAdapterURL != "http://localhost:8403" {
		t.Errorf("ICAAdapterURL = %q, want default http://localhost:8403", cfg.ICAAdapterURL)
	}
	if cfg.HemkopAdapterURL != "http://localhost:8404" {
		t.Errorf("HemkopAdapterURL = %q, want default http://localhost:8404", cfg.HemkopAdapterURL)
	}
	if cfg.SpisordningAddr != ":8080" {
		t.Errorf("SpisordningAddr = %q, want default :8080", cfg.SpisordningAddr)
	}
	if cfg.SpisordningMCPAddr != ":8081" {
		t.Errorf("SpisordningMCPAddr = %q, want default :8081", cfg.SpisordningMCPAddr)
	}
}

func TestLoad_ICAAuthFile(t *testing.T) {
	clearAllConfigEnv(t)
	setEnv(t, map[string]string{
		"ICA_AUTH_FILE": "/home/andreas/.config/spisordning/ica-auth.json",
	})
	cfg := Load()

	if cfg.ICAAuthFile != "/home/andreas/.config/spisordning/ica-auth.json" {
		t.Errorf("ICAAuthFile = %q, want the configured path", cfg.ICAAuthFile)
	}
	if !cfg.HasICAAuth() {
		t.Errorf("HasICAAuth = false, want true when ICA_AUTH_FILE is set")
	}
}

func TestLoad_ICAAuthFileUnset(t *testing.T) {
	clearAllConfigEnv(t)
	cfg := Load()

	if cfg.ICAAuthFile != "" {
		t.Errorf("ICAAuthFile = %q, want empty when ICA_AUTH_FILE is unset", cfg.ICAAuthFile)
	}
	if cfg.HasICAAuth() {
		t.Errorf("HasICAAuth = true, want false when ICA_AUTH_FILE is unset")
	}
}

func TestLoad_ICAElevatedCredentialPath(t *testing.T) {
	clearAllConfigEnv(t)
	setEnv(t, map[string]string{
		"ICA_ELEVATED_CREDENTIAL_PATH": "/etc/spisordning/ica-ecom-cookies.json",
	})
	cfg := Load()

	if cfg.ICAElevatedCredentialPath != "/etc/spisordning/ica-ecom-cookies.json" {
		t.Errorf("ICAElevatedCredentialPath = %q, want the configured path", cfg.ICAElevatedCredentialPath)
	}
}

func TestLoad_ICAElevatedCredentialPathUnset(t *testing.T) {
	clearAllConfigEnv(t)
	cfg := Load()

	if cfg.ICAElevatedCredentialPath != "" {
		t.Errorf("ICAElevatedCredentialPath = %q, want empty when ICA_ELEVATED_CREDENTIAL_PATH is unset", cfg.ICAElevatedCredentialPath)
	}
}

func TestValidate_ServeMissingDatabase(t *testing.T) {
	clearAllConfigEnv(t)
	cfg := Load()

	err := cfg.Validate("serve")
	if err == nil {
		t.Fatal("Validate(serve) = nil, want error")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Errorf("Validate(serve) error = %q, want it to mention DATABASE_URL", err)
	}
}

func TestValidate_MigrateMissingDatabase(t *testing.T) {
	clearAllConfigEnv(t)
	cfg := Load()

	err := cfg.Validate("migrate")
	if err == nil {
		t.Fatal("Validate(migrate) = nil, want error")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Errorf("Validate(migrate) error = %q, want it to mention DATABASE_URL", err)
	}
}

func TestValidate_PlanMissingMealie(t *testing.T) {
	clearAllConfigEnv(t)
	cfg := Load()

	err := cfg.Validate("plan")
	if err == nil {
		t.Fatal("Validate(plan) = nil, want error")
	}
	if !strings.Contains(err.Error(), "MEALIE_BASE_URL") {
		t.Errorf("Validate(plan) error = %q, want it to mention MEALIE_BASE_URL", err)
	}
}

func TestValidate_PlanMissingToken(t *testing.T) {
	clearAllConfigEnv(t)
	setEnv(t, map[string]string{
		"MEALIE_BASE_URL": "http://mealie:9000",
	})
	cfg := Load()

	err := cfg.Validate("plan")
	if err == nil {
		t.Fatal("Validate(plan) = nil, want error")
	}
	if !strings.Contains(err.Error(), "MEALIE_API_TOKEN") {
		t.Errorf("Validate(plan) error = %q, want it to mention MEALIE_API_TOKEN", err)
	}
}

func TestValidate_DemoRequiresNothing(t *testing.T) {
	clearAllConfigEnv(t)
	cfg := Load()

	if err := cfg.Validate("demo"); err != nil {
		t.Errorf("Validate(demo) = %v, want nil", err)
	}
}

func TestValidate_UnknownCommand(t *testing.T) {
	clearAllConfigEnv(t)
	cfg := Load()

	// Unknown commands should not fail validation.
	if err := cfg.Validate("some-future-command"); err != nil {
		t.Errorf("Validate(unknown) = %v, want nil", err)
	}
}

func TestValidate_SyncWithDatabase(t *testing.T) {
	clearAllConfigEnv(t)
	setEnv(t, map[string]string{
		"DATABASE_URL": "postgres://user:pass@localhost:5432/db",
	})
	cfg := Load()

	if err := cfg.Validate("sync"); err != nil {
		t.Errorf("Validate(sync) with database = %v, want nil", err)
	}
}

func TestValidate_SyncWithSLV(t *testing.T) {
	clearAllConfigEnv(t)
	setEnv(t, map[string]string{
		"SLV_BASE_URL": "http://slv.example.com",
	})
	cfg := Load()

	if err := cfg.Validate("sync"); err != nil {
		t.Errorf("Validate(sync) with SLV = %v, want nil", err)
	}
}

func TestValidate_SyncMissingBoth(t *testing.T) {
	clearAllConfigEnv(t)
	cfg := Load()

	err := cfg.Validate("sync")
	if err == nil {
		t.Fatal("Validate(sync) = nil, want error")
	}
	if !strings.Contains(err.Error(), "database") {
		t.Errorf("Validate(sync) error = %q, want it to mention database", err)
	}
}

func TestValidate_SyncOffersWithWillys(t *testing.T) {
	clearAllConfigEnv(t)
	// AdapterURL gets a default, so HasWillys is true.
	cfg := Load()

	if err := cfg.Validate("sync-offers"); err != nil {
		t.Errorf("Validate(sync-offers) with default willys = %v, want nil", err)
	}
}

func TestMissingVars_Serve(t *testing.T) {
	clearAllConfigEnv(t)
	cfg := Load()

	missing := cfg.MissingVars("serve")
	if len(missing) != 1 {
		t.Fatalf("MissingVars(serve) = %v, want 1 item", missing)
	}
	if !strings.Contains(missing[0], "DATABASE_URL") {
		t.Errorf("MissingVars(serve)[0] = %q, want it to mention DATABASE_URL", missing[0])
	}
}

func TestMissingVars_Plan(t *testing.T) {
	clearAllConfigEnv(t)
	cfg := Load()

	missing := cfg.MissingVars("plan")
	if len(missing) != 2 {
		t.Fatalf("MissingVars(plan) = %v, want 2 items", missing)
	}
	joined := strings.Join(missing, ",")
	if !strings.Contains(joined, "MEALIE_BASE_URL") || !strings.Contains(joined, "MEALIE_API_TOKEN") {
		t.Errorf("MissingVars(plan) = %v, want both MEALIE_BASE_URL and MEALIE_API_TOKEN", missing)
	}
}

func TestMissingVars_None(t *testing.T) {
	clearAllConfigEnv(t)
	setEnv(t, map[string]string{
		"DATABASE_URL": "postgres://user:pass@localhost:5432/db",
	})
	cfg := Load()

	missing := cfg.MissingVars("serve")
	if len(missing) != 0 {
		t.Errorf("MissingVars(serve) = %v, want empty", missing)
	}
}

func TestFormatMissing(t *testing.T) {
	clearAllConfigEnv(t)
	cfg := Load()

	s := cfg.FormatMissing("plan")
	if s == "" {
		t.Fatal("FormatMissing(plan) = empty, want non-empty")
	}
	if !strings.Contains(s, "MEALIE_BASE_URL") {
		t.Errorf("FormatMissing(plan) = %q, want it to mention MEALIE_BASE_URL", s)
	}

	// When nothing is missing, FormatMissing returns "".
	setEnv(t, map[string]string{
		"DATABASE_URL": "postgres://user:pass@localhost:5432/db",
	})
	cfg2 := Load()
	if s := cfg2.FormatMissing("serve"); s != "" {
		t.Errorf("FormatMissing(serve) = %q, want empty", s)
	}
}

func TestHasDatabase_ViaPostgresFields(t *testing.T) {
	clearAllConfigEnv(t)
	setEnv(t, map[string]string{
		"POSTGRES_HOST":     "localhost",
		"POSTGRES_PORT":     "5432",
		"POSTGRES_DB":       "spisordning",
		"POSTGRES_USER":     "spisordning",
		"POSTGRES_PASSWORD": "secret",
	})
	cfg := Load()

	if !cfg.HasDatabase() {
		t.Errorf("HasDatabase = false, want true (via POSTGRES_* fields)")
	}
	if err := cfg.Validate("serve"); err != nil {
		t.Errorf("Validate(serve) = %v, want nil", err)
	}
}

func TestHasDatabase_NotConfigured(t *testing.T) {
	clearAllConfigEnv(t)
	cfg := Load()

	if cfg.HasDatabase() {
		t.Errorf("HasDatabase = true, want false")
	}
}
