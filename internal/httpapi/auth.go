package httpapi

import (
	"errors"
	"net/http"

	"github.com/thomas666-beast/sindri/internal/auth"
	"github.com/thomas666-beast/sindri/internal/httpapi/requests"
	"github.com/thomas666-beast/sindri/internal/user"
)

type authResponse struct {
	User  *user.PublicUser `json:"user"`
	Token string           `json:"token"`
}

func registerHandler(svc *user.Service, mgr *auth.Manager, cookieSecure bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req requests.Register
		if ok, status, body := bind(r, &req); !ok {
			writeJSON(w, status, body)
			return
		}

		u, err := svc.Register(r.Context(), user.RegisterInput{
			Email:    req.Email,
			Username: req.Username,
			Password: req.Password,
		})
		switch {
		case errors.Is(err, user.ErrEmailTaken):
			writeError(w, http.StatusConflict, "email already registered")
			return
		case errors.Is(err, user.ErrUsernameTaken):
			writeError(w, http.StatusConflict, "username already taken")
			return
		case errors.Is(err, user.ErrWeakPassword):
			writeError(w, http.StatusBadRequest, user.ErrWeakPassword.Error())
			return
		case errors.Is(err, user.ErrInvalidUsername):
			writeError(w, http.StatusUnprocessableEntity, user.ErrInvalidUsername.Error())
			return
		case err != nil:
			writeError(w, http.StatusInternalServerError, "registration failed")
			return
		}

		token, err := mgr.Issue(u.ID, u.IsAdmin)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "token issuance failed")
			return
		}
		mgr.SetCookie(w, token, cookieSecure)
		writeJSON(w, http.StatusCreated, authResponse{User: u, Token: token})
	}
}

func loginHandler(svc *user.Service, mgr *auth.Manager, cookieSecure bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req requests.Login
		if ok, status, body := bind(r, &req); !ok {
			writeJSON(w, status, body)
			return
		}

		u, err := svc.Authenticate(r.Context(), req.Email, req.Password)
		if err != nil {
			if errors.Is(err, user.ErrInvalidCreds) {
				writeError(w, http.StatusUnauthorized, "invalid email or password")
				return
			}
			writeError(w, http.StatusInternalServerError, "login failed")
			return
		}

		token, err := mgr.Issue(u.ID, u.IsAdmin)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "token issuance failed")
			return
		}
		mgr.SetCookie(w, token, cookieSecure)
		writeJSON(w, http.StatusOK, authResponse{User: u, Token: token})
	}
}

func logoutHandler(mgr *auth.Manager, cookieSecure bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mgr.ClearCookie(w, cookieSecure)
		w.WriteHeader(http.StatusNoContent)
	}
}

func meHandler(svc *user.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := auth.UserIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		u, err := svc.GetByID(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load user")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"user": u})
	}
}
