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
// @Description  Accepts asynchronous payment status notifications from Midtrans. Verifies the request signature, then publishes the raw payload to a queue for processing by a background worker — actual wallet crediting happens asynchronously, not within this request. NOT intended to be called manually; documented here for completeness only.
// @Tags         payment
// @Accept       json
// @Produce      json
// @Param        request body dto.MidtransWebhookPayload true "Midtrans notification payload"
// @Success      200 {object} map[string]string
// @Failure      400 {object} response.ErrorResponse{error=apperror.AppError} "BAD_REQUEST / WEBHOOK_PAYLOAD_TOO_LARGE"
// @Failure      401 {object} response.ErrorResponse{error=apperror.AppError} "INVALID_WEBHOOK_SIGNATURE"
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

	err = h.paymentUsecase.ReceiveWebhook(ctx.Request.Context(), rawPayload)
	if err != nil {
		response.Handle(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// XenditNotification godoc
// @Summary      Xendit withdrawal notification webhook
// @Description  Accepts asynchronous withdrawal/payout status notifications from Xendit, verified via the X-CALLBACK-TOKEN header. Publishes the raw payload to a queue for processing by a background worker — wallet finalization/reversal happens asynchronously, not within this request. NOT intended to be called manually; documented here for completeness only.
// @Tags         withdrawal
// @Accept       json
// @Produce      json
// @Param        request body dto.XenditPayoutWebhookPayload true "Xendit notification payload"
// @Success      200 {object} map[string]string
// @Failure      400 {object} response.ErrorResponse{error=apperror.AppError} "BAD_REQUEST / WEBHOOK_PAYLOAD_TOO_LARGE"
// @Failure      401 {object} response.ErrorResponse{error=apperror.AppError} "INVALID_WEBHOOK_SIGNATURE"
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

	if err := h.withdrawalUsecase.ReceivePayoutWebhook(ctx.Request.Context(), ctx.Request.Header, rawPayload); err != nil {
		response.Handle(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}
