package chat_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thomas666-beast/sindri/internal/chat"
	"github.com/thomas666-beast/sindri/internal/crypto"
	"github.com/thomas666-beast/sindri/internal/db/dbgen"
	"github.com/thomas666-beast/sindri/internal/user"
)

type fx struct {
	pool    *pgxpool.Pool
	svc     *chat.Service
	a, b, c string
}

func setup(t *testing.T) *fx {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	c, err := crypto.NewCipher([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}

	us := user.NewService(dbgen.New(pool))
	s := time.Now().UnixNano()
	reg := func(prefix string) string {
		u, err := us.Register(context.Background(), user.RegisterInput{
			Email:    fmt.Sprintf("%s_%d@example.com", prefix, s),
			Username: fmt.Sprintf("%s_%d", prefix, s%1_000_000),
			Password: "hunter2hunter2",
		})
		if err != nil {
			t.Fatalf("register %s: %v", prefix, err)
		}
		return u.ID
	}
	a, b, c2 := reg("ca"), reg("cb"), reg("cc")

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `
			DELETE FROM chat_messages WHERE conversation_id IN
			  (SELECT id FROM chat_conversations WHERE created_by IN ($1,$2,$3))
		`, a, b, c2)
		_, _ = pool.Exec(context.Background(), `
			DELETE FROM chat_members WHERE conversation_id IN
			  (SELECT id FROM chat_conversations WHERE created_by IN ($1,$2,$3))
		`, a, b, c2)
		_, _ = pool.Exec(context.Background(), `
			DELETE FROM chat_conversations WHERE created_by IN ($1,$2,$3)
		`, a, b, c2)
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id IN ($1,$2,$3)", a, b, c2)
	})

	return &fx{
		pool: pool,
		svc:  chat.NewService(dbgen.New(pool), c, nil, t.TempDir()),
		a:    a, b: b, c: c2,
	}
}

func TestStartDM_Idempotent(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	c1, err := f.svc.StartDM(ctx, f.a, f.b)
	if err != nil {
		t.Fatalf("dm 1: %v", err)
	}
	c2, err := f.svc.StartDM(ctx, f.b, f.a) // reversed, same result
	if err != nil {
		t.Fatalf("dm 2: %v", err)
	}
	if c1.ID != c2.ID {
		t.Fatalf("expected same conversation id, got %s and %s", c1.ID, c2.ID)
	}

	var count int
	_ = f.pool.QueryRow(ctx,
		`SELECT count(*) FROM chat_conversations WHERE kind='dm' AND (created_by=$1 OR created_by=$2)`,
		f.a, f.b).Scan(&count)
	if count != 1 {
		t.Errorf("dm conversations in DB = %d, want 1", count)
	}
}

func TestStartDM_SelfRejected(t *testing.T) {
	f := setup(t)
	_, err := f.svc.StartDM(context.Background(), f.a, f.a)
	if err != chat.ErrInvalid {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

func TestSendAndListMessages(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	conv, _ := f.svc.StartDM(ctx, f.a, f.b)

	if _, err := f.svc.SendMessage(ctx, conv.ID, f.a, "hello"); err != nil {
		t.Fatalf("send a: %v", err)
	}
	if _, err := f.svc.SendMessage(ctx, conv.ID, f.b, "hi there"); err != nil {
		t.Fatalf("send b: %v", err)
	}

	page, err := f.svc.ListMessages(ctx, conv.ID, f.a, "", 50)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(page.Messages))
	}
	// Newest first
	if page.Messages[0].Body != "hi there" {
		t.Errorf("newest = %q", page.Messages[0].Body)
	}
}

func TestSendMessage_NonMemberForbidden(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	conv, _ := f.svc.StartDM(ctx, f.a, f.b)

	_, err := f.svc.SendMessage(ctx, conv.ID, f.c, "intruder")
	if err != chat.ErrForbidden {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

func TestListMessages_CursorPagination(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	conv, _ := f.svc.StartDM(ctx, f.a, f.b)
	for i := 0; i < 5; i++ {
		if _, err := f.svc.SendMessage(ctx, conv.ID, f.a, fmt.Sprintf("msg %d", i)); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}
	page1, err := f.svc.ListMessages(ctx, conv.ID, f.a, "", 2)
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if len(page1.Messages) != 2 {
		t.Fatalf("page1 len = %d, want 2", len(page1.Messages))
	}
	if !page1.HasMore {
		t.Error("page1 should have more")
	}
	if page1.NextCursor == "" {
		t.Error("page1 should have cursor")
	}

	page2, err := f.svc.ListMessages(ctx, conv.ID, f.a, page1.NextCursor, 2)
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	if len(page2.Messages) != 2 {
		t.Fatalf("page2 len = %d, want 2", len(page2.Messages))
	}
	if page2.Messages[0].ID == page1.Messages[0].ID {
		t.Error("page2 overlaps page1")
	}
}

func TestCreateGroup(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	g, err := f.svc.CreateGroup(ctx, f.a, "test group", []string{f.b, f.c})
	if err != nil {
		t.Fatalf("group: %v", err)
	}
	if g.Kind != "group" {
		t.Errorf("kind = %q", g.Kind)
	}
	if g.Title != "test group" {
		t.Errorf("title = %q", g.Title)
	}

	members, _ := f.svc.Members(ctx, g.ID)
	if len(members) != 3 {
		t.Errorf("member count = %d, want 3", len(members))
	}
}

func TestMarkRead_UpdatesLastRead(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	conv, err := f.svc.StartDM(ctx, f.a, f.b)
	if err != nil {
		t.Fatalf("start dm: %v", err)
	}
	if _, err := f.svc.SendMessage(ctx, conv.ID, f.a, "hello"); err != nil {
		t.Fatalf("send: %v", err)
	}
	if err := f.svc.MarkRead(ctx, conv.ID, f.b); err != nil {
		t.Fatalf("mark read: %v", err)
	}

	c, err := f.svc.Get(ctx, conv.ID, f.b)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	for _, m := range c.Members {
		if m.ID == f.b && m.LastReadAtUnix == nil {
			t.Error("B should have last_read_at set")
		}
		if m.ID == f.a && m.LastReadAtUnix != nil {
			t.Error("A should not have last_read_at set")
		}
	}
}

func TestMarkRead_NonMemberForbidden(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	conv, _ := f.svc.StartDM(ctx, f.a, f.b)

	err := f.svc.MarkRead(ctx, conv.ID, f.c)
	if !errors.Is(err, chat.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

func TestListMine_IncludesReadState(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	conv, _ := f.svc.StartDM(ctx, f.a, f.b)
	if err := f.svc.MarkRead(ctx, conv.ID, f.a); err != nil {
		t.Fatalf("mark read: %v", err)
	}

	list, err := f.svc.ListMine(ctx, f.a, 50, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var target *chat.Conversation
	for i := range list {
		if list[i].ID == conv.ID {
			target = &list[i]
			break
		}
	}
	if target == nil {
		t.Fatal("conversation not in list")
	}
	for _, m := range target.Members {
		if m.ID == f.a && m.LastReadAtUnix == nil {
			t.Error("A should have last_read_at set in list")
		}
	}
}
