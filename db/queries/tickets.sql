-- name: CreateTicket :one
INSERT INTO tickets (user_id, subject, priority)
VALUES ($1, $2, $3)
RETURNING id, user_id, subject, status, priority, created_at, updated_at;

-- name: GetTicket :one
SELECT t.*, u.username AS user_username
FROM tickets t
JOIN users u ON u.id = t.user_id
WHERE t.id = $1;

-- name: ListTicketsForUser :many
SELECT id, user_id, subject, status, priority, created_at, updated_at
FROM tickets
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: AdminListTickets :many
SELECT id, user_id, subject, status, priority, created_at, updated_at
FROM tickets
WHERE (sqlc.arg('status_filter')::text = '' OR status = sqlc.arg('status_filter')::text)
ORDER BY created_at DESC
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');

-- name: SetTicketStatus :one
UPDATE tickets SET status = $2, updated_at = now()
WHERE id = $1
RETURNING id, user_id, subject, status, priority, created_at, updated_at;

-- name: AddTicketMessage :one
INSERT INTO ticket_messages (ticket_id, author_id, is_staff, body)
VALUES ($1, $2, $3, $4)
RETURNING id, ticket_id, author_id, is_staff, body, created_at;

-- name: ListTicketMessages :many
SELECT id, ticket_id, author_id, is_staff, body, created_at
FROM ticket_messages
WHERE ticket_id = $1
ORDER BY created_at ASC;
