package admin_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thomas666-beast/sindri/internal/admin"
	"github.com/thomas666-beast/sindri/internal/db/dbgen"
	"github.com/thomas666-beast/sindri/internal/user"
)

type fx struct {
	pool      *pgxpool.Pool
	adminSvc  *admin.Service
	adminID   string
	userID    string
}

func setup(t *testing.T) *fx {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil { t.Fatalf("connect: %v", err) }
	t.Cleanup(pool.Close)

	us := user.NewService(dbgen.New(pool))
	suffix := time.Now().UnixNano()

	a, err := us.Register(context.Background(), user.RegisterInput{
		Email:    fmt.Sprintf("adm_%d@example.com", suffix),
		Username: fmt.Sprintf("adm_%d", suffix%1_000_000),
		Password: "hunter2hunter2",
	})
	if err != nil { t.Fatalf("register admin: %v", err) }
	u, err := us.Register(context.Background(), user.RegisterInput{
		Email:    fmt.Sprintf("usr_%d@example.com", suffix),
		Username: fmt.Sprintf("usr_%d", (suffix+1)%1_000_000),
		Password: "hunter2hunter2",
	})
	if err != nil { t.Fatalf("register user: %v", err) }

	// Make 'a' an admin directly via SQL (like the CLI would)
	if _, err := pool.Exec(context.Background(),
		`UPDATE users SET is_admin = TRUE WHERE id = $1`, a.ID); err != nil {
		t.Fatalf("promote: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM warnings WHERE user_id IN ($1,$2)", a.ID, u.ID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM ticket_messages WHERE author_id IN ($1,$2)", a.ID, u.ID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM tickets WHERE user_id IN ($1,$2)", a.ID, u.ID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id IN ($1,$2)", a.ID, u.ID)
	})

	return &fx{
		pool: pool, adminSvc: admin.NewService(dbgen.New(pool), nil),
		adminID: a.ID, userID: u.ID,
	}
}

func TestDisableAndEnableUser(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	u, err := f.adminSvc.DisableUser(ctx, f.userID, "spam")
	if err != nil { t.Fatalf("disable: %v", err) }
	if u.IsActive { t.Error("expected is_active=false") }
	if u.DisableReason != "spam" { t.Errorf("reason = %q", u.DisableReason) }
	if u.DisabledAtUnix == nil { t.Error("disabled_at should be set") }

	u, err = f.adminSvc.EnableUser(ctx, f.userID)
	if err != nil { t.Fatalf("enable: %v", err) }
	if !u.IsActive { t.Error("expected is_active=true") }
	if u.DisableReason != "" { t.Errorf("reason should be cleared, got %q", u.DisableReason) }
}

func TestSetKYC(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	u, err := f.adminSvc.SetKYC(ctx, f.userID, "verified")
	if err != nil { t.Fatalf("set kyc: %v", err) }
	if u.KYCStatus != "verified" { t.Errorf("status = %q", u.KYCStatus) }
	if !u.KYCOverride { t.Error("kyc_override should be TRUE after admin action") }
}

func TestCreateWarningAndAcknowledge(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	w, err := f.adminSvc.CreateWarning(ctx, f.userID, f.adminID, "rude language", "warning")
	if err != nil { t.Fatalf("create warning: %v", err) }

	list, err := f.adminSvc.ListMyWarnings(ctx, f.userID, 50, 0)
	if err != nil { t.Fatalf("list: %v", err) }
	if len(list) != 1 { t.Fatalf("expected 1 warning, got %d", len(list)) }
	if list[0].AcknowledgedAtUnix != nil { t.Error("should not be acked yet") }

	if err := f.adminSvc.AcknowledgeWarning(ctx, f.userID, w.ID); err != nil {
		t.Fatalf("ack: %v", err)
	}
	list, _ = f.adminSvc.ListMyWarnings(ctx, f.userID, 50, 0)
	if list[0].AcknowledgedAtUnix == nil { t.Error("should be acked now") }
}

func TestTicketLifecycle(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	t1, err := f.adminSvc.CreateTicket(ctx, f.userID, "Cannot withdraw", "high")
	if err != nil { t.Fatalf("create: %v", err) }
	if t1.Status != "open" { t.Errorf("status = %q", t1.Status) }

	if _, err := f.adminSvc.AddMessage(ctx, t1.ID, f.userID, false, "I tried twice."); err != nil {
		t.Fatalf("user msg: %v", err)
	}
	if _, err := f.adminSvc.AddMessage(ctx, t1.ID, f.adminID, true, "Investigating."); err != nil {
		t.Fatalf("admin msg: %v", err)
	}

	msgs, err := f.adminSvc.ListMessages(ctx, t1.ID, f.adminID, true)
	if err != nil { t.Fatalf("list msgs: %v", err) }
	if len(msgs) != 2 { t.Fatalf("expected 2 msgs, got %d", len(msgs)) }
	if !msgs[1].IsStaff { t.Error("second msg should be staff") }

	t2, err := f.adminSvc.SetTicketStatus(ctx, t1.ID, "closed")
	if err != nil { t.Fatalf("close: %v", err) }
	if t2.Status != "closed" { t.Errorf("status = %q", t2.Status) }
}

func TestListUsers(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	_, total, err := f.adminSvc.ListUsers(ctx, "", "", 50, 0)
	if err != nil { t.Fatalf("list users: %v", err) }
	if total < 2 { t.Errorf("expected >=2 users, got %d", total) }
}

func TestListUsers_TotalRespectsFilters(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	// Any search that matches nothing must return total == 0.
	// Under the old code this returned the global user count.
	_, total, err := f.adminSvc.ListUsers(ctx, "zzz_no_such_user_zzz", "", 50, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 0 {
		t.Errorf("total for no-match search = %d, want 0", total)
	}

	// The two users created in setup() are both active.
	// Filtering by 'disabled' must not count them.
	_, total, err = f.adminSvc.ListUsers(ctx, "", "disabled", 50, 0)
	if err != nil {
		t.Fatalf("list disabled: %v", err)
	}
	// total can only be >= 0; the strong assertion is that it does NOT
	// equal the unfiltered total.
	_, globalTotal, err := f.adminSvc.ListUsers(ctx, "", "", 50, 0)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if total == globalTotal && globalTotal > 0 {
		t.Errorf("filtered total=%d equals global total=%d — filter ignored", total, globalTotal)
	}
}
