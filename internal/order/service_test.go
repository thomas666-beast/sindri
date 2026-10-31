package order_test

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
	"github.com/thomas666-beast/sindri/internal/order"
	"github.com/thomas666-beast/sindri/internal/product"
	"github.com/thomas666-beast/sindri/internal/settings"
	"github.com/thomas666-beast/sindri/internal/user"
	"github.com/thomas666-beast/sindri/internal/wallet"
	"github.com/thomas666-beast/sindri/internal/crypto"
)

type fx struct {
	pool      *pgxpool.Pool
	orderSvc  *order.Service
	userSvc   *user.Service
	walletSvc *wallet.Service
	productSvc *product.Service
	sellerID  string
	buyerID   string
	adminID   string
	productID string
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
	ps := product.NewService(dbgen.New(pool))
	ws := wallet.NewService(pool)
	ss := settings.NewService(dbgen.New(pool))
	cipher, err := crypto.NewCipher([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	os_ := order.NewService(pool, ss, nil, cipher, t.TempDir())

	suffix := time.Now().UnixNano()
	seller, err := us.Register(context.Background(), user.RegisterInput{
		Email:    fmt.Sprintf("oseller_%d@example.com", suffix),
		Username: fmt.Sprintf("oseller_%d", suffix%1_000_000),
		Password: "hunter2hunter2",
	})
	if err != nil {
		t.Fatalf("register seller: %v", err)
	}
	buyer, err := us.Register(context.Background(), user.RegisterInput{
		Email:    fmt.Sprintf("obuyer_%d@example.com", suffix),
		Username: fmt.Sprintf("obuyer_%d", suffix%1_000_000),
		Password: "hunter2hunter2",
	})
	if err != nil {
		t.Fatalf("register buyer: %v", err)
	}
	admin, err := us.Register(context.Background(), user.RegisterInput{
		Email:    fmt.Sprintf("oadmin_%d@example.com", suffix),
		Username: fmt.Sprintf("oadmin_%d", (suffix+1)%1_000_000),
		Password: "hunter2hunter2",
	})
	if err != nil {
		t.Fatalf("register admin: %v", err)
	}
	if _, err := pool.Exec(context.Background(),
		`UPDATE users SET is_admin = TRUE WHERE id = $1`, admin.ID); err != nil {
		t.Fatalf("promote admin: %v", err)
	}

	p, err := ps.Create(context.Background(), product.CreateInput{
		SellerID: seller.ID, Title: "Test widget", Price: 10000, CurrencyCode: "USD",
	})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM wallet_transactions WHERE user_id IN ($1,$2,$3)", seller.ID, buyer.ID, admin.ID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM orders WHERE buyer_id=$1 OR seller_id=$1", buyer.ID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM wallets WHERE user_id IN ($1,$2,$3)", seller.ID, buyer.ID, admin.ID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM products WHERE seller_id=$1", seller.ID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id IN ($1,$2,$3)", seller.ID, buyer.ID, admin.ID)
	})

	return &fx{
		pool: pool, orderSvc: os_, userSvc: us, walletSvc: ws, productSvc: ps,
		sellerID: seller.ID, buyerID: buyer.ID, adminID: admin.ID, productID: p.ID,
	}
}

func TestCreateOrder_HoldsFunds(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if _, err := f.walletSvc.Deposit(ctx, f.buyerID, "USD", 20000, ""); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	o, err := f.orderSvc.CreateOrder(ctx, order.CreateInput{
		BuyerID: f.buyerID, ProductID: f.productID,
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if o.Status != "held" {
		t.Errorf("status = %q, want held", o.Status)
	}
	if o.Amount != 10000 {
		t.Errorf("amount = %d", o.Amount)
	}

	balances, _ := f.walletSvc.GetBalances(ctx, f.buyerID)
	var avail, held int64
	for _, b := range balances {
		if b.CurrencyCode == "USD" {
			avail, held = b.Available, b.Held
		}
	}
	if avail != 10000 || held != 10000 {
		t.Errorf("buyer USD wallet = avail %d held %d, want 10000/10000", avail, held)
	}
}

func TestCreateOrder_SelfDealingRejected(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if _, err := f.walletSvc.Deposit(ctx, f.sellerID, "USD", 20000, ""); err != nil {
		t.Fatalf("deposit seller: %v", err)
	}
	_, err := f.orderSvc.CreateOrder(ctx, order.CreateInput{
		BuyerID: f.sellerID, ProductID: f.productID,
	})
	if err != order.ErrSelfDealing {
		t.Fatalf("err = %v, want ErrSelfDealing", err)
	}
}

func TestCreateOrder_InsufficientFunds(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	_, err := f.orderSvc.CreateOrder(ctx, order.CreateInput{
		BuyerID: f.buyerID, ProductID: f.productID,
	})
	if err != order.ErrInsufficientFunds {
		t.Fatalf("err = %v, want ErrInsufficientFunds", err)
	}
}

func TestConfirm_ReleasesToSellerAndCommission(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if _, err := f.walletSvc.Deposit(ctx, f.buyerID, "USD", 20000, ""); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	o, err := f.orderSvc.CreateOrder(ctx, order.CreateInput{
		BuyerID: f.buyerID, ProductID: f.productID,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	confirmed, err := f.orderSvc.Confirm(ctx, o.ID, f.buyerID)
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if confirmed.Status != "completed" {
		t.Errorf("status = %q, want completed", confirmed.Status)
	}

	sellerBal, _ := f.walletSvc.GetBalances(ctx, f.sellerID)
	var sellerAvail int64
	for _, b := range sellerBal {
		if b.CurrencyCode == "USD" {
			sellerAvail = b.Available
		}
	}
	if sellerAvail != 9500 {
		t.Errorf("seller available = %d, want 9500", sellerAvail)
	}

	buyerBal, _ := f.walletSvc.GetBalances(ctx, f.buyerID)
	var buyerAvail, buyerHeld int64
	for _, b := range buyerBal {
		if b.CurrencyCode == "USD" {
			buyerAvail, buyerHeld = b.Available, b.Held
		}
	}
	if buyerAvail != 10000 || buyerHeld != 0 {
		t.Errorf("buyer USD wallet = avail %d held %d, want 10000/0", buyerAvail, buyerHeld)
	}
}

func TestConfirm_ConcurrentIdempotent(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if _, err := f.walletSvc.Deposit(ctx, f.buyerID, "USD", 20000, ""); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	o, err := f.orderSvc.CreateOrder(ctx, order.CreateInput{
		BuyerID: f.buyerID, ProductID: f.productID,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	const n = 20
	var wg sync.WaitGroup
	var successes int32
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			if _, err := f.orderSvc.Confirm(ctx, o.ID, f.buyerID); err == nil {
				atomic.AddInt32(&successes, 1)
			}
		}()
	}
	wg.Wait()

	var sellerAvail int64
	balances, _ := f.walletSvc.GetBalances(ctx, f.sellerID)
	for _, b := range balances {
		if b.CurrencyCode == "USD" {
			sellerAvail = b.Available
		}
	}
	if sellerAvail != 9500 {
		t.Fatalf("seller available = %d, want 9500 (released exactly once)", sellerAvail)
	}

	var count int
	_ = f.pool.QueryRow(ctx,
		`SELECT count(*) FROM wallet_transactions WHERE user_id=$1 AND type='escrow_release'`,
		f.sellerID).Scan(&count)
	if count != 1 {
		t.Errorf("seller escrow_release ledger rows = %d, want 1", count)
	}
}

func TestConfirm_WrongBuyerForbidden(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if _, err := f.walletSvc.Deposit(ctx, f.buyerID, "USD", 20000, ""); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	o, err := f.orderSvc.CreateOrder(ctx, order.CreateInput{
		BuyerID: f.buyerID, ProductID: f.productID,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err = f.orderSvc.Confirm(ctx, o.ID, f.sellerID)
	if err != order.ErrForbidden {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

func TestRefund_ReturnsBuyerFunds(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if _, err := f.walletSvc.Deposit(ctx, f.buyerID, "USD", 20000, ""); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	o, err := f.orderSvc.CreateOrder(ctx, order.CreateInput{
		BuyerID: f.buyerID, ProductID: f.productID,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	refunded, err := f.orderSvc.Refund(ctx, o.ID, f.adminID)
	if err != nil {
		t.Fatalf("refund: %v", err)
	}
	if refunded.Status != "refunded" {
		t.Errorf("status = %q", refunded.Status)
	}

	buyerBal, _ := f.walletSvc.GetBalances(ctx, f.buyerID)
	var avail, held int64
	for _, b := range buyerBal {
		if b.CurrencyCode == "USD" {
			avail, held = b.Available, b.Held
		}
	}
	if avail != 20000 || held != 0 {
		t.Errorf("buyer USD wallet = avail %d held %d, want 20000/0", avail, held)
	}
}

func TestAutoRelease_ReleasesStaleOrders(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if _, err := f.walletSvc.Deposit(ctx, f.buyerID, "USD", 20000, ""); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	o, err := f.orderSvc.CreateOrder(ctx, order.CreateInput{
		BuyerID: f.buyerID, ProductID: f.productID,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := f.pool.Exec(ctx,
		`UPDATE orders SET auto_release_at = now() - interval '1 minute' WHERE id=$1`, o.ID); err != nil {
		t.Fatalf("force stale: %v", err)
	}

	if _, err := f.orderSvc.AutoRelease(ctx, o.ID); err != nil {
		t.Fatalf("auto release: %v", err)
	}

	var status string
	_ = f.pool.QueryRow(ctx, `SELECT status FROM orders WHERE id=$1`, o.ID).Scan(&status)
	if status != "completed" {
		t.Errorf("status = %q, want completed", status)
	}
}
