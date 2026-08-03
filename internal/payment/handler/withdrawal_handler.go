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
