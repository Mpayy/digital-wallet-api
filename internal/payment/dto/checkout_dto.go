package dto

type CheckoutRequest struct {
	Amount int64 `json:"amount" validate:"required,gt=0"`
}

type CheckoutResponse struct {
	RedirectURL string `json:"redirect_url"`
}
