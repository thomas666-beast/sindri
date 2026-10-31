-- name: GetWalletByUserAndCurrency :one
SELECT id, user_id, currency_code, available, held, updated_at
FROM wallets
WHERE user_id = $1 AND currency_code = $2;

-- name: ListWalletsByUser :many
SELECT id, user_id, currency_code, available, held, updated_at
FROM wallets
WHERE user_id = $1
ORDER BY currency_code;

-- name: CreateWallet :one
INSERT INTO wallets (user_id, currency_code, available, held)
VALUES ($1, $2, 0, 0)
RETURNING id, user_id, currency_code, available, held, updated_at;

-- name: LockWalletForUpdate :one
SELECT id, user_id, currency_code, available, held, updated_at
FROM wallets
WHERE id = $1
FOR UPDATE;

-- name: AddToAvailable :one
UPDATE wallets
SET available = available + $2, updated_at = now()
WHERE id = $1
RETURNING available;

-- name: SubtractFromAvailable :one
UPDATE wallets
SET available = available - $2, updated_at = now()
WHERE id = $1 AND available >= $2
RETURNING available;

-- name: InsertWalletTransaction :one
INSERT INTO wallet_transactions (
    wallet_id, user_id, currency_code, type, direction, amount,
    balance_after, idempotency_key, metadata, order_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id, wallet_id, user_id, currency_code, type, direction,
          amount, balance_after, status, idempotency_key, created_at;

-- name: GetTransactionByIdempotencyKey :one
SELECT id, wallet_id, user_id, currency_code, type, direction,
       amount, balance_after, status, idempotency_key, created_at
FROM wallet_transactions
WHERE idempotency_key = $1;

-- name: HoldAvailable :one
UPDATE wallets
SET available = available - $2, held = held + $2, updated_at = now()
WHERE id = $1 AND available >= $2
RETURNING available, held;

-- name: ReleaseHeld :one
UPDATE wallets
SET held = held - $2, updated_at = now()
WHERE id = $1 AND held >= $2
RETURNING available, held;

-- name: AddAvailable :one
UPDATE wallets
SET available = available + $2, updated_at = now()
WHERE id = $1
RETURNING available;

-- name: GetSystemUserID :one
SELECT id FROM users WHERE is_system = TRUE LIMIT 1;

-- name: ListTransactionsForUser :many
SELECT
    t.id,
    t.type,
    t.direction,
    t.amount,
    t.currency_code,
    t.balance_after,
    t.order_id,
    t.status,
    t.metadata,
    t.created_at
FROM wallet_transactions t
WHERE t.user_id = $1
  AND (sqlc.arg('currency')::text = '' OR t.currency_code = sqlc.arg('currency')::text)
  -- Hide the initial "hold" phase of a withdrawal from public history
  AND NOT (t.type = 'withdraw' AND t.metadata->>'phase' = 'request')
  AND (
      sqlc.narg('cursor_ts')::timestamptz IS NULL
      OR t.created_at < sqlc.narg('cursor_ts')::timestamptz
      OR (t.created_at = sqlc.narg('cursor_ts')::timestamptz
          AND t.id < sqlc.narg('cursor_id')::uuid)
  )
ORDER BY t.created_at DESC, t.id DESC
LIMIT sqlc.arg('lim');

-- name: AdminListTransactions :many
SELECT 
    t.id, t.user_id, u.username, t.currency_code, t.type, t.direction, 
    t.amount, t.balance_after, t.order_id, t.status, t.metadata, t.created_at
FROM wallet_transactions t
JOIN users u ON u.id = t.user_id
WHERE (sqlc.arg('type_filter')::text = '' OR t.type = sqlc.arg('type_filter')::text)
  AND (sqlc.arg('id_filter')::text = '' OR t.id = sqlc.arg('id_filter')::uuid)
  AND (sqlc.arg('user_filter')::text = '' OR t.user_id = sqlc.arg('user_filter')::uuid)
ORDER BY t.created_at DESC, t.id DESC
LIMIT $1 OFFSET $2;
