package order_test

import (
	"context"
	"testing"

	"github.com/thomas666-beast/sindri/internal/order"
)

func TestInvoice_SellerCreatesAndBuyerPays(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	if _, err := f.walletSvc.Deposit(ctx, f.buyerID, "USD", 20000, ""); err != nil {
		t.Fatalf("deposit: %v", err)
	}

	// Seller creates invoice for buyer
	o, err := f.orderSvc.CreateInvoice(ctx, order.CreateInvoiceInput{
		SellerID:    f.sellerID,
		BuyerID:     f.buyerID,
		Title:       "Custom logo design",
		Description: "3 concepts, 2 revisions",
		Amount:      15000,
		Currency:    "USD",
	})
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	if o.Status != "pending" {
		t.Errorf("status = %q, want pending", o.Status)
	}
	if o.InvoiceTitle != "Custom logo design" {
		t.Errorf("invoice_title = %q", o.InvoiceTitle)
	}
	if o.ProductID != "" {
		t.Errorf("product_id = %q, want empty", o.ProductID)
	}

	// Buyer sees it
	pending, _, err := f.orderSvc.ListInvoices(ctx, f.buyerID, "buyer", "pending", "", 20, 0)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	found := false
	for _, p := range pending {
		if p.ID == o.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("invoice not in buyer's pending list")
	}

	// Buyer pays
	paid, err := f.orderSvc.PayInvoice(ctx, o.ID, f.buyerID)
	if err != nil {
		t.Fatalf("pay invoice: %v", err)
	}
	if paid.Status != "held" {
		t.Errorf("status after pay = %q, want held", paid.Status)
	}

	// Buyer's wallet: available reduced, held increased
	balances, _ := f.walletSvc.GetBalances(ctx, f.buyerID)
	var avail, held int64
	for _, b := range balances {
		if b.CurrencyCode == "USD" {
			avail, held = b.Available, b.Held
		}
	}
	if avail != 5000 || held != 15000 {
		t.Errorf("buyer wallet = avail %d held %d, want 5000/15000", avail, held)
	}

	// Buyer confirms, seller + platform get paid
	confirmed, err := f.orderSvc.Confirm(ctx, o.ID, f.buyerID)
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if confirmed.Status != "completed" {
		t.Errorf("status = %q", confirmed.Status)
	}

	balances, _ = f.walletSvc.GetBalances(ctx, f.sellerID)
	var sellerAvail int64
	for _, b := range balances {
		if b.CurrencyCode == "USD" {
			sellerAvail = b.Available
		}
	}
	// commission 5% of 15000 = 750; seller gets 14250
	if sellerAvail != 14250 {
		t.Errorf("seller available = %d, want 14250", sellerAvail)
	}
}

func TestInvoice_WrongBuyerCannotPay(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if _, err := f.walletSvc.Deposit(ctx, f.sellerID, "USD", 50000, ""); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	o, err := f.orderSvc.CreateInvoice(ctx, order.CreateInvoiceInput{
		SellerID: f.sellerID,
		BuyerID:  f.buyerID,
		Title:    "Not for you",
		Amount:   1000,
		Currency: "USD",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Seller tries to pay their own invoice (as a random "buyer")
	_, err = f.orderSvc.PayInvoice(ctx, o.ID, f.sellerID)
	if err != order.ErrForbidden {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

func TestInvoice_InsufficientFunds(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	// No deposit
	o, err := f.orderSvc.CreateInvoice(ctx, order.CreateInvoiceInput{
		SellerID: f.sellerID,
		BuyerID:  f.buyerID,
		Title:    "Expensive",
		Amount:   99999,
		Currency: "USD",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	_, err = f.orderSvc.PayInvoice(ctx, o.ID, f.buyerID)
	if err != order.ErrInsufficientFunds {
		t.Fatalf("err = %v, want ErrInsufficientFunds", err)
	}
}

func TestInvoice_PayTwiceIsIdempotent(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if _, err := f.walletSvc.Deposit(ctx, f.buyerID, "USD", 20000, ""); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	o, err := f.orderSvc.CreateInvoice(ctx, order.CreateInvoiceInput{
		SellerID: f.sellerID, BuyerID: f.buyerID,
		Title: "Test", Amount: 10000, Currency: "USD",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := f.orderSvc.PayInvoice(ctx, o.ID, f.buyerID); err != nil {
		t.Fatalf("pay 1: %v", err)
	}
	if _, err := f.orderSvc.PayInvoice(ctx, o.ID, f.buyerID); err != nil {
		t.Fatalf("pay 2 (idempotent): %v", err)
	}

	// Wallet should show only one hold.
	balances, _ := f.walletSvc.GetBalances(ctx, f.buyerID)
	var held int64
	for _, b := range balances {
		if b.CurrencyCode == "USD" {
			held = b.Held
		}
	}
	if held != 10000 {
		t.Errorf("held = %d, want 10000 (only one hold)", held)
	}
}
