package httpapi_test

import (
	"os"
	"testing"

	"github.com/joho/godotenv"
)

func TestMain(m *testing.M) {
	// Load .env if present. The Makefile already exports DATABASE_URL,
	// but this makes `go test ./...` work from any shell.
	_ = godotenv.Load("../../.env")

	if os.Getenv("DATABASE_URL") == "" && os.Getenv("CI") != "" {
		panic("DATABASE_URL is required in CI")
	}
	os.Exit(m.Run())
}
