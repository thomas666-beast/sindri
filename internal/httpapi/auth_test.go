package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thomas666-beast/sindri/internal/httpapi"
)

func newTestRouter(t *testing.T) (*httptest.Server, *pgxpool.Pool) {
	t.Helper()
	deps, pool := testDeps(t)
	router := httpapi.NewRouter(deps)
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return srv, pool
}

type authResp struct {
	User struct {
		ID       string `json:"id"`
		Email    string `json:"email"`
		Username string `json:"username"`
	} `json:"user"`
	Token string `json:"token"`
}

func postJSON(t *testing.T, url string, body any) *http.Response {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	return resp
}

func uniqueSuffix() string {
	return fmt.Sprintf("%d", time.Now().UnixNano()%1_000_000_000)
}

func TestAuthFlow_RegisterLoginMe(t *testing.T) {
	srv, pool := newTestRouter(t)
	suffix := uniqueSuffix()
	email := "flow_" + suffix + "@example.com"
	username := "flow_" + suffix

	resp := postJSON(t, srv.URL+"/api/v1/auth/register", map[string]string{
		"email": email, "username": username, "password": "hunter2hunter2",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register status = %d", resp.StatusCode)
	}
	var reg authResp
	_ = json.NewDecoder(resp.Body).Decode(&reg)
	resp.Body.Close()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id=$1", reg.User.ID)
	})
	if reg.Token == "" {
		t.Fatal("no token from register")
	}

	resp = postJSON(t, srv.URL+"/api/v1/auth/login", map[string]string{
		"email": email, "password": "hunter2hunter2",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d", resp.StatusCode)
	}
	var login authResp
	_ = json.NewDecoder(resp.Body).Decode(&login)
	resp.Body.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+login.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("me: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("me status = %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestAuthFlow_DuplicateEmail(t *testing.T) {
	srv, pool := newTestRouter(t)
	suffix := uniqueSuffix()
	email := "dupflow_" + suffix + "@example.com"

	resp := postJSON(t, srv.URL+"/api/v1/auth/register", map[string]string{
		"email": email, "username": "dupflow_a_" + suffix, "password": "hunter2hunter2",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("first register: %d", resp.StatusCode)
	}
	var reg authResp
	_ = json.NewDecoder(resp.Body).Decode(&reg)
	resp.Body.Close()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id=$1", reg.User.ID)
	})

	resp = postJSON(t, srv.URL+"/api/v1/auth/register", map[string]string{
		"email": email, "username": "dupflow_b_" + suffix, "password": "hunter2hunter2",
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate email status = %d, want 409", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestAuthFlow_LoginWrongPassword(t *testing.T) {
	srv, pool := newTestRouter(t)
	suffix := uniqueSuffix()
	email := "wrongpw_" + suffix + "@example.com"

	resp := postJSON(t, srv.URL+"/api/v1/auth/register", map[string]string{
		"email": email, "username": "wrongpw_" + suffix, "password": "hunter2hunter2",
	})
	var reg authResp
	_ = json.NewDecoder(resp.Body).Decode(&reg)
	resp.Body.Close()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id=$1", reg.User.ID)
	})

	resp = postJSON(t, srv.URL+"/api/v1/auth/login", map[string]string{
		"email": email, "password": "nottherightone",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong password status = %d, want 401", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestAuthFlow_MeWithoutToken(t *testing.T) {
	srv, _ := newTestRouter(t)
	resp, err := http.Get(srv.URL + "/api/v1/me")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestAuthFlow_CookieSetOnLogin(t *testing.T) {
	srv, pool := newTestRouter(t)
	suffix := uniqueSuffix()
	email := "ck_" + suffix + "@example.com"

	resp := postJSON(t, srv.URL+"/api/v1/auth/register", map[string]string{
		"email": email, "username": "ck_" + suffix, "password": "hunter2hunter2",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: %d", resp.StatusCode)
	}
	var reg authResp
	_ = json.NewDecoder(resp.Body).Decode(&reg)
	resp.Body.Close()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id=$1", reg.User.ID)
	})

	var cookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "barter_token" {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("expected barter_token cookie on register")
	}
	if !cookie.HttpOnly {
		t.Error("cookie should be HttpOnly")
	}
	if cookie.Value != reg.Token {
		t.Errorf("cookie value differs from JSON token")
	}
}

func TestAuthFlow_MeWithCookieOnly(t *testing.T) {
	srv, pool := newTestRouter(t)
	suffix := uniqueSuffix()

	resp := postJSON(t, srv.URL+"/api/v1/auth/register", map[string]string{
		"email": "co_" + suffix + "@example.com",
		"username": "co_" + suffix,
		"password": "hunter2hunter2",
	})
	var reg authResp
	_ = json.NewDecoder(resp.Body).Decode(&reg)
	var cookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "barter_token" {
			cookie = c
		}
	}
	resp.Body.Close()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id=$1", reg.User.ID)
	})
	if cookie == nil {
		t.Fatal("no cookie")
	}

	// Hit /me with ONLY the cookie — no Authorization header.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/me", nil)
	req.AddCookie(cookie)
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("me: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("me with cookie: %d, want 200", resp2.StatusCode)
	}
}

func TestAuthFlow_LogoutClearsCookie(t *testing.T) {
	srv, pool := newTestRouter(t)
	suffix := uniqueSuffix()

	resp := postJSON(t, srv.URL+"/api/v1/auth/register", map[string]string{
		"email": "lo_" + suffix + "@example.com",
		"username": "lo_" + suffix,
		"password": "hunter2hunter2",
	})
	var reg authResp
	_ = json.NewDecoder(resp.Body).Decode(&reg)
	var cookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "barter_token" {
			cookie = c
		}
	}
	resp.Body.Close()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id=$1", reg.User.ID)
	})

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/auth/logout", nil)
	req.AddCookie(cookie)
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("logout: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusNoContent {
		t.Fatalf("logout: %d, want 204", resp2.StatusCode)
	}
	var clearing *http.Cookie
	for _, c := range resp2.Cookies() {
		if c.Name == "barter_token" {
			clearing = c
		}
	}
	if clearing == nil || clearing.MaxAge != -1 {
		t.Fatalf("expected clearing cookie with MaxAge=-1, got %+v", clearing)
	}
}
