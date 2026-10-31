package auth

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type errorBody struct {
	Error string `json:"error"`
}

func RequireAuth(m *Manager, checker UserChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, err := m.ExtractToken(r)
			if err != nil {
				writeAuthError(w, http.StatusUnauthorized, "authentication required")
				return
			}
			claims, err := m.Verify(token)
			if err != nil {
				writeAuthError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			// Enforce account state from the DB, not the token.
			// Otherwise a disabled user (or a demoted admin) keeps access
			// for the full JWT TTL (~24h).
			if checker == nil {
				// Defensive: a nil checker means RequireAuth was wired
				// incorrectly. Fail closed rather than silently skipping
				// the check.
				slog.Error("RequireAuth called without UserChecker")
				writeAuthError(w, http.StatusInternalServerError, "auth misconfigured")
				return
			}
			active, isAdmin, err := checker.CheckActive(r.Context(), claims.UserID)
			if err != nil {
				slog.Error("auth: user lookup failed", "user_id", claims.UserID, "error", err)
				writeAuthError(w, http.StatusInternalServerError, "authentication failed")
				return
			}
			if !active {
				writeAuthError(w, http.StatusUnauthorized, "account disabled")
				return
			}

			// Use the DB's isAdmin, not claims.IsAdmin. If an admin was
			// demoted, the DB is the source of truth; the token is stale.
			ctx := WithUser(r.Context(), claims.UserID, isAdmin)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !IsAdminFromContext(r.Context()) {
			writeAuthError(w, http.StatusForbidden, "admin access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeAuthError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorBody{Error: msg})
}
