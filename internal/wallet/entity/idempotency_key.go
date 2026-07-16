package entity

import "time"

type IdempotencyStatus string

const (
	IdemStatusProcessing IdempotencyStatus = "PROCESSING"
	IdemStatusCompleted  IdempotencyStatus = "COMPLETED"
	IdemStatusFailed     IdempotencyStatus = "FAILED"
)

type IdempotencyKey struct {
	ID             uint              `gorm:"primaryKey"`
	Key            string            `gorm:"uniqueIndex:uq_idempotency_key;not null;size:100"`
	UserID         uint              `gorm:"not null;index"`
	Endpoint       string            `gorm:"not null;size:50"` // "TOPUP" | "TRANSFER"
	RequestHash    string            `gorm:"not null;size:64"` // sha256 hex dari request body ternormalisasi
	Status         IdempotencyStatus `gorm:"type:varchar(20);not null;default:'PROCESSING'"`
	ResponseStatus int
	ResponseBody   string `gorm:"type:text"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (IdempotencyKey) TableName() string { return "idempotency_keys" }
