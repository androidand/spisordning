package config

import "testing"

// withEnv sets vars for the duration of the test and restores the previous
// values (including "unset") afterward, so tests don't leak into each other
// or the real environment.
func withEnv(t *testing.T, vars map[string]string) {
	t.Helper()
	for k, v := range vars {
		t.Setenv(k, v)
	}
}

func TestLoad_Defaults(t *testing.T) {
	// Explicitly clear every var this package reads so the test is immune to
	// whatever's in the ambient environment.
	withEnv(t, map[string]string{
		"SPISORNING_ADDR": "", "SPISORNING_MCP_ADDR": "",
		"MEALIE_BASE_URL": "", "MEALIE_API_TOKEN": "",
		"SKOLMATEN_BASE_URL": "", "SKOLMATEN_CLIENT_TOKEN": "", "SKOLMATEN_SCHOOL": "",
		"ADAPTER_URL": "", "ICA_ADAPTER_URL": "",
		"SLV_BASE_URL": "", "DABAS_ENABLED": "", "MPK_ENABLED": "",
		"OLLA_OPENAI_BASE_URL": "", "OLLA_MODEL": "",
	})

	cfg := Load()

	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.MCPAddr != ":8081" {
		t.Errorf("MCPAddr = %q, want :8081", cfg.MCPAddr)
	}
	if cfg.WillysAdapterURL != "http://localhost:8402" {
		t.Errorf("WillysAdapterURL = %q, want http://localhost:8402", cfg.WillysAdapterURL)
	}
	if cfg.ICAAdapterURL != "http://localhost:8403" {
		t.Errorf("ICAAdapterURL = %q, want http://localhost:8403", cfg.ICAAdapterURL)
	}
	if cfg.SkolmatenBaseURL != "http://192.168.1.120:8787" {
		t.Errorf("SkolmatenBaseURL = %q, want http://192.168.1.120:8787", cfg.SkolmatenBaseURL)
	}
	if cfg.MealieEnabled() {
		t.Error("MealieEnabled() = true with no Mealie vars set, want false")
	}
	if cfg.OllaEnabled() {
		t.Error("OllaEnabled() = true with no Olla vars set, want false")
	}
	if cfg.DabasEnabled || cfg.MPKEnabled {
		t.Error("optional integrations should default to disabled")
	}
}

func TestLoad_OverridesAndOptionalIntegrations(t *testing.T) {
	withEnv(t, map[string]string{
		"SPISORNING_ADDR": ":9999",
		"MEALIE_BASE_URL": "http://mealie.local:9000", "MEALIE_API_TOKEN": "tok",
		"OLLA_OPENAI_BASE_URL": "http://olla.local", "OLLA_MODEL": "llama",
		"DABAS_ENABLED": "1", "MPK_ENABLED": "true",
	})

	cfg := Load()

	if cfg.HTTPAddr != ":9999" {
		t.Errorf("HTTPAddr override not applied, got %q", cfg.HTTPAddr)
	}
	if !cfg.MealieEnabled() {
		t.Error("MealieEnabled() = false with both vars set, want true")
	}
	if !cfg.OllaEnabled() {
		t.Error("OllaEnabled() = false with both vars set, want true")
	}
	if !cfg.DabasEnabled {
		t.Error("DabasEnabled = false with DABAS_ENABLED=1, want true")
	}
	if !cfg.MPKEnabled {
		t.Error("MPKEnabled = false with MPK_ENABLED=true, want true")
	}
}

func TestLoad_PartialMealieIsNotEnabled(t *testing.T) {
	withEnv(t, map[string]string{
		"MEALIE_BASE_URL": "http://mealie.local:9000", "MEALIE_API_TOKEN": "",
	})

	cfg := Load()

	if cfg.MealieEnabled() {
		t.Error("MealieEnabled() = true with only base URL set, want false (token also required)")
	}
}

func TestEnvBool_ExplicitFalseStaysDisabled(t *testing.T) {
	withEnv(t, map[string]string{"DABAS_ENABLED": "false"})

	cfg := Load()

	if cfg.DabasEnabled {
		t.Error("DabasEnabled = true with DABAS_ENABLED=false, want false")
	}
}
