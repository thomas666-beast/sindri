package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/thomas666-beast/sindri/internal/db/dbgen"
)

// DBUserChecker implements UserChecker using the sqlc-generated queries.
// Lives in the auth package so it's easy to find; depends on dbgen for the
// concrete query, keeping auth from having to import anything else.
type DBUserChecker struct {
	q dbgen.Querier
}

func NewDBUserChecker(q dbgen.Querier) *DBUserChecker {
	return &DBUserChecker{q: q}
}

func (c *DBUserChecker) CheckActive(ctx context.Context, userID string) (bool, bool, error) {
	row, err := c.q.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// User was deleted — treat as inactive, not an error.
			return false, false, nil
		}
		return false, false, fmt.Errorf("get user: %w", err)
	}
	return row.IsActive, row.IsAdmin, nil
}
