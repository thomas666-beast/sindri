package order_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/thomas666-beast/sindri/internal/order"
)

func TestDispute_BuyerOnly(t *testing.T) {
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

	_, err = f.orderSvc.Dispute(ctx, o.ID, f.sellerID, "seller trying to dispute")
	if !errors.Is(err, order.ErrNotBuyerDispute) {
		t.Fatalf("err = %v, want ErrNotBuyerDispute", err)
	}
}

func TestDispute_OnlyWhileHeld(t *testing.T) {
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
	if _, err := f.orderSvc.Confirm(ctx, o.ID, f.buyerID); err != nil {
		t.Fatalf("confirm: %v", err)
	}

	_, err = f.orderSvc.Dispute(ctx, o.ID, f.buyerID, "too late")
	if !errors.Is(err, order.ErrCannotDispute) {
		t.Fatalf("err = %v, want ErrCannotDispute", err)
	}
}

func TestDispute_HappyPath(t *testing.T) {
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

	disputed, err := f.orderSvc.Dispute(ctx, o.ID, f.buyerID, "item not as described")
	if err != nil {
		t.Fatalf("dispute: %v", err)
	}
	if disputed.Status != "disputed" {
		t.Errorf("status = %q, want disputed", disputed.Status)
	}

	var reason string
	if err := f.pool.QueryRow(ctx, `SELECT reason FROM disputes WHERE order_id=$1`, o.ID).Scan(&reason); err != nil {
		t.Fatalf("read dispute: %v", err)
	}
	if reason != "item not as described" {
		t.Errorf("reason = %q", reason)
	}
}

func TestDispute_AlreadyDisputed(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if _, err := f.walletSvc.Deposit(ctx, f.buyerID, "USD", 20000, ""); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	o, _ := f.orderSvc.CreateOrder(ctx, order.CreateInput{
		BuyerID: f.buyerID, ProductID: f.productID,
	})
	if _, err := f.orderSvc.Dispute(ctx, o.ID, f.buyerID, "first"); err != nil {
		t.Fatalf("first dispute: %v", err)
	}
	_, err := f.orderSvc.Dispute(ctx, o.ID, f.buyerID, "second")
	if !errors.Is(err, order.ErrAlreadyDisputed) {
		t.Fatalf("err = %v, want ErrAlreadyDisputed", err)
	}
}

func TestDispute_AutoReleaseSkips(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if _, err := f.walletSvc.Deposit(ctx, f.buyerID, "USD", 20000, ""); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	o, _ := f.orderSvc.CreateOrder(ctx, order.CreateInput{
		BuyerID: f.buyerID, ProductID: f.productID,
	})
	if _, err := f.orderSvc.Dispute(ctx, o.ID, f.buyerID, "problem"); err != nil {
		t.Fatalf("dispute: %v", err)
	}

	if _, err := f.pool.Exec(ctx,
		`UPDATE orders SET auto_release_at = now() - interval '1 minute' WHERE id=$1`, o.ID); err != nil {
		t.Fatalf("force stale: %v", err)
	}

	rows, err := f.pool.Query(ctx,
		`SELECT id FROM orders WHERE status='held' AND auto_release_at <= now() AND id=$1`, o.ID)
	if err != nil {
		t.Fatalf("query stale: %v", err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("disputed order appeared in stale-held query")
	}
}

func TestAdminRefundDispute(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if _, err := f.walletSvc.Deposit(ctx, f.buyerID, "USD", 20000, ""); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	o, _ := f.orderSvc.CreateOrder(ctx, order.CreateInput{
		BuyerID: f.buyerID, ProductID: f.productID,
	})
	if _, err := f.orderSvc.Dispute(ctx, o.ID, f.buyerID, "problem"); err != nil {
		t.Fatalf("dispute: %v", err)
	}

	refunded, err := f.orderSvc.AdminRefundDispute(ctx, o.ID, f.adminID)
	if err != nil {
		t.Fatalf("admin refund: %v", err)
	}
	if refunded.Status != "refunded" {
		t.Errorf("status = %q, want refunded", refunded.Status)
	}

	balances, _ := f.walletSvc.GetBalances(ctx, f.buyerID)
	var avail, held int64
	for _, b := range balances {
		if b.CurrencyCode == "USD" {
			avail, held = b.Available, b.Held
		}
	}
	if avail != 20000 || held != 0 {
		t.Errorf("buyer wallet = avail %d held %d, want 20000/0", avail, held)
	}

	var resolution *string
	_ = f.pool.QueryRow(ctx, `SELECT resolution FROM disputes WHERE order_id=$1`, o.ID).Scan(&resolution)
	if resolution == nil || *resolution != "refunded" {
		t.Errorf("dispute resolution = %v, want refunded", resolution)
	}
}

func TestAdminReleaseDispute(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if _, err := f.walletSvc.Deposit(ctx, f.buyerID, "USD", 20000, ""); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	o, _ := f.orderSvc.CreateOrder(ctx, order.CreateInput{
		BuyerID: f.buyerID, ProductID: f.productID,
	})
	if _, err := f.orderSvc.Dispute(ctx, o.ID, f.buyerID, "problem"); err != nil {
		t.Fatalf("dispute: %v", err)
	}

	released, err := f.orderSvc.AdminRelease(ctx, o.ID, f.adminID)
	if err != nil {
		t.Fatalf("admin release: %v", err)
	}
	if released.Status != "completed" {
		t.Errorf("status = %q, want completed", released.Status)
	}

	balances, _ := f.walletSvc.GetBalances(ctx, f.sellerID)
	var sellerAvail int64
	for _, b := range balances {
		if b.CurrencyCode == "USD" {
			sellerAvail = b.Available
		}
	}
	if sellerAvail != 9500 {
		t.Errorf("seller available = %d, want 9500", sellerAvail)
	}

	var resolution *string
	_ = f.pool.QueryRow(ctx, `SELECT resolution FROM disputes WHERE order_id=$1`, o.ID).Scan(&resolution)
	if resolution == nil || *resolution != "released" {
		t.Errorf("dispute resolution = %v, want released", resolution)
	}
}

func TestConfirm_RejectedAfterDispute(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if _, err := f.walletSvc.Deposit(ctx, f.buyerID, "USD", 20000, ""); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	o, _ := f.orderSvc.CreateOrder(ctx, order.CreateInput{
		BuyerID: f.buyerID, ProductID: f.productID,
	})
	if _, err := f.orderSvc.Dispute(ctx, o.ID, f.buyerID, "problem"); err != nil {
		t.Fatalf("dispute: %v", err)
	}

	_, err := f.orderSvc.Confirm(ctx, o.ID, f.buyerID)
	if !errors.Is(err, order.ErrCannotDispute) {
		t.Fatalf("err = %v, want ErrCannotDispute", err)
	}
}

func TestAutoRelease_StillWorksOnHeldOrders(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if _, err := f.walletSvc.Deposit(ctx, f.buyerID, "USD", 20000, ""); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	o, _ := f.orderSvc.CreateOrder(ctx, order.CreateInput{
		BuyerID: f.buyerID, ProductID: f.productID,
	})
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
	_ = time.Now()
}
