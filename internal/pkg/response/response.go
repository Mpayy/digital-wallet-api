package response

import (
	"errors"
	"net/http"

	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/gin-gonic/gin"
)

type SuccessResponse struct {
	Success bool `json:"success" example:"true"`
	Data    any  `json:"data,omitempty"`
}

type ErrorResponse struct {
	Success bool `json:"success" example:"false"`
	Error   any  `json:"error,omitempty"`
}

func ResponseSuccess(ctx *gin.Context, code int, data any) {
	ctx.JSON(code, SuccessResponse{
		Success: true,
		Data:    data,
	})
}

func ResponseError(ctx *gin.Context, code int, err any) {
	ctx.AbortWithStatusJSON(code, ErrorResponse{
		Success: false,
		Error:   err,
	})
}

func Handle(ctx *gin.Context, err error) {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) && appErr.Status < http.StatusInternalServerError {
		ResponseError(ctx, appErr.Status, appErr)
		return
	}
	_ = ctx.Error(err)
	ResponseError(ctx, http.StatusInternalServerError, apperror.ErrInternalServer)
}
