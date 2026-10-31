-- name: SearchUsers :many
SELECT
    u.id,
    u.username,
    u.preferred_currency,
    u.created_at AS user_created_at,
    COALESCE(p.display_name, '') AS display_name,
    COALESCE(p.bio, '') AS bio,
    COALESCE(p.skills, '{}'::text[]) AS skills,
    COALESCE(p.interests, '{}'::text[]) AS interests,
    p.avatar_url
FROM users u
LEFT JOIN user_profiles p ON p.user_id = u.id
WHERE u.is_active = TRUE
  AND u.is_system = FALSE
  AND (
      sqlc.arg('q')::text = ''
      OR u.username ILIKE '%' || sqlc.arg('q')::text || '%'
      OR COALESCE(p.bio, '') ILIKE '%' || sqlc.arg('q')::text || '%'
      OR EXISTS (
          SELECT 1 FROM unnest(COALESCE(p.skills, '{}'::text[])) s
          WHERE s ILIKE '%' || sqlc.arg('q')::text || '%'
      )
      OR EXISTS (
          SELECT 1 FROM unnest(COALESCE(p.interests, '{}'::text[])) i
          WHERE i ILIKE '%' || sqlc.arg('q')::text || '%'
      )
  )
ORDER BY u.username ASC
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');

-- name: CountSearchUsers :one
SELECT count(*)
FROM users u
LEFT JOIN user_profiles p ON p.user_id = u.id
WHERE u.is_active = TRUE
  AND u.is_system = FALSE
  AND (
      sqlc.arg('q')::text = ''
      OR u.username ILIKE '%' || sqlc.arg('q')::text || '%'
      OR COALESCE(p.bio, '') ILIKE '%' || sqlc.arg('q')::text || '%'
      OR EXISTS (
          SELECT 1 FROM unnest(COALESCE(p.skills, '{}'::text[])) s
          WHERE s ILIKE '%' || sqlc.arg('q')::text || '%'
      )
      OR EXISTS (
          SELECT 1 FROM unnest(COALESCE(p.interests, '{}'::text[])) i
          WHERE i ILIKE '%' || sqlc.arg('q')::text || '%'
      )
  );
