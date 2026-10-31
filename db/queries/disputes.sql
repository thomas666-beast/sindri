-- name: CreateDispute :one
INSERT INTO disputes (order_id, opened_by, reason)
VALUES ($1, $2, $3)
RETURNING id, order_id, opened_by, reason, resolution, resolved_by, resolved_at, created_at;

-- name: GetDisputeByOrder :one
SELECT id, order_id, opened_by, reason, resolution, resolved_by, resolved_at, created_at
FROM disputes WHERE order_id = $1;

-- name: ListOpenDisputes :many
SELECT d.id, d.order_id, d.opened_by, d.reason, d.resolution, d.resolved_by,
       d.resolved_at, d.created_at,
       o.amount, o.currency_code, o.buyer_id, o.seller_id
FROM disputes d
JOIN orders o ON o.id = d.order_id
WHERE d.resolution IS NULL
ORDER BY d.created_at ASC
LIMIT $1 OFFSET $2;

-- name: ResolveDispute :one
UPDATE disputes
SET resolution = $2, resolved_by = $3, resolved_at = now()
WHERE order_id = $1 AND resolution IS NULL
RETURNING id, order_id, opened_by, reason, resolution, resolved_by, resolved_at, created_at;
