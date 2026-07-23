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

// ListTransactions godoc
// @Summary      List transaction history
// @Description  Returns a paginated, filterable list of the authenticated user's wallet transactions.
// @Tags         transaction
// @Produce      json
// @Security     BearerAuth
// @Param        type       query string false "Filter by type" Enums(TOPUP, TRANSFER_IN, TRANSFER_OUT)
// @Param        start_date query string false "Start date (YYYY-MM-DD)"
// @Param        end_date   query string false "End date (YYYY-MM-DD), defaults to today if start_date is set"
// @Param        page       query int    false "Page number" default(1)
// @Param        limit      query int    false "Items per page (max 100)" default(10)
// @Success      200 {object} response.SuccessResponse{data=dto.TransactionListResponse}
// @Failure      400 {object} response.ErrorResponse{error=apperror.AppError} "BAD_REQUEST / VALIDATION_ERROR"
// @Failure      401 {object} response.ErrorResponse{error=apperror.AppError} "UNAUTHORIZED / INVALID_TOKEN / TOKEN_HAS_EXPIRED"
// @Failure      500 {object} response.ErrorResponse{error=apperror.AppError} "INTERNAL_SERVER_ERROR"
// @Router       /transactions [get]
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

// GetTransactionDetail godoc
// @Summary      Get transaction detail
// @Description  Returns a single transaction. Returns 404 both when the transaction does not exist and when it belongs to another user, to avoid leaking existence.
// @Tags         transaction
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Transaction ID"
// @Success      200 {object} response.SuccessResponse{data=dto.TransactionResponse}
// @Failure      400 {object} response.ErrorResponse{error=apperror.AppError} "BAD_REQUEST"
// @Failure      401 {object} response.ErrorResponse{error=apperror.AppError} "UNAUTHORIZED / INVALID_TOKEN / TOKEN_HAS_EXPIRED"
// @Failure      404 {object} response.ErrorResponse{error=apperror.AppError} "TRANSACTION_NOT_FOUND"
// @Failure      500 {object} response.ErrorResponse{error=apperror.AppError} "INTERNAL_SERVER_ERROR"
// @Router       /transactions/{id} [get]
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
