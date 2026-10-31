package order

import (
	"context"
	"errors"
	"fmt"
	"time"
	"encoding/json"

	"os"
	"path/filepath"
	"github.com/google/uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thomas666-beast/sindri/internal/crypto"
	"github.com/thomas666-beast/sindri/internal/db/dbgen"
	"github.com/thomas666-beast/sindri/internal/settings"
	"github.com/thomas666-beast/sindri/internal/wallet"
)

var (
	ErrNotFound          = errors.New("order not found")
	ErrNotHeld           = errors.New("order is not in held or disputed state")
	ErrSelfDealing       = errors.New("cannot buy your own product")
	ErrInsufficientFunds = wallet.ErrInsufficientFunds
	ErrForbidden         = errors.New("forbidden")
	ErrAlreadyDisputed   = errors.New("order is already disputed")
	ErrNotBuyerDispute   = errors.New("only the buyer can open a dispute")
	ErrCannotDispute     = errors.New("order cannot be disputed in its current state")
)

type Notifier interface {
	Notify(ctx context.Context, userID, kind, title, body string, metadata []byte) error
}

type Order struct {
	ID                 string `json:"id"`
	BuyerID            string `json:"buyer_id"`
	BuyerUsername      string `json:"buyer_username"`
	SellerID           string `json:"seller_id"`
	SellerUsername     string `json:"seller_username"`
	ProductID          string `json:"product_id,omitempty"`
	InvoiceTitle       string `json:"invoice_title,omitempty"`
	InvoiceDescription string `json:"invoice_description,omitempty"`
	CurrencyCode       string `json:"currency_code"`
	Amount             int64  `json:"amount"`
	CommissionAmount   int64  `json:"commission_amount"`
	CommissionPercent  int32  `json:"commission_percent"`
	Status             string `json:"status"`
	MyRole             string `json:"my_role,omitempty"`
	AutoReleaseAtUnix  *int64 `json:"auto_release_at_unix,omitempty"`
	ReleasedAtUnix     *int64 `json:"released_at_unix,omitempty"`
	ReleasedBy         string `json:"released_by,omitempty"`
	CreatedAtUnix      int64  `json:"created_at_unix"`

	DisputeReason           string `json:"dispute_reason,omitempty"`
	DisputeOpenedByUsername string `json:"dispute_opened_by_username,omitempty"`
}

type InvoiceListItem struct {
	Order
	CancelReason  string `json:"cancel_reason,omitempty"`
	CancelledByID string `json:"cancelled_by_id,omitempty"`
}

type Page struct {
	Orders     []Order `json:"orders"`
	NextCursor string  `json:"next_cursor,omitempty"`
	HasMore    bool    `json:"has_more"`
}

type DashboardStats struct {
	TotalEarned        int64 `json:"total_earned"`
	ActiveOrders       int64 `json:"active_orders"`
	PendingWithdrawals int64 `json:"pending_withdrawals"`
	ActiveProducts     int64 `json:"active_products"`
}

type Service struct {
	pool       *pgxpool.Pool
	q          dbgen.Querier
	settings   *settings.Service
	notifier   Notifier
	cipher     *crypto.Cipher
	uploadsDir string
}

func NewService(pool *pgxpool.Pool, st *settings.Service, notifier Notifier, c *crypto.Cipher, uploadsDir string) *Service {
	return &Service{
		pool: pool, q: dbgen.New(pool), settings: st, notifier: notifier,
		cipher: c, uploadsDir: uploadsDir,
	}
}


type CreateInput struct {
	BuyerID   string
	ProductID string
}

func (s *Service) CreateOrder(ctx context.Context, in CreateInput) (*Order, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)

	p, err := q.GetProduct(ctx, in.ProductID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get product: %w", err)
	}
	if p.SellerID == in.BuyerID {
		return nil, ErrSelfDealing
	}

	cpct, _ := s.settings.Int(ctx, "commission_percent", 5)
	commission := p.Price * int64(cpct) / 100

	w, err := q.GetWalletByUserAndCurrency(ctx, dbgen.GetWalletByUserAndCurrencyParams{
		UserID: in.BuyerID, CurrencyCode: p.CurrencyCode,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInsufficientFunds
		}
		return nil, fmt.Errorf("get wallet: %w", err)
	}

	held, err := q.HoldAvailable(ctx, dbgen.HoldAvailableParams{
		ID: w.ID, Available: p.Price,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInsufficientFunds
		}
		return nil, fmt.Errorf("hold funds: %w", err)
	}

	days, _ := s.settings.Int(ctx, "escrow_auto_release_days", 3)
	autoRelease := time.Now().Add(time.Duration(days) * 24 * time.Hour)

	row, err := q.CreateOrder(ctx, dbgen.CreateOrderParams{
		BuyerID:           in.BuyerID,
		SellerID:          p.SellerID,
		ProductID:         pgUUID(p.ID),
		CurrencyCode:      p.CurrencyCode,
		Amount:            p.Price,
		CommissionAmount:  commission,
		CommissionPercent: int32(cpct),
		AutoReleaseAt:     pgtype.Timestamptz{Time: autoRelease, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	if _, err := q.InsertWalletTransaction(ctx, dbgen.InsertWalletTransactionParams{
		WalletID:     w.ID,
		UserID:       in.BuyerID,
		CurrencyCode: p.CurrencyCode,
		Type:         "escrow_hold",
		Direction:    "debit",
		Amount:       p.Price,
		BalanceAfter: held.Available,
		OrderID:      pgUUID(row.ID),
		Metadata:     []byte("{}"),
	}); err != nil {
		return nil, fmt.Errorf("insert hold txn: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	// NOTIFICATION LOGIC
	if s.notifier != nil {
		buyerUsername := "user"
		if buyer, err := s.q.GetUserByID(ctx, in.BuyerID); err == nil {
			buyerUsername = buyer.Username
		}

		// Include the product title here!
		titleText := fmt.Sprintf("New order from @%s", buyerUsername)
		amountStr := fmt.Sprintf("%.2f %s", float64(row.Amount)/100.0, row.CurrencyCode)
		bodyText := fmt.Sprintf("@%s purchased your product \"%s\" for %s.", buyerUsername, p.Title, amountStr)

		metaMap := map[string]any{
			"order_id":       row.ID,
			"buyer_username": buyerUsername,
			"amount":         row.Amount,
			"currency":       row.CurrencyCode,
			"product_title":  p.Title,
		}
		metaBytes, _ := json.Marshal(metaMap)

		_ = s.notifier.Notify(ctx, row.SellerID, "order.placed", titleText, bodyText, metaBytes)
	}

	return toDTO(row, in.BuyerID), nil
}

func (s *Service) Confirm(ctx context.Context, orderID, requesterID string) (*Order, error) {
	return s.release(ctx, orderID, requesterID, "buyer")
}

func (s *Service) AutoRelease(ctx context.Context, orderID string) (*Order, error) {
	return s.release(ctx, orderID, "", "auto")
}

func (s *Service) release(ctx context.Context, orderID, requesterID, releasedBy string) (*Order, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)

	o, err := q.GetOrderForUpdate(ctx, orderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get order: %w", err)
	}

	if releasedBy == "buyer" && o.BuyerID != requesterID {
		return nil, ErrForbidden
	}
	if releasedBy == "buyer" && o.Status == "disputed" {
		return nil, ErrCannotDispute
	}

	// CHANGE: Allow releasing from 'held' OR 'delivered'
	if o.Status != "held" && o.Status != "delivered" && o.Status != "disputed" {
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit: %w", err)
		}
		return toDTO(o, requesterID), nil
	}

	updated, err := q.ReleaseOrder(ctx, dbgen.ReleaseOrderParams{
		ID: orderID, ReleasedBy: ptrString(releasedBy),
	})
	if err != nil {
		return nil, fmt.Errorf("release order: %w", err)
	}

	buyerWallet, err := q.GetWalletByUserAndCurrency(ctx, dbgen.GetWalletByUserAndCurrencyParams{
		UserID: o.BuyerID, CurrencyCode: o.CurrencyCode,
	})
	if err != nil {
		return nil, fmt.Errorf("buyer wallet: %w", err)
	}
	if _, err := q.ReleaseHeld(ctx, dbgen.ReleaseHeldParams{
		ID: buyerWallet.ID, Held: o.Amount,
	}); err != nil {
		return nil, fmt.Errorf("release held: %w", err)
	}

	sellerShare := o.Amount - o.CommissionAmount

	sellerWallet, err := ensureWallet(ctx, q, o.SellerID, o.CurrencyCode)
	if err != nil {
		return nil, fmt.Errorf("seller wallet: %w", err)
	}
	sellerBal, err := q.AddAvailable(ctx, dbgen.AddAvailableParams{
		ID: sellerWallet.ID, Available: sellerShare,
	})
	if err != nil {
		return nil, fmt.Errorf("credit seller: %w", err)
	}

	if _, err := q.InsertWalletTransaction(ctx, dbgen.InsertWalletTransactionParams{
		WalletID:     buyerWallet.ID,
		UserID:       o.BuyerID,
		CurrencyCode: o.CurrencyCode,
		Type:         "escrow_release",
		Direction:    "debit",
		Amount:       o.Amount,
		BalanceAfter: buyerWallet.Available,
		OrderID:      pgUUID(o.ID),
		Metadata:     []byte("{}"),
	}); err != nil {
		return nil, fmt.Errorf("buyer ledger: %w", err)
	}
	if _, err := q.InsertWalletTransaction(ctx, dbgen.InsertWalletTransactionParams{
		WalletID:     sellerWallet.ID,
		UserID:       o.SellerID,
		CurrencyCode: o.CurrencyCode,
		Type:         "escrow_release",
		Direction:    "credit",
		Amount:       sellerShare,
		BalanceAfter: sellerBal,
		OrderID:      pgUUID(o.ID),
		Metadata:     []byte("{}"),
	}); err != nil {
		return nil, fmt.Errorf("seller ledger: %w", err)
	}

	if o.CommissionAmount > 0 {
		systemID, err := q.GetSystemUserID(ctx)
		if err != nil {
			return nil, fmt.Errorf("system user: %w", err)
		}
		systemWallet, err := ensureWallet(ctx, q, systemID, o.CurrencyCode)
		if err != nil {
			return nil, fmt.Errorf("system wallet: %w", err)
		}
		systemBal, err := q.AddAvailable(ctx, dbgen.AddAvailableParams{
			ID: systemWallet.ID, Available: o.CommissionAmount,
		})
		if err != nil {
			return nil, fmt.Errorf("credit system: %w", err)
		}
		if _, err := q.InsertWalletTransaction(ctx, dbgen.InsertWalletTransactionParams{
			WalletID:     systemWallet.ID,
			UserID:       systemID,
			CurrencyCode: o.CurrencyCode,
			Type:         "commission",
			Direction:    "credit",
			Amount:       o.CommissionAmount,
			BalanceAfter: systemBal,
			OrderID:      pgUUID(o.ID),
			Metadata:     []byte("{}"),
		}); err != nil {
			return nil, fmt.Errorf("commission ledger: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	if s.notifier != nil {
		meta := []byte(fmt.Sprintf(`{"order_id":"%s"}`, updated.ID))
		_ = s.notifier.Notify(ctx, o.SellerID, "order.completed",
			"Order completed",
			fmt.Sprintf("%s released to your balance", formatAmount(sellerShare, o.CurrencyCode)),
			meta)
	}

	return toDTO(updated, requesterID), nil
}

func (s *Service) Refund(ctx context.Context, orderID, adminID string) (*Order, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)

	o, err := q.GetOrderForUpdate(ctx, orderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get order: %w", err)
	}
	if o.Status != "held" && o.Status != "disputed" {
		return nil, ErrNotHeld
	}

	updated, err := q.RefundOrder(ctx, dbgen.RefundOrderParams{
		ID: orderID, ReleasedBy: ptrString("admin"),
	})
	if err != nil {
		return nil, fmt.Errorf("refund order: %w", err)
	}

	buyerWallet, err := q.GetWalletByUserAndCurrency(ctx, dbgen.GetWalletByUserAndCurrencyParams{
		UserID: o.BuyerID, CurrencyCode: o.CurrencyCode,
	})
	if err != nil {
		return nil, fmt.Errorf("buyer wallet: %w", err)
	}
	if _, err := q.ReleaseHeld(ctx, dbgen.ReleaseHeldParams{
		ID: buyerWallet.ID, Held: o.Amount,
	}); err != nil {
		return nil, fmt.Errorf("release held: %w", err)
	}
	newBal, err := q.AddAvailable(ctx, dbgen.AddAvailableParams{
		ID: buyerWallet.ID, Available: o.Amount,
	})
	if err != nil {
		return nil, fmt.Errorf("credit buyer: %w", err)
	}
	if _, err := q.InsertWalletTransaction(ctx, dbgen.InsertWalletTransactionParams{
		WalletID:     buyerWallet.ID,
		UserID:       o.BuyerID,
		CurrencyCode: o.CurrencyCode,
		Type:         "escrow_refund",
		Direction:    "credit",
		Amount:       o.Amount,
		BalanceAfter: newBal,
		OrderID:      pgUUID(o.ID),
		Metadata:     []byte("{}"),
	}); err != nil {
		return nil, fmt.Errorf("refund ledger: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	if s.notifier != nil {
		buyerUsername := "The buyer"
		productTitle := "the order"

		// Fetch buyer username
		if buyer, err := s.q.GetUserByID(ctx, o.BuyerID); err == nil {
			buyerUsername = fmt.Sprintf("@%s", buyer.Username)
		}
		// Fetch product title
		if fullOrder, err := s.q.GetOrder(ctx, updated.ID); err == nil && fullOrder.ProductID.Valid {
			if prod, err := s.q.GetProduct(ctx, pgUUIDString(fullOrder.ProductID)); err == nil {
				productTitle = fmt.Sprintf(`"%s"`, prod.Title)
			}
		}

		titleText := "Order refunded"
		amountStr := fmt.Sprintf("%.2f %s", float64(o.Amount)/100.0, o.CurrencyCode)
		bodyText := fmt.Sprintf("%s was refunded %s for %s.", buyerUsername, amountStr, productTitle)

		metaMap := map[string]any{
			"order_id": updated.ID,
			"amount":   o.Amount,
			"currency": o.CurrencyCode,
		}
		metaBytes, _ := json.Marshal(metaMap)

		_ = s.notifier.Notify(ctx, o.SellerID, "order.refunded", titleText, bodyText, metaBytes)
	}
	return toDTO(updated, adminID), nil
}

func (s *Service) Get(ctx context.Context, orderID, requesterID string) (*Order, error) {
	row, err := s.q.GetOrder(ctx, orderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get: %w", err)
	}
	if row.BuyerID != requesterID && row.SellerID != requesterID {
		return nil, ErrForbidden
	}
	base := dbgen.Order{
		ID: row.ID, BuyerID: row.BuyerID, SellerID: row.SellerID,
		ProductID:          row.ProductID,
		InvoiceTitle:       row.InvoiceTitle,
		InvoiceDescription: row.InvoiceDescription,
		CurrencyCode:       row.CurrencyCode,
		Amount:             row.Amount,
		CommissionAmount:   row.CommissionAmount,
		CommissionPercent:  row.CommissionPercent,
		Status:             row.Status,
		AutoReleaseAt:      row.AutoReleaseAt,
		ReleasedAt:         row.ReleasedAt,
		ReleasedBy:         row.ReleasedBy,
		CreatedAt:          row.CreatedAt,
	}
	return toDTOWithUsernames(base, row.BuyerUsername, row.SellerUsername, requesterID), nil
}

func (s *Service) List(ctx context.Context, userID, role, cursor, idFilter string, limit int32) (*Page, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	params := dbgen.ListOrdersForUserParams{
		UserID: userID,
		Role:   role,
		IDFilter: idFilter,
		Lim:    limit + 1,
	}

	if cursor != "" {
		t, id, err := decodeCursor(cursor)
		if err != nil {
			return nil, err
		}
		params.CursorTs = pgtype.Timestamptz{Time: t, Valid: true}
		params.CursorID = pgUUID(id)
	}

	rows, err := s.q.ListOrdersForUser(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}

	hasMore := len(rows) > int(limit)
	if hasMore {
		rows = rows[:limit]
	}

	out := make([]Order, 0, len(rows))
	for _, r := range rows {
		base := dbgen.Order{
			ID: r.ID, BuyerID: r.BuyerID, SellerID: r.SellerID,
			ProductID:          r.ProductID,
			InvoiceTitle:       r.InvoiceTitle,
			InvoiceDescription: r.InvoiceDescription,
			CurrencyCode:       r.CurrencyCode,
			Amount:             r.Amount,
			CommissionAmount:   r.CommissionAmount,
			CommissionPercent:  r.CommissionPercent,
			Status:             r.Status,
			AutoReleaseAt:      r.AutoReleaseAt,
			ReleasedAt:         r.ReleasedAt,
			ReleasedBy:         r.ReleasedBy,
			CreatedAt:          r.CreatedAt,
		}
		out = append(out, *toDTOWithUsernames(base, r.BuyerUsername, r.SellerUsername, userID))
	}

	page := &Page{Orders: out, HasMore: hasMore}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		page.NextCursor = encodeCursor(last.CreatedAt, last.ID)
	}
	return page, nil
}

// ---- Invoices ----

type CreateInvoiceInput struct {
	SellerID    string
	BuyerID     string
	Title       string
	Description string
	Amount      int64
	Currency    string
}

func (s *Service) CreateInvoice(ctx context.Context, in CreateInvoiceInput) (*Order, error) {
	if in.SellerID == in.BuyerID {
		return nil, ErrSelfDealing
	}
	if len(in.Title) < 1 || len(in.Title) > 200 {
		return nil, fmt.Errorf("title must be 1-200 chars")
	}
	if in.Amount <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}
	if len(in.Currency) != 3 {
		return nil, fmt.Errorf("currency must be 3 chars")
	}

	cpct, _ := s.settings.Int(ctx, "commission_percent", 5)
	commission := in.Amount * int64(cpct) / 100

	// 1. Insert the invoice into the DB
	row, err := s.q.CreatePendingInvoice(ctx, dbgen.CreatePendingInvoiceParams{
		BuyerID:            in.BuyerID,
		SellerID:           in.SellerID,
		CurrencyCode:       in.Currency,
		Amount:             in.Amount,
		CommissionAmount:   commission,
		CommissionPercent:  int32(cpct),
		InvoiceTitle:       &in.Title,
		InvoiceDescription: &in.Description,
	})
	if err != nil {
		return nil, fmt.Errorf("create invoice: %w", err)
	}

	// 2. Query the full invoice data with the user JOIN
	if s.notifier != nil {
		// Fetch the complete invoice (this includes seller_username via JOIN)
		fullInvoice, err := s.q.GetInvoice(ctx, row.ID)
		if err == nil {
			// Format the text exactly as we want it saved permanently
			title := "an invoice"
			if fullInvoice.InvoiceTitle != nil && *fullInvoice.InvoiceTitle != "" {
				title = fmt.Sprintf(`"%s"`, *fullInvoice.InvoiceTitle)
			}
			
			// Format the amount
			amountStr := fmt.Sprintf("%.2f %s", float64(fullInvoice.Amount)/100.0, fullInvoice.CurrencyCode)
			
			// Build the metadata JSON using Go's safe JSON builder
			metaMap := map[string]any{
				"order_id":        fullInvoice.ID,
				"seller_username": fullInvoice.SellerUsername,
				"amount":          fullInvoice.Amount,
				"currency":        fullInvoice.CurrencyCode,
				"title":           *fullInvoice.InvoiceTitle,
			}
			metaBytes, _ := json.Marshal(metaMap)

			// Build the permanent text
			titleText := fmt.Sprintf("New invoice from @%s", fullInvoice.SellerUsername)
			bodyText := fmt.Sprintf("@%s sent you an invoice: %s for %s.", fullInvoice.SellerUsername, title, amountStr)

			// 3. Save to notification DB table
			_ = s.notifier.Notify(ctx, in.BuyerID, "invoice.created", titleText, bodyText, metaBytes)
		}
	}

	return toDTO(row, in.SellerID), nil
}

func (s *Service) PayInvoice(ctx context.Context, orderID, requesterID string) (*Order, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)

	o, err := q.GetOrderForUpdate(ctx, orderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get order: %w", err)
	}
	if o.BuyerID != requesterID {
		return nil, ErrForbidden
	}
	if o.Status != "pending" {
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit: %w", err)
		}
		return toDTO(o, requesterID), nil
	}

	w, err := q.GetWalletByUserAndCurrency(ctx, dbgen.GetWalletByUserAndCurrencyParams{
		UserID: requesterID, CurrencyCode: o.CurrencyCode,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInsufficientFunds
		}
		return nil, fmt.Errorf("wallet: %w", err)
	}

	held, err := q.HoldAvailable(ctx, dbgen.HoldAvailableParams{
		ID: w.ID, Available: o.Amount,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInsufficientFunds
		}
		return nil, fmt.Errorf("hold: %w", err)
	}

	days, _ := s.settings.Int(ctx, "escrow_auto_release_days", 3)
	autoRelease := time.Now().Add(time.Duration(days) * 24 * time.Hour)

	updated, err := q.MarkInvoicePaid(ctx, dbgen.MarkInvoicePaidParams{
		ID:            orderID,
		AutoReleaseAt: pgtype.Timestamptz{Time: autoRelease, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("mark paid: %w", err)
	}

	if _, err := q.InsertWalletTransaction(ctx, dbgen.InsertWalletTransactionParams{
		WalletID:     w.ID,
		UserID:       requesterID,
		CurrencyCode: o.CurrencyCode,
		Type:         "escrow_hold",
		Direction:    "debit",
		Amount:       o.Amount,
		BalanceAfter: held.Available,
		OrderID:      pgUUID(o.ID),
		Metadata:     []byte("{}"),
	}); err != nil {
		return nil, fmt.Errorf("insert hold txn: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	// NOTIFICATION LOGIC — notify the seller that the buyer paid
	if s.notifier != nil {
		buyerUsername := "The buyer"
		if buyer, err := s.q.GetUserByID(ctx, requesterID); err == nil {
			buyerUsername = fmt.Sprintf("@%s", buyer.Username)
		}

		invoiceTitle := "the invoice"
		if updated.InvoiceTitle != nil && *updated.InvoiceTitle != "" {
			invoiceTitle = fmt.Sprintf(`"%s"`, *updated.InvoiceTitle)
		}

		amountStr := fmt.Sprintf("%.2f %s", float64(updated.Amount)/100.0, updated.CurrencyCode)

		titleText := "Invoice paid"
		bodyText := fmt.Sprintf("%s paid your invoice %s for %s.", buyerUsername, invoiceTitle, amountStr)

		metaMap := map[string]any{
			"order_id":       updated.ID,
			"amount":         updated.Amount,
			"currency":       updated.CurrencyCode,
			"buyer_username": buyerUsername,
			"invoice_title":  pgTextString(updated.InvoiceTitle),
		}
		metaBytes, _ := json.Marshal(metaMap)

		_ = s.notifier.Notify(ctx, updated.SellerID, "invoice.paid", titleText, bodyText, metaBytes)
	}

	return toDTO(updated, requesterID), nil
}

// ---- Disputes ----

func (s *Service) Dispute(ctx context.Context, orderID, buyerID, reason string) (*Order, error) {
	if len(reason) < 3 || len(reason) > 2000 {
		return nil, fmt.Errorf("reason must be 3-2000 chars")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)

	o, err := q.GetOrderForUpdate(ctx, orderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get order: %w", err)
	}
	if o.BuyerID != buyerID {
		return nil, ErrNotBuyerDispute
	}
	switch o.Status {
	case "held":
	case "disputed":
		return nil, ErrAlreadyDisputed
	default:
		return nil, ErrCannotDispute
	}

	updated, err := q.DisputeOrder(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("mark disputed: %w", err)
	}

	if _, err := q.CreateDispute(ctx, dbgen.CreateDisputeParams{
		OrderID: orderID, OpenedBy: buyerID, Reason: reason,
	}); err != nil {
		return nil, fmt.Errorf("create dispute: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	if s.notifier != nil {
		buyerUsername := "The buyer"
		productTitle := "the order"

		// Fetch buyer username
		if buyer, err := s.q.GetUserByID(ctx, buyerID); err == nil {
			buyerUsername = fmt.Sprintf("@%s", buyer.Username)
		}
		// Fetch product title
		if fullOrder, err := s.q.GetOrder(ctx, updated.ID); err == nil && fullOrder.ProductID.Valid {
			if prod, err := s.q.GetProduct(ctx, pgUUIDString(fullOrder.ProductID)); err == nil {
				productTitle = fmt.Sprintf(`"%s"`, prod.Title)
			}
		}

		titleText := "Order disputed"
		amountStr := fmt.Sprintf("%.2f %s", float64(o.Amount)/100.0, o.CurrencyCode)
		bodyText := fmt.Sprintf("%s disputed %s for %s. Reason: %s", buyerUsername, productTitle, amountStr, reason)

		metaMap := map[string]any{
			"order_id": updated.ID,
			"amount":   o.Amount,
			"currency": o.CurrencyCode,
		}
		metaBytes, _ := json.Marshal(metaMap)

		_ = s.notifier.Notify(ctx, o.SellerID, "order.disputed", titleText, bodyText, metaBytes)
	}

	return toDTO(updated, buyerID), nil
}

func (s *Service) AdminRelease(ctx context.Context, orderID, adminID string) (*Order, error) {
	o, err := s.release(ctx, orderID, adminID, "admin")
	if err != nil {
		return nil, err
	}
	// Mark any unresolved dispute as resolved. Surface the error — if the
	// adminID doesn't exist (FK violation on resolved_by) or the row vanished,
	// we want the caller to know instead of silently leaving resolution NULL.
	if _, derr := s.pool.Exec(ctx,
		`UPDATE disputes SET resolution = 'released', resolved_by = $2, resolved_at = now()
		 WHERE order_id = $1 AND resolution IS NULL`,
		orderID, adminID); derr != nil {
		return nil, fmt.Errorf("mark dispute resolved: %w", derr)
	}
	return o, nil
}

func (s *Service) AdminRefundDispute(ctx context.Context, orderID, adminID string) (*Order, error) {
	o, err := s.Refund(ctx, orderID, adminID)
	if err != nil {
		return nil, err
	}
	if _, derr := s.pool.Exec(ctx,
		`UPDATE disputes SET resolution = 'refunded', resolved_by = $2, resolved_at = now()
		 WHERE order_id = $1 AND resolution IS NULL`,
		orderID, adminID); derr != nil {
		return nil, fmt.Errorf("mark dispute resolved: %w", derr)
	}
	return o, nil
}

// AdminListDisputed returns all disputed orders with the dispute reason attached.
func (s *Service) AdminListDisputed(ctx context.Context, idFilter string, limit, offset int32) ([]Order, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.q.AdminListDisputedOrdersWithReason(ctx, dbgen.AdminListDisputedOrdersWithReasonParams{
		IDFilter: idFilter,
		Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("admin list disputed: %w", err)
	}
	out := make([]Order, 0, len(rows))
	for _, r := range rows {
		base := dbgen.Order{
			ID: r.ID, BuyerID: r.BuyerID, SellerID: r.SellerID,
			ProductID:          r.ProductID,
			InvoiceTitle:       r.InvoiceTitle,
			InvoiceDescription: r.InvoiceDescription,
			CurrencyCode:       r.CurrencyCode,
			Amount:             r.Amount,
			CommissionAmount:   r.CommissionAmount,
			CommissionPercent:  r.CommissionPercent,
			Status:             r.Status,
			AutoReleaseAt:      r.AutoReleaseAt,
			ReleasedAt:         r.ReleasedAt,
			ReleasedBy:         r.ReleasedBy,
			CreatedAt:          r.CreatedAt,
		}
		o := toDTOWithUsernames(base, r.BuyerUsername, r.SellerUsername, "")
		o.DisputeReason = r.DisputeReason
		if r.DisputeOpenedByUsername != nil {
			o.DisputeOpenedByUsername = *r.DisputeOpenedByUsername
		}
		out = append(out, *o)
	}
	return out, nil
}

// AdminGet returns a single order with the dispute reason attached.
// No party check — used for admin dispute review.
func (s *Service) AdminGet(ctx context.Context, orderID string) (*Order, error) {
	row, err := s.q.AdminGetOrderWithDispute(ctx, orderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("admin get: %w", err)
	}
	base := dbgen.Order{
		ID: row.ID, BuyerID: row.BuyerID, SellerID: row.SellerID,
		ProductID:          row.ProductID,
		InvoiceTitle:       row.InvoiceTitle,
		InvoiceDescription: row.InvoiceDescription,
		CurrencyCode:       row.CurrencyCode,
		Amount:             row.Amount,
		CommissionAmount:   row.CommissionAmount,
		CommissionPercent:  row.CommissionPercent,
		Status:             row.Status,
		AutoReleaseAt:      row.AutoReleaseAt,
		ReleasedAt:         row.ReleasedAt,
		ReleasedBy:         row.ReleasedBy,
		CreatedAt:          row.CreatedAt,
	}
	o := toDTOWithUsernames(base, row.BuyerUsername, row.SellerUsername, "")
	if row.DisputeReason != nil {
		o.DisputeReason = *row.DisputeReason
	}
	if row.DisputeOpenedByUsername != nil {
		o.DisputeOpenedByUsername = *row.DisputeOpenedByUsername
	}
	return o, nil
}

func formatAmount(cents int64, currency string) string {
	// Assumes 2 decimal digits for simplicity in notifications.
	// For a production app, look up the currency's decimal digits.
	return fmt.Sprintf("%.2f %s", float64(cents)/100.0, currency)
}

// ListInvoices returns all invoice-orders for the user, filtered by role + status.
func (s *Service) ListInvoices(ctx context.Context, userID, role, status, idFilter string, limit, offset int32) ([]InvoiceListItem, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}
	if role != "" && role != "buyer" && role != "seller" {
		return nil, 0, fmt.Errorf("role must be buyer, seller, or empty")
	}
	switch status {
	case "", "pending", "held", "completed", "refunded", "cancelled":
	default:
		return nil, 0, fmt.Errorf("invalid status")
	}

	rows, err := s.q.ListInvoicesForUser(ctx, dbgen.ListInvoicesForUserParams{
		UserID: userID, Role: role, Status: status,
		IDFilter: idFilter, // add this
		Lim: limit, Off: offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list invoices: %w", err)
	}
	total, err := s.q.CountInvoicesForUser(ctx, dbgen.CountInvoicesForUserParams{
		UserID: userID, Role: role, Status: status,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count invoices: %w", err)
	}

	out := make([]InvoiceListItem, 0, len(rows))
	for _, r := range rows {
		base := dbgen.Order{
			ID: r.ID, BuyerID: r.BuyerID, SellerID: r.SellerID,
			ProductID:          r.ProductID,
			InvoiceTitle:       r.InvoiceTitle,
			InvoiceDescription: r.InvoiceDescription,
			CurrencyCode:       r.CurrencyCode,
			Amount:             r.Amount,
			CommissionAmount:   r.CommissionAmount,
			CommissionPercent:  r.CommissionPercent,
			Status:             r.Status,
			AutoReleaseAt:      r.AutoReleaseAt,
			ReleasedAt:         r.ReleasedAt,
			ReleasedBy:         r.ReleasedBy,
			CreatedAt:          r.CreatedAt,
			CancelReason:       r.CancelReason,
			CancelledBy:        r.CancelledBy,
		}
		o := toDTOWithUsernames(base, r.BuyerUsername, r.SellerUsername, userID)
		item := InvoiceListItem{Order: *o}
		if r.CancelReason != nil {
			item.CancelReason = *r.CancelReason
		}
		item.CancelledByID = pgUUIDString(r.CancelledBy)
		out = append(out, item)
	}
	return out, total, nil
}

// GetInvoice returns one invoice by ID, with party check.
func (s *Service) GetInvoice(ctx context.Context, orderID, requesterID string) (*InvoiceListItem, error) {
	row, err := s.q.GetInvoice(ctx, orderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get invoice: %w", err)
	}
	if row.BuyerID != requesterID && row.SellerID != requesterID {
		return nil, ErrForbidden
	}
	base := dbgen.Order{
		ID: row.ID, BuyerID: row.BuyerID, SellerID: row.SellerID,
		ProductID:          row.ProductID,
		InvoiceTitle:       row.InvoiceTitle,
		InvoiceDescription: row.InvoiceDescription,
		CurrencyCode:       row.CurrencyCode,
		Amount:             row.Amount,
		CommissionAmount:   row.CommissionAmount,
		CommissionPercent:  row.CommissionPercent,
		Status:             row.Status,
		AutoReleaseAt:      row.AutoReleaseAt,
		ReleasedAt:         row.ReleasedAt,
		ReleasedBy:         row.ReleasedBy,
		CreatedAt:          row.CreatedAt,
		CancelReason:       row.CancelReason,
		CancelledBy:        row.CancelledBy,
	}
	o := toDTOWithUsernames(base, row.BuyerUsername, row.SellerUsername, requesterID)
	item := InvoiceListItem{Order: *o}
	if row.CancelReason != nil {
		item.CancelReason = *row.CancelReason
	}
	item.CancelledByID = pgUUIDString(row.CancelledBy)
	return &item, nil
}

// CancelInvoice — either party can cancel a pending invoice, with a reason.
func (s *Service) CancelInvoice(ctx context.Context, orderID, requesterID, reason string) (*InvoiceListItem, error) {
	if len(reason) < 3 || len(reason) > 1000 {
		return nil, fmt.Errorf("reason must be 3-1000 chars")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)

	row, err := q.GetInvoice(ctx, orderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get invoice: %w", err)
	}
	if row.BuyerID != requesterID && row.SellerID != requesterID {
		return nil, ErrForbidden
	}
	if row.Status != "pending" {
		return nil, fmt.Errorf("only pending invoices can be cancelled")
	}

	updated, err := q.CancelInvoice(ctx, dbgen.CancelInvoiceParams{
		ID:          orderID,
		CancelReason: &reason,
		CancelledBy: pgUUID(requesterID),
	})
	if err != nil {
		return nil, fmt.Errorf("cancel: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	if s.notifier != nil {
		otherParty := row.SellerID
		if requesterID == row.SellerID {
			otherParty = row.BuyerID
		}

		cancellerUsername := "The user"
		if canceller, err := s.q.GetUserByID(ctx, requesterID); err == nil {
			cancellerUsername = fmt.Sprintf("@%s", canceller.Username)
		}

		titleText := "Invoice cancelled"
		bodyText := fmt.Sprintf("%s cancelled the invoice \"%s\". Reason: %s", cancellerUsername, *row.InvoiceTitle, reason)

		metaMap := map[string]any{
			"order_id": orderID,
			"reason":   reason,
		}
		metaBytes, _ := json.Marshal(metaMap)

		_ = s.notifier.Notify(ctx, otherParty, "invoice.cancelled", titleText, bodyText, metaBytes)
	}

	base := dbgen.Order{
		ID: updated.ID, BuyerID: updated.BuyerID, SellerID: updated.SellerID,
		ProductID:          updated.ProductID,
		InvoiceTitle:       updated.InvoiceTitle,
		InvoiceDescription: updated.InvoiceDescription,
		CurrencyCode:       updated.CurrencyCode,
		Amount:             updated.Amount,
		CommissionAmount:   updated.CommissionAmount,
		CommissionPercent:  updated.CommissionPercent,
		Status:             updated.Status,
		AutoReleaseAt:      updated.AutoReleaseAt,
		ReleasedAt:         updated.ReleasedAt,
		ReleasedBy:         updated.ReleasedBy,
		CreatedAt:          updated.CreatedAt,
		CancelReason:       updated.CancelReason,
		CancelledBy:        updated.CancelledBy,
	}
	o := toDTOWithUsernames(base, row.BuyerUsername, row.SellerUsername, requesterID)
	item := InvoiceListItem{Order: *o}
	item.CancelReason = reason
	item.CancelledByID = requesterID
	return &item, nil
}

func (s *Service) GetDashboardStats(ctx context.Context, sellerID string) (*DashboardStats, error) {
	row, err := s.q.GetSellerDashboardStats(ctx, sellerID)
	if err != nil {
		return nil, fmt.Errorf("get dashboard stats: %w", err)
	}
	return &DashboardStats{
		TotalEarned:        row.TotalEarned,
		ActiveOrders:       row.ActiveOrders,
		PendingWithdrawals: row.PendingWithdrawals,
		ActiveProducts:     row.ActiveProducts,
	}, nil
}

// 1. Upload Deliverable
func (s *Service) UploadDeliverable(ctx context.Context, orderID, sellerID, filename, contentType string, data []byte) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)

	o, err := q.GetOrderForUpdate(ctx, orderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("get order: %w", err)
	}
	if o.SellerID != sellerID {
		return ErrForbidden
	}
	if o.Status != "held" {
		return fmt.Errorf("cannot upload: order is not in held state")
	}

	encrypted, nonce, err := s.cipher.Encrypt(data)
	if err != nil { return fmt.Errorf("encrypt: %w", err) }

	secureDir := filepath.Join(s.uploadsDir, "secure")
	if err := os.MkdirAll(secureDir, 0o755); err != nil { return fmt.Errorf("mkdir: %w", err) }
	
	filePath := filepath.Join(secureDir, uuid.NewString()+".enc")
	if err := os.WriteFile(filePath, encrypted, 0o600); err != nil { return fmt.Errorf("write: %w", err) }

	if err := q.CreateOrderDeliverable(ctx, dbgen.CreateOrderDeliverableParams{
		OrderID:       orderID,
		Filename:      filename,
		ContentType:   contentType,
		SizeBytes:     int64(len(data)),
		FilePath:      &filePath, // <-- ADD THE & HERE
		Nonce:         nonce,
	}); err != nil {
		_ = os.Remove(filePath)
		return fmt.Errorf("save deliverable: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// 2. Seal Delivery
func (s *Service) SealDelivery(ctx context.Context, orderID, sellerID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)

	o, err := q.GetOrderForUpdate(ctx, orderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("get order: %w", err)
	}
	if o.SellerID != sellerID {
		return ErrForbidden
	}
	if o.Status != "held" {
		return fmt.Errorf("cannot seal: order is not in held state")
	}

	// Check if there is at least one file
	files, err := q.ListOrderDeliverables(ctx, orderID)
	if err != nil {
		return fmt.Errorf("list deliverables: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("cannot seal empty delivery")
	}

	// UPDATE THE STATUS
	if err := q.SealOrderDelivery(ctx, orderID); err != nil {
		return fmt.Errorf("seal delivery: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	// Notify buyer AFTER the transaction commits successfully
	if s.notifier != nil {
		meta := []byte(fmt.Sprintf(`{"order_id":"%s"}`, orderID))
		_ = s.notifier.Notify(ctx, o.BuyerID, "order.delivered", "Delivery Received", "The seller has delivered your files. Please review and accept.", meta)
	}
	return nil
}

// 3. Download Deliverable
func (s *Service) DownloadDeliverable(ctx context.Context, orderID, fileID, requesterID string) ([]byte, string, string, error) {
	o, err := s.q.GetOrder(ctx, orderID)
	if err != nil {
		return nil, "", "", ErrNotFound
	}
	if o.BuyerID != requesterID && o.SellerID != requesterID {
		return nil, "", "", ErrForbidden
	}

	f, err := s.q.GetOrderDeliverable(ctx, fileID)
	if err != nil || f.OrderID != orderID { return nil, "", "", ErrNotFound }

	// Read from disk
	if f.FilePath == nil {
		return nil, "", "", fmt.Errorf("legacy deliverable (no file path)")
	}
	encrypted, err := os.ReadFile(*f.FilePath) // <-- ADD THE * HERE
	if err != nil {
		return nil, "", "", fmt.Errorf("read file: %w", err)
	}

	data, err := s.cipher.Decrypt(encrypted, f.Nonce)
	if err != nil { return nil, "", "", fmt.Errorf("decrypt: %w", err) }

	return data, f.Filename, f.ContentType, nil
}

// 4. List Deliverables
func (s *Service) ListDeliverables(ctx context.Context, orderID, requesterID string) ([]dbgen.ListOrderDeliverablesRow, error) {
	o, err := s.q.GetOrder(ctx, orderID)
	if err != nil {
		return nil, ErrNotFound
	}
	if o.BuyerID != requesterID && o.SellerID != requesterID {
		return nil, ErrForbidden
	}
	return s.q.ListOrderDeliverables(ctx, orderID)
}

// ---- helpers ----

func toDTO(r dbgen.Order, requesterID string) *Order {
	o := &Order{
		ID:                r.ID,
		BuyerID:           r.BuyerID,
		SellerID:          r.SellerID,
		ProductID:         pgUUIDString(r.ProductID),
		CurrencyCode:      r.CurrencyCode,
		Amount:            r.Amount,
		CommissionAmount:  r.CommissionAmount,
		CommissionPercent: r.CommissionPercent,
		Status:            r.Status,
		ReleasedBy:        pgTextString(r.ReleasedBy),
		CreatedAtUnix:     r.CreatedAt.Unix(),
	}
	if r.AutoReleaseAt.Valid {
		t := r.AutoReleaseAt.Time.Unix()
		o.AutoReleaseAtUnix = &t
	}
	if r.ReleasedAt.Valid {
		t := r.ReleasedAt.Time.Unix()
		o.ReleasedAtUnix = &t
	}
	if r.InvoiceTitle != nil {
		o.InvoiceTitle = *r.InvoiceTitle
	}
	if r.InvoiceDescription != nil {
		o.InvoiceDescription = *r.InvoiceDescription
	}
	if requesterID != "" {
		if r.BuyerID == requesterID {
			o.MyRole = "buyer"
		} else if r.SellerID == requesterID {
			o.MyRole = "seller"
		}
	}
	return o
}

func toDTOWithUsernames(base dbgen.Order, buyerUN, sellerUN, requesterID string) *Order {
	o := toDTO(base, requesterID)
	o.BuyerUsername = buyerUN
	o.SellerUsername = sellerUN
	return o
}

func ensureWallet(ctx context.Context, q dbgen.Querier, userID, currency string) (dbgen.Wallet, error) {
	w, err := q.GetWalletByUserAndCurrency(ctx, dbgen.GetWalletByUserAndCurrencyParams{
		UserID: userID, CurrencyCode: currency,
	})
	if err == nil {
		return w, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return dbgen.Wallet{}, err
	}
	created, err := q.CreateWallet(ctx, dbgen.CreateWalletParams{
		UserID: userID, CurrencyCode: currency,
	})
	if err != nil {
		return dbgen.Wallet{}, err
	}
	return dbgen.Wallet{
		ID: created.ID, UserID: created.UserID, CurrencyCode: created.CurrencyCode,
		Available: created.Available, Held: created.Held, UpdatedAt: created.UpdatedAt,
	}, nil
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

func ptrString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func pgTextString(t *string) string {
	if t == nil {
		return ""
	}
	return *t
}
