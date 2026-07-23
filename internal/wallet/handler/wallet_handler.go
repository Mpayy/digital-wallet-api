package handler

import (
	"net/http"

	"github.com/Mpayy/digital-wallet-api/internal/auth/middleware"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/response"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/dto"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/usecase"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type WalletHandler interface {
	GetMyWallet(c *gin.Context) // user_id dari JWT context -> usecase.GetWalletByUserID -> 200
	TopUp(c *gin.Context)       // bind JSON + header Idempotency-Key -> usecase.TopUp -> 201
	Transfer(c *gin.Context)    // bind JSON + header Idempotency-Key -> transferUC.Transfer -> 201
}

type walletHandlerImpl struct {
	walletUsecase   usecase.WalletUsecase
	transferUsecase usecase.TransferUsecase
	validator       *validator.Validate
}

func NewWalletHandler(walletUsecase usecase.WalletUsecase, transferUsecase usecase.TransferUsecase, validator *validator.Validate) WalletHandler {
	return &walletHandlerImpl{
		walletUsecase:   walletUsecase,
		transferUsecase: transferUsecase,
		validator:       validator,
	}
}

// GetMyWallet godoc
// @Summary      Get my wallet
// @Description  Returns the balance and metadata of the authenticated user's wallet.
// @Tags         wallet
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} response.SuccessResponse{data=dto.WalletResponse}
// @Failure      401 {object} response.ErrorResponse{error=apperror.AppError} "UNAUTHORIZED / INVALID_TOKEN / TOKEN_HAS_EXPIRED"
// @Failure      500 {object} response.ErrorResponse{error=apperror.AppError} "INTERNAL_SERVER_ERROR"
// @Router       /wallets/me [get]
func (h *walletHandlerImpl) GetMyWallet(ctx *gin.Context) {
	auth := middleware.GetAuthUser(ctx)
	if auth == nil {
		response.Handle(ctx, apperror.ErrUnauthorized)
		return
	}

	wallet, err := h.walletUsecase.GetWalletByUserID(ctx.Request.Context(), auth.ID)
	if err != nil {
		response.Handle(ctx, err)
		return
	}

	response.ResponseSuccess(ctx, http.StatusOK, wallet)
}

// TopUp godoc
// @Summary      Top up wallet balance
// @Description  Adds balance to the authenticated user's wallet. Requires a client-generated Idempotency-Key to safely retry on network failure.
// @Tags         wallet
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key header string true "Client-generated UUID v4, unique per logical operation"
// @Param        request body dto.TopUpRequest true "Top up payload"
// @Success      201 {object} response.SuccessResponse{data=dto.TopUpResponse}
// @Failure      400 {object} response.ErrorResponse{error=apperror.AppError} "BAD_REQUEST / VALIDATION_ERROR / MISSING_IDEMPOTENCY_KEY / INVALID_AMOUNT"
// @Failure      401 {object} response.ErrorResponse{error=apperror.AppError} "UNAUTHORIZED / INVALID_TOKEN / TOKEN_HAS_EXPIRED"
// @Failure      404 {object} response.ErrorResponse{error=apperror.AppError} "WALLET_NOT_FOUND"
// @Failure      409 {object} response.ErrorResponse{error=apperror.AppError} "IDEMPOTENCY_KEY_CONFLICT / REQUEST_IN_PROGRESS"
// @Failure      422 {object} response.ErrorResponse{error=apperror.AppError} "PREVIOUS_ATTEMPT_FAILED"
// @Failure      500 {object} response.ErrorResponse{error=apperror.AppError} "INTERNAL_SERVER_ERROR"
// @Router       /wallets/top-up [post]
func (h *walletHandlerImpl) TopUp(ctx *gin.Context) {
	auth := middleware.GetAuthUser(ctx)
	if auth == nil {
		response.Handle(ctx, apperror.ErrUnauthorized)
		return
	}

	var request dto.TopUpRequest
	err := ctx.ShouldBindJSON(&request)
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

	idemKey := ctx.GetHeader("Idempotency-Key")
	if idemKey == "" {
		response.Handle(ctx, apperror.ErrMissingIdempotencyKey)
		return
	}

	wallet, err := h.walletUsecase.TopUp(ctx.Request.Context(), auth.ID, request, idemKey)
	if err != nil {
		response.Handle(ctx, err)
		return
	}

	response.ResponseSuccess(ctx, http.StatusCreated, wallet)
}

// Transfer godoc
// @Summary      Transfer balance to another user
// @Description  Moves balance to another user's wallet using ordered row locking to prevent deadlock, with idempotency key support for safe retry.
// @Tags         wallet
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key header string true "Client-generated UUID v4, unique per logical operation"
// @Param        request body dto.TransferRequest true "Transfer payload"
// @Success      201 {object} response.SuccessResponse{data=dto.TransferResponse}
// @Failure      400 {object} response.ErrorResponse{error=apperror.AppError} "BAD_REQUEST / VALIDATION_ERROR / MISSING_IDEMPOTENCY_KEY / INVALID_AMOUNT"
// @Failure      401 {object} response.ErrorResponse{error=apperror.AppError} "UNAUTHORIZED / INVALID_TOKEN / TOKEN_HAS_EXPIRED"
// @Failure      404 {object} response.ErrorResponse{error=apperror.AppError} "WALLET_NOT_FOUND / RECIPIENT_NOT_FOUND"
// @Failure      409 {object} response.ErrorResponse{error=apperror.AppError} "IDEMPOTENCY_KEY_CONFLICT / REQUEST_IN_PROGRESS"
// @Failure      422 {object} response.ErrorResponse{error=apperror.AppError} "INSUFFICIENT_BALANCE / SELF_TRANSFER_NOT_ALLOWED / PREVIOUS_ATTEMPT_FAILED"
// @Failure      500 {object} response.ErrorResponse{error=apperror.AppError} "INTERNAL_SERVER_ERROR"
// @Router       /wallets/transfer [post]
func (h *walletHandlerImpl) Transfer(ctx *gin.Context) {
	auth := middleware.GetAuthUser(ctx)
	if auth == nil {
		response.Handle(ctx, apperror.ErrUnauthorized)
		return
	}

	var request dto.TransferRequest
	err := ctx.ShouldBindJSON(&request)
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

	idemKey := ctx.GetHeader("Idempotency-Key")
	if idemKey == "" {
		response.Handle(ctx, apperror.ErrMissingIdempotencyKey)
		return
	}

	result, err := h.transferUsecase.Transfer(ctx.Request.Context(), auth.ID, request, idemKey)
	if err != nil {
		response.Handle(ctx, err)
		return
	}

	response.ResponseSuccess(ctx, http.StatusCreated, result)
}
