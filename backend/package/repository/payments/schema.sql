CREATE TABLE IF NOT EXISTS payments (
    id                   TEXT    NOT NULL PRIMARY KEY CHECK (length(id) = 26),
    order_id             TEXT    NOT NULL CHECK (length(order_id) BETWEEN 1 AND 128),
    user_id              TEXT    NOT NULL CHECK (length(user_id) BETWEEN 1 AND 128),
    provider             TEXT    NOT NULL CHECK (provider IN ('INSTAMOJO', 'SELF_UPI')),
    state                TEXT    NOT NULL CHECK (state IN ('CREATED', 'AWAITING_PAYMENT', 'VERIFYING', 'SUCCEEDED', 'FAILED', 'EXPIRED')),
    amount               INTEGER NOT NULL CHECK (amount > 0),
    currency             TEXT    NOT NULL CHECK (currency = 'INR'),
    purpose              TEXT    NOT NULL CHECK (length(purpose) <= 100),
    gateway_reference    TEXT    CHECK (gateway_reference IS NULL OR length(gateway_reference) BETWEEN 1 AND 128),
    gateway_payment_id   TEXT    CHECK (gateway_payment_id IS NULL OR length(gateway_payment_id) BETWEEN 1 AND 128),
    checkout_url         TEXT    CHECK (checkout_url IS NULL OR length(checkout_url) BETWEEN 1 AND 2048),
    proof_transaction_id TEXT    CHECK (proof_transaction_id IS NULL OR length(proof_transaction_id) BETWEEN 6 AND 35),
    proof_image          BLOB,
    proof_image_mime     TEXT,
    proof_submitted_at   INTEGER,
    failure_reason       TEXT    NOT NULL DEFAULT '' CHECK (length(failure_reason) <= 500),
    metadata             TEXT    NOT NULL DEFAULT '{}',
    created_at           INTEGER NOT NULL,
    updated_at           INTEGER NOT NULL,
    CHECK ((proof_image IS NULL) = (proof_image_mime IS NULL)),
    CHECK ((proof_image IS NULL) = (proof_transaction_id IS NULL)),
    CHECK ((proof_image IS NULL) = (proof_submitted_at IS NULL))
);

CREATE UNIQUE INDEX IF NOT EXISTS payments_provider_reference_idx ON payments (provider, gateway_reference) WHERE gateway_reference IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS payments_open_order_idx ON payments (order_id) WHERE state IN ('CREATED', 'AWAITING_PAYMENT', 'VERIFYING');
CREATE UNIQUE INDEX IF NOT EXISTS payments_proof_transaction_idx ON payments (proof_transaction_id) WHERE proof_transaction_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS payments_created_idx ON payments (created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS payments_order_created_idx ON payments (order_id, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS payment_events (
    id          INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    payment_id  TEXT    NOT NULL REFERENCES payments (id) ON DELETE CASCADE,
    source      TEXT    NOT NULL CHECK (source IN ('USER', 'ADMIN', 'WEBHOOK', 'REDIRECT', 'POLL', 'SYSTEM')),
    kind        TEXT    NOT NULL CHECK (length(kind) BETWEEN 1 AND 64),
    from_state  TEXT    CHECK (from_state IS NULL OR from_state IN ('CREATED', 'AWAITING_PAYMENT', 'VERIFYING', 'SUCCEEDED', 'FAILED', 'EXPIRED')),
    to_state    TEXT    CHECK (to_state IS NULL OR to_state IN ('CREATED', 'AWAITING_PAYMENT', 'VERIFYING', 'SUCCEEDED', 'FAILED', 'EXPIRED')),
    data        TEXT    NOT NULL DEFAULT '{}',
    dedupe_key  TEXT    CHECK (dedupe_key IS NULL OR length(dedupe_key) BETWEEN 1 AND 256),
    created_at  INTEGER NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS payment_events_dedupe_idx ON payment_events (payment_id, dedupe_key) WHERE dedupe_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS payment_events_payment_idx ON payment_events (payment_id, id);
