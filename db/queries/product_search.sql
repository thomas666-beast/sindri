-- name: SearchProducts :many
SELECT
    p.id, p.seller_id, p.title, p.description, p.price, p.currency_code,
    p.category_id, p.created_at,
    u.username AS seller_username,
    COALESCE(c.slug, '') AS category_slug,
    COALESCE(c.name, '') AS category_name,
    COALESCE(pi.url, '') AS cover_image
FROM products p
JOIN users u ON u.id = p.seller_id
LEFT JOIN categories c ON c.id = p.category_id
LEFT JOIN product_images pi ON pi.product_id = p.id
WHERE p.is_active = TRUE
  AND (sqlc.arg('q')::text = ''
       OR p.title ILIKE '%' || sqlc.arg('q')::text || '%'
       OR p.description ILIKE '%' || sqlc.arg('q')::text || '%')
  AND (
      sqlc.narg('category_id')::uuid IS NULL
      OR p.category_id = sqlc.narg('category_id')::uuid
      OR p.category_id IN (
          SELECT id FROM categories WHERE parent_id = sqlc.narg('category_id')::uuid
      )
  )
  AND (sqlc.narg('seller_id')::uuid IS NULL OR p.seller_id = sqlc.narg('seller_id')::uuid)
  AND (
      sqlc.arg('id_filter')::text = ''
      OR (length(sqlc.arg('id_filter')::text) = 36 AND p.id = sqlc.arg('id_filter')::uuid)
      OR (length(sqlc.arg('id_filter')::text) != 36 AND p.id::text ILIKE '%' || sqlc.arg('id_filter')::text || '%')
  )
GROUP BY p.id, u.username, c.slug, c.name, pi.url
ORDER BY p.created_at DESC, p.id DESC
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');

-- name: CountSearchProducts :one
SELECT count(*)
FROM products p
WHERE p.is_active = TRUE
  AND (sqlc.arg('q')::text = ''
       OR p.title ILIKE '%' || sqlc.arg('q')::text || '%'
       OR p.description ILIKE '%' || sqlc.arg('q')::text || '%')
  AND (
      sqlc.narg('category_id')::uuid IS NULL
      OR p.category_id = sqlc.narg('category_id')::uuid
      OR p.category_id IN (
          SELECT id FROM categories WHERE parent_id = sqlc.narg('category_id')::uuid
      )
  )
  AND (sqlc.narg('seller_id')::uuid IS NULL OR p.seller_id = sqlc.narg('seller_id')::uuid)
  AND (
      sqlc.arg('id_filter')::text = ''
      OR (length(sqlc.arg('id_filter')::text) = 36 AND p.id = sqlc.arg('id_filter')::uuid)
      OR (length(sqlc.arg('id_filter')::text) != 36 AND p.id::text ILIKE '%' || sqlc.arg('id_filter')::text || '%')
  );
  