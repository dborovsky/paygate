-- +goose Up
CREATE TABLE payments (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id      BIGINT      NOT NULL REFERENCES merchants(id),
    idempotency_key  TEXT        NOT NULL,
    request_hash     BYTEA       NOT NULL,  -- sha256 of the request body, detects key reuse
    amount           BIGINT      NOT NULL CHECK (amount > 0),  -- minor units: 1050 = 10.50 USD
    currency         TEXT        NOT NULL CHECK (char_length(currency) = 3),
    status           TEXT        NOT NULL DEFAULT 'pending',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- The heart of idempotency: the database itself guarantees one payment per key.
    UNIQUE (merchant_id, idempotency_key)
);

-- +goose Down
DROP TABLE payments;
