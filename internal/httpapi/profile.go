package httpapi

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/thomas666-beast/sindri/internal/auth"
	"github.com/thomas666-beast/sindri/internal/httpapi/requests"
	"github.com/thomas666-beast/sindri/internal/profile"
)

var allowedAvatarMimes = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/webp": ".webp",
}

const maxAvatarBytes = 2 << 20 // 2 MiB

func getPublicProfileHandler(svc *profile.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := chi.URLParam(r, "username")
		p, err := svc.GetByUsername(r.Context(), username)
		if err != nil {
			if errors.Is(err, profile.ErrNotFound) {
				writeError(w, http.StatusNotFound, "profile not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "get failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": p})
	}
}

// NEW: Get Geek Stats
func getGeekStatsHandler(svc *profile.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := chi.URLParam(r, "username")
		
		// First, find the user ID from the username
		p, err := svc.GetByUsername(r.Context(), username)
		if err != nil {
			if errors.Is(err, profile.ErrNotFound) {
				writeError(w, http.StatusNotFound, "profile not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "lookup failed")
			return
		}

		// Then fetch the stats
		stats, err := svc.GetStats(r.Context(), p.UserID)
		if err != nil {
			slog.Error("get geek stats", "error", err, "user_id", p.UserID)
			writeError(w, http.StatusInternalServerError, "stats failed")
			return
		}
		
		writeJSON(w, http.StatusOK, map[string]any{"data": stats})
	}
}

func getMyProfileHandler(svc *profile.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromContext(r.Context())
		p, err := svc.GetByUserID(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "get failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": p})
	}
}

func updateMyProfileHandler(svc *profile.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromContext(r.Context())

		var req requests.UpdateProfile
		if ok, status, body := bind(r, &req); !ok {
			writeJSON(w, status, body)
			return
		}

		p, err := svc.Upsert(r.Context(), profile.UpsertInput{
			UserID:      userID,
			DisplayName: req.DisplayName,
			Bio:         req.Bio,
			Skills:      req.Skills,
			Interests:   req.Interests,
			AvatarURL:   req.AvatarURL,
		})
		if err != nil {
			if errors.Is(err, profile.ErrInvalidInput) {
				writeError(w, http.StatusUnprocessableEntity, "invalid profile")
				return
			}
			writeError(w, http.StatusInternalServerError, "update failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": p})
	}
}

func uploadAvatarHandler(svc *profile.Service, uploadsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromContext(r.Context())

		if err := r.ParseMultipartForm(maxAvatarBytes); err != nil {
			writeError(w, http.StatusBadRequest, "file too large or malformed")
			return
		}

		file, header, err := r.FormFile("avatar")
		if err != nil {
			writeError(w, http.StatusBadRequest, "avatar file required")
			return
		}
		defer file.Close()

		if header.Size > maxAvatarBytes {
			writeError(w, http.StatusBadRequest, "avatar must be under 2MB")
			return
		}

		// Detect MIME from magic bytes, not Content-Type header
		head := make([]byte, 512)
		n, err := file.Read(head)
		if err != nil && n == 0 {
			writeError(w, http.StatusBadRequest, "cannot read file")
			return
		}
		mime := http.DetectContentType(head[:n])
		ext, ok := allowedAvatarMimes[mime]
		if !ok {
			writeError(w, http.StatusBadRequest, "avatar must be PNG, JPEG, or WEBP")
			return
		}

		// Rewind
		if _, err := file.Seek(0, 0); err != nil {
			writeError(w, http.StatusInternalServerError, "cannot process file")
			return
		}

		// Write to disk with UUID filename
		name := uuid.NewString() + ext
		dest := filepath.Join(uploadsDir, "avatars", name)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			writeError(w, http.StatusInternalServerError, "cannot create storage")
			return
		}

		out, err := os.Create(dest)
		if err != nil {
			slog.Error("avatar os.Create failed", "path", dest, "error", err)
			writeError(w, http.StatusInternalServerError, "cannot save file")
			return
		}
		defer out.Close()
		if _, err := io.Copy(out, file); err != nil {
			slog.Error("avatar io.Copy failed", "path", dest, "error", err)
			writeError(w, http.StatusInternalServerError, "cannot write file")
			return
		}

		url := "/uploads/avatars/" + name
		if err := svc.SetAvatarURL(r.Context(), userID, url); err != nil {
			// Roll back file
			_ = os.Remove(dest)
			writeError(w, http.StatusInternalServerError, "cannot update profile")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]string{"avatar_url": url}})
	}
}
