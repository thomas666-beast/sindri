package httpapi_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thomas666-beast/reqval"

	"github.com/thomas666-beast/sindri/internal/auth"
	"github.com/thomas666-beast/sindri/internal/config"
	"github.com/thomas666-beast/sindri/internal/crypto"
	"github.com/thomas666-beast/sindri/internal/db"
	"github.com/thomas666-beast/sindri/internal/dbadapter"
	"github.com/thomas666-beast/sindri/internal/httpapi"
)

// testAllowedOrigins returns the WS origin allowlist for tests.
//
// It reads the SAME env var that production uses (APP_ALLOWED_ORIGINS) and
// parses it with the SAME parser config.Load uses. Tests therefore exercise
// the production code path end-to-end: if config parsing breaks, tests fail.
func testAllowedOrigins(t *testing.T) []string {
	t.Helper()
	raw := os.Getenv("APP_ALLOWED_ORIGINS")
	if raw == "" {
		t.Fatalf("APP_ALLOWED_ORIGINS is required for httpapi tests; " +
			"set it in .env (or your shell) — e.g. " +
			"APP_ALLOWED_ORIGINS=http://localhost:5173,http://localhost:8080")
	}
	origins, err := config.ParseOrigins(raw)
	if err != nil {
		t.Fatalf("APP_ALLOWED_ORIGINS: %v", err)
	}
	return origins
}

func testDeps(t *testing.T) (httpapi.Deps, *pgxpool.Pool) {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set")
	}
	pool, err := db.Connect(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	t.Cleanup(pool.Close)

	// reqval DB rules — Postgres uses $1 placeholders.
	dbq := dbadapter.New(pool)
	t.Cleanup(func() { _ = dbq.Close() })
	reqval.RegisterDBWithPlaceholder(dbq, "$")

	cipher, err := crypto.NewCipher([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}

	return httpapi.Deps{
		Pool:           pool,
		Auth:           auth.NewManager("test-secret"),
		ChatCipher:     cipher,
		AllowedOrigins: testAllowedOrigins(t),
	}, pool
}
