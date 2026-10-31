-- name: CreateNotification :one
INSERT INTO notifications (user_id, kind, title, body, metadata)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, user_id, kind, title, body, metadata, read_at, created_at;

-- name: ListNotifications :many
SELECT id, user_id, kind, title, body, metadata, read_at, created_at
FROM notifications
WHERE user_id = $1
  AND (sqlc.narg('only_unread')::boolean IS NULL OR sqlc.narg('only_unread')::boolean = FALSE OR read_at IS NULL)
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: CountUnreadNotifications :one
SELECT count(*) FROM notifications WHERE user_id = $1 AND read_at IS NULL;

-- name: MarkNotificationRead :execrows
UPDATE notifications SET read_at = now()
WHERE id = $1 AND user_id = $2 AND read_at IS NULL;

-- name: MarkAllNotificationsRead :execrows
UPDATE notifications SET read_at = now()
WHERE user_id = $1 AND read_at IS NULL;
