package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/thomas666-beast/sindri/internal/auth"
	"github.com/thomas666-beast/sindri/internal/notification"
)

func listNotificationsHandler(svc *notification.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromContext(r.Context())
		onlyUnread := r.URL.Query().Get("only_unread") == "true"
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		page, err := svc.List(r.Context(), userID, onlyUnread, int32(limit), int32(offset))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "list failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": page})
	}
}

func unreadCountHandler(svc *notification.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromContext(r.Context())
		n, err := svc.UnreadCount(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "count failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"count": n}})
	}
}

func markNotificationReadHandler(svc *notification.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromContext(r.Context())
		id := chi.URLParam(r, "id")
		if err := svc.MarkRead(r.Context(), id, userID); err != nil {
			if errors.Is(err, notification.ErrNotFound) {
				writeError(w, http.StatusNotFound, "notification not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "mark failed")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func markAllNotificationsReadHandler(svc *notification.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromContext(r.Context())
		n, err := svc.MarkAllRead(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "mark all failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"updated": n}})
	}
}
