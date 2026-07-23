package middleware

import (
	"fmt"
	"strings"

	"github.com/Mpayy/digital-wallet-api/internal/auth/entity"
	"github.com/Mpayy/digital-wallet-api/internal/auth/repository"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/jwt"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type JwtMiddleware struct {
	JwtToken      jwt.JwtToken
	AuthRedisRepo repository.AuthRedisRepository
	Log           *logrus.Logger
}

func NewJwtMiddleware(token jwt.JwtToken, authRedisRepo repository.AuthRedisRepository, log *logrus.Logger) *JwtMiddleware {
	return &JwtMiddleware{JwtToken: token, AuthRedisRepo: authRedisRepo, Log: log}
}

func (m *JwtMiddleware) AuthMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authHeader := ctx.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			response.Handle(ctx, apperror.ErrUnauthorized)
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")

		if token == "" || token == "Bearer" {
			response.Handle(ctx, apperror.ErrUnauthorized)
			return
		}

		auth, err := m.JwtToken.Validate(token)
		if err != nil {
			m.Log.WithError(err).Debug("jwt validation failed")
			response.Handle(ctx, err)
			return
		}

		exists, err := m.AuthRedisRepo.SessionExists(ctx, entity.AuthPrefix+token)
		if err != nil {
			response.Handle(ctx, fmt.Errorf("check session: %w", err))
			return
		}
		if !exists {
			response.Handle(ctx, apperror.ErrUnauthorized)
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
