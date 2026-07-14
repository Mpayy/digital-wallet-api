package response

import "github.com/gin-gonic/gin"

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
