package entity

import "time"

type User struct {
	ID        uint   `gorm:"column:id;primaryKey"`
	Name      string `gorm:"column:name;type:varchar(100);not null"`
	Email     string `gorm:"column:email;type:varchar(150);not null;uniqueIndex"`
	Password  string `gorm:"column:password;type:varchar(255);not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (User) TableName() string {
	return "users"
}
