package auth

import "context"

// UserChecker is the minimal contract RequireAuth needs to enforce
// account state on every request. Implementations live outside this
// package (e.g. backed by dbgen.Queries) so auth stays decoupled from
// the persistence layer.
//
// Return values:
//   - isActive = false, err = nil → user exists but is disabled → 401
//   - err != nil                  → DB failure → 500
//   - isActive = true             → request continues
type UserChecker interface {
	CheckActive(ctx context.Context, userID string) (isActive bool, isAdmin bool, err error)
}
