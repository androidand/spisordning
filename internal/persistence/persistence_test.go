package persistence

import "testing"

func TestFromEnv_Defaults(t *testing.T) {
	cfg, err := FromEnv(func(k string) string {
		if k == "POSTGRES_PASSWORD" {
			return "change-me"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	if cfg.Host != DefaultHost || cfg.Port != DefaultPort || cfg.Database != DefaultDatabase || cfg.User != DefaultUser {
		t.Errorf("defaults not applied: %+v", cfg)
	}
	if cfg.SSLMode != "disable" {
		t.Errorf("expected sslmode disable, got %q", cfg.SSLMode)
	}
}

func TestFromEnv_IndividualVars(t *testing.T) {
	env := map[string]string{
		"POSTGRES_HOST": "postgres", "POSTGRES_PORT": "5433", "POSTGRES_DB": "db",
		"POSTGRES_USER": "u", "POSTGRES_PASSWORD": "p", "POSTGRES_SSLMODE": "require",
	}
	cfg, err := FromEnv(func(k string) string { return env[k] })
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	want := Config{Host: "postgres", Port: "5433", Database: "db", User: "u", Password: "p", SSLMode: "require"}
	if cfg != want {
		t.Errorf("got %+v, want %+v", cfg, want)
	}
}

func TestFromEnv_DatabaseURLWins(t *testing.T) {
	env := map[string]string{
		"DATABASE_URL":      "postgres://alice:s3cret@db.example.com:5433/meals?sslmode=require",
		"POSTGRES_PASSWORD": "ignored",
	}
	cfg, err := FromEnv(func(k string) string { return env[k] })
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	want := Config{Host: "db.example.com", Port: "5433", Database: "meals", User: "alice", Password: "s3cret", SSLMode: "require"}
	if cfg != want {
		t.Errorf("got %+v, want %+v", cfg, want)
	}
}

func TestFromEnv_MissingPassword(t *testing.T) {
	_, err := FromEnv(func(string) string { return "" })
	if err == nil {
		t.Fatal("expected error when POSTGRES_PASSWORD is missing")
	}
}

func TestDSN_EscapesSpecialChars(t *testing.T) {
	cfg := Config{Host: "localhost", Port: "5432", Database: "spisordning", User: "u", Password: "p@ss:word/!", SSLMode: "disable"}
	dsn := cfg.DSN()
	if dsn != "postgres://u:p%40ss%3Aword%2F%21@localhost:5432/spisordning?sslmode=disable" {
		t.Errorf("unexpected DSN: %s", dsn)
	}
}
