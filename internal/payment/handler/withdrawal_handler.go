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

type WithdrawalHandler interface {
	CreateWithdrawal(ctx *gin.Context)
}

type withdrawalHandlerImpl struct {
	withdrawalUsecase usecase.WithdrawalUsecase
	validator         *validator.Validate
}

func NewWithdrawalHandler(withdrawalUsecase usecase.WithdrawalUsecase, validator *validator.Validate) WithdrawalHandler {
	return &withdrawalHandlerImpl{withdrawalUsecase: withdrawalUsecase, validator: validator}
}

// CreateWithdrawal godoc
// @Summary      Create a withdrawal request
// @Description  Debits the authenticated user's wallet immediately and initiates a Xendit payout to the given bank/e-wallet destination. Final settlement (or automatic reversal if the payout fails) is confirmed asynchronously via webhook, processed by a background worker.
// @Tags         withdrawal
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key header string true "Client-generated UUID v4, unique per withdrawal attempt"
// @Param        request body dto.WithdrawalRequest true "Withdrawal payload"
// @Success      201 {object} response.SuccessResponse{data=dto.WithdrawalResponse}
// @Failure      400 {object} response.ErrorResponse{error=apperror.AppError} "BAD_REQUEST / VALIDATION_ERROR / MISSING_IDEMPOTENCY_KEY / INVALID_AMOUNT"
// @Failure      401 {object} response.ErrorResponse{error=apperror.AppError} "UNAUTHORIZED / INVALID_TOKEN / TOKEN_HAS_EXPIRED"
// @Failure      404 {object} response.ErrorResponse{error=apperror.AppError} "WALLET_NOT_FOUND"
// @Failure      409 {object} response.ErrorResponse{error=apperror.AppError} "IDEMPOTENCY_KEY_CONFLICT / REQUEST_IN_PROGRESS"
// @Failure      422 {object} response.ErrorResponse{error=apperror.AppError} "PREVIOUS_ATTEMPT_FAILED / INSUFFICIENT_BALANCE"
// @Failure      500 {object} response.ErrorResponse{error=apperror.AppError} "INTERNAL_SERVER_ERROR"
// @Router       /wallets/withdraw [post]
func (h *withdrawalHandlerImpl) CreateWithdrawal(ctx *gin.Context) {
	auth := middleware.GetAuthUser(ctx)
	if auth == nil {
		response.Handle(ctx, apperror.ErrUnauthorized)
		return
	}

	var req dto.WithdrawalRequest
	err := ctx.ShouldBindJSON(&req)
	if err != nil {
		response.Handle(ctx, apperror.ErrBadRequest)
		return
	}

	err = h.validator.Struct(&req)
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

	withdrawal, err := h.withdrawalUsecase.CreateWithdrawal(ctx.Request.Context(), auth.ID, req, idemKey)
	if err != nil {
		response.Handle(ctx, err)
		return
	}

	response.ResponseSuccess(ctx, http.StatusCreated, withdrawal)
}
