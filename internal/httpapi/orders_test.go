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

func TestOrdersFlow_BuyAndConfirm(t *testing.T) {
	srv, pool := newTestRouter(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano()%1_000_000_000)

	r1 := postJSON(t, srv.URL+"/api/v1/auth/register", map[string]string{
		"email":    "os_seller_" + suffix + "@example.com",
		"username": "os_seller_" + suffix,
		"password": "hunter2hunter2",
	})
	var seller authResp
	_ = json.NewDecoder(r1.Body).Decode(&seller)
	r1.Body.Close()

	r2 := postJSON(t, srv.URL+"/api/v1/auth/register", map[string]string{
		"email":    "os_buyer_" + suffix + "@example.com",
		"username": "os_buyer_" + suffix,
		"password": "hunter2hunter2",
	})
	var buyer authResp
	_ = json.NewDecoder(r2.Body).Decode(&buyer)
	r2.Body.Close()

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM wallet_transactions WHERE user_id IN ($1,$2)", seller.User.ID, buyer.User.ID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM orders WHERE buyer_id=$1 OR seller_id=$1", buyer.User.ID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM wallets WHERE user_id IN ($1,$2)", seller.User.ID, buyer.User.ID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM products WHERE seller_id=$1", seller.User.ID)
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

	resp := do(seller.Token, http.MethodPost, "/api/v1/products", map[string]any{
		"title": "Test Product", "price": 10000, "currency_code": "USD",
	})
	var prod struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&prod)
	resp.Body.Close()

	resp = do(buyer.Token, http.MethodPost, "/api/v1/wallet/deposit", map[string]any{
		"currency": "USD", "amount": 20000,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("deposit status = %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = do(buyer.Token, http.MethodPost, "/api/v1/orders", map[string]any{
		"product_id": prod.Data.ID,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create order status = %d", resp.StatusCode)
	}
	var ord struct {
		Data struct {
			ID     string `json:"id"`
			MyRole string `json:"my_role"`
			Status string `json:"status"`
		} `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&ord)
	resp.Body.Close()

	if ord.Data.MyRole != "buyer" {
		t.Errorf("my_role = %q, want buyer", ord.Data.MyRole)
	}
	if ord.Data.Status != "held" {
		t.Errorf("status = %q, want held", ord.Data.Status)
	}

	resp = do(buyer.Token, http.MethodPost, "/api/v1/orders/"+ord.Data.ID+"/confirm", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("confirm status = %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = do(seller.Token, http.MethodGet, "/api/v1/orders", nil)
	var page struct {
		Data struct {
			Orders []struct {
				ID     string `json:"id"`
				MyRole string `json:"my_role"`
				Status string `json:"status"`
			} `json:"orders"`
		} `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&page)
	resp.Body.Close()

	if len(page.Data.Orders) < 1 {
		t.Fatal("seller sees no orders")
	}
	found := false
	for _, o := range page.Data.Orders {
		if o.ID == ord.Data.ID {
			found = true
			if o.MyRole != "seller" {
				t.Errorf("seller's my_role = %q", o.MyRole)
			}
			if o.Status != "completed" {
				t.Errorf("order status = %q, want completed", o.Status)
			}
		}
	}
	if !found {
		t.Fatal("confirmed order not found in seller list")
	}
}
