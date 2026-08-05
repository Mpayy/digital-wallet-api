CREATE TABLE transfers (
    id BIGSERIAL PRIMARY KEY,
    from_wallet_id BIGINT NOT NULL REFERENCES wallets(id),
    to_wallet_id BIGINT NOT NULL REFERENCES wallets(id),
    amount BIGINT NOT NULL,
    note VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_transfers_from ON transfers(from_wallet_id);
CREATE INDEX IF NOT EXISTS idx_transfers_to ON transfers(to_wallet_id);