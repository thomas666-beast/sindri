package db_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// testPool connects using DATABASE_URL. Tests that need a DB skip if unset,
// so `go test ./...` still passes on a machine without Postgres.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping DB-backed test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestSchema_TablesExist(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	expected := []string{"currencies", "users", "user_profiles"}
	for _, name := range expected {
		var exists bool
		err := pool.QueryRow(ctx,
			`SELECT EXISTS (
			   SELECT 1 FROM information_schema.tables
			   WHERE table_schema = 'public' AND table_name = $1
			 )`, name).Scan(&exists)
		if err != nil {
			t.Fatalf("query %s: %v", name, err)
		}
		if !exists {
			t.Errorf("table %q does not exist — run 'make migrate'", name)
		}
	}
}

func TestSchema_UsersFKToCurrencies(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	// Try inserting a user with a currency that does not exist.
	_, err := pool.Exec(ctx, `
		INSERT INTO users (email, password_hash, username, preferred_currency)
		VALUES ('fk-test@example.com', 'x', 'fk_test_user', 'ZZZ')
	`)
	if err == nil {
		// Clean up the accidental row before failing.
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE email='fk-test@example.com'`)
		t.Fatal("expected FK violation on preferred_currency='ZZZ', got nil")
	}
	t.Logf("FK enforced as expected: %v", err)
}

func TestSchema_UsersUniqueEmailAndUsername(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	// Insert a valid user.
	_, err := pool.Exec(ctx, `
		INSERT INTO users (email, password_hash, username, preferred_currency)
		VALUES ('unique-test@example.com', 'x', 'unique_test_user', 'USD')
	`)
	if err != nil {
		t.Fatalf("insert base user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM users WHERE username='unique_test_user'`)
	})

	// Same email, different username → should fail.
	_, err = pool.Exec(ctx, `
		INSERT INTO users (email, password_hash, username, preferred_currency)
		VALUES ('unique-test@example.com', 'x', 'unique_test_user_2', 'USD')
	`)
	if err == nil {
		t.Fatal("expected unique violation on email, got nil")
	}

	// Same username, different email → should fail.
	_, err = pool.Exec(ctx, `
		INSERT INTO users (email, password_hash, username, preferred_currency)
		VALUES ('unique-test-2@example.com', 'x', 'unique_test_user', 'USD')
	`)
	if err == nil {
		t.Fatal("expected unique violation on username, got nil")
	}
}

func TestSchema_CITEXTIsCaseInsensitive(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	_, err := pool.Exec(ctx, `
		INSERT INTO users (email, password_hash, username, preferred_currency)
		VALUES ('Case@Example.com', 'x', 'case_test_user', 'USD')
	`)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM users WHERE username='case_test_user'`)
	})

	// Insert with lowercase email — must violate UNIQUE because CITEXT.
	_, err = pool.Exec(ctx, `
		INSERT INTO users (email, password_hash, username, preferred_currency)
		VALUES ('case@example.com', 'x', 'case_test_user_2', 'USD')
	`)
	if err == nil {
		t.Fatal("expected CITEXT unique violation, got nil")
	}
}

func TestSchema_ProfileCascadeOnUserDelete(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	var userID string
	err := pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, username, preferred_currency)
		VALUES ('cascade-test@example.com', 'x', 'cascade_test_user', 'USD')
		RETURNING id::text
	`).Scan(&userID)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO user_profiles (user_id, display_name)
		VALUES ($1, 'Cascade Tester')
	`, userID)
	if err != nil {
		t.Fatalf("insert profile: %v", err)
	}

	// Delete the user — profile must vanish via ON DELETE CASCADE.
	_, err = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	if err != nil {
		t.Fatalf("delete user: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM user_profiles WHERE user_id = $1`, userID).
		Scan(&count); err != nil {
		t.Fatalf("count profiles: %v", err)
	}
	if count != 0 {
		t.Fatalf("profile was not cascaded; found %d rows", count)
	}
}
