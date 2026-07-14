package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Mpayy/digital-wallet-api/internal/config"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/jwt"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type JwtMiddleware struct {
	jwtToken jwt.JwtToken
	redisCli *redis.Client
}

func NewJwtMiddleware(token jwt.JwtToken, client *redis.Client) *JwtMiddleware {
	return &JwtMiddleware{jwtToken: token, redisCli: client}
}

func (m *JwtMiddleware) AuthMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authHeader := ctx.GetHeader("Authorization")
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		if tokenString == "" || tokenString == "Bearer" {
			response.ResponseError(ctx, http.StatusUnauthorized, apperror.ErrUnauthorized)
			return
		}

		auth, err := m.jwtToken.Validate(tokenString)
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

		result, err := m.redisCli.Exists(ctx, config.AuthPrefix+tokenString).Result()
		if err != nil {
			response.ResponseError(ctx, http.StatusInternalServerError, apperror.ErrInternalServer)
			return
		}

		if result == 0 {
			response.ResponseError(ctx, http.StatusUnauthorized, apperror.ErrUnauthorized)
			return
		}

		ctx.Set("auth", auth)
		ctx.Set("token", tokenString)

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
