package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrMissingToken = errors.New("missing authorization token")
	ErrInvalidToken = errors.New("invalid authorization token")
)

type contextKey string

const (
	userIDKey contextKey = "auth.user_id"
	adminKey  contextKey = "auth.is_admin"
)

type Claims struct {
	UserID  string `json:"uid"`
	IsAdmin bool   `json:"adm"`
	jwt.RegisteredClaims
}

type Manager struct {
	secret []byte
	ttl    time.Duration
}

func NewManager(secret string) *Manager {
	return &Manager{secret: []byte(secret), ttl: 24 * time.Hour}
}

func (m *Manager) Issue(userID string, isAdmin bool) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:  userID,
		IsAdmin: isAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
			Subject:   userID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *Manager) Verify(tokenStr string) (*Claims, error) {
	var claims Claims
	token, err := jwt.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	return &claims, nil
}

// ExtractBearer pulls "Bearer <token>" out of Authorization header.
func ExtractBearer(r *http.Request) (string, error) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "", ErrMissingToken
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", ErrInvalidToken
	}
	return parts[1], nil
}

func WithUser(ctx context.Context, userID string, isAdmin bool) context.Context {
	ctx = context.WithValue(ctx, userIDKey, userID)
	ctx = context.WithValue(ctx, adminKey, isAdmin)
	return ctx
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(userIDKey).(string)
	return v, ok
}

func IsAdminFromContext(ctx context.Context) bool {
	v, _ := ctx.Value(adminKey).(bool)
	return v
}

const (
	tokenCookieName = "barter_token"
	tokenCookieTTL  = 24 * time.Hour
)

// SetCookie writes the JWT into an HttpOnly cookie.
func (m *Manager) SetCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     tokenCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(tokenCookieTTL.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

// ClearCookie instructs the browser to delete the cookie.
func (m *Manager) ClearCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     tokenCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

// ExtractToken returns the JWT from the cookie if present, else falls back
// to the Authorization: Bearer header. Cookie wins.
func (m *Manager) ExtractToken(r *http.Request) (string, error) {
	if c, err := r.Cookie(tokenCookieName); err == nil && c.Value != "" {
		return c.Value, nil
	}
	return ExtractBearer(r)
}
