-- name: StatsDashboard :one
WITH
  user_stats AS (
    SELECT
      count(*) FILTER (WHERE is_active = TRUE)::bigint AS active_users,
      count(*) FILTER (WHERE is_active = FALSE)::bigint AS disabled_users
    FROM users
    WHERE is_system = FALSE
  ),
  order_stats AS (
    SELECT
      count(*) FILTER (WHERE status = 'held')::bigint AS orders_held,
      count(*) FILTER (WHERE status = 'disputed')::bigint AS orders_disputed
    FROM orders
  ),
  withdrawal_stats AS (
    SELECT count(*)::bigint AS pending_withdrawals
    FROM withdrawal_requests
    WHERE status = 'pending'
  ),
  ticket_stats AS (
    SELECT count(*)::bigint AS open_tickets
    FROM tickets
    WHERE status IN ('open', 'answered')
  ),
  commission_stats AS (
    SELECT
      COALESCE(jsonb_object_agg(currency_code, total), '{}'::jsonb) AS commission_by_currency
    FROM (
      SELECT currency_code, sum(amount)::bigint AS total
      FROM wallet_transactions
      WHERE type = 'commission' AND status = 'completed'
      GROUP BY currency_code
    ) c
  )
SELECT
  u.active_users,
  u.disabled_users,
  o.orders_held,
  o.orders_disputed,
  w.pending_withdrawals,
  t.open_tickets,
  c.commission_by_currency
FROM user_stats u
CROSS JOIN order_stats o
CROSS JOIN withdrawal_stats w
CROSS JOIN ticket_stats t
CROSS JOIN commission_stats c;

-- name: GetSellerDashboardStats :one
SELECT
    -- 1. Total earnings from completed orders (seller share only)
    COALESCE((
        SELECT SUM(amount - commission_amount)
        FROM orders
        WHERE seller_id = sqlc.arg('seller_id')::uuid AND status = 'completed'
    ), 0)::bigint AS total_earned,
    
    -- 2. Active orders that need action
    (SELECT count(*) FROM orders WHERE seller_id = sqlc.arg('seller_id')::uuid AND status = 'held')::bigint AS active_orders,
    
    -- 3. Pending withdrawals
    (SELECT count(*) FROM withdrawal_requests WHERE user_id = sqlc.arg('seller_id')::uuid AND status = 'pending')::bigint AS pending_withdrawals,
    
    -- 4. Active listings
    (SELECT count(*) FROM products WHERE seller_id = sqlc.arg('seller_id')::uuid AND is_active = TRUE)::bigint AS active_products;
