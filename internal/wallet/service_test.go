package wallet_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thomas666-beast/sindri/internal/db/dbgen"
	"github.com/thomas666-beast/sindri/internal/user"
	"github.com/thomas666-beast/sindri/internal/wallet"
)

func setup(t *testing.T) (*wallet.Service, *pgxpool.Pool, string) {
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

	// Create a dedicated user for this test
	us := user.NewService(dbgen.New(pool))
	u, err := us.Register(context.Background(), user.RegisterInput{
		Email:    fmt.Sprintf("wtest_%d@example.com", time.Now().UnixNano()),
		Username: fmt.Sprintf("wtest_%d", time.Now().UnixNano()%1_000_000),
		Password: "hunter2hunter2",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM wallet_transactions WHERE user_id=$1", u.ID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM wallets WHERE user_id=$1", u.ID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id=$1", u.ID)
	})

	return wallet.NewService(pool), pool, u.ID
}

func TestDeposit_CreatesWalletAndCredits(t *testing.T) {
	w, _, userID := setup(t)
	ctx := context.Background()

	bal, err := w.Deposit(ctx, userID, "USD", 5000, "")
	if err != nil {
		t.Fatalf("deposit: %v", err)
	}
	if bal.Available != 5000 {
		t.Errorf("available = %d, want 5000", bal.Available)
	}
	if bal.Held != 0 {
		t.Errorf("held = %d, want 0", bal.Held)
	}

	// Second deposit accumulates
	bal, err = w.Deposit(ctx, userID, "USD", 2500, "")
	if err != nil {
		t.Fatalf("deposit 2: %v", err)
	}
	if bal.Available != 7500 {
		t.Errorf("available = %d, want 7500", bal.Available)
	}
}

func TestDeposit_MultiCurrencyIndependent(t *testing.T) {
	w, _, userID := setup(t)
	ctx := context.Background()

	if _, err := w.Deposit(ctx, userID, "USD", 5000, ""); err != nil {
		t.Fatalf("deposit USD: %v", err)
	}
	if _, err := w.Deposit(ctx, userID, "BRL", 10000, ""); err != nil {
		t.Fatalf("deposit BRL: %v", err)
	}

	list, err := w.GetBalances(ctx, userID)
	if err != nil {
		t.Fatalf("get balances: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 wallets, got %d", len(list))
	}
	byCur := map[string]int64{}
	for _, b := range list {
		byCur[b.CurrencyCode] = b.Available
	}
	if byCur["USD"] != 5000 || byCur["BRL"] != 10000 {
		t.Errorf("unexpected balances: %+v", byCur)
	}
}

func TestDeposit_IdempotencyReturnsOneLedgerRow(t *testing.T) {
	w, pool, userID := setup(t)
	ctx := context.Background()

	key := "idem-" + fmt.Sprint(time.Now().UnixNano())
	if _, err := w.Deposit(ctx, userID, "USD", 3000, key); err != nil {
		t.Fatalf("deposit 1: %v", err)
	}
	if _, err := w.Deposit(ctx, userID, "USD", 3000, key); err != nil {
		t.Fatalf("deposit 2: %v", err)
	}

	var count int
	err := pool.QueryRow(ctx,
		`SELECT count(*) FROM wallet_transactions WHERE idempotency_key=$1`, key).Scan(&count)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("ledger rows for idem key = %d, want 1", count)
	}

	var available int64
	err = pool.QueryRow(ctx,
		`SELECT available FROM wallets WHERE user_id=$1 AND currency_code='USD'`, userID).Scan(&available)
	if err != nil {
		t.Fatalf("read balance: %v", err)
	}
	if available != 3000 {
		t.Fatalf("available = %d, want 3000 (idempotent — only one deposit applied)", available)
	}
}

func TestWithdraw_RejectsWhenInsufficient(t *testing.T) {
	w, _, userID := setup(t)
	ctx := context.Background()

	if _, err := w.Deposit(ctx, userID, "USD", 1000, ""); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	_, err := w.Withdraw(ctx, userID, "USD", 2000, "")
	if err != wallet.ErrInsufficientFunds {
		t.Fatalf("err = %v, want ErrInsufficientFunds", err)
	}
}

func TestWithdraw_ConcurrentNoOverdraft(t *testing.T) {
	w, pool, userID := setup(t)
	ctx := context.Background()

	if _, err := w.Deposit(ctx, userID, "USD", 1000, ""); err != nil {
		t.Fatalf("seed deposit: %v", err)
	}

	const attempts = 50
	var successes, failures int32
	var wg sync.WaitGroup
	wg.Add(attempts)
	for i := 0; i < attempts; i++ {
		go func() {
			defer wg.Done()
			_, err := w.Withdraw(ctx, userID, "USD", 100, "")
			if err == nil {
				atomic.AddInt32(&successes, 1)
			} else {
				atomic.AddInt32(&failures, 1)
			}
		}()
	}
	wg.Wait()

	if successes != 10 {
		t.Errorf("successes = %d, want 10 (1000 / 100)", successes)
	}
	if failures != attempts-10 {
		t.Errorf("failures = %d, want %d", failures, attempts-10)
	}

	var available int64
	if err := pool.QueryRow(ctx,
		`SELECT available FROM wallets WHERE user_id=$1 AND currency_code='USD'`, userID).
		Scan(&available); err != nil {
		t.Fatalf("read: %v", err)
	}
	if available != 0 {
		t.Errorf("final available = %d, want 0", available)
	}

	var count int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM wallet_transactions WHERE user_id=$1 AND type='withdraw'`, userID).
		Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 10 {
		t.Errorf("withdraw ledger rows = %d, want 10", count)
	}
}

func TestDeposit_RejectsNonPositiveAmount(t *testing.T) {
	w, _, userID := setup(t)
	ctx := context.Background()

	for _, amt := range []int64{0, -1, -1000} {
		_, err := w.Deposit(ctx, userID, "USD", amt, "")
		if err != wallet.ErrInvalidAmount {
			t.Errorf("amount=%d: err = %v, want ErrInvalidAmount", amt, err)
		}
	}
}
