package handler

import (
	"net/http"

	"github.com/Mpayy/digital-wallet-api/internal/auth/middleware"
	"github.com/Mpayy/digital-wallet-api/internal/payment/dto"
	"github.com/Mpayy/digital-wallet-api/internal/payment/usecase"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type PaymentHandler interface {
	CreateTopUpCheckout(ctx *gin.Context)
}

type paymentHandlerImpl struct {
	paymentUsecase usecase.PaymentUsecase
	validator      *validator.Validate
}

func NewPaymentHandler(paymentUsecase usecase.PaymentUsecase, validator *validator.Validate) PaymentHandler {
	return &paymentHandlerImpl{
		paymentUsecase: paymentUsecase,
		validator:      validator,
	}
}

func (h *paymentHandlerImpl) CreateTopUpCheckout(ctx *gin.Context) {
	auth := middleware.GetAuthUser(ctx)
	if auth == nil {
		response.Handle(ctx, apperror.ErrUnauthorized)
		return
	}

	var request dto.CheckoutRequest
	err := ctx.ShouldBindJSON(&request)
	if err != nil {
		response.Handle(ctx, apperror.ErrBadRequest)
		return
	}

	err = h.validator.Struct(&request)
	if err != nil {
		validationErrors := apperror.ExtractValidationErrors(err)
		response.Handle(ctx, validationErrors)
		return
	}

	idemKey := ctx.GetHeader("Idempotency-Key")
	if idemKey == "" {
		response.Handle(ctx, apperror.ErrMissingIdempotencyKey)
		return
	}

	checkout, err := h.paymentUsecase.CreateTopUpCheckout(ctx.Request.Context(), auth.ID, request.Amount, idemKey)
	if err != nil {
		response.Handle(ctx, err)
		return
	}

	response.ResponseSuccess(ctx, http.StatusCreated, checkout)
}
