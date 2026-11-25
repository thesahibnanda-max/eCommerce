-- name: InsertPayment :exec
INSERT INTO payments (id, order_id, user_id, provider, state, amount, currency, purpose, metadata, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetPayment :one
SELECT id, order_id, user_id, provider, state, amount, currency, purpose, gateway_reference, gateway_payment_id, checkout_url,
       proof_transaction_id, proof_image_mime, proof_submitted_at, failure_reason, metadata, created_at, updated_at
FROM payments
WHERE id = ?;

-- name: GetOpenPaymentForOrder :one
SELECT id, order_id, user_id, provider, state, amount, currency, purpose, gateway_reference, gateway_payment_id, checkout_url,
       proof_transaction_id, proof_image_mime, proof_submitted_at, failure_reason, metadata, created_at, updated_at
FROM payments
WHERE order_id = ? AND state IN ('CREATED', 'AWAITING_PAYMENT', 'VERIFYING');

-- name: GetPaymentByReference :one
SELECT id, order_id, user_id, provider, state, amount, currency, purpose, gateway_reference, gateway_payment_id, checkout_url,
       proof_transaction_id, proof_image_mime, proof_submitted_at, failure_reason, metadata, created_at, updated_at
FROM payments
WHERE provider = ? AND gateway_reference = ?;

-- name: ListOrderPayments :many
SELECT id, order_id, user_id, provider, state, amount, currency, purpose, gateway_reference, gateway_payment_id, checkout_url,
       proof_transaction_id, proof_image_mime, proof_submitted_at, failure_reason, metadata, created_at, updated_at
FROM payments
WHERE order_id = ?
ORDER BY created_at DESC, id DESC;

-- name: UpdatePaymentState :execrows
UPDATE payments SET state = sqlc.arg('to_state'), updated_at = sqlc.arg('updated_at')
WHERE id = sqlc.arg('id') AND state = sqlc.arg('from_state');

-- name: UpdatePaymentFields :exec
UPDATE payments
SET gateway_reference = ?, gateway_payment_id = ?, checkout_url = ?, failure_reason = ?, metadata = ?, updated_at = ?
WHERE id = ?;

-- name: UpdatePaymentProof :exec
UPDATE payments
SET proof_transaction_id = ?, proof_image = ?, proof_image_mime = ?, proof_submitted_at = ?, updated_at = ?
WHERE id = ?;

-- name: GetPaymentProofImage :one
SELECT proof_image, proof_image_mime FROM payments WHERE id = ?;

-- name: ProofTransactionInUse :one
SELECT COUNT(*) FROM payments WHERE proof_transaction_id = ? AND id <> ?;

-- name: EventExists :one
SELECT COUNT(*) FROM payment_events WHERE payment_id = ? AND dedupe_key = ?;

-- name: InsertEvent :exec
INSERT INTO payment_events (payment_id, source, kind, from_state, to_state, data, dedupe_key, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListEvents :many
SELECT id, payment_id, source, kind, from_state, to_state, data, dedupe_key, created_at
FROM payment_events
WHERE payment_id = ?
ORDER BY id ASC;

-- name: ListPayments :many
SELECT id, order_id, user_id, provider, state, amount, currency, purpose, gateway_reference, gateway_payment_id, checkout_url,
       proof_transaction_id, proof_image_mime, proof_submitted_at, failure_reason, metadata, created_at, updated_at
FROM payments
WHERE (created_at, id) < (CAST(sqlc.arg('cursor_at') AS INTEGER), CAST(sqlc.arg('cursor_id') AS TEXT))
  AND (json_array_length(CAST(sqlc.arg('states') AS TEXT)) = 0 OR state IN (SELECT value FROM json_each(CAST(sqlc.arg('states') AS TEXT))))
  AND (CAST(sqlc.arg('provider') AS TEXT) = '' OR provider = CAST(sqlc.arg('provider') AS TEXT))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit');

-- name: CountPaymentsByState :many
SELECT state, COUNT(*) AS count FROM payments GROUP BY state;

-- name: ListPaymentsByOrderIDs :many
SELECT id, order_id, user_id, provider, state, amount, currency, purpose, gateway_reference, gateway_payment_id, checkout_url,
       proof_transaction_id, proof_image_mime, proof_submitted_at, failure_reason, metadata, created_at, updated_at
FROM payments
WHERE order_id IN (sqlc.slice('order_ids'))
ORDER BY created_at DESC, id DESC;

-- name: ListEventsByPaymentIDs :many
SELECT id, payment_id, source, kind, from_state, to_state, data, dedupe_key, created_at
FROM payment_events
WHERE payment_id IN (sqlc.slice('payment_ids'))
ORDER BY id ASC;
