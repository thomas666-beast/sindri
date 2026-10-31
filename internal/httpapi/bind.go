package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/thomas666-beast/reqval"
)

func bind(r *http.Request, dst any) (ok bool, status int, body any) {
	if err := reqval.BindCtx(r.Context(), r.Body, dst); err != nil {
		if ve, ok := reqval.AsValidationError(err); ok {
			slog.Warn("bind validation failed",
				"path", r.URL.Path,
				"fields", ve.Fields,
			)
			return false, http.StatusUnprocessableEntity, map[string]any{"errors": ve.Fields}
		}
		if ae, ok := reqval.AsAuthorizationError(err); ok {
			return false, http.StatusForbidden, map[string]any{"error": ae.Message}
		}
		if errors.Is(err, reqval.ErrEmptyBody) {
			return false, http.StatusBadRequest, map[string]any{"error": "empty request body"}
		}
		var syntaxErr *json.SyntaxError
		if errors.As(err, &syntaxErr) {
			return false, http.StatusBadRequest, map[string]any{"error": "invalid JSON"}
		}
		slog.Warn("bind error", "path", r.URL.Path, "error", err)
		return false, http.StatusBadRequest, map[string]any{"error": "invalid request body"}
	}
	return true, 0, nil
}
