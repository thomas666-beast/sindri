package wallet

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thomas666-beast/sindri/internal/db/dbgen"
)

var (
	ErrInsufficientFunds = errors.New("insufficient available balance")
	ErrInvalidAmount     = errors.New("amount must be positive")
	ErrKYCRequired       = errors.New("KYC verification required")
)

type Balance struct {
	CurrencyCode string `json:"currency_code"`
	Available    int64  `json:"available"`
	Held         int64  `json:"held"`
}

type Service struct {
	pool *pgxpool.Pool
	q    dbgen.Querier
}

type Transaction struct {
	ID            string `json:"id"`
	Type          string `json:"type"`
	Direction     string `json:"direction"`
	Amount        int64  `json:"amount"`
	CurrencyCode  string `json:"currency_code"`
	BalanceAfter  int64  `json:"balance_after"`
	OrderID       string `json:"order_id,omitempty"`
	Status        string `json:"status"`
	Metadata      string `json:"metadata"`
	CreatedAtUnix int64  `json:"created_at_unix"`
}

type TransactionPage struct {
	Transactions []Transaction `json:"transactions"`
	NextCursor   string        `json:"next_cursor,omitempty"`
	HasMore      bool          `json:"has_more"`
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, q: dbgen.New(pool)}
}

// GetBalances returns every wallet the user holds, one per currency.
func (s *Service) GetBalances(ctx context.Context, userID string) ([]Balance, error) {
	rows, err := s.q.ListWalletsByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list wallets: %w", err)
	}
	out := make([]Balance, 0, len(rows))
	for _, r := range rows {
		out = append(out, Balance{
			CurrencyCode: r.CurrencyCode,
			Available:    r.Available,
			Held:         r.Held,
		})
	}
	return out, nil
}

// Deposit adds funds to a user's wallet in the given currency, creating the
// wallet on first use. Idempotent on idemKey.
func (s *Service) Deposit(ctx context.Context, userID, currency string, amount int64, idemKey string) (*Balance, error) {
	if amount <= 0 {
		return nil, ErrInvalidAmount
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	q := dbgen.New(tx)

	if idemKey != "" {
		existing, err := q.GetTransactionByIdempotencyKey(ctx, ptrString(idemKey))
		if err == nil && existing.Status == "completed" {
			bal, _ := q.GetWalletByUserAndCurrency(ctx, dbgen.GetWalletByUserAndCurrencyParams{
				UserID: userID, CurrencyCode: currency,
			})
			return &Balance{
				CurrencyCode: bal.CurrencyCode,
				Available:    bal.Available,
				Held:         bal.Held,
			}, nil
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("check idempotency: %w", err)
		}
	}

	w, err := q.GetWalletByUserAndCurrency(ctx, dbgen.GetWalletByUserAndCurrencyParams{
		UserID: userID, CurrencyCode: currency,
	})
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("get wallet: %w", err)
		}
		w, err = q.CreateWallet(ctx, dbgen.CreateWalletParams{
			UserID: userID, CurrencyCode: currency,
		})
		if err != nil {
			return nil, fmt.Errorf("create wallet: %w", err)
		}
	}

	newAvail, err := q.AddToAvailable(ctx, dbgen.AddToAvailableParams{
		ID: w.ID, Available: amount,
	})
	if err != nil {
		return nil, fmt.Errorf("add available: %w", err)
	}

	_, err = q.InsertWalletTransaction(ctx, dbgen.InsertWalletTransactionParams{
		WalletID:       w.ID,
		UserID:         userID,
		CurrencyCode:   currency,
		Type:           "deposit",
		Direction:      "credit",
		Amount:         amount,
		BalanceAfter:   newAvail,
		IdempotencyKey: ptrString(idemKey),
		Metadata:       []byte("{}"),
	})
	if err != nil {
		return nil, fmt.Errorf("insert txn: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &Balance{
		CurrencyCode: currency,
		Available:    newAvail,
		Held:         w.Held,
	}, nil
}

// Withdraw reduces available balance immediately.
// ⚠️ INTERNAL ONLY — Do NOT expose this as a public HTTP endpoint.
func (s *Service) Withdraw(ctx context.Context, userID, currency string, amount int64, idemKey string) (*Balance, error) {
	if amount <= 0 {
		return nil, ErrInvalidAmount
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	q := dbgen.New(tx)

	if idemKey != "" {
		existing, err := q.GetTransactionByIdempotencyKey(ctx, ptrString(idemKey))
		if err == nil && existing.Status == "completed" {
			bal, _ := q.GetWalletByUserAndCurrency(ctx, dbgen.GetWalletByUserAndCurrencyParams{
				UserID: userID, CurrencyCode: currency,
			})
			return &Balance{
				CurrencyCode: bal.CurrencyCode,
				Available:    bal.Available,
				Held:         bal.Held,
			}, nil
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("check idempotency: %w", err)
		}
	}

	w, err := q.GetWalletByUserAndCurrency(ctx, dbgen.GetWalletByUserAndCurrencyParams{
		UserID: userID, CurrencyCode: currency,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInsufficientFunds
		}
		return nil, fmt.Errorf("get wallet: %w", err)
	}

	newAvail, err := q.SubtractFromAvailable(ctx, dbgen.SubtractFromAvailableParams{
		ID: w.ID, Available: amount,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInsufficientFunds
		}
		return nil, fmt.Errorf("subtract available: %w", err)
	}

	_, err = q.InsertWalletTransaction(ctx, dbgen.InsertWalletTransactionParams{
		WalletID:       w.ID,
		UserID:         userID,
		CurrencyCode:   currency,
		Type:           "withdraw",
		Direction:      "debit",
		Amount:         amount,
		BalanceAfter:   newAvail,
		IdempotencyKey: ptrString(idemKey),
		Metadata:       []byte("{}"),
	})
	if err != nil {
		return nil, fmt.Errorf("insert txn: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &Balance{
		CurrencyCode: currency,
		Available:    newAvail,
		Held:         w.Held,
	}, nil
}

func (s *Service) ListTransactions(
	ctx context.Context,
	userID, currency, cursor string,
	limit int32,
) (*TransactionPage, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}

	params := dbgen.ListTransactionsForUserParams{
		UserID:   userID,
		Currency: currency,
		Lim:      limit + 1,
	}

	if cursor != "" {
		t, id, err := decodeTxCursor(cursor)
		if err != nil {
			return nil, err
		}
		params.CursorTs = pgtype.Timestamptz{Time: t, Valid: true}
		params.CursorID = pgUUID(id)
	}

	rows, err := s.q.ListTransactionsForUser(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}

	hasMore := len(rows) > int(limit)
	if hasMore {
		rows = rows[:limit]
	}

	out := make([]Transaction, 0, len(rows))
	for _, r := range rows {
		out = append(out, Transaction{
			ID:            r.ID,
			Type:          r.Type,
			Direction:     r.Direction,
			Amount:        r.Amount,
			CurrencyCode:  r.CurrencyCode,
			BalanceAfter:  r.BalanceAfter,
			OrderID:       pgUUIDString(r.OrderID),
			Status:        r.Status,
			Metadata:      string(r.Metadata),
			CreatedAtUnix: r.CreatedAt.Unix(),
		})
	}

	page := &TransactionPage{Transactions: out, HasMore: hasMore}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		page.NextCursor = encodeTxCursor(last.CreatedAt, last.ID)
	}
	return page, nil
}

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
