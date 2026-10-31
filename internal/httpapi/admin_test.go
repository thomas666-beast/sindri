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

func TestAdminFlow_NonAdminForbidden(t *testing.T) {
	srv, pool := newTestRouter(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano()%1_000_000_000)

	resp := postJSON(t, srv.URL+"/api/v1/auth/register", map[string]string{
		"email":    "nadmin_" + suffix + "@example.com",
		"username": "nadmin_" + suffix,
		"password": "hunter2hunter2",
	})
	var u authResp
	_ = json.NewDecoder(resp.Body).Decode(&u)
	resp.Body.Close()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id=$1", u.User.ID)
	})

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+u.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

func TestAdminFlow_DisableUser(t *testing.T) {
	srv, pool := newTestRouter(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano()%1_000_000_000)

	regAdmin := postJSON(t, srv.URL+"/api/v1/auth/register", map[string]string{
		"email": "a_" + suffix + "@example.com", "username": "a_" + suffix, "password": "hunter2hunter2",
	})
	var admin authResp
	_ = json.NewDecoder(regAdmin.Body).Decode(&admin)
	regAdmin.Body.Close()

	regTarget := postJSON(t, srv.URL+"/api/v1/auth/register", map[string]string{
		"email": "t_" + suffix + "@example.com", "username": "t_" + suffix, "password": "hunter2hunter2",
	})
	var target authResp
	_ = json.NewDecoder(regTarget.Body).Decode(&target)
	regTarget.Body.Close()

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id IN ($1,$2)", admin.User.ID, target.User.ID)
	})

	if _, err := pool.Exec(context.Background(),
		`UPDATE users SET is_admin = TRUE WHERE id=$1`, admin.User.ID); err != nil {
		t.Fatalf("promote: %v", err)
	}

	resp := postJSON(t, srv.URL+"/api/v1/auth/login", map[string]string{
		"email": "a_" + suffix + "@example.com", "password": "hunter2hunter2",
	})
	var admin2 authResp
	_ = json.NewDecoder(resp.Body).Decode(&admin2)
	resp.Body.Close()

	body, _ := json.Marshal(map[string]string{"reason": "spam"})
	req, _ := http.NewRequest(http.MethodPost,
		srv.URL+"/api/v1/admin/users/"+target.User.ID+"/disable",
		bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+admin2.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("disable: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("disable status = %d", resp.StatusCode)
	}

	var active bool
	var reason *string
	_ = pool.QueryRow(context.Background(),
		`SELECT is_active, disable_reason FROM users WHERE id=$1`, target.User.ID).Scan(&active, &reason)
	if active {
		t.Error("user should be inactive")
	}
	if reason == nil || *reason != "spam" {
		t.Errorf("reason = %v", reason)
	}
}
