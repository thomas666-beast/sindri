package withdrawal_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thomas666-beast/sindri/internal/db/dbgen"
	"github.com/thomas666-beast/sindri/internal/user"
	"github.com/thomas666-beast/sindri/internal/wallet"
	"github.com/thomas666-beast/sindri/internal/withdrawal"
)

type fx struct {
	svc     *withdrawal.Service
	walsvc  *wallet.Service
	pool    *pgxpool.Pool
	userID  string
	adminID string
}

func setup(t *testing.T) *fx {
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

	us := user.NewService(dbgen.New(pool))
	suffix := time.Now().UnixNano() % 1_000_000_000

	// Regular user
	u, err := us.Register(context.Background(), user.RegisterInput{
		Email:    fmt.Sprintf("wd_%d@example.com", suffix),
		Username: fmt.Sprintf("wd_%d", suffix%1_000_000),
		Password: "hunter2hunter2",
	})
	if err != nil {
		t.Fatalf("register user: %v", err)
	}

	// Admin user — needed because reviewed_by FK references users(id)
	admin, err := us.Register(context.Background(), user.RegisterInput{
		Email:    fmt.Sprintf("wda_%d@example.com", suffix),
		Username: fmt.Sprintf("wda_%d", (suffix+1)%1_000_000),
		Password: "hunter2hunter2",
	})
	if err != nil {
		t.Fatalf("register admin: %v", err)
	}
	// Promote to admin (not strictly required for the FK, but semantically correct)
	if _, err := pool.Exec(context.Background(),
		`UPDATE users SET is_admin = TRUE WHERE id = $1`, admin.ID); err != nil {
		t.Fatalf("promote admin: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			"DELETE FROM wallet_transactions WHERE user_id IN ($1,$2)", u.ID, admin.ID)
		_, _ = pool.Exec(context.Background(),
			"DELETE FROM withdrawal_requests WHERE user_id IN ($1,$2)", u.ID, admin.ID)
		_, _ = pool.Exec(context.Background(),
			"DELETE FROM wallets WHERE user_id IN ($1,$2)", u.ID, admin.ID)
		_, _ = pool.Exec(context.Background(),
			"DELETE FROM users WHERE id IN ($1,$2)", u.ID, admin.ID)
	})

	return &fx{
		svc:     withdrawal.NewService(pool, nil),
		walsvc:  wallet.NewService(pool),
		pool:    pool,
		userID:  u.ID,
		adminID: admin.ID,
	}
}

func TestRequest_HoldsFunds(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	if _, err := f.walsvc.Deposit(ctx, f.userID, "USD", 10000, ""); err != nil {
		t.Fatalf("deposit: %v", err)
	}

	req, err := f.svc.RequestWithdrawal(ctx, f.userID, "USD", 3000)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if req.Status != "pending" {
		t.Errorf("status = %q, want pending", req.Status)
	}

	balances, _ := f.walsvc.GetBalances(ctx, f.userID)
	var avail, held int64
	for _, b := range balances {
		if b.CurrencyCode == "USD" {
			avail, held = b.Available, b.Held
		}
	}
	if avail != 7000 || held != 3000 {
		t.Errorf("wallet = avail %d held %d, want 7000/3000", avail, held)
	}
}

func TestRequest_InsufficientFunds(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	if _, err := f.walsvc.Deposit(ctx, f.userID, "USD", 1000, ""); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	_, err := f.svc.RequestWithdrawal(ctx, f.userID, "USD", 5000)
	if !errors.Is(err, withdrawal.ErrInsufficientFunds) {
		t.Fatalf("err = %v, want ErrInsufficientFunds", err)
	}
}

func TestAdminApprove_RemovesHeld(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	if _, err := f.walsvc.Deposit(ctx, f.userID, "USD", 10000, ""); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	req, err := f.svc.RequestWithdrawal(ctx, f.userID, "USD", 3000)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	approved, err := f.svc.AdminApprove(ctx, req.ID, f.adminID)
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if approved.Status != "approved" {
		t.Errorf("status = %q", approved.Status)
	}

	balances, _ := f.walsvc.GetBalances(ctx, f.userID)
	var avail, held int64
	for _, b := range balances {
		if b.CurrencyCode == "USD" {
			avail, held = b.Available, b.Held
		}
	}
	if avail != 7000 || held != 0 {
		t.Errorf("wallet = avail %d held %d, want 7000/0", avail, held)
	}
}

func TestAdminReject_ReturnsFunds(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	if _, err := f.walsvc.Deposit(ctx, f.userID, "USD", 10000, ""); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	req, err := f.svc.RequestWithdrawal(ctx, f.userID, "USD", 3000)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	rejected, err := f.svc.AdminReject(ctx, req.ID, f.adminID, "invalid bank account")
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if rejected.Status != "rejected" {
		t.Errorf("status = %q", rejected.Status)
	}
	if rejected.RejectionReason != "invalid bank account" {
		t.Errorf("reason = %q", rejected.RejectionReason)
	}

	balances, _ := f.walsvc.GetBalances(ctx, f.userID)
	var avail, held int64
	for _, b := range balances {
		if b.CurrencyCode == "USD" {
			avail, held = b.Available, b.Held
		}
	}
	if avail != 10000 || held != 0 {
		t.Errorf("wallet = avail %d held %d, want 10000/0", avail, held)
	}
}

func TestAdminApprove_TwiceRejected(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	if _, err := f.walsvc.Deposit(ctx, f.userID, "USD", 10000, ""); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	req, _ := f.svc.RequestWithdrawal(ctx, f.userID, "USD", 3000)
	if _, err := f.svc.AdminApprove(ctx, req.ID, f.adminID); err != nil {
		t.Fatalf("first approve: %v", err)
	}

	_, err := f.svc.AdminApprove(ctx, req.ID, f.adminID)
	if !errors.Is(err, withdrawal.ErrAlreadyReviewed) {
		t.Fatalf("err = %v, want ErrAlreadyReviewed", err)
	}
}

func TestAdminReject_ReasonRequired(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	if _, err := f.walsvc.Deposit(ctx, f.userID, "USD", 10000, ""); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	req, _ := f.svc.RequestWithdrawal(ctx, f.userID, "USD", 3000)

	_, err := f.svc.AdminReject(ctx, req.ID, f.adminID, "no")
	if !errors.Is(err, withdrawal.ErrReasonRequired) {
		t.Fatalf("err = %v, want ErrReasonRequired", err)
	}
}

func TestListMine(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	if _, err := f.walsvc.Deposit(ctx, f.userID, "USD", 10000, ""); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	f.svc.RequestWithdrawal(ctx, f.userID, "USD", 1000)
	f.svc.RequestWithdrawal(ctx, f.userID, "USD", 2000)

	list, err := f.svc.ListMine(ctx, f.userID, 50, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("got %d withdrawals, want 2", len(list))
	}
}

func TestAdminListPending(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	if _, err := f.walsvc.Deposit(ctx, f.userID, "USD", 10000, ""); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	req1, _ := f.svc.RequestWithdrawal(ctx, f.userID, "USD", 1000)
	req2, _ := f.svc.RequestWithdrawal(ctx, f.userID, "USD", 2000)

	if _, err := f.svc.AdminApprove(ctx, req1.ID, f.adminID); err != nil {
		t.Fatalf("approve: %v", err)
	}

	pending, err := f.svc.AdminList(ctx, "pending", "", 100, 0)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	found := false
	for _, p := range pending {
		if p.ID == req2.ID {
			found = true
		}
		if p.ID == req1.ID {
			t.Error("approved withdrawal appeared in pending list")
		}
	}
	if !found {
		t.Error("pending withdrawal not found")
	}
}

func TestLedgerEntries(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	if _, err := f.walsvc.Deposit(ctx, f.userID, "USD", 10000, ""); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	req, _ := f.svc.RequestWithdrawal(ctx, f.userID, "USD", 3000)

	var count int
	if err := f.pool.QueryRow(ctx,
		`SELECT count(*) FROM wallet_transactions WHERE user_id=$1 AND type='withdraw'`, f.userID).
		Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("withdraw txn count = %d, want 1 (the hold)", count)
	}

	if _, err := f.svc.AdminApprove(ctx, req.ID, f.adminID); err != nil {
		t.Fatalf("approve: %v", err)
	}

	_ = f.pool.QueryRow(ctx,
		`SELECT count(*) FROM wallet_transactions WHERE user_id=$1 AND type='withdraw'`, f.userID).
		Scan(&count)
	if count != 2 {
		t.Errorf("withdraw txn count = %d, want 2", count)
	}
}
