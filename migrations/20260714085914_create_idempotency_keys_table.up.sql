CREATE TABLE idempotency_keys (
    id BIGSERIAL PRIMARY KEY,
    idem_key VARCHAR(100) NOT NULL UNIQUE,
    user_id BIGINT NOT NULL,
    endpoint VARCHAR(50) NOT NULL,
    request_hash CHAR(64) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PROCESSING',
    response_status INT NULL,
    response_body TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);