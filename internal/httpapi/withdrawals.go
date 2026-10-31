package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/thomas666-beast/sindri/internal/auth"
	"github.com/thomas666-beast/sindri/internal/httpapi/requests"
	"github.com/thomas666-beast/sindri/internal/withdrawal"
)

func requestWithdrawalHandler(svc *withdrawal.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromContext(r.Context())

		var req requests.RequestWithdrawal
		if ok, status, body := bind(r, &req); !ok {
			writeJSON(w, status, body)
			return
		}

		wr, err := svc.RequestWithdrawal(r.Context(), userID, req.Currency, req.Amount)
		switch {
		case errors.Is(err, withdrawal.ErrSystemUserCannotWithdraw):
			writeError(w, http.StatusForbidden, err.Error())
		case errors.Is(err, withdrawal.ErrInsufficientFunds):
			writeError(w, http.StatusBadRequest, "insufficient available balance")
		case errors.Is(err, withdrawal.ErrInvalidAmount):
			writeError(w, http.StatusBadRequest, err.Error())
		case err != nil:
			writeError(w, http.StatusInternalServerError, "request failed")
		default:
			writeJSON(w, http.StatusCreated, map[string]any{"data": wr})
		}
	}
}

func listMyWithdrawalsHandler(svc *withdrawal.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromContext(r.Context())
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		list, err := svc.ListMine(r.Context(), userID, int32(limit), int32(offset))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "list failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": list})
	}
}

func adminListWithdrawalsHandler(svc *withdrawal.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := r.URL.Query().Get("status")
		idFilter := r.URL.Query().Get("id") // <-- New ID filter
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		
		list, err := svc.AdminList(r.Context(), status, idFilter, int32(limit), int32(offset))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "list failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": list})
	}
}

func adminApproveWithdrawalHandler(svc *withdrawal.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		adminID, _ := auth.UserIDFromContext(r.Context())
		id := chi.URLParam(r, "id")
		wr, err := svc.AdminApprove(r.Context(), id, adminID)
		switch {
		case errors.Is(err, withdrawal.ErrNotFound):
			writeError(w, http.StatusNotFound, "request not found")
		case errors.Is(err, withdrawal.ErrAlreadyReviewed):
			writeError(w, http.StatusConflict, "request already reviewed")
		case err != nil:
			writeError(w, http.StatusInternalServerError, "approve failed")
		default:
			writeJSON(w, http.StatusOK, map[string]any{"data": wr})
		}
	}
}

func adminRejectWithdrawalHandler(svc *withdrawal.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		adminID, _ := auth.UserIDFromContext(r.Context())
		id := chi.URLParam(r, "id")

		var req requests.RejectWithdrawal
		if ok, status, body := bind(r, &req); !ok {
			writeJSON(w, status, body)
			return
		}

		wr, err := svc.AdminReject(r.Context(), id, adminID, req.Reason)
		switch {
		case errors.Is(err, withdrawal.ErrNotFound):
			writeError(w, http.StatusNotFound, "request not found")
		case errors.Is(err, withdrawal.ErrAlreadyReviewed):
			writeError(w, http.StatusConflict, "request already reviewed")
		case errors.Is(err, withdrawal.ErrReasonRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		case err != nil:
			writeError(w, http.StatusInternalServerError, "reject failed")
		default:
			writeJSON(w, http.StatusOK, map[string]any{"data": wr})
		}
	}
}
