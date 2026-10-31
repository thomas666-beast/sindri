-- name: CreateUser :one
INSERT INTO users (email, password_hash, username, preferred_currency)
VALUES ($1, $2, $3, $4)
RETURNING id, email, username, preferred_currency, is_admin, is_active,
          kyc_status, created_at;

-- name: GetUserByEmail :one
SELECT id, email, password_hash, username, preferred_currency,
       is_admin, is_active, kyc_status, created_at
FROM users
WHERE email = $1;

-- name: GetUserByID :one
SELECT id, email, password_hash, username, preferred_currency,
       is_admin, is_active, kyc_status, created_at
FROM users
WHERE id = $1;

-- name: UpdateLastLogin :exec
UPDATE users SET updated_at = now() WHERE id = $1;

-- name: GetUserIDByUsername :one
SELECT id FROM users
WHERE username = $1 AND is_active = TRUE AND is_system = FALSE;

-- name: UpdatePasswordHash :exec
UPDATE users SET password_hash = $2, updated_at = now()
WHERE id = $1;

-- name: GetGeekStats :one
-- name: GetGeekStats :one
SELECT
    (SELECT count(*) FROM orders WHERE seller_id = u.id AND status = 'completed')::bigint AS completed_orders,
    (SELECT count(*) FROM products WHERE seller_id = u.id AND is_active = TRUE)::bigint AS active_products,
    (SELECT COALESCE(AVG(rating), 0) FROM reviews WHERE seller_id = u.id)::float8 AS avg_rating,
    (SELECT count(*) FROM reviews WHERE seller_id = u.id)::bigint AS review_count
FROM users u
WHERE u.id = $1;


-- name: IsSystemUser :one
SELECT COALESCE(is_system, FALSE) FROM users WHERE id = $1;
