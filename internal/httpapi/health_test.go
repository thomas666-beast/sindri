package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thomas666-beast/sindri/internal/httpapi"
)

func TestHealthz_ReturnsOK(t *testing.T) {
	deps, _ := testDeps(t)
	router := httpapi.NewRouter(deps)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Status string `json:"status"`
		DB     string `json:"db"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Status != "ok" || body.DB != "ok" {
		t.Fatalf("unexpected body: %+v", body)
	}
}
