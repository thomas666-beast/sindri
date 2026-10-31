package withdrawal

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thomas666-beast/sindri/internal/db/dbgen"
	"github.com/thomas666-beast/sindri/internal/wallet"
)

var (
	ErrNotFound                 = errors.New("withdrawal request not found")
	ErrInvalidAmount            = errors.New("amount must be positive")
	ErrInsufficientFunds        = wallet.ErrInsufficientFunds
	ErrAlreadyReviewed          = errors.New("withdrawal request already reviewed")
	ErrReasonRequired           = errors.New("rejection reason is required")
	ErrSystemUserCannotWithdraw = errors.New("system account cannot withdraw")
)

type Notifier interface {
	Notify(ctx context.Context, userID, kind, title, body string, metadata []byte) error
}

type Withdrawal struct {
	ID                 string `json:"id"`
	UserID             string `json:"user_id"`
	Username           string `json:"username"`
	CurrencyCode       string `json:"currency_code"`
	Amount             int64  `json:"amount"`
	Status             string `json:"status"`
	RejectionReason    string `json:"rejection_reason,omitempty"`
	ReviewedBy         string `json:"reviewed_by,omitempty"`
	ReviewedByUsername string `json:"reviewed_by_username,omitempty"`
	ReviewedAtUnix     *int64 `json:"reviewed_at_unix,omitempty"`
	CreatedAtUnix      int64  `json:"created_at_unix"`
}

type Service struct {
	pool     *pgxpool.Pool
	q        dbgen.Querier
	notifier Notifier
}

func NewService(pool *pgxpool.Pool, notifier Notifier) *Service {
	return &Service{pool: pool, q: dbgen.New(pool), notifier: notifier}
}

func (s *Service) RequestWithdrawal(ctx context.Context, userID, currency string, amount int64) (*Withdrawal, error) {
	if amount <= 0 {
		return nil, ErrInvalidAmount
	}
	if len(currency) != 3 {
		return nil, fmt.Errorf("currency must be 3 chars")
	}

	// System account must never withdraw — it exists only to hold commission.
	if isSystem, err := s.q.IsSystemUser(ctx, userID); err != nil {
		return nil, fmt.Errorf("check system user: %w", err)
	} else if isSystem {
		return nil, ErrSystemUserCannotWithdraw
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)

	w, err := q.GetWalletByUserAndCurrency(ctx, dbgen.GetWalletByUserAndCurrencyParams{
		UserID: userID, CurrencyCode: currency,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInsufficientFunds
		}
		return nil, fmt.Errorf("get wallet: %w", err)
	}

	held, err := q.HoldAvailable(ctx, dbgen.HoldAvailableParams{
		ID: w.ID, Available: amount,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInsufficientFunds
		}
		return nil, fmt.Errorf("hold funds: %w", err)
	}

	if _, err := q.InsertWalletTransaction(ctx, dbgen.InsertWalletTransactionParams{
		WalletID:     w.ID,
		UserID:       userID,
		CurrencyCode: currency,
		Type:         "withdraw",
		Direction:    "debit",
		Amount:       amount,
		BalanceAfter: held.Available,
		Metadata:     []byte(`{"phase":"request"}`),
	}); err != nil {
		return nil, fmt.Errorf("insert hold txn: %w", err)
	}

	row, err := q.CreateWithdrawalRequest(ctx, dbgen.CreateWithdrawalRequestParams{
		UserID: userID, CurrencyCode: currency, Amount: amount,
	})
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	
	// We don't have the username here, so pass empty strings
	return toDTO(row.ID, row.UserID, "", row.CurrencyCode, row.Amount, row.Status,
		pgTextString(row.RejectionReason), pgUUIDString(row.ReviewedBy), "",
		pgTimeInt64(row.ReviewedAt), row.CreatedAt.Unix()), nil
}

func (s *Service) ListMine(ctx context.Context, userID string, limit, offset int32) ([]Withdrawal, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.q.ListWithdrawalsForUser(ctx, dbgen.ListWithdrawalsForUserParams{
		UserID: userID, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}
	out := make([]Withdrawal, 0, len(rows))
	for _, r := range rows {
		// ListMine doesn't join usernames, so we pass empty strings
		out = append(out, *toDTO(r.ID, r.UserID, "", r.CurrencyCode, r.Amount, r.Status,
			pgTextString(r.RejectionReason), pgUUIDString(r.ReviewedBy), "",
			pgTimeInt64(r.ReviewedAt), r.CreatedAt.Unix()))
	}
	return out, nil
}

func (s *Service) AdminList(ctx context.Context, status, idFilter string, limit, offset int32) ([]Withdrawal, error) {
	if status != "" && status != "pending" && status != "approved" && status != "rejected" {
		return nil, fmt.Errorf("invalid status filter")
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	
	rows, err := s.q.AdminListWithdrawals(ctx, dbgen.AdminListWithdrawalsParams{
		StatusFilter: status, 
        IDFilter:     idFilter,
		Limit:        limit, 
		Offset:       offset,
	})
	if err != nil {
		return nil, fmt.Errorf("admin list withdrawals: %w", err)
	}
	
	out := make([]Withdrawal, 0, len(rows))
	for _, r := range rows {
		out = append(out, *toDTO(
			r.ID, r.UserID, r.Username, r.CurrencyCode, r.Amount, r.Status,
			pgTextString(r.RejectionReason), pgUUIDString(r.ReviewedBy), r.ReviewedByUsername,
			pgTimeInt64(r.ReviewedAt), r.CreatedAt.Unix(),
		))
	}
	return out, nil
}

func (s *Service) AdminApprove(ctx context.Context, requestID, adminID string) (*Withdrawal, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)

	req, err := q.GetWithdrawalRequest(ctx, requestID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get request: %w", err)
	}
	if req.Status != "pending" {
		return nil, ErrAlreadyReviewed
	}

	w, err := q.GetWalletByUserAndCurrency(ctx, dbgen.GetWalletByUserAndCurrencyParams{
		UserID: req.UserID, CurrencyCode: req.CurrencyCode,
	})
	if err != nil {
		return nil, fmt.Errorf("get wallet: %w", err)
	}

	if _, err := q.ReleaseHeld(ctx, dbgen.ReleaseHeldParams{
		ID: w.ID, Held: req.Amount,
	}); err != nil {
		return nil, fmt.Errorf("release held: %w", err)
	}

	if _, err := q.InsertWalletTransaction(ctx, dbgen.InsertWalletTransactionParams{
		WalletID:     w.ID,
		UserID:       req.UserID,
		CurrencyCode: req.CurrencyCode,
		Type:         "withdraw",
		Direction:    "debit",
		Amount:       req.Amount,
		BalanceAfter: w.Available,
		Metadata:     []byte(`{"phase":"approved"}`),
	}); err != nil {
		return nil, fmt.Errorf("insert withdraw txn: %w", err)
	}

	updated, err := q.ApproveWithdrawal(ctx, dbgen.ApproveWithdrawalParams{
		ID: requestID, ReviewedBy: pgUUID(adminID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAlreadyReviewed
		}
		return nil, fmt.Errorf("approve: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	if s.notifier != nil {
		meta := []byte(fmt.Sprintf(`{"withdrawal_id":"%s"}`, requestID))
		_ = s.notifier.Notify(ctx, req.UserID, "withdrawal.approved",
			"Withdrawal approved",
			fmt.Sprintf("Your withdrawal of %d %s has been processed.", req.Amount, req.CurrencyCode),
			meta)
	}

	return toDTO(updated.ID, updated.UserID, "", updated.CurrencyCode, updated.Amount,
		updated.Status, "", adminID, "", nil, 0), nil
}

func (s *Service) AdminReject(ctx context.Context, requestID, adminID, reason string) (*Withdrawal, error) {
	if len(reason) < 3 || len(reason) > 1000 {
		return nil, ErrReasonRequired
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)

	req, err := q.GetWithdrawalRequest(ctx, requestID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get request: %w", err)
	}
	if req.Status != "pending" {
		return nil, ErrAlreadyReviewed
	}

	w, err := q.GetWalletByUserAndCurrency(ctx, dbgen.GetWalletByUserAndCurrencyParams{
		UserID: req.UserID, CurrencyCode: req.CurrencyCode,
	})
	if err != nil {
		return nil, fmt.Errorf("get wallet: %w", err)
	}

	if _, err := q.ReleaseHeld(ctx, dbgen.ReleaseHeldParams{
		ID: w.ID, Held: req.Amount,
	}); err != nil {
		return nil, fmt.Errorf("release held: %w", err)
	}
	if _, err := q.AddAvailable(ctx, dbgen.AddAvailableParams{
		ID: w.ID, Available: req.Amount,
	}); err != nil {
		return nil, fmt.Errorf("credit available: %w", err)
	}

	if _, err := q.InsertWalletTransaction(ctx, dbgen.InsertWalletTransactionParams{
		WalletID:     w.ID,
		UserID:       req.UserID,
		CurrencyCode: req.CurrencyCode,
		Type:         "withdraw",
		Direction:    "credit",
		Amount:       req.Amount,
		BalanceAfter: w.Available + req.Amount,
		Metadata:     []byte(`{"phase":"rejected"}`),
	}); err != nil {
		return nil, fmt.Errorf("insert reversal txn: %w", err)
	}

	updated, err := q.RejectWithdrawal(ctx, dbgen.RejectWithdrawalParams{
		ID: requestID, ReviewedBy: pgUUID(adminID), RejectionReason: &reason,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAlreadyReviewed
		}
		return nil, fmt.Errorf("reject: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	if s.notifier != nil {
		meta := []byte(fmt.Sprintf(`{"withdrawal_id":"%s"}`, requestID))
		_ = s.notifier.Notify(ctx, req.UserID, "withdrawal.rejected",
			"Withdrawal rejected",
			fmt.Sprintf("Reason: %s", reason),
			meta)
	}

	return toDTO(updated.ID, updated.UserID, "", updated.CurrencyCode, updated.Amount,
		updated.Status, reason, adminID, "", nil, 0), nil
}

// ---- helpers ----

func pgUUID(s string) pgtype.UUID {
	var u pgtype.UUID
	_ = u.Scan(s)
	return u
}

func pgUUIDString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	s, _ := u.Value()
	if str, ok := s.(string); ok {
		return str
	}
	return ""
}

func pgTextString(t *string) string {
	if t == nil {
		return ""
	}
	return *t
}

func pgTimeInt64(t pgtype.Timestamptz) *int64 {
	if !t.Valid {
		return nil
	}
	v := t.Time.Unix()
	return &v
}

func toDTO(
	id, userID, username, currency string,
	amount int64,
	status, reason, reviewedBy, reviewedByUsername string,
	reviewedAt *int64,
	createdAt int64,
) *Withdrawal {
	return &Withdrawal{
		ID:                 id,
		UserID:             userID,
		Username:           username,
		CurrencyCode:       currency,
		Amount:             amount,
		Status:             status,
		RejectionReason:    reason,
		ReviewedBy:         reviewedBy,
		ReviewedByUsername: reviewedByUsername,
		ReviewedAtUnix:     reviewedAt,
		CreatedAtUnix:      createdAt,
	}
}
