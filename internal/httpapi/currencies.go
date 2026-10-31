package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/thomas666-beast/sindri/internal/currency"
)

type currenciesResponse struct {
	Data []currency.Currency `json:"data"`
}

type currencyError struct {
	Error string `json:"error"`
}

func listCurrenciesHandler(svc *currency.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := svc.List(r.Context())
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(currencyError{Error: "internal error"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(currenciesResponse{Data: list})
	}
}
