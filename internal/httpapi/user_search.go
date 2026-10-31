package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/thomas666-beast/sindri/internal/user"
)

func searchUsersHandler(svc *user.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

		page, err := svc.Search(r.Context(), q, int32(limit), int32(offset))
		if err != nil {
			if errors.Is(err, user.ErrSearchQueryTooShort) {
				writeError(w, http.StatusBadRequest, "query must be at least 2 characters")
				return
			}
			writeError(w, http.StatusInternalServerError, "search failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": page})
	}
}
