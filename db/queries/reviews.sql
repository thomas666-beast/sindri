-- name: CreateReview :one
INSERT INTO reviews (order_id, buyer_id, seller_id, product_id, rating, comment)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, order_id, buyer_id, seller_id, product_id, rating, comment, created_at;

-- name: GetReviewByOrder :one
SELECT id, order_id, buyer_id, seller_id, product_id, rating, comment, created_at
FROM reviews WHERE order_id = $1;

-- name: ListReviewsBySeller :many
SELECT r.id, r.order_id, r.buyer_id, r.seller_id, r.product_id, r.rating, r.comment, r.created_at,
       u.username AS buyer_username
FROM reviews r
JOIN users u ON u.id = r.buyer_id
WHERE r.seller_id = $1
ORDER BY r.created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListReviewsByProduct :many
SELECT r.id, r.order_id, r.buyer_id, r.seller_id, r.product_id, r.rating, r.comment, r.created_at,
       u.username AS buyer_username
FROM reviews r
JOIN users u ON u.id = r.buyer_id
WHERE r.product_id = $1
ORDER BY r.created_at DESC
LIMIT $2 OFFSET $3;

-- name: SellerRatingSummary :one
SELECT
    COALESCE(SUM(rating), 0)::bigint AS rating_sum,
    COUNT(*)::bigint AS review_count
FROM reviews
WHERE seller_id = $1;

-- name: ProductRatingSummary :one
SELECT
    COALESCE(SUM(rating), 0)::bigint AS rating_sum,
    COUNT(*)::bigint AS review_count
FROM reviews
WHERE product_id = $1;

