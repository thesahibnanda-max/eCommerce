-- name: InsertOrder :exec
INSERT INTO orders (
    id, user_id, idempotency_key, request_hash, state, currency,
    subtotal, shipping_fee, discount, total_payable,
    address_id, ship_full_name, ship_phone, ship_line1, ship_line2, ship_landmark,
    ship_city, ship_state, ship_state_code, ship_pincode, ship_country,
    payment_gateway, metadata, created_at, updated_at
) VALUES (
    ?, ?, ?, ?, ?, ?,
    ?, ?, ?, ?,
    ?, ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?,
    ?, ?, ?, ?
);

-- name: InsertOrderItem :exec
INSERT INTO order_items (
    order_id, position, product_id, variant_id, sku, product_name, attributes, unit_price, quantity, line_total
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: InsertStateChange :exec
INSERT INTO order_state_history (order_id, from_state, to_state, reason, actor, created_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: InsertPaymentEvent :exec
INSERT INTO order_payment_events (
    order_id, gateway, gateway_event_id, event_type, gateway_order_id, gateway_payment_id, status, amount, raw, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetOrder :one
SELECT * FROM orders WHERE id = ?;

-- name: GetOrderByIdempotencyKey :one
SELECT * FROM orders WHERE user_id = ? AND idempotency_key = ?;

-- name: PaymentEventExists :one
SELECT COUNT(*) FROM order_payment_events WHERE gateway = ? AND gateway_event_id = ?;

-- name: ListItemsByOrderIDs :many
SELECT * FROM order_items WHERE order_id IN (sqlc.slice('order_ids')) ORDER BY order_id, position;

-- name: ListStateHistory :many
SELECT * FROM order_state_history WHERE order_id = ? ORDER BY id;

-- name: ListPaymentEvents :many
SELECT * FROM order_payment_events WHERE order_id = ? ORDER BY id;

-- name: ListUserOrders :many
SELECT * FROM orders
WHERE user_id = sqlc.arg('user_id')
  AND (created_at, id) < (CAST(sqlc.arg('cursor_at') AS INTEGER), CAST(sqlc.arg('cursor_id') AS TEXT))
  AND (json_array_length(CAST(sqlc.arg('states') AS TEXT)) = 0 OR state IN (SELECT value FROM json_each(CAST(sqlc.arg('states') AS TEXT))))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit');

-- name: ListOrders :many
SELECT * FROM orders
WHERE (created_at, id) < (CAST(sqlc.arg('cursor_at') AS INTEGER), CAST(sqlc.arg('cursor_id') AS TEXT))
  AND (json_array_length(CAST(sqlc.arg('states') AS TEXT)) = 0 OR state IN (SELECT value FROM json_each(CAST(sqlc.arg('states') AS TEXT))))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit');

-- name: UpdateOrderState :execrows
UPDATE orders SET state = sqlc.arg('to_state'), updated_at = sqlc.arg('updated_at')
WHERE id = sqlc.arg('id') AND state = sqlc.arg('from_state');

-- name: UpdatePaymentGateway :execrows
UPDATE orders SET payment_gateway = ?, updated_at = ? WHERE id = ?;

-- name: UpdateMetadata :execrows
UPDATE orders SET metadata = ?, updated_at = ? WHERE id = ?;

-- name: CountOrdersByState :many
SELECT state, COUNT(*) AS total FROM orders GROUP BY state;
