package dto

type RegisterRequest struct {
	Name     string `json:"name" validate:"required,min=3,max=100"`
	Email    string `json:"email" validate:"required,email,max=150"`
	Password string `json:"password" validate:"required,min=6,max=255"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email,max=150"`
	Password string `json:"password" validate:"required,min=6,max=255"`
}
