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
}

type webhookHandlerImpl struct {
	webhookUsecase usecase.PaymentUsecase
}

func NewWebhookHandler(webhookUsecase usecase.PaymentUsecase) WebhookHandler {
	return &webhookHandlerImpl{webhookUsecase: webhookUsecase}
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
// @Failure      404 {object} response.ErrorResponse{error=apperror.AppError} "RECORD_NOT_FOUND (provider_ref_id tidak dikenal)"
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

	err = h.webhookUsecase.HandleWebhook(ctx.Request.Context(), rawPayload)
	if err != nil {
		response.Handle(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}
