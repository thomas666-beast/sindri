-- name: AdminListUsers :many
SELECT id, email, username, preferred_currency, is_admin, is_active,
       kyc_status, kyc_override, disabled_at, disable_reason, created_at
FROM users
WHERE is_system = FALSE
  AND (sqlc.arg('search')::text = ''
       OR email    ILIKE '%' || sqlc.arg('search')::text || '%'
       OR username ILIKE '%' || sqlc.arg('search')::text || '%')
  AND (sqlc.arg('status_filter')::text = ''
       OR (sqlc.arg('status_filter')::text = 'active'   AND is_active = TRUE)
       OR (sqlc.arg('status_filter')::text = 'disabled' AND is_active = FALSE))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');

-- name: AdminCountUsers :one
SELECT count(*) FROM users WHERE is_system = FALSE;

-- name: AdminGetUser :one
SELECT id, email, username, preferred_currency, is_admin, is_active,
       kyc_status, kyc_override, disabled_at, disable_reason, created_at
FROM users WHERE id = $1 AND is_system = FALSE;

-- name: AdminDisableUser :one
UPDATE users
SET is_active = FALSE, disabled_at = now(), disable_reason = $2, updated_at = now()
WHERE id = $1 AND is_system = FALSE
RETURNING id, email, username, is_active, disabled_at, disable_reason;

-- name: AdminEnableUser :one
UPDATE users
SET is_active = TRUE, disabled_at = NULL, disable_reason = NULL, updated_at = now()
WHERE id = $1 AND is_system = FALSE
RETURNING id, email, username, is_active, disabled_at, disable_reason;

-- name: AdminSetKYC :one
UPDATE users
SET kyc_status = $2, kyc_override = TRUE, updated_at = now()
WHERE id = $1 AND is_system = FALSE
RETURNING id, email, username, kyc_status, kyc_override;

-- name: AdminSetAdmin :one
UPDATE users
SET is_admin = $2, updated_at = now()
WHERE id = $1 AND is_system = FALSE
RETURNING id, email, username, is_admin;

-- name: AdminCountUsersFiltered :one
SELECT COUNT(*)::bigint
FROM users
WHERE is_system = FALSE
  AND (sqlc.arg('search')::text = ''
       OR email    ILIKE '%' || sqlc.arg('search')::text || '%'
       OR username ILIKE '%' || sqlc.arg('search')::text || '%')
  AND (sqlc.arg('status_filter')::text = ''
       OR (sqlc.arg('status_filter')::text = 'active'   AND is_active = TRUE)
       OR (sqlc.arg('status_filter')::text = 'disabled' AND is_active = FALSE));
