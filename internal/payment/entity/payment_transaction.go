package entity

import "time"

type PaymentTransactionStatus string

const (
	PaymentTransactionStatusPending PaymentTransactionStatus = "PENDING"
	PaymentTransactionStatusSettled PaymentTransactionStatus = "SETTLED"
	PaymentTransactionStatusFailed  PaymentTransactionStatus = "FAILED"
	PaymentTransactionStatusExpired PaymentTransactionStatus = "EXPIRED"
)

const (
	MidtransTestNotifPrefix1 = "payment_notif_test_"
	MidtransTestNotifPrefix2 = "sample-"
)

type PaymentTransaction struct {
	ID                  uint                     `gorm:"column:id;primaryKey"`
	Provider            string                   `gorm:"column:provider;type:varchar(20);not null;uniqueIndex:uq_payment_provider_ref"`
	ProviderRefID       string                   `gorm:"column:provider_ref_id;type:varchar(100);not null;uniqueIndex:uq_payment_provider_ref"`
	UserID              uint                     `gorm:"column:user_id;not null;index"`
	Amount              int64                    `gorm:"column:amount;not null"`
	Status              PaymentTransactionStatus `gorm:"column:status;type:varchar(20);not null;default:'PENDING'"`
	WalletTransactionID *uint                    `gorm:"column:wallet_transaction_id;index"`
	RawNotification     *string                  `gorm:"column:raw_notification;type:text"`
	CreatedAt           time.Time                `gorm:"column:created_at"`
	UpdatedAt           time.Time                `gorm:"column:updated_at"`
}

func (PaymentTransaction) TableName() string {
	return "payment_transactions"
}
