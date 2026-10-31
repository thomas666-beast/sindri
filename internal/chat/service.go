package chat

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/thomas666-beast/sindri/internal/crypto"
	"github.com/thomas666-beast/sindri/internal/db/dbgen"
)

var (
	ErrNotFound  = errors.New("conversation not found")
	ErrForbidden = errors.New("not a conversation member")
	ErrInvalid   = errors.New("invalid input")
)

type Member struct {
	ID             string `json:"id"`
	Username       string `json:"username"`
	LastReadAtUnix *int64 `json:"last_read_at_unix,omitempty"`
}

type Conversation struct {
	ID              string   `json:"id"`
	Kind            string   `json:"kind"`
	Title           string   `json:"title,omitempty"`
	Members         []Member `json:"members"`
	MemberCount     int64    `json:"member_count"`
	LastMessageAt   *int64   `json:"last_message_at_unix,omitempty"`
	LastMessageText string   `json:"last_message_text,omitempty"`
	UnreadCount     int64    `json:"unread_count"`
	CreatedAtUnix   int64    `json:"created_at_unix"`
}

type Attachment struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
}

type Message struct {
	ID             string      `json:"id"`
	ConversationID string      `json:"conversation_id"`
	AuthorID       string      `json:"author_id"`
	Body           string      `json:"body"`
	CreatedAtUnix  int64       `json:"created_at_unix"`
	Attachment     *Attachment `json:"attachment,omitempty"`
}

type Page struct {
	Messages   []Message `json:"messages"`
	NextCursor string    `json:"next_cursor,omitempty"`
	HasMore    bool      `json:"has_more"`
}

type Notifier interface {
	Notify(ctx context.Context, userID, kind, title, body string, metadata []byte) error
}

type Service struct {
	q          dbgen.Querier
	cipher     *crypto.Cipher
	notifier   Notifier
	uploadsDir string
}

func NewService(q dbgen.Querier, c *crypto.Cipher, notifier Notifier, uploadsDir string) *Service {
	return &Service{q: q, cipher: c, notifier: notifier, uploadsDir: uploadsDir}
}

func (s *Service) StartDM(ctx context.Context, userA, userB string) (*Conversation, error) {
	if userA == userB {
		return nil, ErrInvalid
	}
	key := dmKey(userA, userB)

	if c, err := s.q.GetConversationByDMKey(ctx, &key); err == nil {
		return toConv(c.ID, c.Kind, c.Title, 2, nil, c.CreatedAt, nil), nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("lookup dm: %w", err)
	}

	c, err := s.q.CreateConversation(ctx, dbgen.CreateConversationParams{
		Kind: "dm", Title: nil, DmKey: &key, CreatedBy: userA,
	})
	if err != nil {
		if existing, err2 := s.q.GetConversationByDMKey(ctx, &key); err2 == nil {
			return toConv(existing.ID, existing.Kind, existing.Title, 2, nil, existing.CreatedAt, nil), nil
		}
		return nil, fmt.Errorf("create dm: %w", err)
	}
	_ = s.q.AddMember(ctx, dbgen.AddMemberParams{ConversationID: c.ID, UserID: userA})
	_ = s.q.AddMember(ctx, dbgen.AddMemberParams{ConversationID: c.ID, UserID: userB})

	return toConv(c.ID, c.Kind, c.Title, 2, nil, c.CreatedAt, nil), nil
}

func (s *Service) CreateGroup(ctx context.Context, creatorID, title string, memberIDs []string) (*Conversation, error) {
	if title == "" || len(title) > 200 {
		return nil, ErrInvalid
	}
	if len(memberIDs) < 1 || len(memberIDs) > 100 {
		return nil, ErrInvalid
	}
	c, err := s.q.CreateConversation(ctx, dbgen.CreateConversationParams{
		Kind: "group", Title: &title, DmKey: nil, CreatedBy: creatorID,
	})
	if err != nil {
		return nil, fmt.Errorf("create group: %w", err)
	}

	seen := map[string]struct{}{creatorID: {}}
	for _, id := range memberIDs {
		seen[id] = struct{}{}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		_ = s.q.AddMember(ctx, dbgen.AddMemberParams{ConversationID: c.ID, UserID: id})
	}

	return toConv(c.ID, c.Kind, &title, int64(len(ids)), nil, c.CreatedAt, nil), nil
}

func (s *Service) ListMine(ctx context.Context, userID string, limit, offset int32) ([]Conversation, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.q.ListMyConversations(ctx, dbgen.ListMyConversationsParams{
		UserID: userID, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}
	out := make([]Conversation, 0, len(rows))
	for _, r := range rows {
		memberRows, err := s.q.ListMembersWithReadState(ctx, r.ID)
		if err != nil {
			return nil, fmt.Errorf("members for %s: %w", r.ID, err)
		}
		members := make([]Member, 0, len(memberRows))
		for _, m := range memberRows {
			var lastRead *int64
			if m.LastReadAt.Valid {
				t := m.LastReadAt.Time.Unix()
				lastRead = &t
			}
			members = append(members, Member{
				ID: m.ID, Username: m.Username, LastReadAtUnix: lastRead,
			})
		}

		t := r.UpdatedAt.Unix()
		lastAt := &t

		lastMsgText := "No messages yet"
		if len(r.LastCiphertext) > 0 {
			if r.LastAttachmentID != "" {
				lastMsgText = "📎 Attachment"
			} else {
				if plaintext, err := s.cipher.Decrypt(r.LastCiphertext, r.LastNonce); err == nil {
					lastMsgText = string(plaintext)
				} else {
					lastMsgText = "[Encrypted message]"
				}
			}
		}

		conv := toConv(r.ID, r.Kind, r.Title, r.MemberCount, lastAt, r.CreatedAt, members)
		conv.LastMessageText = lastMsgText
		conv.UnreadCount = r.UnreadCount

		out = append(out, *conv)
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, convID, userID string) (*Conversation, error) {
	ok, err := s.isMember(ctx, convID, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrForbidden
	}
	c, err := s.q.GetConversation(ctx, convID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get: %w", err)
	}

	memberRows, err := s.q.ListMembersWithReadState(ctx, c.ID)
	if err != nil {
		return nil, fmt.Errorf("members: %w", err)
	}
	members := make([]Member, 0, len(memberRows))
	for _, m := range memberRows {
		var lastRead *int64
		if m.LastReadAt.Valid {
			t := m.LastReadAt.Time.Unix()
			lastRead = &t
		}
		members = append(members, Member{
			ID: m.ID, Username: m.Username, LastReadAtUnix: lastRead,
		})
	}

	return toConv(c.ID, c.Kind, c.Title, int64(len(members)), nil, c.CreatedAt, members), nil
}

func (s *Service) ListMessages(ctx context.Context, convID, userID, cursor string, limit int32) (*Page, error) {
	ok, err := s.isMember(ctx, convID, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrForbidden
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	params := dbgen.ListMessagesParams{
		UserID:         userID,
		ConversationID: convID,
		Lim:            limit + 1,
	}
	if cursor != "" {
		t, id, err := decodeCursor(cursor)
		if err != nil {
			return nil, err
		}
		params.BeforeTs = pgtype.Timestamptz{Time: t, Valid: true}
		params.BeforeID = pgtype.UUID{Bytes: [16]byte(id), Valid: true}
	}

	rows, err := s.q.ListMessages(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}

	hasMore := len(rows) > int(limit)
	if hasMore {
		rows = rows[:limit]
	}

	out := make([]Message, 0, len(rows))
	for _, r := range rows {
		msg := Message{
			ID: r.ID, ConversationID: r.ConversationID, AuthorID: r.AuthorID,
			CreatedAtUnix: r.CreatedAt.Unix(),
		}

		if r.AttachmentID.Valid {
			filename := ""
			if r.AttachmentFilename != nil {
				filename = *r.AttachmentFilename
			}
			contentType := ""
			if r.AttachmentContentType != nil {
				contentType = *r.AttachmentContentType
			}
			size := int64(0)
			if r.AttachmentSizeBytes != nil {
				size = *r.AttachmentSizeBytes
			}

			msg.Attachment = &Attachment{
				ID:          pgUUIDString(r.AttachmentID),
				Filename:    filename,
				ContentType: contentType,
				SizeBytes:   size,
			}
			msg.Body = filename 
		} else {
			plaintext, err := s.cipher.Decrypt(r.Ciphertext, r.Nonce)
			if err == nil {
				msg.Body = string(plaintext)
			}
		}

		out = append(out, msg)
	}

	page := &Page{Messages: out, HasMore: hasMore}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		page.NextCursor = encodeCursor(last.CreatedAt, last.ID)
	}
	return page, nil
}

func (s *Service) SendMessage(ctx context.Context, convID, authorID, body string) (*Message, error) {
	if len(body) < 1 || len(body) > 10000 {
		return nil, ErrInvalid
	}
	ok, err := s.isMember(ctx, convID, authorID)
	if err != nil || !ok {
		return nil, ErrForbidden
	}

	// ── IRC COMMANDS ──
	if strings.HasPrefix(body, "/") {
		parts := strings.Fields(body)
		cmd := strings.ToLower(parts[0])

		switch cmd {
		case "/clear":
			_ = s.q.ClearChatForUser(ctx, dbgen.ClearChatForUserParams{
				ConversationID: convID, UserID: authorID,
			})
			return &Message{
				ID: "sys", ConversationID: convID, AuthorID: authorID,
				Body: "Chat cleared.", CreatedAtUnix: time.Now().Unix(),
			}, nil

		case "/leave":
			_ = s.q.RemoveMember(ctx, dbgen.RemoveMemberParams{
				ConversationID: convID, UserID: authorID,
			})
			return &Message{
				ID: "sys", ConversationID: convID, AuthorID: authorID,
				Body: "You left the group.", CreatedAtUnix: time.Now().Unix(),
			}, nil

		case "/invite":
			if len(parts) < 2 {
				return nil, fmt.Errorf("usage: /invite @username")
			}
			targetUsername := strings.TrimPrefix(parts[1], "@")
			targetID, err := s.ResolveUserID(ctx, targetUsername)
			if err != nil {
				return nil, fmt.Errorf("user not found: %s", targetUsername)
			}
			isMember, _ := s.isMember(ctx, convID, targetID)
			if isMember {
				return nil, fmt.Errorf("%s is already in the group", targetUsername)
			}
			_ = s.q.AddMember(ctx, dbgen.AddMemberParams{
				ConversationID: convID, UserID: targetID,
			})
			return &Message{
				ID: "sys", ConversationID: convID, AuthorID: authorID,
				Body: fmt.Sprintf("Invited @%s to the group.", targetUsername), CreatedAtUnix: time.Now().Unix(),
			}, nil

		case "/remove":
			if len(parts) < 2 {
				return nil, fmt.Errorf("usage: /remove @username")
			}
			targetUsername := strings.TrimPrefix(parts[1], "@")
			targetID, err := s.ResolveUserID(ctx, targetUsername)
			if err != nil {
				return nil, fmt.Errorf("user not found: %s", targetUsername)
			}
			_ = s.q.RemoveMember(ctx, dbgen.RemoveMemberParams{
				ConversationID: convID, UserID: targetID,
			})
			return &Message{
				ID: "sys", ConversationID: convID, AuthorID: authorID,
				Body: fmt.Sprintf("Removed @%s from the group.", targetUsername), CreatedAtUnix: time.Now().Unix(),
			}, nil
		}
	}

	// ── NORMAL MESSAGE ──
	ct, nonce, err := s.cipher.Encrypt([]byte(body))
	if err != nil {
		return nil, fmt.Errorf("encrypt: %w", err)
	}

	row, err := s.q.InsertMessage(ctx, dbgen.InsertMessageParams{
		ConversationID: convID, AuthorID: authorID,
		Ciphertext: ct, Nonce: nonce,
	})
	if err != nil {
		return nil, fmt.Errorf("insert: %w", err)
	}
	
	_ = s.q.TouchConversation(ctx, convID)

	return &Message{
		ID: row.ID, ConversationID: row.ConversationID, AuthorID: row.AuthorID,
		Body: body, CreatedAtUnix: row.CreatedAt.Unix(),
	}, nil
}

func (s *Service) UploadAttachment(ctx context.Context, convID, authorID, filename, contentType string, data []byte) (*Message, error) {
	ok, _ := s.isMember(ctx, convID, authorID)
	if !ok {
		return nil, ErrForbidden
	}

	// 1. Encrypt the file data
	encryptedData, nonce, err := s.cipher.Encrypt(data)
	if err != nil {
		return nil, fmt.Errorf("encrypt file: %w", err)
	}

	// 2. Encrypt the filename for the message body
	encryptedName, nameNonce, err := s.cipher.Encrypt([]byte(filename))
	if err != nil {
		return nil, fmt.Errorf("encrypt filename: %w", err)
	}

	// 3. Save encrypted bytes to disk
	secureDir := filepath.Join(s.uploadsDir, "secure")
	if err := os.MkdirAll(secureDir, 0o755); err != nil {
		return nil, fmt.Errorf("create secure dir: %w", err)
	}

	storedName := uuid.NewString() + ".enc"
	filePath := filepath.Join(secureDir, storedName)

	if err := os.WriteFile(filePath, encryptedData, 0o600); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}

	// 4. Insert the message with the encrypted filename as the body
	row, err := s.q.InsertMessage(ctx, dbgen.InsertMessageParams{
		ConversationID: convID, AuthorID: authorID,
		Ciphertext: encryptedName, Nonce: nameNonce,
	})
	if err != nil {
		_ = os.Remove(filePath) // rollback
		return nil, fmt.Errorf("insert message: %w", err)
	}

	// 5. Insert the attachment record
	safeFilename := uuid.NewString()
	att, err := s.q.CreateChatAttachment(ctx, dbgen.CreateChatAttachmentParams{
		MessageID: row.ID, Filename: safeFilename, ContentType: contentType,
		SizeBytes: int64(len(data)), FilePath: &filePath, Nonce: nonce,
	})
	if err != nil {
		_ = os.Remove(filePath)
		return nil, fmt.Errorf("insert attachment: %w", err)
	}
	
	_ = s.q.TouchConversation(ctx, convID)

	return &Message{
		ID: row.ID, ConversationID: row.ConversationID, AuthorID: row.AuthorID,
		Body: filename, CreatedAtUnix: row.CreatedAt.Unix(),
		Attachment: &Attachment{
			ID: att.ID, Filename: filename, 
			ContentType: att.ContentType, SizeBytes: att.SizeBytes,
		},
	}, nil
}

func (s *Service) DownloadAttachment(ctx context.Context, attID, userID string) ([]byte, string, string, error) {
	att, err := s.q.GetChatAttachment(ctx, attID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", "", ErrNotFound
		}
		return nil, "", "", fmt.Errorf("get attachment: %w", err)
	}

	msg, err := s.q.GetMessage(ctx, att.MessageID)
	if err != nil {
		return nil, "", "", fmt.Errorf("get message: %w", err)
	}

	ok, _ := s.isMember(ctx, msg.ConversationID, userID)
	if !ok {
		return nil, "", "", ErrForbidden
	}

	// Read from disk
	if att.FilePath == nil {
		return nil, "", "", fmt.Errorf("legacy attachment (no file path)")
	}

	encryptedData, err := os.ReadFile(*att.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", "", fmt.Errorf("file no longer exists on server")
		}
		return nil, "", "", fmt.Errorf("read file: %w", err)
	}

	data, err := s.cipher.Decrypt(encryptedData, att.Nonce)
	if err != nil {
		return nil, "", "", fmt.Errorf("decrypt attachment: %w", err)
	}

	// Decrypt the real filename from the message body
	filename := att.Filename
	if plaintext, err := s.cipher.Decrypt(msg.Ciphertext, msg.Nonce); err == nil {
		filename = string(plaintext)
	}

	return data, filename, att.ContentType, nil
}

func (s *Service) MarkRead(ctx context.Context, convID, userID string) error {
	ok, err := s.isMember(ctx, convID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return s.q.MarkConversationRead(ctx, dbgen.MarkConversationReadParams{
		ConversationID: convID, UserID: userID,
	})
}

func (s *Service) Members(ctx context.Context, convID string) ([]string, error) {
	return s.q.ListMembers(ctx, convID)
}

func (s *Service) ResolveUserID(ctx context.Context, username string) (string, error) {
	id, err := s.q.GetUserIDByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("resolve username: %w", err)
	}
	return id, nil
}

func (s *Service) isMember(ctx context.Context, convID, userID string) (bool, error) {
	ok, err := s.q.IsMember(ctx, dbgen.IsMemberParams{
		ConversationID: convID, UserID: userID,
	})
	if err != nil {
		return false, fmt.Errorf("is member: %w", err)
	}
	return ok, nil
}

func (s *Service) UnreadCount(ctx context.Context, userID string) (int64, error) {
	return s.q.CountUnreadMessages(ctx, userID)
}

// ---- helpers ----

func dmKey(a, b string) string {
	if strings.Compare(a, b) < 0 {
		return a + ":" + b
	}
	return b + ":" + a
}

func toConv(id, kind string, title *string, memberCount int64, lastAt *int64, created time.Time, members []Member) *Conversation {
	t := ""
	if title != nil {
		t = *title
	}
	if members == nil {
		members = []Member{}
	}
	return &Conversation{
		ID: id, Kind: kind, Title: t, Members: members, MemberCount: memberCount,
		LastMessageAt: lastAt, CreatedAtUnix: created.Unix(),
	}
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