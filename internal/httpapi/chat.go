package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"fmt"
	"io"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"github.com/thomas666-beast/sindri/internal/auth"
	"github.com/thomas666-beast/sindri/internal/chat"
	"github.com/thomas666-beast/sindri/internal/httpapi/requests"
)

// newWSUpgrader builds an Upgrader whose CheckOrigin enforces the configured
// allowlist. Requests without an Origin header (curl, websocat, server-side
// clients) are permitted — they can't be CSWSH'd because they don't carry a
// victim's cookie automatically. Browser-initiated upgrades MUST match the
// allowlist exactly.
func newWSUpgrader(allowedOrigins []string) websocket.Upgrader {
	allow := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allow[o] = struct{}{}
	}
	return websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true // non-browser client
			}
			_, ok := allow[origin]
			return ok
		},
	}
}

func chatDownloadAttachmentHandler(svc *chat.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		me, _ := auth.UserIDFromContext(r.Context())
		attID := chi.URLParam(r, "id")

		data, filename, contentType, err := svc.DownloadAttachment(r.Context(), attID, me)
		if err != nil {
			if errors.Is(err, chat.ErrForbidden) {
				writeError(w, http.StatusForbidden, "not allowed")
				return
			}
			writeError(w, http.StatusInternalServerError, "download failed")
			return
		}

		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		w.Header().Set("Content-Type", contentType)
		w.Write(data)
	}
}

func chatUploadAttachmentHandler(svc *chat.Service, hub *chat.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		me, _ := auth.UserIDFromContext(r.Context())
		convID := chi.URLParam(r, "id")

		// Max 10MB
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			writeError(w, http.StatusBadRequest, "file too large or malformed")
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
			writeError(w, http.StatusInternalServerError, "could not read file")
			return
		}

		msg, err := svc.UploadAttachment(r.Context(), convID, me, header.Filename, header.Header.Get("Content-Type"), data)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "upload failed")
			return
		}

		// BROADCAST TO WEBSOCKET HERE
		hub.BroadcastMessage(convID, me, msg)

		writeJSON(w, http.StatusCreated, map[string]any{"data": msg})
	}
}

func chatStartDMHandler(svc *chat.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		me, _ := auth.UserIDFromContext(r.Context())

		var req requests.ChatDM
		if ok, status, body := bind(r, &req); !ok {
			writeJSON(w, status, body)
			return
		}

		var targetID string
		switch {
		case req.UserID != "":
			targetID = req.UserID
		case req.Username != "":
			id, err := svc.ResolveUserID(r.Context(), req.Username)
			if err != nil {
				if errors.Is(err, chat.ErrNotFound) {
					writeError(w, http.StatusBadRequest, "user not found")
					return
				}
				writeError(w, http.StatusInternalServerError, "resolve failed")
				return
			}
			targetID = id
		default:
			writeError(w, http.StatusBadRequest, "user_id or username required")
			return
		}

		c, err := svc.StartDM(r.Context(), me, targetID)
		if err != nil {
			if errors.Is(err, chat.ErrInvalid) {
				writeError(w, http.StatusBadRequest, "cannot DM yourself")
				return
			}
			writeError(w, http.StatusInternalServerError, "dm failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": c})
	}
}

func chatCreateGroupHandler(svc *chat.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		me, _ := auth.UserIDFromContext(r.Context())

		var req requests.ChatGroup
		if ok, status, body := bind(r, &req); !ok {
			writeJSON(w, status, body)
			return
		}

		members := append([]string{}, req.Members...)
		for _, uname := range req.Usernames {
			id, err := svc.ResolveUserID(r.Context(), uname)
			if err != nil {
				if errors.Is(err, chat.ErrNotFound) {
					writeError(w, http.StatusBadRequest, "user not found: "+uname)
					return
				}
				writeError(w, http.StatusInternalServerError, "resolve failed")
				return
			}
			members = append(members, id)
		}

		if len(members) == 0 {
			writeError(w, http.StatusBadRequest, "at least one member required")
			return
		}
		if len(members) > 100 {
			writeError(w, http.StatusBadRequest, "too many members (max 100)")
			return
		}

		c, err := svc.CreateGroup(r.Context(), me, req.Title, members)
		if err != nil {
			if errors.Is(err, chat.ErrInvalid) {
				writeError(w, http.StatusBadRequest, "invalid group")
				return
			}
			writeError(w, http.StatusInternalServerError, "create group failed")
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"data": c})
	}
}

func chatListMineHandler(svc *chat.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		me, _ := auth.UserIDFromContext(r.Context())
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		list, err := svc.ListMine(r.Context(), me, int32(limit), int32(offset))
		if err != nil {
			slog.Error("chat list mine", "error", err, "user_id", me)
			writeError(w, http.StatusInternalServerError, "list failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": list})
	}
}

func chatGetHandler(svc *chat.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		me, _ := auth.UserIDFromContext(r.Context())
		c, err := svc.Get(r.Context(), chi.URLParam(r, "id"), me)
		switch {
		case errors.Is(err, chat.ErrForbidden):
			writeError(w, http.StatusForbidden, "not a member")
		case errors.Is(err, chat.ErrNotFound):
			writeError(w, http.StatusNotFound, "not found")
		case err != nil:
			writeError(w, http.StatusInternalServerError, "get failed")
		default:
			writeJSON(w, http.StatusOK, map[string]any{"data": c})
		}
	}
}

func chatListMessagesHandler(svc *chat.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		me, _ := auth.UserIDFromContext(r.Context())
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		page, err := svc.ListMessages(r.Context(), chi.URLParam(r, "id"), me, r.URL.Query().Get("before"), int32(limit))
		switch {
		case errors.Is(err, chat.ErrForbidden):
			writeError(w, http.StatusForbidden, "not a member")
		case err != nil:
			writeError(w, http.StatusInternalServerError, "list failed")
		default:
			writeJSON(w, http.StatusOK, map[string]any{"data": page})
		}
	}
}

func chatSendMessageHandler(svc *chat.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		me, _ := auth.UserIDFromContext(r.Context())

		var req requests.ChatMessage
		if ok, status, body := bind(r, &req); !ok {
			writeJSON(w, status, body)
			return
		}

		msg, err := svc.SendMessage(r.Context(), chi.URLParam(r, "id"), me, req.Body)
		switch {
		case errors.Is(err, chat.ErrForbidden):
			writeError(w, http.StatusForbidden, "not a member")
		case errors.Is(err, chat.ErrInvalid):
			writeError(w, http.StatusBadRequest, "invalid message")
		case err != nil:
			writeError(w, http.StatusInternalServerError, "send failed")
		default:
			writeJSON(w, http.StatusCreated, map[string]any{"data": msg})
		}
	}
}

func chatMarkReadHandler(svc *chat.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		me, _ := auth.UserIDFromContext(r.Context())
		id := chi.URLParam(r, "id")
		err := svc.MarkRead(r.Context(), id, me)
		if err != nil {
			if errors.Is(err, chat.ErrForbidden) {
				writeError(w, http.StatusForbidden, "not a member")
				return
			}
			writeError(w, http.StatusInternalServerError, "mark read failed")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// chatWSHandler authenticates the WebSocket handshake via the HttpOnly
// "barter_token" cookie (falling back to Authorization: Bearer). The
// ?token= query-string fallback has been removed — tokens in URLs leak
// via access logs, proxy logs, browser history, and Referer headers.
//
// Origin checking is enforced by the upgrader built from allowedOrigins
// (see newWSUpgrader) to prevent Cross-Site WebSocket Hijacking.
func chatWSHandler(mgr *auth.Manager, hub *chat.Hub, allowedOrigins []string) http.HandlerFunc {
	upgrader := newWSUpgrader(allowedOrigins)
	return func(w http.ResponseWriter, r *http.Request) {
		token, err := mgr.ExtractToken(r)
		if err != nil || token == "" {
			writeError(w, http.StatusUnauthorized, "token required")
			return
		}
		claims, err := mgr.Verify(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		client := &chat.Client{UserID: claims.UserID, Conn: conn}
		hub.Serve(ctx, client)
	}
}

func unreadMessagesHandler(svc *chat.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		me, _ := auth.UserIDFromContext(r.Context())
		count, err := svc.UnreadCount(r.Context(), me)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "count failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": count})
	}
}
