CREATE TABLE IF NOT EXISTS orders (
    id               TEXT    NOT NULL PRIMARY KEY CHECK (length(id) = 63),
    user_id          TEXT    NOT NULL CHECK (length(user_id) = 26),
    idempotency_key  TEXT    NOT NULL CHECK (length(idempotency_key) BETWEEN 1 AND 128),
    request_hash     TEXT    NOT NULL,
    state            TEXT    NOT NULL CHECK (state IN ('CREATED', 'DITCHED', 'PROCESSING', 'APPROVED', 'PACKED', 'STARTED_SHIPPING', 'OUT_FOR_DELIVERY', 'DELIVERED', 'CANCELLED', 'RETURNED', 'REFUNDED', 'FAILED', 'ON_HOLD')),
    currency         TEXT    NOT NULL CHECK (currency = 'INR'),
    subtotal         INTEGER NOT NULL CHECK (subtotal >= 0),
    shipping_fee     INTEGER NOT NULL CHECK (shipping_fee >= 0),
    discount         INTEGER NOT NULL CHECK (discount >= 0),
    total_payable    INTEGER NOT NULL CHECK (total_payable >= 0 AND total_payable = subtotal + shipping_fee - discount),
    address_id       TEXT,
    ship_full_name   TEXT    NOT NULL CHECK (length(ship_full_name) > 0),
    ship_phone       TEXT    NOT NULL CHECK (length(ship_phone) > 0),
    ship_line1       TEXT    NOT NULL CHECK (length(ship_line1) > 0),
    ship_line2       TEXT    NOT NULL,
    ship_landmark    TEXT    NOT NULL,
    ship_city        TEXT    NOT NULL CHECK (length(ship_city) > 0),
    ship_state       TEXT    NOT NULL CHECK (length(ship_state) > 0),
    ship_state_code  TEXT    NOT NULL CHECK (length(ship_state_code) > 0),
    ship_pincode     TEXT    NOT NULL CHECK (length(ship_pincode) = 6),
    ship_country     TEXT    NOT NULL CHECK (length(ship_country) > 0),
    payment_gateway  TEXT,
    metadata         TEXT    NOT NULL,
    created_at       INTEGER NOT NULL,
    updated_at       INTEGER NOT NULL,
    UNIQUE (user_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS orders_user_created_idx ON orders (user_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS orders_state_created_idx ON orders (state, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS orders_created_idx ON orders (created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS order_items (
    id            INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    order_id      TEXT    NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    position      INTEGER NOT NULL CHECK (position >= 0),
    product_id    TEXT    NOT NULL,
    variant_id    TEXT    NOT NULL,
    sku           TEXT    NOT NULL CHECK (length(sku) > 0),
    product_name  TEXT    NOT NULL CHECK (length(product_name) > 0),
    attributes    TEXT    NOT NULL,
    unit_price    INTEGER NOT NULL CHECK (unit_price > 0),
    quantity      INTEGER NOT NULL CHECK (quantity > 0),
    line_total    INTEGER NOT NULL CHECK (line_total = unit_price * quantity),
    UNIQUE (order_id, variant_id),
    UNIQUE (order_id, position)
);

CREATE TABLE IF NOT EXISTS order_state_history (
    id          INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    order_id    TEXT    NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    from_state  TEXT,
    to_state    TEXT    NOT NULL,
    reason      TEXT    NOT NULL,
    actor       TEXT    NOT NULL CHECK (actor IN ('USER', 'ADMIN', 'SYSTEM', 'GATEWAY')),
    created_at  INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS order_state_history_order_idx ON order_state_history (order_id, id);

CREATE TABLE IF NOT EXISTS order_payment_events (
    id                  INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    order_id            TEXT    NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    gateway             TEXT    NOT NULL CHECK (length(gateway) > 0),
    gateway_event_id    TEXT    NOT NULL CHECK (length(gateway_event_id) > 0),
    event_type          TEXT    NOT NULL CHECK (length(event_type) > 0),
    gateway_order_id    TEXT    NOT NULL,
    gateway_payment_id  TEXT    NOT NULL,
    status              TEXT    NOT NULL,
    amount              INTEGER NOT NULL CHECK (amount >= 0),
    raw                 TEXT    NOT NULL,
    created_at          INTEGER NOT NULL,
    UNIQUE (gateway, gateway_event_id)
);

CREATE INDEX IF NOT EXISTS order_payment_events_order_idx ON order_payment_events (order_id, id);
