package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Mpayy/digital-wallet-api/internal/payment/dto"
	"github.com/Mpayy/digital-wallet-api/internal/payment/usecase"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type WebhookHandler interface {
	MidtransNotification(ctx *gin.Context)
	XenditNotification(ctx *gin.Context)
}

type webhookHandlerImpl struct {
	paymentUsecase    usecase.PaymentUsecase
	withdrawalUsecase usecase.WithdrawalUsecase
}

func NewWebhookHandler(paymentUsecase usecase.PaymentUsecase, withdrawalUsecase usecase.WithdrawalUsecase) WebhookHandler {
	return &webhookHandlerImpl{paymentUsecase: paymentUsecase, withdrawalUsecase: withdrawalUsecase}
}

// MidtransNotification godoc
// @Summary      Midtrans payment notification webhook
// @Description  Receives asynchronous payment status updates from Midtrans. NOT intended to be called manually — Midtrans invokes this using a payload signed with the Server Key. "Try it out" in this UI will always fail signature verification since it requires a real Midtrans-signed request; documented here for completeness only.
// @Tags         payment
// @Accept       json
// @Produce      json
// @Param        request body dto.MidtransWebhookPayload true "Midtrans notification payload"
// @Success      200 {object} map[string]string
// @Failure      400 {object} response.ErrorResponse{error=apperror.AppError} "WEBHOOK_PAYLOAD_TOO_LARGE"
// @Failure      401 {object} response.ErrorResponse{error=apperror.AppError} "INVALID_WEBHOOK_SIGNATURE"
// @Failure      404 {object} response.ErrorResponse{error=apperror.AppError} "RECORD_NOT_FOUND (unknown provider_ref_id)"
// @Failure      500 {object} response.ErrorResponse{error=apperror.AppError} "INTERNAL_SERVER_ERROR"
// @Router       /webhooks/midtrans [post]
func (h *webhookHandlerImpl) MidtransNotification(ctx *gin.Context) {
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 1*1024*1024)
	rawPayload, err := ctx.GetRawData()
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			response.Handle(ctx, apperror.ErrWebhookPayloadTooLarge)
			return
		}
		response.Handle(ctx, apperror.ErrBadRequest)
		return
	}

	var notif dto.MidtransWebhookPayload
	if err := json.Unmarshal(rawPayload, &notif); err != nil {
		response.Handle(ctx, apperror.ErrBadRequest)
		return
	}

	err = h.paymentUsecase.HandleWebhook(ctx.Request.Context(), rawPayload)
	if err != nil {
		response.Handle(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// XenditNotification godoc
// @Summary      Xendit withdrawal notification webhook
// @Description  Receives asynchronous withdrawal status updates from Xendit. NOT intended to be called manually — Xendit triggers this endpoint automatically, including a verification token in the `X-CALLBACK-TOKEN` request header. "Try it out" in this UI will always fail unless a valid `X-CALLBACK-TOKEN` matching your backend configuration is provided; documented here for completeness only.
// @Tags         withdrawal
// @Accept       json
// @Produce      json
// @Param        request body dto.XenditWebhookPayload true "Xendit notification payload"
// @Success      200 {object} map[string]string
// @Failure      400 {object} response.ErrorResponse{error=apperror.AppError} "WEBHOOK_PAYLOAD_TOO_LARGE"
// @Failure      401 {object} response.ErrorResponse{error=apperror.AppError} "INVALID_WEBHOOK_SIGNATURE"
// @Failure      404 {object} response.ErrorResponse{error=apperror.AppError} "RECORD_NOT_FOUND (unknown provider_ref_id)"
// @Failure      500 {object} response.ErrorResponse{error=apperror.AppError} "INTERNAL_SERVER_ERROR"
// @Router       /webhooks/xendit [post]
func (h *webhookHandlerImpl) XenditNotification(ctx *gin.Context) {
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 1*1024*1024)
	rawPayload, err := ctx.GetRawData()
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			response.Handle(ctx, apperror.ErrWebhookPayloadTooLarge)
			return
		}
		response.Handle(ctx, apperror.ErrBadRequest)
		return
	}

	var notif dto.XenditPayoutWebhookPayload
	if err := json.Unmarshal(rawPayload, &notif); err != nil {
		response.Handle(ctx, apperror.ErrBadRequest)
		return
	}

	if err := h.withdrawalUsecase.HandlePayoutWebhook(ctx.Request.Context(), rawPayload, ctx.Request.Header); err != nil {
		response.Handle(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}
