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
	ID            uint              `gorm:"column:id;primaryKey"`
	WalletID      uint              `gorm:"column:wallet_id;not null;index"`
	Type          TransactionType   `gorm:"column:type;type:varchar(20);not null"`
	Amount        int64             `gorm:"column:amount;not null"`// selalu positif; arah ditentukan Type
	BalanceBefore int64             `gorm:"column:balance_before;not null"`
	BalanceAfter  int64             `gorm:"column:balance_after;not null"`
	TransferID    *uint             `gorm:"column:transfer_id;index"`// diisi hanya utk TRANSFER_IN/OUT
	Status        TransactionStatus `gorm:"column:status;type:varchar(20);not null;default:'SUCCESS'"`
	CreatedAt     time.Time         `gorm:"column:created_at"`
}

func (Transaction) TableName() string { return "transactions" }
