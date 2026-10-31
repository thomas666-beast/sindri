-- name: GetProfile :one
SELECT p.user_id, p.display_name, p.bio, p.skills, p.interests, p.avatar_url, p.updated_at,
       u.username, u.created_at AS user_created_at
FROM user_profiles p
JOIN users u ON u.id = p.user_id
WHERE u.username = $1 AND u.is_active = TRUE AND u.is_system = FALSE;

-- name: GetProfileByUserID :one
SELECT p.user_id, p.display_name, p.bio, p.skills, p.interests, p.avatar_url, p.updated_at,
       u.username, u.created_at AS user_created_at
FROM user_profiles p
JOIN users u ON u.id = p.user_id
WHERE p.user_id = $1;

-- name: UpsertProfile :one
INSERT INTO user_profiles (user_id, display_name, bio, skills, interests, avatar_url)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (user_id) DO UPDATE
SET display_name = EXCLUDED.display_name,
    bio = EXCLUDED.bio,
    skills = EXCLUDED.skills,
    interests = EXCLUDED.interests,
    avatar_url = EXCLUDED.avatar_url,
    updated_at = now()
RETURNING user_id, display_name, bio, skills, interests, avatar_url, updated_at;

-- name: GetUserBasicByUsername :one
SELECT id, username, created_at
FROM users
WHERE username = $1 AND is_active = TRUE AND is_system = FALSE;
