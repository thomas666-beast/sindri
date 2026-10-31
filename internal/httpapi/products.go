package httpapi

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/thomas666-beast/sindri/internal/auth"
	"github.com/thomas666-beast/sindri/internal/httpapi/requests"
	"github.com/thomas666-beast/sindri/internal/product"
)

type productListResponse struct {
	Data []product.Product `json:"data"`
}

func createProductHandler(svc *product.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromContext(r.Context())

		var req requests.CreateProduct
		if ok, status, body := bind(r, &req); !ok {
			writeJSON(w, status, body)
			return
		}

		p, err := svc.Create(r.Context(), product.CreateInput{
			SellerID:     userID,
			Title:        req.Title,
			Description:  req.Description,
			Price:        req.Price,
			CurrencyCode: req.CurrencyCode,
			CategoryID:   req.CategoryID,
		})
		if err != nil {
			if errors.Is(err, product.ErrInvalidInput) {
				writeError(w, http.StatusBadRequest, "invalid product fields")
				return
			}
			writeError(w, http.StatusInternalServerError, "create failed")
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"data": p})
	}
}

func listProductsHandler(svc *product.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		category := r.URL.Query().Get("category")
		list, err := svc.List(r.Context(), category, int32(limit), int32(offset))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "list failed")
			return
		}
		writeJSON(w, http.StatusOK, productListResponse{Data: list})
	}
}

func getProductHandler(svc *product.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		p, err := svc.Get(r.Context(), id)
		if err != nil {
			if errors.Is(err, product.ErrNotFound) {
				writeError(w, http.StatusNotFound, "product not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "get failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": p})
	}
}

func deleteProductHandler(svc *product.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromContext(r.Context())
		id := chi.URLParam(r, "id")
		if err := svc.Deactivate(r.Context(), id, userID); err != nil {
			if errors.Is(err, product.ErrNotFound) {
				writeError(w, http.StatusNotFound, "product not found or not yours")
				return
			}
			writeError(w, http.StatusInternalServerError, "delete failed")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func setProductCategoryHandler(svc *product.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromContext(r.Context())
		id := chi.URLParam(r, "id")

		var req requests.SetProductCategory
		if ok, status, body := bind(r, &req); !ok {
			writeJSON(w, status, body)
			return
		}

		if err := svc.SetCategory(r.Context(), id, userID, req.CategoryID); err != nil {
			switch {
			case errors.Is(err, product.ErrNotFound):
				writeError(w, http.StatusNotFound, "product not found or not yours")
			case errors.Is(err, product.ErrInvalidInput):
				writeError(w, http.StatusUnprocessableEntity, err.Error())
			default:
				writeError(w, http.StatusInternalServerError, "update failed")
			}
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": "category updated"})
	}
}

func searchProductsHandler(svc *product.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		category := r.URL.Query().Get("category")
		seller := r.URL.Query().Get("seller")
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

		id := r.URL.Query().Get("id")

		if id != "" {
			if _, err := uuid.Parse(id); err != nil {
				writeError(w, http.StatusBadRequest, "invalid id filter")
				return
			}
		}

		page, err := svc.Search(r.Context(), q, category, seller, id, int32(limit), int32(offset))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "search failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": page})
	}
}

func updateProductHandler(svc *product.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromContext(r.Context())
		id := chi.URLParam(r, "id")

		var req requests.UpdateProduct
		if ok, status, body := bind(r, &req); !ok {
			writeJSON(w, status, body)
			return
		}

		p, err := svc.Update(r.Context(), product.UpdateInput{
			ID:          id,
			SellerID:    userID,
			Title:       req.Title,
			Description: req.Description,
			Price:       req.Price,
			CategoryID:  req.CategoryID,
		})
		switch {
		case errors.Is(err, product.ErrNotFound):
			writeError(w, http.StatusNotFound, "product not found or not yours")
		case errors.Is(err, product.ErrInvalidInput):
			writeError(w, http.StatusUnprocessableEntity, err.Error())
		case err != nil:
			writeError(w, http.StatusInternalServerError, "update failed")
		default:
			writeJSON(w, http.StatusOK, map[string]any{"data": p})
		}
	}
}

// NEW: Upload product image
func uploadProductImageHandler(svc *product.Service, uploadsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromContext(r.Context())
		productID := chi.URLParam(r, "id")

		// Max 5MB
		if err := r.ParseMultipartForm(5 << 20); err != nil {
			writeError(w, http.StatusBadRequest, "file too large or malformed")
			return
		}

		file, _, err := r.FormFile("image")
		if err != nil {
			writeError(w, http.StatusBadRequest, "image file required")
			return
		}
		defer file.Close()

		// Check MIME type
		head := make([]byte, 512)
		n, err := file.Read(head)
		if err != nil && n == 0 {
			writeError(w, http.StatusBadRequest, "cannot read file")
			return
		}
		mime := http.DetectContentType(head[:n])
		
		allowed := map[string]string{
			"image/jpeg": ".jpg",
			"image/png":  ".png",
			"image/webp": ".webp",
		}
		ext, ok := allowed[mime]
		if !ok {
			writeError(w, http.StatusBadRequest, "must be JPEG, PNG, or WEBP")
			return
		}

		// Rewind file
		if _, err := file.Seek(0, 0); err != nil {
			writeError(w, http.StatusInternalServerError, "cannot process file")
			return
		}

		// Save to disk
		name := uuid.NewString() + ext
		dest := filepath.Join(uploadsDir, "products", name)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			writeError(w, http.StatusInternalServerError, "cannot create storage")
			return
		}

		out, err := os.Create(dest)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "cannot save file")
			return
		}
		defer out.Close()
		
		if _, err := io.Copy(out, file); err != nil {
			writeError(w, http.StatusInternalServerError, "cannot write file")
			return
		}

		url := "/uploads/products/" + name

		// Save to DB
		if err := svc.AddImage(r.Context(), productID, userID, url, 0); err != nil {
			_ = os.Remove(dest) // Rollback file
			if errors.Is(err, product.ErrNotOwner) {
				writeError(w, http.StatusForbidden, "not your product")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to save image")
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"url": url})
	}
}

func getProductImagesHandler(svc *product.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		images, err := svc.GetImages(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load images")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": images})
	}
}
