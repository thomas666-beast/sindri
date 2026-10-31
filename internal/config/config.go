package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv            string
	HTTPPort          int
	DatabaseURL       string
	JWTSecret         string
	ChatEncryptionKey []byte
	CookieSecure      bool
	UploadsDir        string
	AllowedOrigins    []string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	appEnv := getEnv("APP_ENV", "development")

	cookieSecure := appEnv == "production"
	if v := os.Getenv("COOKIE_SECURE"); v != "" {
		cookieSecure = v == "true"
	}

	portStr := getEnv("HTTP_PORT", "8080")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid HTTP_PORT %q: %w", portStr, err)
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		if appEnv == "production" {
			return nil, fmt.Errorf("JWT_SECRET is required in production")
		}
		jwtSecret = "dev-only-insecure-secret-change-me"
	}

	rawKey := os.Getenv("CHAT_ENCRYPTION_KEY")
	if rawKey == "" || rawKey == "change-me-base64-32-bytes" {
		if appEnv == "production" {
			return nil, fmt.Errorf("CHAT_ENCRYPTION_KEY is required in production")
		}
		// Dev fallback: deterministic key so dev restarts don't lose messages
		rawKey = base64.StdEncoding.EncodeToString([]byte("dev-insecure-chat-key-32-bytes!!"))
	}

	keyBytes, err := base64.StdEncoding.DecodeString(rawKey)
	if err != nil {
		return nil, fmt.Errorf("CHAT_ENCRYPTION_KEY not base64: %w", err)
	}
	if len(keyBytes) != 32 {
		return nil, fmt.Errorf("CHAT_ENCRYPTION_KEY must be 32 bytes after base64 decode, got %d", len(keyBytes))
	}

	uploadsDir := getEnv("UPLOADS_DIR", "./uploads")

	// APP_ALLOWED_ORIGINS is required in every environment — no hidden
	// defaults. It's security-relevant (WebSocket origin allowlist) and
	// must be an explicit deployment decision. Comma-separated.
	allowedOrigins, err := ParseOrigins(os.Getenv("APP_ALLOWED_ORIGINS"))
	if err != nil {
		return nil, err
	}

	return &Config{
		AppEnv:            appEnv,
		HTTPPort:          port,
		DatabaseURL:       dbURL,
		JWTSecret:         jwtSecret,
		ChatEncryptionKey: keyBytes,
		CookieSecure:      cookieSecure,
		UploadsDir:        uploadsDir,
		AllowedOrigins:    allowedOrigins,
	}, nil
}

// ParseOrigins splits a comma-separated list of origins, trims whitespace,
// drops empty entries, and errors if the result is empty.
//
// There are no defaults — the caller must supply APP_ALLOWED_ORIGINS.
// This is exported so tests can exercise the same parsing rules the
// production loader uses.
//
// Examples:
//
//	"http://localhost:5173"                              → ["http://localhost:5173"]
//	"http://a, http://b"                                 → ["http://a", "http://b"]
//	"http://a,,http://b,"                                → ["http://a", "http://b"]
//	""                                                   → error
//	" , , "                                              → error
func ParseOrigins(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf(
			"APP_ALLOWED_ORIGINS is required (comma-separated list of origins, e.g. http://localhost:5173,http://localhost:8080)",
		)
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("APP_ALLOWED_ORIGINS is set but contains no valid origins")
	}
	return out, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
