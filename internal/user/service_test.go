package user_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thomas666-beast/sindri/internal/db/dbgen"
	"github.com/thomas666-beast/sindri/internal/user"
)

func testSvc(t *testing.T) (*user.Service, *pgxpool.Pool) {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return user.NewService(dbgen.New(pool)), pool
}

func uniqueEmail(prefix string) string {
	return fmt.Sprintf("%s-%d@example.com", prefix, timeNowNano())
}

func uniqueUsername(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, timeNowNano()%1_000_000)
}

func TestRegister_Success(t *testing.T) {
	svc, pool := testSvc(t)
	ctx := context.Background()

	email := uniqueEmail("reg_ok")
	username := uniqueUsername("reg_ok")
	u, err := svc.Register(ctx, user.RegisterInput{
		Email: email, Username: username, Password: "hunter2hunter2",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id=$1", u.ID) })

	if u.Email != email {
		t.Errorf("email = %q, want %q", u.Email, email)
	}
	if u.PreferredCurrency != "USD" {
		t.Errorf("currency = %q, want USD", u.PreferredCurrency)
	}
	if u.IsAdmin {
		t.Error("new user should not be admin")
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	svc, pool := testSvc(t)
	ctx := context.Background()

	email := uniqueEmail("dup")
	username := uniqueUsername("dup1")
	u, err := svc.Register(ctx, user.RegisterInput{
		Email: email, Username: username, Password: "hunter2hunter2",
	})
	if err != nil {
		t.Fatalf("first register: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id=$1", u.ID) })

	_, err = svc.Register(ctx, user.RegisterInput{
		Email: email, Username: uniqueUsername("dup2"), Password: "hunter2hunter2",
	})
	if !errors.Is(err, user.ErrEmailTaken) {
		t.Fatalf("err = %v, want ErrEmailTaken", err)
	}
}

func TestRegister_WeakPassword(t *testing.T) {
	svc, _ := testSvc(t)
	_, err := svc.Register(context.Background(), user.RegisterInput{
		Email: uniqueEmail("weak"), Username: uniqueUsername("weak"), Password: "short",
	})
	if !errors.Is(err, user.ErrWeakPassword) {
		t.Fatalf("err = %v, want ErrWeakPassword", err)
	}
}

func TestAuthenticate_Success(t *testing.T) {
	svc, pool := testSvc(t)
	ctx := context.Background()

	email := uniqueEmail("auth")
	u, err := svc.Register(ctx, user.RegisterInput{
		Email: email, Username: uniqueUsername("auth"), Password: "hunter2hunter2",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id=$1", u.ID) })

	got, err := svc.Authenticate(ctx, email, "hunter2hunter2")
	if err != nil {
		t.Fatalf("auth: %v", err)
	}
	if got.ID != u.ID {
		t.Errorf("id = %q, want %q", got.ID, u.ID)
	}
}

func TestAuthenticate_WrongPassword(t *testing.T) {
	svc, pool := testSvc(t)
	ctx := context.Background()

	email := uniqueEmail("wrongpw")
	u, err := svc.Register(ctx, user.RegisterInput{
		Email: email, Username: uniqueUsername("wrongpw"), Password: "hunter2hunter2",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id=$1", u.ID) })

	_, err = svc.Authenticate(ctx, email, "wrongpassword")
	if !errors.Is(err, user.ErrInvalidCreds) {
		t.Fatalf("err = %v, want ErrInvalidCreds", err)
	}
}

func TestAuthenticate_UnknownEmail(t *testing.T) {
	svc, _ := testSvc(t)
	_, err := svc.Authenticate(context.Background(), "nobody-here@example.com", "whatever")
	if !errors.Is(err, user.ErrInvalidCreds) {
		t.Fatalf("err = %v, want ErrInvalidCreds", err)
	}
}
