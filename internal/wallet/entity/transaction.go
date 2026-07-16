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
	ID            uint              `gorm:"column:id;primaryKey" json:"id"`
	WalletID      uint              `gorm:"column:wallet_id;not null;index" json:"wallet_id"`
	Type          TransactionType   `gorm:"column:type;type:varchar(20);not null" json:"type"`
	Amount        int64             `gorm:"column:amount;not null" json:"amount"`
	BalanceBefore int64             `gorm:"column:balance_before;not null" json:"balance_before"`
	BalanceAfter  int64             `gorm:"column:balance_after;not null" json:"balance_after"`
	TransferID    *uint             `gorm:"column:transfer_id;index" json:"transfer_id,omitempty"`
	Status        TransactionStatus `gorm:"column:status;type:varchar(20);not null;default:'SUCCESS'" json:"status"`
	CreatedAt     time.Time         `gorm:"column:created_at" json:"created_at"`
}

func (Transaction) TableName() string { return "transactions" }
