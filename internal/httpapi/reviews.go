package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/thomas666-beast/sindri/internal/auth"
	"github.com/thomas666-beast/sindri/internal/httpapi/requests"
	"github.com/thomas666-beast/sindri/internal/review"
)

func createReviewHandler(svc *review.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		buyerID, _ := auth.UserIDFromContext(r.Context())

		var req requests.CreateReview
		if ok, status, body := bind(r, &req); !ok {
			writeJSON(w, status, body)
			return
		}

		rv, err := svc.Create(r.Context(), review.CreateInput{
			OrderID: req.OrderID, BuyerID: buyerID,
			Rating: req.Rating, Comment: req.Comment,
		})
		switch {
		case errors.Is(err, review.ErrOrderNotFound):
			writeError(w, http.StatusNotFound, "order not found")
		case errors.Is(err, review.ErrNotBuyer):
			writeError(w, http.StatusForbidden, "only the buyer can review this order")
		case errors.Is(err, review.ErrOrderNotCompleted):
			writeError(w, http.StatusConflict, "order is not completed")
		case errors.Is(err, review.ErrAlreadyReviewed):
			writeError(w, http.StatusConflict, "order already reviewed")
		case errors.Is(err, review.ErrInvalidRating), errors.Is(err, review.ErrInvalidComment):
			writeError(w, http.StatusBadRequest, err.Error())
		case err != nil:
			writeError(w, http.StatusInternalServerError, "create failed")
		default:
			writeJSON(w, http.StatusCreated, map[string]any{"data": rv})
		}
	}
}

func listSellerReviewsHandler(svc *review.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sellerID := chi.URLParam(r, "id")
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		page, err := svc.ListBySeller(r.Context(), sellerID, int32(limit), int32(offset))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "list failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": page})
	}
}

func listProductReviewsHandler(svc *review.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		productID := chi.URLParam(r, "id")
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		page, err := svc.ListByProduct(r.Context(), productID, int32(limit), int32(offset))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "list failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": page})
	}
}

func sellerSummaryHandler(svc *review.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sellerID := chi.URLParam(r, "id")
		sum, err := svc.SellerSummary(r.Context(), sellerID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "summary failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": sum})
	}
}

func productSummaryHandler(svc *review.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		productID := chi.URLParam(r, "id")
		sum, err := svc.ProductSummary(r.Context(), productID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "summary failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": sum})
	}
}
