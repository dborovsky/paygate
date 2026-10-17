-- +goose Up
CREATE TABLE merchants (
    id          BIGSERIAL PRIMARY KEY,
    name        TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE api_keys (
    id           BIGSERIAL PRIMARY KEY,
    merchant_id  BIGINT      NOT NULL REFERENCES merchants(id),
    key_prefix   TEXT        NOT NULL,         -- e.g. "sk_live_8fK2": lets a human recognize the key
    key_hash     BYTEA       NOT NULL UNIQUE,  -- sha256 of the full key (the key itself is never stored)
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at   TIMESTAMPTZ                   -- NULL = key is active
);

-- +goose Down
DROP TABLE api_keys;
DROP TABLE merchants;
