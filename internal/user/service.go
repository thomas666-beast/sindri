package user

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"

	kitpassword "github.com/thomas666-beast/cryptkit/password"

	"github.com/thomas666-beast/sindri/internal/db/dbgen"
)

var (
	ErrEmailTaken      = errors.New("email already registered")
	ErrUsernameTaken   = errors.New("username already taken")
	ErrInvalidCreds    = errors.New("invalid email or password")
	ErrWeakPassword    = errors.New("password must be 8-72 characters")
	ErrInvalidUsername = errors.New("username must be 3-32 lowercase letters, digits, or underscores")
	ErrNotFound        = errors.New("user not found")
)

// maxPasswordBytes caps the byte length of accepted passwords. It mirrors
// bcrypt's silent truncation at 72 bytes so that any password accepted at
// registration is also usable by legacy bcrypt hashes during the migration
// window. Once every stored hash is Argon2id, this can be relaxed — Argon2id
// has no such limit.
const maxPasswordBytes = 72

type PublicUser struct {
	ID                string `json:"id"`
	Email             string `json:"email"`
	Username          string `json:"username"`
	PreferredCurrency string `json:"preferred_currency"`
	IsAdmin           bool   `json:"is_admin"`
	KYCStatus         string `json:"kyc_status"`
}

type Service struct {
	q dbgen.Querier
}

func NewService(q dbgen.Querier) *Service {
	return &Service{q: q}
}

type RegisterInput struct {
	Email             string
	Username          string
	Password          string
	PreferredCurrency string
}

func (s *Service) Register(ctx context.Context, in RegisterInput) (*PublicUser, error) {
	in.Email = strings.TrimSpace(strings.ToLower(in.Email))
	in.Username = strings.TrimSpace(strings.ToLower(in.Username))
	if len(in.Password) < 8 {
		return nil, ErrWeakPassword
	}
	if len([]byte(in.Password)) > maxPasswordBytes {
		// Argon2id itself has no length limit; the cap exists so that
		// every accepted password could, at some point, have been hashed
		// by a legacy bcrypt path (which truncates at 72 bytes).
		return nil, ErrWeakPassword
	}
	if in.PreferredCurrency == "" {
		in.PreferredCurrency = "USD"
	}

	hash, err := kitpassword.HashPassword(in.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	row, err := s.q.CreateUser(ctx, dbgen.CreateUserParams{
		Email:             in.Email,
		PasswordHash:      hash,
		Username:          in.Username,
		PreferredCurrency: in.PreferredCurrency,
	})
	if err != nil {
		if isUniqueViolation(err, "users_email_key") {
			return nil, ErrEmailTaken
		}
		if isUniqueViolation(err, "users_username_key") {
			return nil, ErrUsernameTaken
		}
		if isCheckViolation(err, "users_username_format") {
			return nil, ErrInvalidUsername
		}
		return nil, fmt.Errorf("create user: %w", err)
	}

	return &PublicUser{
		ID:                row.ID,
		Email:             row.Email,
		Username:          row.Username,
		PreferredCurrency: row.PreferredCurrency,
		IsAdmin:           row.IsAdmin,
		KYCStatus:         row.KycStatus,
	}, nil
}

func (s *Service) Authenticate(ctx context.Context, email, pw string) (*PublicUser, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	row, err := s.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidCreds
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	if !row.IsActive {
		return nil, ErrInvalidCreds
	}

	// cryptkit.VerifyPassword handles both PHC Argon2id hashes and legacy
	// bcrypt ($2a$/$2b$/$2y$) hashes, so existing users can still log in
	// during the migration.
	if err := kitpassword.VerifyPassword(pw, row.PasswordHash); err != nil {
		return nil, ErrInvalidCreds
	}

	// Opportunistic rehash: if the stored hash is legacy bcrypt or uses
	// weaker Argon2 params than the current default, upgrade it now.
	// Failure is logged but does NOT fail the login — the user is already
	// authenticated, and the next successful login can retry.
	if kitpassword.NeedsRehash(row.PasswordHash) {
		if newHash, err := kitpassword.HashPassword(pw); err == nil {
			if uerr := s.q.UpdatePasswordHash(ctx, dbgen.UpdatePasswordHashParams{
				ID:           row.ID,
				PasswordHash: newHash,
			}); uerr != nil {
				slog.Warn("opportunistic password rehash failed",
					"user_id", row.ID, "error", uerr)
			}
		} else {
			slog.Warn("opportunistic password rehash (hash) failed",
				"user_id", row.ID, "error", err)
		}
	}

	return &PublicUser{
		ID:                row.ID,
		Email:             row.Email,
		Username:          row.Username,
		PreferredCurrency: row.PreferredCurrency,
		IsAdmin:           row.IsAdmin,
		KYCStatus:         row.KycStatus,
	}, nil
}

func (s *Service) GetByID(ctx context.Context, id string) (*PublicUser, error) {
	row, err := s.q.GetUserByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &PublicUser{
		ID:                row.ID,
		Email:             row.Email,
		Username:          row.Username,
		PreferredCurrency: row.PreferredCurrency,
		IsAdmin:           row.IsAdmin,
		KYCStatus:         row.KycStatus,
	}, nil
}

func (s *Service) GetByUsername(ctx context.Context, username string) (*PublicUser, error) {
	row, err := s.q.GetUserBasicByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get by username: %w", err)
	}
	return &PublicUser{
		ID:       row.ID,
		Username: row.Username,
	}, nil
}
