package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/thomas666-beast/sindri/internal/auth"
	"github.com/thomas666-beast/sindri/internal/httpapi/requests"
	"github.com/thomas666-beast/sindri/internal/settings"
	"github.com/thomas666-beast/sindri/internal/user"
	"github.com/thomas666-beast/sindri/internal/wallet"
)

type walletListResponse struct {
	Data []wallet.Balance `json:"data"`
}

func getWalletHandler(w *wallet.Service) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromContext(r.Context())
		balances, err := w.GetBalances(r.Context(), userID)
		if err != nil {
			writeError(rw, http.StatusInternalServerError, "failed to load wallet")
			return
		}
		writeJSON(rw, http.StatusOK, walletListResponse{Data: balances})
	}
}

func depositHandler(w *wallet.Service, st *settings.Service, us *user.Service) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromContext(r.Context())

		// KYC gate (configurable)
		kyc, _ := st.Bool(r.Context(), "kyc_required_for_deposit", false)
		if kyc {
			u, err := us.GetByID(r.Context(), userID)
			if err != nil || u.KYCStatus != "verified" {
				writeError(rw, http.StatusForbidden, wallet.ErrKYCRequired.Error())
				return
			}
		}

		var req requests.WalletOp
		if ok, status, body := bind(r, &req); !ok {
			writeJSON(rw, status, body)
			return
		}

		bal, err := w.Deposit(r.Context(), userID, req.Currency, req.Amount, req.IdempotencyKey)
		if err != nil {
			if errors.Is(err, wallet.ErrInvalidAmount) {
				writeError(rw, http.StatusBadRequest, err.Error())
				return
			}
			writeError(rw, http.StatusInternalServerError, "deposit failed")
			return
		}
		writeJSON(rw, http.StatusOK, map[string]any{"data": bal})
	}
}

func listTransactionsHandler(svc *wallet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromContext(r.Context())
		currency := r.URL.Query().Get("currency")
		cursor := r.URL.Query().Get("cursor")
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

		page, err := svc.ListTransactions(r.Context(), userID, currency, cursor, int32(limit))
		if err != nil {
			slog.Error("list transactions", "error", err, "user_id", userID)
			writeError(w, http.StatusInternalServerError, "list failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": page})
	}
}
