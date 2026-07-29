CREATE TABLE payment_transactions (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    provider VARCHAR(20) NOT NULL,              -- "MIDTRANS"
    provider_ref_id VARCHAR(100) NOT NULL,      -- order_id yang kamu generate & kirim ke Midtrans
    user_id BIGINT UNSIGNED NOT NULL,           -- no FK, sama kayak wallets.user_id
    amount BIGINT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING', -- PENDING | SETTLED | FAILED | EXPIRED
    wallet_transaction_id BIGINT UNSIGNED NULL,    -- diisi SETELAH TopUp sukses, no FK
    raw_notification TEXT NULL,                    -- simpan payload webhook mentah, buat debug
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_payment_provider_ref (provider, provider_ref_id)
) ENGINE=InnoDB;