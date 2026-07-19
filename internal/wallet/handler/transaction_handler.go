package handler

import (
	"net/http"
	"strconv"

	"github.com/Mpayy/digital-wallet-api/internal/auth/middleware"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/response"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/dto"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/usecase"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type TransactionHandler interface {
	ListTransactions(c *gin.Context)     // query params -> usecase.GetTransactionHistory -> 200
	GetTransactionDetail(c *gin.Context) // path param :id -> usecase.GetTransactionDetail -> 200
}

type transactionHandlerImpl struct {
	transactionUsecase usecase.TransactionUsecase
	validator          *validator.Validate
}

func NewTransactionHandler(transactionUsecase usecase.TransactionUsecase, validator *validator.Validate) TransactionHandler {
	return &transactionHandlerImpl{
		transactionUsecase: transactionUsecase,
		validator:          validator,
	}
}

func (h *transactionHandlerImpl) ListTransactions(ctx *gin.Context) {
	auth := middleware.GetAuthUser(ctx)
	if auth == nil {
		response.Handle(ctx, apperror.ErrUnauthorized)
		return
	}

	var request dto.TransactionFilter
	err := ctx.ShouldBindQuery(&request)
	if err != nil {
		response.Handle(ctx, apperror.ErrBadRequest)
		return
	}

	err = h.validator.Struct(request)
	if err != nil {
		validationErrors := apperror.ExtractValidationErrors(err)
		response.Handle(ctx, validationErrors)
		return
	}

	history, err := h.transactionUsecase.GetTransactionHistory(ctx.Request.Context(), auth.ID, request)
	if err != nil {
		response.Handle(ctx, err)
		return
	}

	response.ResponseSuccess(ctx, http.StatusOK, history)
}

func (h *transactionHandlerImpl) GetTransactionDetail(ctx *gin.Context) {
	auth := middleware.GetAuthUser(ctx)
	if auth == nil {
		response.Handle(ctx, apperror.ErrUnauthorized)
		return
	}

	transactionIDString := ctx.Param("id")

	transactionID64, err := strconv.ParseUint(transactionIDString, 10, 64)
	if err != nil {
		response.Handle(ctx, apperror.ErrBadRequest)
		return
	}

	transaction, err := h.transactionUsecase.GetTransactionDetail(ctx.Request.Context(), auth.ID, uint(transactionID64))
	if err != nil {
		response.Handle(ctx, err)
		return
	}

	response.ResponseSuccess(ctx, http.StatusOK, transaction)
}
