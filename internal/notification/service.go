package notification

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/thomas666-beast/sindri/internal/db/dbgen"
)

var ErrNotFound = errors.New("notification not found")

type Notification struct {
	ID            string `json:"id"`
	UserID        string `json:"user_id"`
	Kind          string `json:"kind"`
	Title         string `json:"title"`
	Body          string `json:"body"`
	Metadata      string `json:"metadata"`
	ReadAtUnix    *int64 `json:"read_at_unix,omitempty"`
	CreatedAtUnix int64  `json:"created_at_unix"`
}

type Page struct {
	Notifications []Notification `json:"notifications"`
	Limit         int32          `json:"limit"`
	Offset        int32          `json:"offset"`
}

type Service struct {
	q        dbgen.Querier
	OnCreate func(*Notification) // Add this hook
}

func NewService(q dbgen.Querier) *Service { return &Service{q: q} }

func (s *Service) Notify(ctx context.Context, userID, kind, title, body string, metadata []byte) error {
	_, err := s.Create(ctx, userID, kind, title, body, metadata)
	return err
}

func (s *Service) Create(ctx context.Context, userID, kind, title, body string, metadata []byte) (*Notification, error) {
	if metadata == nil {
		metadata = []byte("{}")
	}
	row, err := s.q.CreateNotification(ctx, dbgen.CreateNotificationParams{
		UserID:   userID,
		Kind:     kind,
		Title:    title,
		Body:     body,
		Metadata: metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("create notification: %w", err)
	}
	
	n := toDTO(row.ID, row.UserID, row.Kind, row.Title, row.Body, string(row.Metadata),
		pgTimeInt64(row.ReadAt), row.CreatedAt.Unix())

	// TRIGGER WEBSOCKET PUSH
	if s.OnCreate != nil {
		s.OnCreate(n)
	}

	return n, nil
}

func (s *Service) List(ctx context.Context, userID string, onlyUnread bool, limit, offset int32) (*Page, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}

	var unreadFilter *bool
	if onlyUnread {
		unreadFilter = &onlyUnread
	}

	rows, err := s.q.ListNotifications(ctx, dbgen.ListNotificationsParams{
		UserID:     userID,
		OnlyUnread: unreadFilter,
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}

	out := make([]Notification, 0, len(rows))
	for _, r := range rows {
		out = append(out, *toDTO(r.ID, r.UserID, r.Kind, r.Title, r.Body, string(r.Metadata),
			pgTimeInt64(r.ReadAt), r.CreatedAt.Unix()))
	}
	return &Page{Notifications: out, Limit: limit, Offset: offset}, nil
}

func (s *Service) UnreadCount(ctx context.Context, userID string) (int64, error) {
	return s.q.CountUnreadNotifications(ctx, userID)
}

func (s *Service) MarkRead(ctx context.Context, notificationID, userID string) error {
	n, err := s.q.MarkNotificationRead(ctx, dbgen.MarkNotificationReadParams{
		ID: notificationID, UserID: userID,
	})
	if err != nil {
		return fmt.Errorf("mark read: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) MarkAllRead(ctx context.Context, userID string) (int64, error) {
	n, err := s.q.MarkAllNotificationsRead(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("mark all read: %w", err)
	}
	return n, nil
}

func toDTO(id, userID, kind, title, body, metadata string, readAt *int64, createdAt int64) *Notification {
	return &Notification{
		ID: id, UserID: userID, Kind: kind, Title: title, Body: body,
		Metadata: metadata, ReadAtUnix: readAt, CreatedAtUnix: createdAt,
	}
}

func pgTimeInt64(t pgtype.Timestamptz) *int64 {
	if !t.Valid {
		return nil
	}
	v := t.Time.Unix()
	return &v
}
