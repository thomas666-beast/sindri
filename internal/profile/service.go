package profile

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/thomas666-beast/sindri/internal/db/dbgen"
)

var (
	ErrNotFound     = errors.New("profile not found")
	ErrInvalidInput = errors.New("invalid profile input")
)

type Profile struct {
	UserID           string   `json:"user_id"`
	Username         string   `json:"username"`
	DisplayName      string   `json:"display_name"`
	Bio              string   `json:"bio"`
	Skills           []string `json:"skills"`
	Interests        []string `json:"interests"`
	AvatarURL        string   `json:"avatar_url,omitempty"`
	MemberSince      int64    `json:"member_since_unix"`
	HasCustomProfile bool     `json:"has_custom_profile"`
}

type Service struct {
	q dbgen.Querier
}

func NewService(q dbgen.Querier) *Service { return &Service{q: q} }

func (s *Service) GetByUsername(ctx context.Context, username string) (*Profile, error) {
	row, err := s.q.GetProfile(ctx, username)
	if err == nil {
		return &Profile{
			UserID:           row.UserID,
			Username:         row.Username,
			DisplayName:      row.DisplayName,
			Bio:              row.Bio,
			Skills:           row.Skills,
			Interests:        row.Interests,
			AvatarURL:        pgText(row.AvatarUrl),
			MemberSince:      row.UserCreatedAt.Unix(),
			HasCustomProfile: true,
		}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("get profile: %w", err)
	}

	u, err := s.q.GetUserBasicByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &Profile{
		UserID:           u.ID,
		Username:         u.Username,
		DisplayName:      u.Username,
		Bio:              "",
		Skills:           []string{},
		Interests:        []string{},
		MemberSince:      u.CreatedAt.Unix(),
		HasCustomProfile: false,
	}, nil
}

func (s *Service) GetByUserID(ctx context.Context, userID string) (*Profile, error) {
	row, err := s.q.GetProfileByUserID(ctx, userID)
	if err == nil {
		return &Profile{
			UserID:           row.UserID,
			Username:         row.Username,
			DisplayName:      row.DisplayName,
			Bio:              row.Bio,
			Skills:           row.Skills,
			Interests:        row.Interests,
			AvatarURL:        pgText(row.AvatarUrl),
			MemberSince:      row.UserCreatedAt.Unix(),
			HasCustomProfile: true,
		}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("get profile by id: %w", err)
	}

	u, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &Profile{
		UserID:           u.ID,
		Username:         u.Username,
		DisplayName:      u.Username,
		Bio:              "",
		Skills:           []string{},
		Interests:        []string{},
		MemberSince:      u.CreatedAt.Unix(),
		HasCustomProfile: false,
	}, nil
}

type UpsertInput struct {
	UserID      string
	DisplayName string
	Bio         string
	Skills      []string
	Interests   []string
	AvatarURL   string
}

func (s *Service) Upsert(ctx context.Context, in UpsertInput) (*Profile, error) {
	if len(in.DisplayName) < 1 || len(in.DisplayName) > 100 {
		return nil, ErrInvalidInput
	}
	if len(in.Bio) > 2000 {
		return nil, ErrInvalidInput
	}
	if len(in.Skills) > 30 || len(in.Interests) > 30 {
		return nil, ErrInvalidInput
	}

	row, err := s.q.UpsertProfile(ctx, dbgen.UpsertProfileParams{
		UserID:      in.UserID,
		DisplayName: in.DisplayName,
		Bio:         in.Bio,
		Skills:      in.Skills,
		Interests:   in.Interests,
		AvatarUrl:   ptrString(in.AvatarURL),
	})
	if err != nil {
		return nil, fmt.Errorf("upsert profile: %w", err)
	}

	return s.GetByUserID(ctx, row.UserID)
}

func (s *Service) SetAvatarURL(ctx context.Context, userID, avatarURL string) error {
	row, err := s.q.GetProfileByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// No profile yet — create one with defaults
			_, err := s.q.UpsertProfile(ctx, dbgen.UpsertProfileParams{
				UserID:      userID,
				DisplayName: "user",
				Bio:         "",
				Skills:      []string{},
				Interests:   []string{},
				AvatarUrl:   ptrString(avatarURL),
			})
			return err
		}
		return fmt.Errorf("get profile: %w", err)
	}
	_, err = s.q.UpsertProfile(ctx, dbgen.UpsertProfileParams{
		UserID:      userID,
		DisplayName: row.DisplayName,
		Bio:         row.Bio,
		Skills:      row.Skills,
		Interests:   row.Interests,
		AvatarUrl:   ptrString(avatarURL),
	})
	return err
}

// ---- helpers ----

func pgText(t *string) string {
	if t == nil {
		return ""
	}
	return *t
}

func ptrString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
