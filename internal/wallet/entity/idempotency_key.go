package entity

import "time"

type IdempotencyStatus string

const (
	IdemStatusProcessing IdempotencyStatus = "PROCESSING"
	IdemStatusCompleted  IdempotencyStatus = "COMPLETED"
	IdemStatusFailed     IdempotencyStatus = "FAILED"
)

type IdempotencyKey struct {
	ID             uint              `gorm:"column:id;primaryKey;autoIncrement"`
	Key            string            `gorm:"column:idem_key;type:varchar(100);unique;not null"`
	UserID         uint              `gorm:"column:user_id;not null;index"`
	Endpoint       string            `gorm:"column:endpoint;type:varchar(50);not null"`
	RequestHash    string            `gorm:"column:request_hash;type:char(64);not null"`
	Status         IdempotencyStatus `gorm:"column:status;type:varchar(20);not null;default:'PROCESSING'"`
	ResponseStatus *int              `gorm:"column:response_status"`
	ResponseBody   *string           `gorm:"column:response_body;type:text"`
	CreatedAt      time.Time         `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time         `gorm:"column:updated_at;autoUpdateTime"`
}

func (IdempotencyKey) TableName() string { return "idempotency_keys" }
