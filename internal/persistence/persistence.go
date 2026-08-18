// Package persistence is the Postgres repository layer (task 2.1: driver pin,
// env-based connection config). It is the only package that may import pgx;
// the architecture test enforces that domain/application/clients never import
// this package, and that this package imports only domain + external deps.
//
// The driver choice (github.com/jackc/pgx/v5) is documented in
// openspec/changes/establish-enforced-go-architecture/design.md §3.
package persistence

import (
	"context"
	"fmt"
	"net/url"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds Postgres connection parameters. It is parsed from the
// environment rather than constructed by callers so every entry point (CLI,
// HTTP server, tests) connects identically.
type Config struct {
	Host     string
	Port     string
	Database string
	User     string
	Password string
	SSLMode  string
}

// Defaults matching docker-compose.yml's postgres service and the values
// .env.example documents.
const (
	DefaultHost     = "localhost"
	DefaultPort     = "5432"
	DefaultDatabase = "spisordning"
	DefaultUser     = "spisordning"
	DefaultSSLMode  = "disable"
)

// FromEnv builds a Config from the environment. If DATABASE_URL is set it
// takes precedence (single-line 12-factor connection strings); otherwise the
// individual POSTGRES_* variables are used, matching docker-compose's
// interpolation. A missing/empty password is an error — the same requirement
// docker-compose enforces with ${POSTGRES_PASSWORD:?...}.
func FromEnv(getenv func(string) string) (Config, error) {
	if raw := getenv("DATABASE_URL"); raw != "" {
		u, err := url.Parse(raw)
		if err != nil {
			return Config{}, fmt.Errorf("persistence: invalid DATABASE_URL: %w", err)
		}
		password, _ := u.User.Password()
		return Config{
			Host:     u.Hostname(),
			Port:     u.Port(),
			Database: trimPrefix(u.Path, "/"),
			User:     u.User.Username(),
			Password: password,
			SSLMode:  u.Query().Get("sslmode"),
		}, nil
	}

	cfg := Config{
		Host:     getenv("POSTGRES_HOST"),
		Port:     getenv("POSTGRES_PORT"),
		Database: getenv("POSTGRES_DB"),
		User:     getenv("POSTGRES_USER"),
		Password: getenv("POSTGRES_PASSWORD"),
		SSLMode:  getenv("POSTGRES_SSLMODE"),
	}
	if cfg.Host == "" {
		cfg.Host = DefaultHost
	}
	if cfg.Port == "" {
		cfg.Port = DefaultPort
	}
	if cfg.Database == "" {
		cfg.Database = DefaultDatabase
	}
	if cfg.User == "" {
		cfg.User = DefaultUser
	}
	if cfg.SSLMode == "" {
		cfg.SSLMode = DefaultSSLMode
	}
	if cfg.Password == "" {
		return Config{}, fmt.Errorf("persistence: POSTGRES_PASSWORD is required (or set DATABASE_URL)")
	}
	return cfg, nil
}

// DSN renders a postgres:// URL suitable for pgxpool.ParseConfig, with the
// password URL-escaped so special characters survive.
func (c Config) DSN() string {
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.User, c.Password),
		Host:   c.Host + ":" + c.Port,
		Path:   "/" + c.Database,
	}
	q := url.Values{}
	q.Set("sslmode", c.SSLMode)
	u.RawQuery = q.Encode()
	return u.String()
}

// NewPool opens a pgx connection pool from cfg. Callers MUST close the pool
// when done (defer pool.Close()).
func NewPool(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("persistence: create pool: %w", err)
	}
	return pool, nil
}

func trimPrefix(s, prefix string) string {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}
