CREATE TABLE payment_transactions (
    id BIGSERIAL PRIMARY KEY,
    provider VARCHAR(20) NOT NULL,              
    provider_ref_id VARCHAR(100) NOT NULL,      
    user_id BIGINT NOT NULL,          
    type VARCHAR(20) NOT NULL,
    amount BIGINT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    wallet_transaction_id BIGINT NULL,
    raw_notification TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_payment_provider_ref UNIQUE (provider, provider_ref_id)
);