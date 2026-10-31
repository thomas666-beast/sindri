package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestWalletFlow_DepositAndRequestWithdrawal(t *testing.T) {
	srv, pool := newTestRouter(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano()%1_000_000_000)
	email := "wflow_" + suffix + "@example.com"

	resp := postJSON(t, srv.URL+"/api/v1/auth/register", map[string]string{
		"email": email, "username": "wflow_" + suffix, "password": "hunter2hunter2",
	})
	var reg authResp
	_ = json.NewDecoder(resp.Body).Decode(&reg)
	resp.Body.Close()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM wallet_transactions WHERE user_id=$1", reg.User.ID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM withdrawal_requests WHERE user_id=$1", reg.User.ID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM wallets WHERE user_id=$1", reg.User.ID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id=$1", reg.User.ID)
	})

	doReq := func(method, path string, body any) *http.Response {
		var buf *bytes.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			buf = bytes.NewReader(b)
		} else {
			buf = bytes.NewReader(nil)
		}
		req, _ := http.NewRequest(method, srv.URL+path, buf)
		req.Header.Set("Authorization", "Bearer "+reg.Token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
		return resp
	}

	// 1. Empty wallet
	resp = doReq(http.MethodGet, "/api/v1/wallet", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("empty wallet status = %d", resp.StatusCode)
	}
	var empty struct {
		Data []struct{} `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&empty)
	resp.Body.Close()
	if len(empty.Data) != 0 {
		t.Fatalf("expected 0 wallets, got %d", len(empty.Data))
	}

	// 2. Deposit 5000
	resp = doReq(http.MethodPost, "/api/v1/wallet/deposit", map[string]any{
		"currency": "USD", "amount": 5000,
	})
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("deposit status = %d body = %s", resp.StatusCode, b)
	}
	resp.Body.Close()

	// 3. Request withdrawal of 1500 → funds held, NOT debited
	resp = doReq(http.MethodPost, "/api/v1/withdrawals", map[string]any{
		"currency": "USD", "amount": 1500,
	})
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("withdrawal request status = %d body = %s", resp.StatusCode, b)
	}
	resp.Body.Close()

	// 4. Wallet shows available=3500, held=1500
	resp = doReq(http.MethodGet, "/api/v1/wallet", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d", resp.StatusCode)
	}
	var list struct {
		Data []struct {
			CurrencyCode string `json:"currency_code"`
			Available    int64  `json:"available"`
			Held         int64  `json:"held"`
		} `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&list)
	resp.Body.Close()
	if len(list.Data) != 1 {
		t.Fatalf("expected 1 wallet, got %d", len(list.Data))
	}
	if list.Data[0].Available != 3500 || list.Data[0].Held != 1500 {
		t.Errorf("wallet = available %d held %d, want 3500/1500",
			list.Data[0].Available, list.Data[0].Held)
	}

	// 5. Overdraft request → insufficient
	resp = doReq(http.MethodPost, "/api/v1/withdrawals", map[string]any{
		"currency": "USD", "amount": 999999,
	})
	if resp.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("overdraft status = %d, want 400; body = %s", resp.StatusCode, b)
	}
	resp.Body.Close()
}
