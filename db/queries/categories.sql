-- name: CreateCategory :one
INSERT INTO categories (slug, name, description, sort_order, parent_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, slug, name, description, sort_order, is_active, parent_id, created_at, updated_at;

-- name: GetCategoryByID :one
SELECT id, slug, name, description, sort_order, is_active, parent_id, created_at, updated_at
FROM categories WHERE id = $1;

-- name: GetCategoryBySlug :one
SELECT id, slug, name, description, sort_order, is_active, parent_id, created_at, updated_at
FROM categories WHERE slug = $1;

-- name: ListCategories :many
SELECT id, slug, name, description, sort_order, is_active, parent_id, created_at, updated_at
FROM categories
WHERE (sqlc.arg('only_active')::boolean = FALSE OR is_active = TRUE)
  AND (sqlc.arg('parent_filter')::text = ''
       OR (sqlc.arg('parent_filter')::text = 'top'     AND parent_id IS NULL)
       OR (sqlc.arg('parent_filter')::text = 'children' AND parent_id IS NOT NULL))
ORDER BY sort_order ASC, name ASC
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');

-- name: ListChildren :many
SELECT id, slug, name, description, sort_order, is_active, parent_id, created_at, updated_at
FROM categories
WHERE parent_id = $1
  AND (sqlc.arg('only_active')::boolean = FALSE OR is_active = TRUE)
ORDER BY sort_order ASC, name ASC
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');

-- name: CountChildren :one
SELECT count(*) FROM categories WHERE parent_id = $1;

-- name: UpdateCategory :one
UPDATE categories
SET slug = $2, name = $3, description = $4, sort_order = $5, is_active = $6, updated_at = now()
WHERE id = $1
RETURNING id, slug, name, description, sort_order, is_active, parent_id, created_at, updated_at;

-- name: DeleteCategory :execrows
DELETE FROM categories WHERE id = $1;
