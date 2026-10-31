package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"log/slog"
	"fmt"
	"io"

	"github.com/go-chi/chi/v5"

	"github.com/thomas666-beast/sindri/internal/auth"
	"github.com/thomas666-beast/sindri/internal/httpapi/requests"
	"github.com/thomas666-beast/sindri/internal/order"
	"github.com/thomas666-beast/sindri/internal/user"
)

func createOrderHandler(svc *order.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromContext(r.Context())

		var req requests.CreateOrder
		if ok, status, body := bind(r, &req); !ok {
			writeJSON(w, status, body)
			return
		}

		o, err := svc.CreateOrder(r.Context(), order.CreateInput{
			BuyerID: userID, ProductID: req.ProductID,
		})
		switch {
		case errors.Is(err, order.ErrNotFound):
			writeError(w, http.StatusNotFound, "product not found")
		case errors.Is(err, order.ErrSelfDealing):
			writeError(w, http.StatusBadRequest, "cannot buy your own product")
		case errors.Is(err, order.ErrInsufficientFunds):
			writeError(w, http.StatusBadRequest, "insufficient funds")
		case err != nil:
			writeError(w, http.StatusInternalServerError, "create order failed")
		default:
			writeJSON(w, http.StatusCreated, map[string]any{"data": o})
		}
	}
}

func listOrdersHandler(svc *order.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromContext(r.Context())
		role := r.URL.Query().Get("role")
		if role != "" && role != "buyer" && role != "seller" {
			writeError(w, http.StatusBadRequest, "role must be buyer or seller")
			return
		}
		cursor := r.URL.Query().Get("cursor")
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		page, err := svc.List(r.Context(), userID, role, cursor, r.URL.Query().Get("id"), int32(limit))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "list failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": page})
	}
}

func getOrderHandler(svc *order.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromContext(r.Context())
		id := chi.URLParam(r, "id")
		o, err := svc.Get(r.Context(), id, userID)
		switch {
		case errors.Is(err, order.ErrNotFound):
			writeError(w, http.StatusNotFound, "order not found")
		case errors.Is(err, order.ErrForbidden):
			writeError(w, http.StatusForbidden, "not your order")
		case err != nil:
			writeError(w, http.StatusInternalServerError, "get failed")
		default:
			writeJSON(w, http.StatusOK, map[string]any{"data": o})
		}
	}
}

func uploadDeliverableHandler(svc *order.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sellerID, _ := auth.UserIDFromContext(r.Context())
		orderID := chi.URLParam(r, "id")

		if err := r.ParseMultipartForm(10 << 20); err != nil {
			writeError(w, http.StatusBadRequest, "file too large")
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, "file required")
			return
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "read file failed")
			return
		}

		err = svc.UploadDeliverable(r.Context(), orderID, sellerID, header.Filename, header.Header.Get("Content-Type"), data)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "uploaded"})
	}
}

func sealDeliveryHandler(svc *order.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sellerID, _ := auth.UserIDFromContext(r.Context())
		orderID := chi.URLParam(r, "id")

		err := svc.SealDelivery(r.Context(), orderID, sellerID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "sealed"})
	}
}

func downloadDeliverableHandler(svc *order.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromContext(r.Context())
		orderID := chi.URLParam(r, "id")
		fileID := chi.URLParam(r, "fileId")

		data, filename, contentType, err := svc.DownloadDeliverable(r.Context(), orderID, fileID, userID)
		if err != nil {
			writeError(w, http.StatusForbidden, "cannot download")
			return
		}

		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		w.Header().Set("Content-Type", contentType)
		w.Write(data)
	}
}

func listDeliverablesHandler(svc *order.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromContext(r.Context())
		orderID := chi.URLParam(r, "id")

		files, err := svc.ListDeliverables(r.Context(), orderID, userID)
		if err != nil {
			writeError(w, http.StatusForbidden, "cannot list files")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": files})
	}
}

func getDashboardStatsHandler(svc *order.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromContext(r.Context())
		stats, err := svc.GetDashboardStats(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load stats")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": stats})
	}
}

func confirmOrderHandler(svc *order.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromContext(r.Context())
		id := chi.URLParam(r, "id")
		o, err := svc.Confirm(r.Context(), id, userID)
		switch {
		case errors.Is(err, order.ErrNotFound):
			writeError(w, http.StatusNotFound, "order not found")
		case errors.Is(err, order.ErrForbidden):
			writeError(w, http.StatusForbidden, "only the buyer can confirm")
		case errors.Is(err, order.ErrCannotDispute):
			writeError(w, http.StatusBadRequest, "cannot confirm a disputed order")
		case err != nil:
			writeError(w, http.StatusInternalServerError, "confirm failed")
		default:
			writeJSON(w, http.StatusOK, map[string]any{"data": o})
		}
	}
}

func refundOrderHandler(svc *order.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		adminID, _ := auth.UserIDFromContext(r.Context())
		id := chi.URLParam(r, "id")
		o, err := svc.Refund(r.Context(), id, adminID)
		switch {
		case errors.Is(err, order.ErrNotFound):
			writeError(w, http.StatusNotFound, "order not found")
		case errors.Is(err, order.ErrNotHeld):
			writeError(w, http.StatusBadRequest, "order is not in held or disputed state")
		case err != nil:
			slog.Error("refund failed", "error", err, "order_id", id, "admin_id", adminID)
			writeError(w, http.StatusInternalServerError, "refund failed")
			return
		default:
			writeJSON(w, http.StatusOK, map[string]any{"data": o})
		}
	}
}

func disputeOrderHandler(svc *order.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		buyerID, _ := auth.UserIDFromContext(r.Context())
		id := chi.URLParam(r, "id")

		var req requests.DisputeOrder
		if ok, status, body := bind(r, &req); !ok {
			writeJSON(w, status, body)
			return
		}

		o, err := svc.Dispute(r.Context(), id, buyerID, req.Reason)
		switch {
		case errors.Is(err, order.ErrNotFound):
			writeError(w, http.StatusNotFound, "order not found")
		case errors.Is(err, order.ErrNotBuyerDispute):
			writeError(w, http.StatusForbidden, "only the buyer can dispute")
		case errors.Is(err, order.ErrAlreadyDisputed):
			writeError(w, http.StatusConflict, "order is already disputed")
		case errors.Is(err, order.ErrCannotDispute):
			writeError(w, http.StatusBadRequest, "order cannot be disputed in its current state")
		case err != nil:
			writeError(w, http.StatusInternalServerError, "dispute failed")
		default:
			writeJSON(w, http.StatusOK, map[string]any{"data": o})
		}
	}
}

func adminReleaseOrderHandler(svc *order.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		adminID, _ := auth.UserIDFromContext(r.Context())
		id := chi.URLParam(r, "id")
		o, err := svc.AdminRelease(r.Context(), id, adminID)
		switch {
		case errors.Is(err, order.ErrNotFound):
			writeError(w, http.StatusNotFound, "order not found")
		case err != nil:
			writeError(w, http.StatusInternalServerError, "release failed")
		default:
			writeJSON(w, http.StatusOK, map[string]any{"data": o})
		}
	}
}

func adminListDisputedOrdersHandler(svc *order.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		list, err := svc.AdminListDisputed(r.Context(), r.URL.Query().Get("id"), int32(limit), int32(offset))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "list failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": list})
	}
}

func adminGetOrderHandler(svc *order.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		o, err := svc.AdminGet(r.Context(), id)
		if err != nil {
			if errors.Is(err, order.ErrNotFound) {
				writeError(w, http.StatusNotFound, "order not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "get failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": o})
	}
}

// ---- Invoices ----

func createInvoiceHandler(svc *order.Service, userSvc *user.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sellerID, _ := auth.UserIDFromContext(r.Context())

		var req requests.CreateInvoice
		if ok, status, body := bind(r, &req); !ok {
			writeJSON(w, status, body)
			return
		}

		buyer, err := userSvc.GetByUsername(r.Context(), req.BuyerUsername)
		if err != nil {
			if errors.Is(err, user.ErrNotFound) {
				writeError(w, http.StatusBadRequest, "buyer not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "lookup failed")
			return
		}
		if buyer.ID == sellerID {
			writeError(w, http.StatusBadRequest, "cannot invoice yourself")
			return
		}

		o, err := svc.CreateInvoice(r.Context(), order.CreateInvoiceInput{
			SellerID: sellerID, BuyerID: buyer.ID,
			Title: req.Title, Description: req.Description,
			Amount: req.Amount, Currency: req.Currency,
		})
		switch {
		case errors.Is(err, order.ErrSelfDealing):
			writeError(w, http.StatusBadRequest, "cannot invoice yourself")
		case err != nil:
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeJSON(w, http.StatusCreated, map[string]any{"data": o})
		}
	}
}

func listInvoicesHandler(svc *order.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromContext(r.Context())
		role := r.URL.Query().Get("role")
		status := r.URL.Query().Get("status")
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

		list, total, err := svc.ListInvoices(r.Context(), userID, role, status, r.URL.Query().Get("id"), int32(limit), int32(offset))
		if err != nil {
			slog.Error("list invoices", "error", err, "user_id", userID)
			writeError(w, http.StatusInternalServerError, "list failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": list, "total": total})
	}
}

func getInvoiceHandler(svc *order.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromContext(r.Context())
		id := chi.URLParam(r, "id")

		inv, err := svc.GetInvoice(r.Context(), id, userID)
		switch {
		case errors.Is(err, order.ErrNotFound):
			writeError(w, http.StatusNotFound, "invoice not found")
		case errors.Is(err, order.ErrForbidden):
			writeError(w, http.StatusForbidden, "not your invoice")
		case err != nil:
			slog.Error("get invoice", "error", err, "order_id", id)
			writeError(w, http.StatusInternalServerError, "get failed")
		default:
			writeJSON(w, http.StatusOK, map[string]any{"data": inv})
		}
	}
}

func cancelInvoiceHandler(svc *order.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromContext(r.Context())
		id := chi.URLParam(r, "id")

		var req requests.CancelInvoice
		if ok, status, body := bind(r, &req); !ok {
			writeJSON(w, status, body)
			return
		}

		inv, err := svc.CancelInvoice(r.Context(), id, userID, req.Reason)
		switch {
		case errors.Is(err, order.ErrNotFound):
			writeError(w, http.StatusNotFound, "invoice not found")
		case errors.Is(err, order.ErrForbidden):
			writeError(w, http.StatusForbidden, "not your invoice")
		case err != nil:
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeJSON(w, http.StatusOK, map[string]any{"data": inv})
		}
	}
}

func payInvoiceHandler(svc *order.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		buyerID, _ := auth.UserIDFromContext(r.Context())
		id := chi.URLParam(r, "id")
		o, err := svc.PayInvoice(r.Context(), id, buyerID)
		switch {
		case errors.Is(err, order.ErrNotFound):
			writeError(w, http.StatusNotFound, "invoice not found")
		case errors.Is(err, order.ErrForbidden):
			writeError(w, http.StatusForbidden, "not addressed to you")
		case errors.Is(err, order.ErrInsufficientFunds):
			writeError(w, http.StatusBadRequest, "insufficient funds")
		case err != nil:
			writeError(w, http.StatusInternalServerError, "pay failed")
		default:
			writeJSON(w, http.StatusOK, map[string]any{"data": o})
		}
	}
}
