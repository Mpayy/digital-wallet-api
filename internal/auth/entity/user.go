package entity

import "time"

type User struct {
	ID        uint      `gorm:"column:id;primaryKey" json:"id"`
	Name      string    `gorm:"column:name;type:varchar(100);not null" json:"name"`
	Email     string    `gorm:"column:email;type:varchar(150);not null;uniqueIndex" json:"email"`
	Password  string    `gorm:"column:password;type:varchar(255);not null" json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (User) TableName() string {
	return "users"
}
