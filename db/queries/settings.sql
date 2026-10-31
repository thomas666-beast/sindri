-- name: GetSetting :one
SELECT value FROM platform_settings WHERE key = $1;

-- name: SetSetting :exec
INSERT INTO platform_settings (key, value)
VALUES ($1, $2)
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();
