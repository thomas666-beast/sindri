-- name: CreateConversation :one
INSERT INTO chat_conversations (kind, title, dm_key, created_by)
VALUES ($1, $2, $3, $4)
RETURNING id, kind, title, dm_key, created_by, created_at, updated_at;

-- name: GetConversation :one
SELECT id, kind, title, dm_key, created_by, created_at, updated_at
FROM chat_conversations WHERE id = $1;

-- name: GetConversationByDMKey :one
SELECT id, kind, title, dm_key, created_by, created_at, updated_at
FROM chat_conversations WHERE dm_key = $1;

-- name: AddMember :exec
INSERT INTO chat_members (conversation_id, user_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: IsMember :one
SELECT EXISTS (
    SELECT 1 FROM chat_members WHERE conversation_id = $1 AND user_id = $2
) AS is_member;

-- name: ListMembers :many
SELECT user_id FROM chat_members WHERE conversation_id = $1;

-- name: GetConversationMembersWithUsername :many
SELECT u.id, u.username
FROM chat_members m
JOIN users u ON u.id = m.user_id
WHERE m.conversation_id = $1
ORDER BY m.joined_at ASC;

-- name: InsertMessage :one
INSERT INTO chat_messages (conversation_id, author_id, ciphertext, nonce)
VALUES ($1, $2, $3, $4)
RETURNING id, conversation_id, author_id, ciphertext, nonce, created_at;

-- name: ListMessages :many
SELECT 
    m.id, 
    m.conversation_id, 
    m.author_id, 
    m.ciphertext, 
    m.nonce, 
    m.created_at,
    a.id AS attachment_id,
    a.filename AS attachment_filename,
    a.content_type AS attachment_content_type,
    a.size_bytes AS attachment_size_bytes
FROM chat_messages m
JOIN chat_members cm ON cm.conversation_id = m.conversation_id AND cm.user_id = sqlc.arg('user_id')::uuid
LEFT JOIN chat_attachments a ON a.message_id = m.id
WHERE m.conversation_id = sqlc.arg('conversation_id')::uuid
  AND (cm.cleared_at IS NULL OR m.created_at > cm.cleared_at)
  AND (sqlc.narg('before_ts')::timestamptz IS NULL
       OR m.created_at < sqlc.narg('before_ts')::timestamptz
       OR (m.created_at = sqlc.narg('before_ts')::timestamptz
           AND m.id < sqlc.narg('before_id')::uuid))
ORDER BY m.created_at DESC, m.id DESC
LIMIT sqlc.arg('lim');

-- name: TouchConversation :exec
UPDATE chat_conversations SET updated_at = now() WHERE id = $1;

-- name: MarkConversationRead :exec
UPDATE chat_members
SET last_read_at = now()
WHERE conversation_id = $1 AND user_id = $2;

-- name: ListMembersWithReadState :many
SELECT
    u.id,
    u.username,
    m.last_read_at
FROM chat_members m
JOIN users u ON u.id = m.user_id
WHERE m.conversation_id = $1
ORDER BY m.joined_at ASC;

-- name: CountUnreadMessages :one
SELECT count(*)::bigint
FROM chat_messages m
JOIN chat_members cm ON cm.conversation_id = m.conversation_id
WHERE cm.user_id = $1
  AND m.author_id <> $1
  AND (cm.last_read_at IS NULL OR m.created_at > cm.last_read_at);

-- name: ClearChatForUser :exec
UPDATE chat_members
SET cleared_at = now()
WHERE conversation_id = $1 AND user_id = $2;

-- name: RemoveMember :exec
DELETE FROM chat_members
WHERE conversation_id = $1 AND user_id = $2;

-- name: CreateChatAttachment :one
INSERT INTO chat_attachments (message_id, filename, content_type, size_bytes, file_path, nonce)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, message_id, filename, content_type, size_bytes, created_at;

-- name: GetChatAttachment :one
SELECT id, message_id, filename, content_type, size_bytes, file_path, nonce, created_at
FROM chat_attachments WHERE id = $1;

-- name: ListAttachmentsByMessage :many
SELECT id, message_id, filename, content_type, size_bytes, created_at
FROM chat_attachments WHERE message_id = ANY(sqlc.arg('message_ids')::uuid[]);

-- name: GetMessage :one
SELECT id, conversation_id, author_id, ciphertext, nonce, created_at
FROM chat_messages WHERE id = $1;

-- name: GetAttachmentByMessage :one
SELECT id FROM chat_attachments WHERE message_id = $1;

-- name: ListMyConversations :many
SELECT
    c.id,
    c.kind,
    c.title,
    c.created_at,
    c.updated_at,
    (SELECT count(*) FROM chat_members WHERE conversation_id = c.id) AS member_count,
    
    -- Last Message Details
    (SELECT m.ciphertext FROM chat_messages m WHERE m.conversation_id = c.id ORDER BY m.created_at DESC LIMIT 1) AS last_ciphertext,
    (SELECT m.nonce FROM chat_messages m WHERE m.conversation_id = c.id ORDER BY m.created_at DESC LIMIT 1) AS last_nonce,
    
    -- Last Attachment ID (ONLY if it belongs to the last message)
    COALESCE(
        (SELECT a.id::text 
         FROM chat_attachments a 
         JOIN chat_messages m ON m.id = a.message_id 
         WHERE m.conversation_id = c.id 
           AND m.id = (SELECT id FROM chat_messages WHERE conversation_id = c.id ORDER BY created_at DESC LIMIT 1)
         LIMIT 1), 
        ''
    ) AS last_attachment_id,
    
    -- Unread Count
    (
        SELECT count(*)::bigint 
        FROM chat_messages m 
        WHERE m.conversation_id = c.id
          AND m.created_at > COALESCE(
              (SELECT last_read_at FROM chat_members WHERE conversation_id = c.id AND user_id = sqlc.arg('user_id')::uuid),
              '1970-01-01'::timestamptz
          )
          AND m.author_id != sqlc.arg('user_id')::uuid
    ) AS unread_count

FROM chat_conversations c
JOIN chat_members m ON m.conversation_id = c.id
WHERE m.user_id = sqlc.arg('user_id')::uuid
ORDER BY c.updated_at DESC, c.id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

