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

// Register godoc
// @Summary      Register a new user
// @Description  Creates a new user account with a bcrypt-hashed password. Email must be unique.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body dto.RegisterRequest true "User payload"
// @Success      201 {object} response.SuccessResponse{data=dto.RegisterResponse}
// @Failure      400 {object} response.ErrorResponse{error=apperror.AppError} "BAD_REQUEST (malformed JSON) atau VALIDATION_ERROR (lihat field 'fields')"
// @Failure      409 {object} response.ErrorResponse{error=apperror.AppError} "EMAIL_ALREADY_EXISTS"
// @Failure      500 {object} response.ErrorResponse{error=apperror.AppError} "INTERNAL_SERVER_ERROR"
// @Router       /auth/register [post]
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

// Login godoc
// @Summary      Login
// @Description  Authenticates a user with email and password, returns a JWT access token and creates a session in Redis.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body dto.LoginRequest true "Login payload"
// @Success      200 {object} response.SuccessResponse{data=dto.LoginResponse}
// @Failure      400 {object} response.ErrorResponse{error=apperror.AppError} "BAD_REQUEST atau VALIDATION_ERROR"
// @Failure      401 {object} response.ErrorResponse{error=apperror.AppError} "INVALID_CREDENTIALS"
// @Failure      500 {object} response.ErrorResponse{error=apperror.AppError} "INTERNAL_SERVER_ERROR"
// @Router       /auth/login [post]
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


// Logout godoc
// @Summary      Logout
// @Description  Revokes the current session by deleting it from Redis.
// @Tags         auth
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} response.SuccessResponse
// @Failure      401 {object} response.ErrorResponse{error=apperror.AppError} "UNAUTHORIZED / INVALID_TOKEN / TOKEN_HAS_EXPIRED"
// @Failure      500 {object} response.ErrorResponse{error=apperror.AppError} "INTERNAL_SERVER_ERROR"
// @Router       /auth/logout [post]
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
