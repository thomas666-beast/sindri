package user

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/thomas666-beast/sindri/internal/db/dbgen"
)

var ErrSearchQueryTooShort = errors.New("search query must be at least 2 characters")

type SearchResult struct {
	ID                string   `json:"id"`
	Username          string   `json:"username"`
	DisplayName       string   `json:"display_name"`
	Bio               string   `json:"bio"`
	Skills            []string `json:"skills"`
	Interests         []string `json:"interests"`
	AvatarURL         string   `json:"avatar_url,omitempty"`
	PreferredCurrency string   `json:"preferred_currency"`
	MemberSince       int64    `json:"member_since_unix"`
}

type SearchPage struct {
	Results []SearchResult `json:"results"`
	Total   int64          `json:"total"`
	Limit   int32          `json:"limit"`
	Offset  int32          `json:"offset"`
}

// Search finds users whose username, bio, skills, or interests match q
// (case-insensitive substring). q must be at least 2 characters, or empty
// (empty = browse all, used by /geeks).
func (s *Service) Search(ctx context.Context, q string, limit, offset int32) (*SearchPage, error) {
	q = strings.TrimSpace(q)
	// Empty query = list all users (used by /geeks browse mode).
	// Non-empty queries must be at least 2 chars.
	if q != "" && len(q) < 2 {
		return nil, ErrSearchQueryTooShort
	}
	if len(q) > 100 {
		q = q[:100]
	}
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := s.q.SearchUsers(ctx, dbgen.SearchUsersParams{
		Q:   q,
		Lim: limit,
		Off: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("search users: %w", err)
	}
	total, err := s.q.CountSearchUsers(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("count search: %w", err)
	}

	results := make([]SearchResult, 0, len(rows))
	for _, r := range rows {
		var avatarURL string
		if r.AvatarUrl != nil {
			avatarURL = *r.AvatarUrl
		}
		displayName := r.DisplayName
		if displayName == "" {
			displayName = r.Username
		}
		skills := r.Skills
		if skills == nil {
			skills = []string{}
		}
		interests := r.Interests
		if interests == nil {
			interests = []string{}
		}
		results = append(results, SearchResult{
			ID:                r.ID,
			Username:          r.Username,
			DisplayName:       displayName,
			Bio:               r.Bio,
			Skills:            skills,
			Interests:         interests,
			AvatarURL:         avatarURL,
			PreferredCurrency: r.PreferredCurrency,
			MemberSince:       r.UserCreatedAt.Unix(),
		})
	}

	return &SearchPage{
		Results: results,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
	}, nil
}
