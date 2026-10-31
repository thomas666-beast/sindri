-- name: CreateWithdrawalRequest :one
INSERT INTO withdrawal_requests (user_id, currency_code, amount)
VALUES ($1, $2, $3)
RETURNING id, user_id, currency_code, amount, status, rejection_reason,
          reviewed_by, reviewed_at, created_at;

-- name: GetWithdrawalRequest :one
SELECT id, user_id, currency_code, amount, status, rejection_reason,
       reviewed_by, reviewed_at, created_at
FROM withdrawal_requests WHERE id = $1;

-- name: ListWithdrawalsForUser :many
SELECT id, user_id, currency_code, amount, status, rejection_reason,
       reviewed_by, reviewed_at, created_at
FROM withdrawal_requests
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: AdminListWithdrawals :many
SELECT w.id, w.user_id, u.username, w.currency_code, w.amount, w.status, w.rejection_reason,
       w.reviewed_by, COALESCE(ru.username, '') AS reviewed_by_username,
       w.reviewed_at, w.created_at
FROM withdrawal_requests w
JOIN users u ON u.id = w.user_id
LEFT JOIN users ru ON ru.id = w.reviewed_by
WHERE (sqlc.arg('status_filter')::text = '' OR w.status = sqlc.arg('status_filter')::text)
  AND (
    sqlc.arg('id_filter')::text = ''
    OR (length(sqlc.arg('id_filter')::text) = 36 AND w.id = sqlc.arg('id_filter')::uuid)
    OR (length(sqlc.arg('id_filter')::text) != 36 AND w.id::text ILIKE '%' || sqlc.arg('id_filter')::text || '%')
  )
ORDER BY w.created_at DESC
LIMIT $1 OFFSET $2;

-- name: ApproveWithdrawal :one
UPDATE withdrawal_requests
SET status = 'approved', reviewed_by = $2, reviewed_at = now()
WHERE id = $1 AND status = 'pending'
RETURNING id, user_id, currency_code, amount, status;

-- name: RejectWithdrawal :one
UPDATE withdrawal_requests
SET status = 'rejected', reviewed_by = $2, reviewed_at = now(), rejection_reason = $3
WHERE id = $1 AND status = 'pending'
RETURNING id, user_id, currency_code, amount, status, rejection_reason;
