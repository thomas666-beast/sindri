package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thomas666-beast/sindri/internal/httpapi"
)

func TestListCurrencies_ReturnsSeededCurrencies(t *testing.T) {
	deps, _ := testDeps(t)
	router := httpapi.NewRouter(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/currencies", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Data []struct {
			Code          string `json:"code"`
			Name          string `json:"name"`
			Symbol        string `json:"symbol"`
			DecimalDigits int16  `json:"decimal_digits"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(body.Data) < 10 {
		t.Fatalf("expected at least 10 currencies from seed, got %d", len(body.Data))
	}

	index := make(map[string]struct {
		Code          string
		Name          string
		Symbol        string
		DecimalDigits int16
	})
	for _, c := range body.Data {
		index[c.Code] = struct {
			Code          string
			Name          string
			Symbol        string
			DecimalDigits int16
		}{c.Code, c.Name, c.Symbol, c.DecimalDigits}
	}

	if _, ok := index["USD"]; !ok {
		t.Error("USD missing from response")
	}
	if _, ok := index["JPY"]; !ok {
		t.Error("JPY missing from response")
	}
	if jpy, ok := index["JPY"]; ok {
		if jpy.DecimalDigits != 0 {
			t.Errorf("JPY decimal_digits = %d, want 0", jpy.DecimalDigits)
		}
	}
	if usd, ok := index["USD"]; ok {
		if usd.DecimalDigits != 2 {
			t.Errorf("USD decimal_digits = %d, want 2", usd.DecimalDigits)
		}
		if usd.Symbol != "$" {
			t.Errorf("USD symbol = %q, want $", usd.Symbol)
		}
	}
}
