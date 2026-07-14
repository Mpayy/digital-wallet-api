CREATE TABLE transfers (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    from_wallet_id BIGINT UNSIGNED NOT NULL,
    to_wallet_id BIGINT UNSIGNED NOT NULL,
    amount BIGINT NOT NULL,
    note VARCHAR(255),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_transfers_from_wallet FOREIGN KEY (from_wallet_id) REFERENCES wallets(id),
    CONSTRAINT fk_transfers_to_wallet FOREIGN KEY (to_wallet_id) REFERENCES wallets(id),
    INDEX idx_transfers_from (from_wallet_id),
    INDEX idx_transfers_to (to_wallet_id)
) ENGINE=InnoDB;