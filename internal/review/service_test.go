package review_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thomas666-beast/sindri/internal/db/dbgen"
	"github.com/thomas666-beast/sindri/internal/order"
	"github.com/thomas666-beast/sindri/internal/product"
	"github.com/thomas666-beast/sindri/internal/review"
	"github.com/thomas666-beast/sindri/internal/settings"
	"github.com/thomas666-beast/sindri/internal/user"
	"github.com/thomas666-beast/sindri/internal/wallet"
	"github.com/thomas666-beast/sindri/internal/crypto"
)

type fx struct {
	reviewSvc  *review.Service
	orderSvc   *order.Service
	walletSvc  *wallet.Service
	productSvc *product.Service
	pool       *pgxpool.Pool
	sellerID   string
	buyerID    string
	productID  string
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
	rs := review.NewService(dbgen.New(pool))

	suffix := time.Now().UnixNano() % 1_000_000_000
	seller, err := us.Register(context.Background(), user.RegisterInput{
		Email: fmt.Sprintf("rev_s_%d@example.com", suffix),
		Username: fmt.Sprintf("revs_%d", suffix%1_000_000),
		Password: "hunter2hunter2",
	})
	if err != nil { t.Fatalf("register seller: %v", err) }
	buyer, err := us.Register(context.Background(), user.RegisterInput{
		Email: fmt.Sprintf("rev_b_%d@example.com", suffix),
		Username: fmt.Sprintf("revb_%d", (suffix+1)%1_000_000),
		Password: "hunter2hunter2",
	})
	if err != nil { t.Fatalf("register buyer: %v", err) }

	p, err := ps.Create(context.Background(), product.CreateInput{
		SellerID: seller.ID, Title: "Review test product", Price: 5000, CurrencyCode: "USD",
	})
	if err != nil { t.Fatalf("create product: %v", err) }

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM reviews WHERE buyer_id IN ($1,$2)", seller.ID, buyer.ID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM wallet_transactions WHERE user_id IN ($1,$2)", seller.ID, buyer.ID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM orders WHERE buyer_id IN ($1,$2) OR seller_id IN ($1,$2)", seller.ID, buyer.ID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM wallets WHERE user_id IN ($1,$2)", seller.ID, buyer.ID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM products WHERE seller_id=$1", seller.ID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id IN ($1,$2)", seller.ID, buyer.ID)
	})

	return &fx{
		reviewSvc: rs, orderSvc: os_, walletSvc: ws, productSvc: ps,
		pool: pool, sellerID: seller.ID, buyerID: buyer.ID, productID: p.ID,
	}
}

// helper: place order, deposit, confirm. Returns order ID.
func placeAndConfirmOrder(t *testing.T, f *fx) string {
	t.Helper()
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
	if _, err := f.orderSvc.Confirm(ctx, o.ID, f.buyerID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	return o.ID
}

func TestCreate_AfterCompleted(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	orderID := placeAndConfirmOrder(t, f)

	rv, err := f.reviewSvc.Create(ctx, review.CreateInput{
		OrderID: orderID, BuyerID: f.buyerID, Rating: 5, Comment: "great",
	})
	if err != nil {
		t.Fatalf("create review: %v", err)
	}
	if rv.Rating != 5 {
		t.Errorf("rating = %d", rv.Rating)
	}
	if rv.Comment != "great" {
		t.Errorf("comment = %q", rv.Comment)
	}
	if rv.SellerID != f.sellerID {
		t.Errorf("seller_id = %q, want %q", rv.SellerID, f.sellerID)
	}
	if rv.ProductID != f.productID {
		t.Errorf("product_id = %q, want %q", rv.ProductID, f.productID)
	}
}

func TestCreate_OrderNotCompleted(t *testing.T) {
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
	// Order is 'held', not completed.
	_, err = f.reviewSvc.Create(ctx, review.CreateInput{
		OrderID: o.ID, BuyerID: f.buyerID, Rating: 5,
	})
	if !errors.Is(err, review.ErrOrderNotCompleted) {
		t.Fatalf("err = %v, want ErrOrderNotCompleted", err)
	}
}

func TestCreate_NotBuyer(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	orderID := placeAndConfirmOrder(t, f)

	_, err := f.reviewSvc.Create(ctx, review.CreateInput{
		OrderID: orderID, BuyerID: f.sellerID, Rating: 5,
	})
	if !errors.Is(err, review.ErrNotBuyer) {
		t.Fatalf("err = %v, want ErrNotBuyer", err)
	}
}

func TestCreate_AlreadyReviewed(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	orderID := placeAndConfirmOrder(t, f)

	if _, err := f.reviewSvc.Create(ctx, review.CreateInput{
		OrderID: orderID, BuyerID: f.buyerID, Rating: 5,
	}); err != nil {
		t.Fatalf("first review: %v", err)
	}
	_, err := f.reviewSvc.Create(ctx, review.CreateInput{
		OrderID: orderID, BuyerID: f.buyerID, Rating: 3,
	})
	if !errors.Is(err, review.ErrAlreadyReviewed) {
		t.Fatalf("err = %v, want ErrAlreadyReviewed", err)
	}
}

func TestCreate_InvalidRating(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	orderID := placeAndConfirmOrder(t, f)

	for _, r := range []int16{0, 6, -1} {
		_, err := f.reviewSvc.Create(ctx, review.CreateInput{
			OrderID: orderID, BuyerID: f.buyerID, Rating: r,
		})
		if !errors.Is(err, review.ErrInvalidRating) {
			t.Errorf("rating %d: err = %v, want ErrInvalidRating", r, err)
		}
	}
}

func TestListBySeller_AndSummary(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	orderID := placeAndConfirmOrder(t, f)
	if _, err := f.reviewSvc.Create(ctx, review.CreateInput{
		OrderID: orderID, BuyerID: f.buyerID, Rating: 4, Comment: "ok",
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	page, err := f.reviewSvc.ListBySeller(ctx, f.sellerID, 20, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page.Reviews) != 1 {
		t.Fatalf("got %d reviews, want 1", len(page.Reviews))
	}
	if page.Reviews[0].BuyerUsername == "" {
		t.Error("buyer_username should be populated in list")
	}

	sum, err := f.reviewSvc.SellerSummary(ctx, f.sellerID)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if sum.ReviewCount != 1 {
		t.Errorf("count = %d, want 1", sum.ReviewCount)
	}
	if sum.AvgRating != 4.0 {
		t.Errorf("avg = %v, want 4.0", sum.AvgRating)
	}
}

func TestSummary_NoReviews(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	sum, err := f.reviewSvc.SellerSummary(ctx, f.sellerID)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if sum.ReviewCount != 0 {
		t.Errorf("count = %d, want 0", sum.ReviewCount)
	}
	if sum.AvgRating != 0 {
		t.Errorf("avg = %v, want 0", sum.AvgRating)
	}
}
