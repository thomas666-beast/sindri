package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/thomas666-beast/sindri/internal/db/dbgen"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrInvalidInput = errors.New("invalid input")
	ErrAlreadyDone  = errors.New("already done")
)

type Notifier interface {
	Notify(ctx context.Context, userID, kind, title, body string, metadata []byte) error
}

type AdminUser struct {
	ID                string `json:"id"`
	Email             string `json:"email"`
	Username          string `json:"username"`
	PreferredCurrency string `json:"preferred_currency"`
	IsAdmin           bool   `json:"is_admin"`
	IsActive          bool   `json:"is_active"`
	KYCStatus         string `json:"kyc_status"`
	KYCOverride       bool   `json:"kyc_override"`
	DisabledAtUnix    *int64 `json:"disabled_at_unix,omitempty"`
	DisableReason     string `json:"disable_reason,omitempty"`
	CreatedAtUnix     int64  `json:"created_at_unix"`
}

type Warning struct {
	ID                 string `json:"id"`
	UserID             string `json:"user_id"`
	IssuedBy           string `json:"issued_by"`
	Reason             string `json:"reason"`
	Severity           string `json:"severity"`
	AcknowledgedAtUnix *int64 `json:"acknowledged_at_unix,omitempty"`
	CreatedAtUnix      int64  `json:"created_at_unix"`
}

type Ticket struct {
	ID            string `json:"id"`
	UserID        string `json:"user_id"`
	UserUsername  string `json:"user_username,omitempty"`
	Subject       string `json:"subject"`
	Status        string `json:"status"`
	Priority      string `json:"priority"`
	CreatedAtUnix int64  `json:"created_at_unix"`
	UpdatedAtUnix int64  `json:"updated_at_unix"`
}

type TicketMessage struct {
	ID            string `json:"id"`
	TicketID      string `json:"ticket_id"`
	AuthorID      string `json:"author_id"`
	IsStaff       bool   `json:"is_staff"`
	Body          string `json:"body"`
	CreatedAtUnix int64  `json:"created_at_unix"`
}

type Stats struct {
	UsersActive          int64            `json:"users_active"`
	UsersDisabled        int64            `json:"users_disabled"`
	OrdersHeld           int64            `json:"orders_held"`
	OrdersDisputed       int64            `json:"orders_disputed"`
	PendingWithdrawals   int64            `json:"pending_withdrawals"`
	OpenTickets          int64            `json:"open_tickets"`
	CommissionByCurrency map[string]int64 `json:"commission_by_currency"`
}

type Service struct {
	q        dbgen.Querier
	notifier Notifier
}

func NewService(q dbgen.Querier, notifier Notifier) *Service {
	return &Service{q: q, notifier: notifier}
}

// ---- Users ----

func (s *Service) ListUsers(ctx context.Context, search, status string, limit, offset int32) ([]AdminUser, int64, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	if status != "" && status != "active" && status != "disabled" {
		return nil, 0, ErrInvalidInput
	}
	rows, err := s.q.AdminListUsers(ctx, dbgen.AdminListUsersParams{
		Search: search, StatusFilter: status, Lim: limit, Off: offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	total, err := s.q.AdminCountUsersFiltered(ctx, dbgen.AdminCountUsersFilteredParams{
		Search:       search,
		StatusFilter: status,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}
	out := make([]AdminUser, 0, len(rows))
	for _, r := range rows {
		var da *time.Time
		if r.DisabledAt.Valid {
			t := r.DisabledAt.Time
			da = &t
		}
		out = append(out, buildAdminUser(r.ID, r.Email, r.Username, r.PreferredCurrency,
			r.IsAdmin, r.IsActive, r.KycStatus, r.KycOverride, da, r.DisableReason, r.CreatedAt))
	}
	return out, total, nil
}

func (s *Service) GetUser(ctx context.Context, id string) (*AdminUser, error) {
	row, err := s.q.AdminGetUser(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	var da *time.Time
	if row.DisabledAt.Valid {
		t := row.DisabledAt.Time
		da = &t
	}
	u := buildAdminUser(row.ID, row.Email, row.Username, row.PreferredCurrency,
		row.IsAdmin, row.IsActive, row.KycStatus, row.KycOverride, da, row.DisableReason, row.CreatedAt)
	return &u, nil
}

func (s *Service) DisableUser(ctx context.Context, id, reason string) (*AdminUser, error) {
	if len(reason) < 3 {
		return nil, ErrInvalidInput
	}
	if _, err := s.q.AdminDisableUser(ctx, dbgen.AdminDisableUserParams{ID: id, DisableReason: ptrString(reason)}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("disable: %w", err)
	}
	return s.GetUser(ctx, id)
}

func (s *Service) EnableUser(ctx context.Context, id string) (*AdminUser, error) {
	if _, err := s.q.AdminEnableUser(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("enable: %w", err)
	}
	return s.GetUser(ctx, id)
}

func (s *Service) SetKYC(ctx context.Context, id, status string) (*AdminUser, error) {
	if status != "not_required" && status != "pending" && status != "verified" && status != "rejected" {
		return nil, ErrInvalidInput
	}
	if _, err := s.q.AdminSetKYC(ctx, dbgen.AdminSetKYCParams{ID: id, KycStatus: status}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("set kyc: %w", err)
	}
	return s.GetUser(ctx, id)
}

func (s *Service) SetAdmin(ctx context.Context, id string, isAdmin bool) (*AdminUser, error) {
	if _, err := s.q.AdminSetAdmin(ctx, dbgen.AdminSetAdminParams{ID: id, IsAdmin: isAdmin}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("set admin: %w", err)
	}
	return s.GetUser(ctx, id)
}

func (s *Service) Stats(ctx context.Context) (*Stats, error) {
	row, err := s.q.StatsDashboard(ctx)
	if err != nil {
		return nil, fmt.Errorf("stats dashboard: %w", err)
	}

	commission := make(map[string]int64)
	switch v := row.CommissionByCurrency.(type) {
	case []byte:
		if len(v) > 0 {
			_ = json.Unmarshal(v, &commission)
		}
	case string:
		if v != "" {
			_ = json.Unmarshal([]byte(v), &commission)
		}
	case map[string]any:
		for k, val := range v {
			switch n := val.(type) {
			case float64:
				commission[k] = int64(n)
			case int64:
				commission[k] = n
			case json.Number:
				if i, err := n.Int64(); err == nil {
					commission[k] = i
				}
			}
		}
	}

	return &Stats{
		UsersActive:          row.ActiveUsers,
		UsersDisabled:        row.DisabledUsers,
		OrdersHeld:           row.OrdersHeld,
		OrdersDisputed:       row.OrdersDisputed,
		PendingWithdrawals:   row.PendingWithdrawals,
		OpenTickets:          row.OpenTickets,
		CommissionByCurrency: commission,
	}, nil
}

type Transaction struct {
	ID            string `json:"id"`
	UserID        string `json:"user_id"`
	Username      string `json:"username"`
	CurrencyCode  string `json:"currency_code"`
	Type          string `json:"type"`
	Direction     string `json:"direction"`
	Amount        int64  `json:"amount"`
	BalanceAfter  int64  `json:"balance_after"`
	OrderID       string `json:"order_id,omitempty"`
	Status        string `json:"status"`
	Metadata      string `json:"metadata"`
	CreatedAtUnix int64  `json:"created_at_unix"`
}

func (s *Service) ListTransactions(ctx context.Context, typeFilter, idFilter, userFilter string, limit, offset int32) ([]Transaction, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := s.q.AdminListTransactions(ctx, dbgen.AdminListTransactionsParams{
		TypeFilter: typeFilter,
		IDFilter:   idFilter,
		UserFilter: userFilter,
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list txns: %w", err)
	}

	out := make([]Transaction, 0, len(rows))
	for _, r := range rows {
		// Safely extract the OrderID
		var oid string
		if r.OrderID.Valid {
			v, err := r.OrderID.Value()
			if err == nil {
				if s, ok := v.(string); ok {
					oid = s
				}
			}
		}

		out = append(out, Transaction{
			ID:            r.ID,
			UserID:        r.UserID,
			Username:      r.Username,
			CurrencyCode:  r.CurrencyCode,
			Type:          r.Type,
			Direction:     r.Direction,
			Amount:        r.Amount,
			BalanceAfter:  r.BalanceAfter,
			OrderID:       oid,
			Status:        r.Status,
			Metadata:      string(r.Metadata),
			CreatedAtUnix: r.CreatedAt.Unix(),
		})
	}
	return out, nil
}

func (s *Service) GetSettings(ctx context.Context) (map[string]string, error) {
	// Assuming you have a query to list all settings, or we can just fetch the two we care about
	cpct, _ := s.q.GetSetting(ctx, "commission_percent")
	days, _ := s.q.GetSetting(ctx, "escrow_auto_release_days")
	
	out := make(map[string]string)
	if cpct != nil {
		out["commission_percent"] = string(cpct)
	}
	if days != nil {
		out["escrow_auto_release_days"] = string(days)
	}
	return out, nil
}

func (s *Service) UpdateSetting(ctx context.Context, key, value string) error {
	// Safely marshal the string into valid JSONB (e.g. "5" becomes 5)
	jsonValue, err := json.Marshal(value)
	if err != nil {
		return err
	}
	
	// If it's a number, we want to store it as a number in JSONB, not a string.
	// A quick trick is to try wrapping it in raw JSON if it's numeric.
	raw := json.RawMessage(value)
	if json.Valid(raw) {
		jsonValue = raw
	}

	return s.q.SetSetting(ctx, dbgen.SetSettingParams{
		Key:   key,
		Value: jsonValue, 
	})
}

// ---- Warnings ----

func (s *Service) CreateWarning(ctx context.Context, userID, issuedBy, reason, severity string) (*Warning, error) {
	if severity != "info" && severity != "warning" && severity != "severe" {
		return nil, ErrInvalidInput
	}
	if len(reason) < 1 || len(reason) > 2000 {
		return nil, ErrInvalidInput
	}
	row, err := s.q.CreateWarning(ctx, dbgen.CreateWarningParams{
		UserID: userID, IssuedBy: issuedBy, Reason: reason, Severity: severity,
	})
	if err != nil {
		return nil, fmt.Errorf("create warning: %w", err)
	}

	if s.notifier != nil {
		meta := []byte(fmt.Sprintf(`{"warning_id":"%s"}`, row.ID))
		_ = s.notifier.Notify(ctx, userID, "warning",
			fmt.Sprintf("Warning issued (%s)", severity),
			reason,
			meta)
	}

	return toWarning(row.ID, row.UserID, row.IssuedBy, row.Reason, row.Severity,
		nil, row.CreatedAt), nil
}

func (s *Service) ListWarnings(ctx context.Context, userID string, limit, offset int32) ([]Warning, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.q.ListWarningsForUser(ctx, dbgen.ListWarningsForUserParams{
		UserID: userID, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list warnings: %w", err)
	}
	out := make([]Warning, 0, len(rows))
	for _, r := range rows {
		var ack *int64
		if r.AcknowledgedAt.Valid {
			t := r.AcknowledgedAt.Time.Unix()
			ack = &t
		}
		out = append(out, *toWarning(r.ID, r.UserID, r.IssuedBy, r.Reason, r.Severity, ack, r.CreatedAt))
	}
	return out, nil
}

func (s *Service) ListMyWarnings(ctx context.Context, userID string, limit, offset int32) ([]Warning, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.q.ListMyWarnings(ctx, dbgen.ListMyWarningsParams{
		UserID: userID, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list my warnings: %w", err)
	}
	out := make([]Warning, 0, len(rows))
	for _, r := range rows {
		var ack *int64
		if r.AcknowledgedAt.Valid {
			t := r.AcknowledgedAt.Time.Unix()
			ack = &t
		}
		out = append(out, *toWarning(r.ID, r.UserID, r.IssuedBy, r.Reason, r.Severity, ack, r.CreatedAt))
	}
	return out, nil
}

func (s *Service) AcknowledgeWarning(ctx context.Context, userID, warningID string) error {
	n, err := s.q.AcknowledgeWarning(ctx, dbgen.AcknowledgeWarningParams{
		ID: warningID, UserID: userID,
	})
	if err != nil {
		return fmt.Errorf("ack: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- Tickets ----

func (s *Service) CreateTicket(ctx context.Context, userID, subject, priority string) (*Ticket, error) {
	if len(subject) < 1 || len(subject) > 300 {
		return nil, ErrInvalidInput
	}
	if priority == "" {
		priority = "normal"
	}
	if priority != "low" && priority != "normal" && priority != "high" {
		return nil, ErrInvalidInput
	}
	row, err := s.q.CreateTicket(ctx, dbgen.CreateTicketParams{
		UserID: userID, Subject: subject, Priority: priority,
	})
	if err != nil {
		return nil, fmt.Errorf("create ticket: %w", err)
	}
	return toTicket(row.ID, row.UserID, "", row.Subject, row.Status, row.Priority,
		row.CreatedAt, row.UpdatedAt), nil
}

func (s *Service) GetTicket(ctx context.Context, id, requesterID string, isAdmin bool) (*Ticket, error) {
	row, err := s.q.GetTicket(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get ticket: %w", err)
	}
	if !isAdmin && row.UserID != requesterID {
		return nil, ErrNotFound
	}
	return toTicket(row.ID, row.UserID, row.UserUsername, row.Subject, row.Status,
		row.Priority, row.CreatedAt, row.UpdatedAt), nil
}

func (s *Service) ListMyTickets(ctx context.Context, userID string, limit, offset int32) ([]Ticket, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.q.ListTicketsForUser(ctx, dbgen.ListTicketsForUserParams{
		UserID: userID, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list my tickets: %w", err)
	}
	out := make([]Ticket, 0, len(rows))
	for _, r := range rows {
		out = append(out, *toTicket(r.ID, r.UserID, "", r.Subject, r.Status, r.Priority,
			r.CreatedAt, r.UpdatedAt))
	}
	return out, nil
}

func (s *Service) AdminListTickets(ctx context.Context, status string, limit, offset int32) ([]Ticket, error) {
	if status != "" && status != "open" && status != "answered" && status != "closed" {
		return nil, ErrInvalidInput
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.q.AdminListTickets(ctx, dbgen.AdminListTicketsParams{
		StatusFilter: status, Lim: limit, Off: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("admin list tickets: %w", err)
	}
	out := make([]Ticket, 0, len(rows))
	for _, r := range rows {
		out = append(out, *toTicket(r.ID, r.UserID, "", r.Subject, r.Status, r.Priority,
			r.CreatedAt, r.UpdatedAt))
	}
	return out, nil
}

func (s *Service) SetTicketStatus(ctx context.Context, id, status string) (*Ticket, error) {
	if status != "open" && status != "answered" && status != "closed" {
		return nil, ErrInvalidInput
	}
	row, err := s.q.SetTicketStatus(ctx, dbgen.SetTicketStatusParams{ID: id, Status: status})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("set status: %w", err)
	}
	return toTicket(row.ID, row.UserID, "", row.Subject, row.Status, row.Priority,
		row.CreatedAt, row.UpdatedAt), nil
}

func (s *Service) AddMessage(ctx context.Context, ticketID, authorID string, isStaff bool, body string) (*TicketMessage, error) {
	if len(body) < 1 || len(body) > 10000 {
		return nil, ErrInvalidInput
	}
	row, err := s.q.AddTicketMessage(ctx, dbgen.AddTicketMessageParams{
		TicketID: ticketID, AuthorID: authorID, IsStaff: isStaff, Body: body,
	})
	if err != nil {
		return nil, fmt.Errorf("add message: %w", err)
	}
	return &TicketMessage{
		ID: row.ID, TicketID: row.TicketID, AuthorID: row.AuthorID,
		IsStaff: row.IsStaff, Body: row.Body, CreatedAtUnix: row.CreatedAt.Unix(),
	}, nil
}

func (s *Service) ListMessages(ctx context.Context, ticketID, requesterID string, isAdmin bool) ([]TicketMessage, error) {
	if _, err := s.GetTicket(ctx, ticketID, requesterID, isAdmin); err != nil {
		return nil, err
	}
	rows, err := s.q.ListTicketMessages(ctx, ticketID)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	out := make([]TicketMessage, 0, len(rows))
	for _, r := range rows {
		out = append(out, TicketMessage{
			ID: r.ID, TicketID: r.TicketID, AuthorID: r.AuthorID,
			IsStaff: r.IsStaff, Body: r.Body, CreatedAtUnix: r.CreatedAt.Unix(),
		})
	}
	return out, nil
}

// ---- helpers ----

func buildAdminUser(
	id, email, username, currency string,
	isAdmin, isActive bool,
	kycStatus string, kycOverride bool,
	disabledAt *time.Time, disableReason *string,
	createdAt time.Time,
) AdminUser {
	u := AdminUser{
		ID: id, Email: email, Username: username, PreferredCurrency: currency,
		IsAdmin: isAdmin, IsActive: isActive, KYCStatus: kycStatus, KYCOverride: kycOverride,
		CreatedAtUnix: createdAt.Unix(),
	}
	if disabledAt != nil && !disabledAt.IsZero() {
		t := disabledAt.Unix()
		u.DisabledAtUnix = &t
	}
	if disableReason != nil {
		u.DisableReason = *disableReason
	}
	return u
}

func toWarning(id, userID, issuedBy, reason, severity string, ackAt *int64, createdAt time.Time) *Warning {
	return &Warning{
		ID: id, UserID: userID, IssuedBy: issuedBy, Reason: reason,
		Severity: severity, AcknowledgedAtUnix: ackAt, CreatedAtUnix: createdAt.Unix(),
	}
}

func toTicket(id, userID, userUsername, subject, status, priority string, created, updated time.Time) *Ticket {
	return &Ticket{
		ID: id, UserID: userID, UserUsername: userUsername, Subject: subject, Status: status,
		Priority: priority, CreatedAtUnix: created.Unix(), UpdatedAtUnix: updated.Unix(),
	}
}

func ptrString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

var _ = pgtype.Text{}
