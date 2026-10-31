-- name: CreateProduct :one
INSERT INTO products (seller_id, title, description, price, currency_code, category_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, seller_id, title, description, price, currency_code, is_active, category_id, created_at;

-- name: GetProduct :one
SELECT p.id, p.seller_id, p.title, p.description, p.price, p.currency_code,
       p.is_active, p.category_id, p.created_at,
       u.username AS seller_username
FROM products p
JOIN users u ON u.id = p.seller_id
WHERE p.id = $1 AND p.is_active = TRUE;

-- name: ListProducts :many
SELECT id, seller_id, title, description, price, currency_code, is_active, category_id, created_at
FROM products
WHERE is_active = TRUE
  AND (sqlc.narg('category_id')::uuid IS NULL OR category_id = sqlc.narg('category_id')::uuid)
ORDER BY created_at DESC, id DESC
LIMIT $1 OFFSET $2;

-- name: ListProductsBySeller :many
SELECT id, seller_id, title, description, price, currency_code, is_active, category_id, created_at
FROM products
WHERE seller_id = $1 AND is_active = TRUE
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: DeactivateProduct :execrows
UPDATE products SET is_active = FALSE, updated_at = now()
WHERE id = $1 AND seller_id = $2 AND is_active = TRUE;

-- name: SetProductCategory :execrows
UPDATE products SET category_id = $2, updated_at = now()
WHERE id = $1 AND seller_id = $3 AND is_active = TRUE;

-- name: CountActiveProducts :one
SELECT count(*) FROM products WHERE is_active = TRUE;

-- name: UpdateProduct :one
UPDATE products
SET title = $3,
    description = $4,
    price = $5,
    category_id = $6,
    updated_at = now()
WHERE id = $1 AND seller_id = $2 AND is_active = TRUE
RETURNING id, seller_id, title, description, price, currency_code, is_active, category_id, created_at;

-- name: AddProductImage :exec
INSERT INTO product_images (product_id, url, sort_order)
VALUES ($1, $2, $3);

-- name: ListProductImages :many
SELECT id, product_id, url, sort_order
FROM product_images
WHERE product_id = $1
ORDER BY sort_order ASC, created_at ASC;

-- name: DeleteProductImages :exec
DELETE FROM product_images WHERE product_id = $1;
