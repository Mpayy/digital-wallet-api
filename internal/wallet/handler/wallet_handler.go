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
