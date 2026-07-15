package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/jwt"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type JwtMiddleware struct {
	JwtToken jwt.JwtToken
	RedisCli *redis.Client
}

func NewJwtMiddleware(token jwt.JwtToken, client *redis.Client) *JwtMiddleware {
	return &JwtMiddleware{JwtToken: token, RedisCli: client}
}

func (m *JwtMiddleware) AuthMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authHeader := ctx.GetHeader("Authorization")
		token := strings.TrimPrefix(authHeader, "Bearer ")

		if token == "" || token == "Bearer" {
			response.ResponseError(ctx, http.StatusUnauthorized, apperror.ErrUnauthorized)
			return
		}

		auth, err := m.JwtToken.Validate(token)
		if err != nil {
			if errors.Is(err, apperror.ErrExpiredToken) {
				response.ResponseError(ctx, http.StatusUnauthorized, err.Error())
				return
			}
			if errors.Is(err, apperror.ErrInvalidToken) {
				response.ResponseError(ctx, http.StatusUnauthorized, err.Error())
				return
			}
			response.ResponseError(ctx, http.StatusInternalServerError, apperror.ErrInternalServer)
			return
		}

		result, err := m.RedisCli.Exists(ctx, token).Result()
		if err != nil {
			response.ResponseError(ctx, http.StatusInternalServerError, apperror.ErrInternalServer)
			return
		}

		if result == 0 {
			response.ResponseError(ctx, http.StatusUnauthorized, apperror.ErrUnauthorized)
			return
		}

		ctx.Set("auth", auth)
		ctx.Set("token", token)

		ctx.Next()
	}
}

func GetAuthUser(ctx *gin.Context) *jwt.Auth {
	authValue, exists := ctx.Get("auth")
	if !exists {
		return nil
	}

	auth, ok := authValue.(*jwt.Auth)
	if !ok {
		return nil
	}

	return auth
}
