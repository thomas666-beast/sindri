package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestInvoiceFlow_CreatePayConfirm(t *testing.T) {
	srv, pool := newTestRouter(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano()%1_000_000_000)

	r1 := postJSON(t, srv.URL+"/api/v1/auth/register", map[string]string{
		"email": "inv_s_" + suffix + "@example.com", "username": "inv_s_" + suffix, "password": "hunter2hunter2",
	})
	var seller authResp
	_ = json.NewDecoder(r1.Body).Decode(&seller)
	r1.Body.Close()

	r2 := postJSON(t, srv.URL+"/api/v1/auth/register", map[string]string{
		"email": "inv_b_" + suffix + "@example.com", "username": "inv_b_" + suffix, "password": "hunter2hunter2",
	})
	var buyer authResp
	_ = json.NewDecoder(r2.Body).Decode(&buyer)
	r2.Body.Close()

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM wallet_transactions WHERE user_id IN ($1,$2)", seller.User.ID, buyer.User.ID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM orders WHERE buyer_id=$1 OR seller_id=$1", buyer.User.ID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM wallets WHERE user_id IN ($1,$2)", seller.User.ID, buyer.User.ID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id IN ($1,$2)", seller.User.ID, buyer.User.ID)
	})

	do := func(token, method, path string, body any) *http.Response {
		var buf *bytes.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			buf = bytes.NewReader(b)
		} else {
			buf = bytes.NewReader(nil)
		}
		req, _ := http.NewRequest(method, srv.URL+path, buf)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
		return resp
	}

	// Buyer deposits
	resp := do(buyer.Token, http.MethodPost, "/api/v1/wallet/deposit", map[string]any{
		"currency": "USD", "amount": 20000,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("deposit status = %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Seller creates invoice addressed to buyer
	resp = do(seller.Token, http.MethodPost, "/api/v1/invoices", map[string]any{
		"buyer_username": "inv_b_" + suffix,
		"title":          "Custom work",
		"description":    "Bespoke task",
		"amount":         15000,
		"currency":       "USD",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create invoice status = %d", resp.StatusCode)
	}
	var inv struct {
		Data struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&inv)
	resp.Body.Close()
	if inv.Data.Status != "pending" {
		t.Fatalf("status = %q, want pending", inv.Data.Status)
	}

	// Buyer sees the invoice
	resp = do(buyer.Token, http.MethodGet, "/api/v1/invoices", nil)
	var pending struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&pending)
	resp.Body.Close()
	if len(pending.Data) != 1 || pending.Data[0].ID != inv.Data.ID {
		t.Fatalf("pending invoices mismatch: %+v", pending.Data)
	}

	// Buyer pays
	resp = do(buyer.Token, http.MethodPost, "/api/v1/invoices/"+inv.Data.ID+"/pay", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("pay status = %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Buyer confirms
	resp = do(buyer.Token, http.MethodPost, "/api/v1/orders/"+inv.Data.ID+"/confirm", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("confirm status = %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Seller's wallet has the funds
	resp = do(seller.Token, http.MethodGet, "/api/v1/wallet", nil)
	var wallets struct {
		Data []struct {
			CurrencyCode string `json:"currency_code"`
			Available    int64  `json:"available"`
		} `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&wallets)
	resp.Body.Close()
	if len(wallets.Data) != 1 || wallets.Data[0].Available != 14250 {
		t.Fatalf("seller wallet mismatch: %+v", wallets.Data)
	}
}
