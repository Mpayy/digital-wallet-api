package authhandler

import (
	"errors"
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
		h.Log.WithField("error", err).Warn("Failed to bind register request")
		response.ResponseError(ctx, http.StatusBadRequest, apperror.ErrBadRequest)
		return
	}

	err = h.Validator.Struct(request)
	if err != nil {
		h.Log.WithFields(logrus.Fields{"error": err}).Warn("Failed to validate register request")
		validationErrors := apperror.ExtractValidationErrors(err)
		response.ResponseError(ctx, http.StatusBadRequest, validationErrors)
		return
	}

	result, err := h.AuthUsecase.Register(ctx.Request.Context(), request)
	if err != nil {
		switch {
		case errors.Is(err, apperror.ErrDuplicatedEmail):
			response.ResponseError(ctx, http.StatusConflict, err)
		default:
			response.ResponseError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	response.ResponseSuccess(ctx, http.StatusCreated, result)
}

func (h *authHandlerImpl) Login(ctx *gin.Context) {
	var request dto.LoginRequest

	err := ctx.ShouldBindJSON(&request)
	if err != nil {
		h.Log.WithField("error", err).Warn("Failed to bind login request")
		response.ResponseError(ctx, http.StatusBadRequest, apperror.ErrBadRequest)
		return
	}

	err = h.Validator.Struct(request)
	if err != nil {
		h.Log.WithField("error", err).Warn("Failed to validate login request")
		validationErrors := apperror.ExtractValidationErrors(err)
		response.ResponseError(ctx, http.StatusBadRequest, validationErrors)
		return
	}

	result, err := h.AuthUsecase.Login(ctx.Request.Context(), request)
	if err != nil {
		switch {
		case errors.Is(err, apperror.ErrInvalidCredentials):
			response.ResponseError(ctx, http.StatusUnauthorized, err)
		default:
			response.ResponseError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	response.ResponseSuccess(ctx, http.StatusOK, result)
}

func (h *authHandlerImpl) Logout(ctx *gin.Context) {
	token := ctx.GetHeader("token")
	if token == "" {
		h.Log.WithField("error", apperror.ErrInvalidToken).Warn("Failed to logout user")
		response.ResponseError(ctx, http.StatusUnauthorized, apperror.ErrInvalidToken)
		return
	}

	err := h.AuthUsecase.Logout(ctx.Request.Context(), token)
	if err != nil {
		response.ResponseError(ctx, http.StatusInternalServerError, err)
		return
	}

	response.ResponseSuccess(ctx, http.StatusOK, nil)
}
