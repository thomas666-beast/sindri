package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/thomas666-beast/sindri/internal/admin"
	"github.com/thomas666-beast/sindri/internal/auth"
	"github.com/thomas666-beast/sindri/internal/httpapi/requests"
)

// ---- users ----

func adminListUsersHandler(svc *admin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		limit, _ := strconv.Atoi(q.Get("limit"))
		offset, _ := strconv.Atoi(q.Get("offset"))
		users, total, err := svc.ListUsers(r.Context(), q.Get("search"), q.Get("status"), int32(limit), int32(offset))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "list failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": users, "total": total})
	}
}

func adminGetUserHandler(svc *admin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, err := svc.GetUser(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			if errors.Is(err, admin.ErrNotFound) {
				writeError(w, http.StatusNotFound, "user not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "get failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": u})
	}
}

func adminDisableUserHandler(svc *admin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req requests.DisableUser
		if ok, status, body := bind(r, &req); !ok {
			writeJSON(w, status, body)
			return
		}

		u, err := svc.DisableUser(r.Context(), chi.URLParam(r, "id"), req.Reason)
		if err != nil {
			if errors.Is(err, admin.ErrNotFound) {
				writeError(w, http.StatusNotFound, "user not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "disable failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": u})
	}
}

func adminEnableUserHandler(svc *admin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, err := svc.EnableUser(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			if errors.Is(err, admin.ErrNotFound) {
				writeError(w, http.StatusNotFound, "user not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "enable failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": u})
	}
}

func adminSetKYCHandler(svc *admin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req requests.SetKYC
		if ok, status, body := bind(r, &req); !ok {
			writeJSON(w, status, body)
			return
		}

		u, err := svc.SetKYC(r.Context(), chi.URLParam(r, "id"), req.Status)
		if err != nil {
			if errors.Is(err, admin.ErrNotFound) {
				writeError(w, http.StatusNotFound, "user not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "set kyc failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": u})
	}
}

func adminSetAdminHandler(svc *admin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req requests.SetAdmin
		if ok, status, body := bind(r, &req); !ok {
			writeJSON(w, status, body)
			return
		}

		u, err := svc.SetAdmin(r.Context(), chi.URLParam(r, "id"), *req.IsAdmin)
		if err != nil {
			if errors.Is(err, admin.ErrNotFound) {
				writeError(w, http.StatusNotFound, "user not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "set admin failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": u})
	}
}

func adminStatsHandler(svc *admin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := svc.Stats(r.Context())
		if err != nil {
			slog.Error("admin stats", "error", err)
			writeError(w, http.StatusInternalServerError, "stats failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": s})
	}
}

// ---- warnings ----

func adminCreateWarningHandler(svc *admin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		adminID, _ := auth.UserIDFromContext(r.Context())

		var req requests.CreateWarning
		if ok, status, body := bind(r, &req); !ok {
			writeJSON(w, status, body)
			return
		}

		// Verify target user exists so a nonexistent user_id yields a clean
		// 404 instead of a raw FK violation surfacing as 500.
		if _, err := svc.GetUser(r.Context(), req.UserID); err != nil {
			if errors.Is(err, admin.ErrNotFound) {
				writeError(w, http.StatusNotFound, "user not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "lookup failed")
			return
		}

		warn, err := svc.CreateWarning(r.Context(), req.UserID, adminID, req.Reason, req.Severity)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "create warning failed")
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"data": warn})
	}
}

func adminListWarningsHandler(svc *admin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		warnings, err := svc.ListWarnings(r.Context(), chi.URLParam(r, "id"), int32(limit), int32(offset))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "list failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": warnings})
	}
}

func myWarningsHandler(svc *admin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromContext(r.Context())
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		warnings, err := svc.ListMyWarnings(r.Context(), userID, int32(limit), int32(offset))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "list failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": warnings})
	}
}

func acknowledgeWarningHandler(svc *admin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromContext(r.Context())
		if err := svc.AcknowledgeWarning(r.Context(), userID, chi.URLParam(r, "id")); err != nil {
			if errors.Is(err, admin.ErrNotFound) {
				writeError(w, http.StatusNotFound, "warning not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "ack failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": "acknowledged"})
	}
}

// ---- tickets ----

func createTicketHandler(svc *admin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromContext(r.Context())

		var req requests.CreateTicket
		if ok, status, body := bind(r, &req); !ok {
			writeJSON(w, status, body)
			return
		}

		t, err := svc.CreateTicket(r.Context(), userID, req.Subject, req.Priority)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "create failed")
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"data": t})
	}
}

func myTicketsHandler(svc *admin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromContext(r.Context())
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		list, err := svc.ListMyTickets(r.Context(), userID, int32(limit), int32(offset))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "list failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": list})
	}
}

func getTicketHandler(svc *admin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromContext(r.Context())
		isAdmin := auth.IsAdminFromContext(r.Context())
		t, err := svc.GetTicket(r.Context(), chi.URLParam(r, "id"), userID, isAdmin)
		if err != nil {
			if errors.Is(err, admin.ErrNotFound) {
				writeError(w, http.StatusNotFound, "ticket not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "get failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": t})
	}
}

func listTicketMessagesHandler(svc *admin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromContext(r.Context())
		isAdmin := auth.IsAdminFromContext(r.Context())
		msgs, err := svc.ListMessages(r.Context(), chi.URLParam(r, "id"), userID, isAdmin)
		if err != nil {
			if errors.Is(err, admin.ErrNotFound) {
				writeError(w, http.StatusNotFound, "ticket not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "list failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": msgs})
	}
}

func addTicketMessageHandler(svc *admin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromContext(r.Context())
		isAdmin := auth.IsAdminFromContext(r.Context())
		ticketID := chi.URLParam(r, "id")

		if _, err := svc.GetTicket(r.Context(), ticketID, userID, isAdmin); err != nil {
			writeError(w, http.StatusNotFound, "ticket not found")
			return
		}

		var req requests.TicketMessage
		if ok, status, body := bind(r, &req); !ok {
			writeJSON(w, status, body)
			return
		}

		msg, err := svc.AddMessage(r.Context(), ticketID, userID, isAdmin, req.Body)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "add failed")
			return
		}

		if isAdmin {
			_, _ = svc.SetTicketStatus(r.Context(), ticketID, "answered")
		}

		writeJSON(w, http.StatusCreated, map[string]any{"data": msg})
	}
}

func adminListTicketsHandler(svc *admin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		list, err := svc.AdminListTickets(r.Context(), r.URL.Query().Get("status"), int32(limit), int32(offset))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "list failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": list})
	}
}

func adminSetTicketStatusHandler(svc *admin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req requests.SetTicketStatus
		if ok, status, body := bind(r, &req); !ok {
			writeJSON(w, status, body)
			return
		}

		t, err := svc.SetTicketStatus(r.Context(), chi.URLParam(r, "id"), req.Status)
		if err != nil {
			if errors.Is(err, admin.ErrNotFound) {
				writeError(w, http.StatusNotFound, "ticket not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "set failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": t})
	}
}

func adminListTransactionsHandler(svc *admin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		typeFilter := r.URL.Query().Get("type")
		idFilter := r.URL.Query().Get("id")
		userFilter := r.URL.Query().Get("user")

		list, err := svc.ListTransactions(r.Context(), typeFilter, idFilter, userFilter, int32(limit), int32(offset))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "list failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": list})
	}
}

func adminGetSettingsHandler(svc *admin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		settings, err := svc.GetSettings(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load settings")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": settings})
	}
}

func adminUpdateSettingHandler(svc *admin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := chi.URLParam(r, "key")
		
		var req requests.UpdateSetting
		if ok, status, body := bind(r, &req); !ok {
			writeJSON(w, status, body)
			return
		}

		if err := svc.UpdateSetting(r.Context(), key, req.Value); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update setting")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}
