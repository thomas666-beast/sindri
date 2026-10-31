-- name: CreateWarning :one
INSERT INTO warnings (user_id, issued_by, reason, severity)
VALUES ($1, $2, $3, $4)
RETURNING id, user_id, issued_by, reason, severity, created_at;

-- name: ListWarningsForUser :many
SELECT id, user_id, issued_by, reason, severity, acknowledged_at, created_at
FROM warnings
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListMyWarnings :many
SELECT id, user_id, issued_by, reason, severity, acknowledged_at, created_at
FROM warnings
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: AcknowledgeWarning :execrows
UPDATE warnings
SET acknowledged_at = now()
WHERE id = $1 AND user_id = $2 AND acknowledged_at IS NULL;
