package entity

import "time"

type IdempotencyStatus string

const (
	IdemStatusProcessing IdempotencyStatus = "PROCESSING"
	IdemStatusCompleted  IdempotencyStatus = "COMPLETED"
	IdemStatusFailed     IdempotencyStatus = "FAILED"
)

type IdempotencyKey struct {
	ID             uint              `gorm:"column:id;primaryKey"`
	Key            string            `gorm:"column:idem_key;uniqueIndex:uq_idempotency_key;not null;size:100"`
	UserID         uint              `gorm:"column:user_id;not null;index"`
	Endpoint       string            `gorm:"column:endpoint;not null;size:50"`
	RequestHash    string            `gorm:"column:request_hash;not null;size:64"`
	Status         IdempotencyStatus `gorm:"column:status;type:varchar(20);not null;default:'PROCESSING'"`
	ResponseStatus int               `gorm:"column:response_status"`
	ResponseBody   string            `gorm:"column:response_body;type:text"`
	CreatedAt      time.Time         `gorm:"column:created_at"`
	UpdatedAt      time.Time         `gorm:"column:updated_at"`
}

func (IdempotencyKey) TableName() string { return "idempotency_keys" }
