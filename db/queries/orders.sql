-- name: CreateOrder :one
INSERT INTO orders (
    buyer_id, seller_id, product_id, currency_code, amount,
    commission_amount, commission_percent, status, auto_release_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, 'held', $8)
RETURNING *;

-- name: GetOrder :one
SELECT o.*, bu.username AS buyer_username, su.username AS seller_username
FROM orders o
JOIN users bu ON bu.id = o.buyer_id
JOIN users su ON su.id = o.seller_id
WHERE o.id = $1;

-- name: GetOrderForUpdate :one
SELECT * FROM orders WHERE id = $1 FOR UPDATE;

-- name: ListOrdersForUser :many
SELECT o.*, bu.username AS buyer_username, su.username AS seller_username
FROM orders o
JOIN users bu ON bu.id = o.buyer_id
JOIN users su ON su.id = o.seller_id
WHERE (o.buyer_id = sqlc.arg('user_id')::uuid OR o.seller_id = sqlc.arg('user_id')::uuid)
  AND (
    sqlc.arg('role')::text = ''
    OR (sqlc.arg('role')::text = 'buyer'  AND o.buyer_id  = sqlc.arg('user_id')::uuid)
    OR (sqlc.arg('role')::text = 'seller' AND o.seller_id = sqlc.arg('user_id')::uuid)
  )
  AND (
    sqlc.arg('id_filter')::text = ''
    OR (length(sqlc.arg('id_filter')::text) = 36 AND o.id = sqlc.arg('id_filter')::uuid)
    OR (length(sqlc.arg('id_filter')::text) != 36 AND o.id::text ILIKE '%' || sqlc.arg('id_filter')::text || '%')
  )
  AND (
    sqlc.narg('cursor_ts')::timestamptz IS NULL
    OR o.created_at < sqlc.narg('cursor_ts')::timestamptz
    OR (o.created_at = sqlc.narg('cursor_ts')::timestamptz AND o.id < sqlc.narg('cursor_id')::uuid)
  )
ORDER BY o.created_at DESC, o.id DESC
LIMIT sqlc.arg('lim');

-- name: RefundOrder :one
UPDATE orders
SET status = 'refunded', released_at = now(), released_by = $2, updated_at = now()
WHERE id = $1 AND status IN ('held', 'disputed')
RETURNING *;

-- name: FindStaleHeldOrders :many
SELECT * FROM orders
WHERE status = 'held' AND auto_release_at IS NOT NULL AND auto_release_at <= now()
ORDER BY auto_release_at
LIMIT $1;

-- name: CreatePendingInvoice :one
INSERT INTO orders (
    buyer_id, seller_id, product_id, currency_code, amount,
    commission_amount, commission_percent, status,
    invoice_title, invoice_description
)
VALUES ($1, $2, NULL, $3, $4, $5, $6, 'pending', $7, $8)
RETURNING *;

-- name: MarkInvoicePaid :one
UPDATE orders
SET status = 'held', auto_release_at = $2, updated_at = now()
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: DisputeOrder :one
UPDATE orders
SET status = 'disputed', updated_at = now()
WHERE id = $1 AND status = 'held'
RETURNING *;

-- name: AdminListOrdersByStatus :many
SELECT o.*, bu.username AS buyer_username, su.username AS seller_username
FROM orders o
JOIN users bu ON bu.id = o.buyer_id
JOIN users su ON su.id = o.seller_id
WHERE (sqlc.arg('status_filter')::text = '' OR o.status = sqlc.arg('status_filter')::text)
ORDER BY o.created_at DESC, o.id DESC
LIMIT $1 OFFSET $2;

-- name: AdminListDisputedOrdersWithReason :many
SELECT
    o.*,
    bu.username AS buyer_username,
    su.username AS seller_username,
    d.reason    AS dispute_reason,
    du.username AS dispute_opened_by_username
FROM orders o
JOIN users bu ON bu.id = o.buyer_id
JOIN users su ON su.id = o.seller_id
JOIN disputes d ON d.order_id = o.id
LEFT JOIN users du ON du.id = d.opened_by
WHERE o.status = 'disputed'
  AND (
    sqlc.arg('id_filter')::text = ''
    OR (length(sqlc.arg('id_filter')::text) = 36 AND o.id = sqlc.arg('id_filter')::uuid)
    OR (length(sqlc.arg('id_filter')::text) != 36 AND o.id::text ILIKE '%' || sqlc.arg('id_filter')::text || '%')
  )
ORDER BY o.created_at DESC, o.id DESC
LIMIT $1 OFFSET $2;

-- name: AdminGetOrderWithDispute :one
SELECT
    o.*,
    bu.username AS buyer_username,
    su.username AS seller_username,
    d.reason    AS dispute_reason,
    du.username AS dispute_opened_by_username
FROM orders o
JOIN users bu ON bu.id = o.buyer_id
JOIN users su ON su.id = o.seller_id
LEFT JOIN disputes d ON d.order_id = o.id
LEFT JOIN users du ON du.id = d.opened_by
WHERE o.id = $1;

-- name: ListInvoicesForUser :many
SELECT o.*, bu.username AS buyer_username, su.username AS seller_username
FROM orders o
JOIN users bu ON bu.id = o.buyer_id
JOIN users su ON su.id = o.seller_id
WHERE o.invoice_title IS NOT NULL
  AND (o.buyer_id = sqlc.arg('user_id')::uuid OR o.seller_id = sqlc.arg('user_id')::uuid)
  AND (
    sqlc.arg('role')::text = ''
    OR (sqlc.arg('role')::text = 'buyer'  AND o.buyer_id  = sqlc.arg('user_id')::uuid)
    OR (sqlc.arg('role')::text = 'seller' AND o.seller_id = sqlc.arg('user_id')::uuid)
  )
  AND (
    sqlc.arg('status')::text = ''
    OR o.status = sqlc.arg('status')::text
  )
  AND (
    sqlc.arg('id_filter')::text = ''
    OR (length(sqlc.arg('id_filter')::text) = 36 AND o.id = sqlc.arg('id_filter')::uuid)
    OR (length(sqlc.arg('id_filter')::text) != 36 AND o.id::text ILIKE '%' || sqlc.arg('id_filter')::text || '%')
  )
ORDER BY o.created_at DESC, o.id DESC
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');

-- name: CountInvoicesForUser :one
SELECT count(*)
FROM orders o
WHERE o.invoice_title IS NOT NULL
  AND (o.buyer_id = sqlc.arg('user_id')::uuid OR o.seller_id = sqlc.arg('user_id')::uuid)
  AND (
    sqlc.arg('role')::text = ''
    OR (sqlc.arg('role')::text = 'buyer'  AND o.buyer_id  = sqlc.arg('user_id')::uuid)
    OR (sqlc.arg('role')::text = 'seller' AND o.seller_id = sqlc.arg('user_id')::uuid)
  )
  AND (
    sqlc.arg('status')::text = ''
    OR o.status = sqlc.arg('status')::text
  );

-- name: GetInvoice :one
SELECT o.*, bu.username AS buyer_username, su.username AS seller_username
FROM orders o
JOIN users bu ON bu.id = o.buyer_id
JOIN users su ON su.id = o.seller_id
WHERE o.id = $1 AND o.invoice_title IS NOT NULL;

-- name: CancelInvoice :one
UPDATE orders
SET status = 'cancelled',
    cancel_reason = $2,
    cancelled_by = $3,
    updated_at = now()
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: CreateOrderDeliverable :exec
INSERT INTO order_deliverables (order_id, filename, content_type, size_bytes, file_path, nonce)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetOrderDeliverable :one
SELECT id, order_id, filename, content_type, size_bytes, file_path, nonce
FROM order_deliverables
WHERE id = $1;

-- name: ListOrderDeliverables :many
SELECT id, filename, content_type, size_bytes, created_at
FROM order_deliverables
WHERE order_id = $1
ORDER BY created_at ASC;

-- name: SealOrderDelivery :exec
UPDATE orders
SET status = 'delivered', updated_at = now()
WHERE id = $1 AND status = 'held';

-- name: ReleaseOrder :one
UPDATE orders
SET status = 'completed', released_at = now(), released_by = $2, updated_at = now()
WHERE id = $1 AND status IN ('held', 'delivered', 'disputed')
RETURNING *;

-- name: DeleteOldDeliverables :execrows
DELETE FROM order_deliverables
WHERE created_at < now() - interval '24 hours';
