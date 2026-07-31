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

// CreateTopUpCheckout godoc
// @Summary      Create a top-up checkout session
// @Description  Creates a Midtrans Snap payment session for the authenticated user. Returns a redirect URL to complete payment; the wallet is credited asynchronously once Midtrans confirms via webhook, not immediately in this response.
// @Tags         payment
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key header string true "Client-generated UUID v4, unique per checkout attempt"
// @Param        request body dto.CheckoutRequest true "Checkout payload"
// @Success      201 {object} response.SuccessResponse{data=dto.CheckoutResponse}
// @Failure      400 {object} response.ErrorResponse{error=apperror.AppError} "BAD_REQUEST / VALIDATION_ERROR / MISSING_IDEMPOTENCY_KEY"
// @Failure      401 {object} response.ErrorResponse{error=apperror.AppError} "UNAUTHORIZED / INVALID_TOKEN / TOKEN_HAS_EXPIRED"
// @Failure      409 {object} response.ErrorResponse{error=apperror.AppError} "IDEMPOTENCY_KEY_CONFLICT / REQUEST_IN_PROGRESS"
// @Failure      422 {object} response.ErrorResponse{error=apperror.AppError} "PREVIOUS_ATTEMPT_FAILED"
// @Failure      500 {object} response.ErrorResponse{error=apperror.AppError} "INTERNAL_SERVER_ERROR"
// @Router       /wallets/topup/checkout [post]
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
