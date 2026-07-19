package authhandler

import (
	"net/http"

	"github.com/Mpayy/digital-wallet-api/internal/auth/dto"
	"github.com/Mpayy/digital-wallet-api/internal/auth/usecase"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/sirupsen/logrus"
)

type AuthHandler interface {
	Register(ctx *gin.Context)
	Login(ctx *gin.Context)
	Logout(ctx *gin.Context)
}

type authHandlerImpl struct {
	AuthUsecase usecase.AuthUsecase
	Validator   *validator.Validate
	Log         *logrus.Logger
}

func NewAuthHandler(authUsecase usecase.AuthUsecase, validator *validator.Validate, log *logrus.Logger) AuthHandler {
	return &authHandlerImpl{AuthUsecase: authUsecase, Validator: validator, Log: log}
}

func (h *authHandlerImpl) Register(ctx *gin.Context) {
	var request dto.RegisterRequest

	err := ctx.ShouldBindJSON(&request)
	if err != nil {
		response.Handle(ctx, apperror.ErrBadRequest)
		return
	}

	err = h.Validator.Struct(request)
	if err != nil {
		validationErrors := apperror.ExtractValidationErrors(err)
		response.Handle(ctx, validationErrors)
		return
	}

	result, err := h.AuthUsecase.Register(ctx.Request.Context(), request)
	if err != nil {
		response.Handle(ctx, err)
		return
	}

	response.ResponseSuccess(ctx, http.StatusCreated, result)
}

func (h *authHandlerImpl) Login(ctx *gin.Context) {
	var request dto.LoginRequest

	err := ctx.ShouldBindJSON(&request)
	if err != nil {
		response.Handle(ctx, apperror.ErrBadRequest)
		return
	}

	err = h.Validator.Struct(request)
	if err != nil {
		validationErrors := apperror.ExtractValidationErrors(err)
		response.Handle(ctx, validationErrors)
		return
	}

	result, err := h.AuthUsecase.Login(ctx.Request.Context(), request)
	if err != nil {
		response.Handle(ctx, err)
		return
	}

	response.ResponseSuccess(ctx, http.StatusOK, result)
}

func (h *authHandlerImpl) Logout(ctx *gin.Context) {
	token := ctx.GetString("token")
	if token == "" {
		response.Handle(ctx, apperror.ErrInvalidToken)
		return
	}

	err := h.AuthUsecase.Logout(ctx.Request.Context(), token)
	if err != nil {
		response.Handle(ctx, err)
		return
	}

	response.ResponseSuccess(ctx, http.StatusOK, nil)
}
