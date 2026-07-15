package entity

import "time"

type TransactionType string

const (
    TxTypeTopup       TransactionType = "TOPUP"
    TxTypeTransferIn  TransactionType = "TRANSFER_IN"
    TxTypeTransferOut TransactionType = "TRANSFER_OUT"
)

type TransactionStatus string

const (
    TxStatusSuccess TransactionStatus = "SUCCESS"
    TxStatusFailed  TransactionStatus = "FAILED"
)

type Transaction struct {
    ID            uint              `gorm:"primaryKey" json:"id"`
    WalletID      uint              `gorm:"not null;index" json:"wallet_id"`
    Type          TransactionType   `gorm:"type:varchar(20);not null" json:"type"`
    Amount        int64             `gorm:"not null" json:"amount"` // selalu positif; arah ditentukan Type
    BalanceBefore int64             `gorm:"not null" json:"balance_before"`
    BalanceAfter  int64             `gorm:"not null" json:"balance_after"`
    TransferID    *uint             `gorm:"index" json:"transfer_id,omitempty"` // diisi hanya utk TRANSFER_IN/OUT
    Status        TransactionStatus `gorm:"type:varchar(20);not null;default:'SUCCESS'" json:"status"`
    CreatedAt     time.Time         `json:"created_at"`
}

func (Transaction) TableName() string { return "transactions" }