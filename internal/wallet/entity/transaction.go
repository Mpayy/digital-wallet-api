package entity

import "time"

type TransactionType string

const (
	TxTypeTopup            TransactionType = "TOPUP"
	TxTypeTransferIn       TransactionType = "TRANSFER_IN"
	TxTypeTransferOut      TransactionType = "TRANSFER_OUT"
	TxTypeWithdrawal       TransactionType = "WITHDRAWAL"
	TxTypeWithdrawalRevert TransactionType = "WITHDRAWAL_REVERT"
)

type TransactionStatus string

const (
	TxStatusSuccess TransactionStatus = "SUCCESS"
	TxStatusFailed  TransactionStatus = "FAILED"
	TxStatusPending TransactionStatus = "PENDING"
)

type Transaction struct {
	ID            uint              `gorm:"column:id;primaryKey;autoIncrement"`
	WalletID      uint              `gorm:"column:wallet_id;not null;index:idx_transactions_wallet_created"`
	Type          TransactionType   `gorm:"column:type;type:varchar(20);not null"`
	Amount        int64             `gorm:"column:amount;not null"`
	BalanceBefore int64             `gorm:"column:balance_before;not null"`
	BalanceAfter  int64             `gorm:"column:balance_after;not null"`
	TransferID    *uint             `gorm:"column:transfer_id;index"`
	Status        TransactionStatus `gorm:"column:status;type:varchar(20);not null;default:'PENDING'"`
	CreatedAt     time.Time         `gorm:"column:created_at;autoCreateTime;index:idx_transactions_wallet_created,sort:desc"`
}

func (Transaction) TableName() string { return "transactions" }
