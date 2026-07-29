package handler

import (
	"net/http"

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

func (h *webhookHandlerImpl) MidtransNotification(ctx *gin.Context) {
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 1*1024*1024)
	payload, err := ctx.GetRawData()
	if err != nil {
		response.Handle(ctx, apperror.ErrWebhookPayloadTooLarge)
		return
	}

	err = h.webhookUsecase.HandleWebhook(ctx.Request.Context(), payload)
	if err != nil {
		response.Handle(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}
