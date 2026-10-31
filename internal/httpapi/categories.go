package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/thomas666-beast/sindri/internal/category"
	"github.com/thomas666-beast/sindri/internal/httpapi/requests"
)

func listCategoriesHandler(svc *category.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		onlyActive, _ := strconv.ParseBool(r.URL.Query().Get("only_active"))
		filter := r.URL.Query().Get("parent")
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		list, err := svc.List(r.Context(), onlyActive, filter, int32(limit), int32(offset))
		if err != nil {
			if errors.Is(err, category.ErrInvalidInput) {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, "list failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": list})
	}
}

func listChildrenHandler(svc *category.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "slug")
		parent, err := svc.GetBySlug(r.Context(), slug)
		if err != nil {
			if errors.Is(err, category.ErrNotFound) {
				writeError(w, http.StatusNotFound, "category not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "get parent failed")
			return
		}
		onlyActive, _ := strconv.ParseBool(r.URL.Query().Get("only_active"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		list, err := svc.ListChildren(r.Context(), parent.ID, onlyActive, int32(limit), int32(offset))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "list failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": list})
	}
}

func getCategoryHandler(svc *category.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		c, err := svc.Get(r.Context(), id)
		if err != nil {
			if errors.Is(err, category.ErrNotFound) {
				writeError(w, http.StatusNotFound, "category not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "get failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": c})
	}
}

func getCategoryBySlugHandler(svc *category.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "slug")
		c, err := svc.GetBySlug(r.Context(), slug)
		if err != nil {
			if errors.Is(err, category.ErrNotFound) {
				writeError(w, http.StatusNotFound, "category not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "get failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": c})
	}
}

func createCategoryHandler(svc *category.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req requests.CreateCategory
		if ok, status, body := bind(r, &req); !ok {
			writeJSON(w, status, body)
			return
		}
		c, err := svc.Create(r.Context(), category.CreateInput{
			Slug: req.Slug, Name: req.Name,
			Description: req.Description, SortOrder: req.SortOrder,
			ParentID: req.ParentID,
		})
		switch {
		case errors.Is(err, category.ErrInvalidInput):
			writeError(w, http.StatusUnprocessableEntity, err.Error())
		case errors.Is(err, category.ErrParentHasParent):
			writeError(w, http.StatusUnprocessableEntity, err.Error())
		case errors.Is(err, category.ErrSlugTaken):
			writeError(w, http.StatusConflict, "slug already used")
		case err != nil:
			writeError(w, http.StatusInternalServerError, "create failed")
		default:
			writeJSON(w, http.StatusCreated, map[string]any{"data": c})
		}
	}
}

func updateCategoryHandler(svc *category.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var req requests.UpdateCategory
		if ok, status, body := bind(r, &req); !ok {
			writeJSON(w, status, body)
			return
		}
		isActive := true
		if req.IsActive != nil {
			isActive = *req.IsActive
		}
		c, err := svc.Update(r.Context(), category.UpdateInput{
			ID: id, Slug: req.Slug, Name: req.Name,
			Description: req.Description, SortOrder: req.SortOrder, IsActive: isActive,
		})
		switch {
		case errors.Is(err, category.ErrNotFound):
			writeError(w, http.StatusNotFound, "category not found")
		case errors.Is(err, category.ErrInvalidInput):
			writeError(w, http.StatusUnprocessableEntity, err.Error())
		case errors.Is(err, category.ErrSlugTaken):
			writeError(w, http.StatusConflict, "slug already used")
		case err != nil:
			writeError(w, http.StatusInternalServerError, "update failed")
		default:
			writeJSON(w, http.StatusOK, map[string]any{"data": c})
		}
	}
}

func deleteCategoryHandler(svc *category.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		err := svc.Delete(r.Context(), id)
		switch {
		case errors.Is(err, category.ErrNotFound):
			writeError(w, http.StatusNotFound, "category not found")
		case errors.Is(err, category.ErrHasChildren):
			writeError(w, http.StatusConflict, "category has subcategories; delete them first")
		case err != nil:
			writeError(w, http.StatusInternalServerError, "delete failed")
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}
}
