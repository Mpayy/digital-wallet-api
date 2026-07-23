package middleware_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Mpayy/digital-wallet-api/internal/auth/entity"
	authMocks "github.com/Mpayy/digital-wallet-api/internal/auth/mocks"
	"github.com/Mpayy/digital-wallet-api/internal/auth/middleware"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/jwt"
	jwtMocks "github.com/Mpayy/digital-wallet-api/internal/pkg/mocks"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestLoggerMiddleware() *logrus.Logger {
	log := logrus.New()
	log.SetOutput(io.Discard)
	return log
}

func setupJwtMiddlewareTest(t *testing.T) (*middleware.JwtMiddleware, *jwtMocks.MockJwtToken, *authMocks.MockAuthRedisRepository) {
	jwtToken := jwtMocks.NewMockJwtToken(t)
	authRedisRepo := authMocks.NewMockAuthRedisRepository(t)
	log := newTestLoggerMiddleware()

	mw := middleware.NewJwtMiddleware(jwtToken, authRedisRepo, log)

	t.Cleanup(func() {
		jwtToken.AssertExpectations(t)
		authRedisRepo.AssertExpectations(t)
	})

	return mw, jwtToken, authRedisRepo
}

func TestJwtMiddleware_AuthMiddleware(t *testing.T) {
	dummyToken := "valid-jwt-token-123"
	expectedAuth := &jwt.Auth{ID: 1}
	dbErr := errors.New("redis connection error")

	t.Run("failed_missing_authorization_header", func(t *testing.T) {
		mw, _, _ := setupJwtMiddlewareTest(t)

		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/test", nil)

		mw.AuthMiddleware()(ctx)

		assert.True(t, ctx.IsAborted())
		_, exists := ctx.Get("auth")
		assert.False(t, exists)
	})

	t.Run("failed_header_without_bearer_prefix", func(t *testing.T) {
		mw, _, _ := setupJwtMiddlewareTest(t)

		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx.Request.Header.Set("Authorization", "Basic dXNlcjpwYXNz")

		mw.AuthMiddleware()(ctx)

		assert.True(t, ctx.IsAborted())
	})

	t.Run("failed_empty_token_after_bearer", func(t *testing.T) {
		mw, _, _ := setupJwtMiddlewareTest(t)

		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx.Request.Header.Set("Authorization", "Bearer ")

		mw.AuthMiddleware()(ctx)

		assert.True(t, ctx.IsAborted())
	})

	t.Run("failed_jwt_validation_error", func(t *testing.T) {
		mw, jwtToken, _ := setupJwtMiddlewareTest(t)

		jwtToken.EXPECT().Validate(dummyToken).Return(nil, apperror.ErrInvalidToken)

		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx.Request.Header.Set("Authorization", "Bearer "+dummyToken)

		mw.AuthMiddleware()(ctx)

		assert.True(t, ctx.IsAborted())
	})

	t.Run("failed_redis_session_check_error", func(t *testing.T) {
		mw, jwtToken, authRedisRepo := setupJwtMiddlewareTest(t)

		jwtToken.EXPECT().Validate(dummyToken).Return(expectedAuth, nil)
		// Verifikasi konsistensi prefix entity.AuthPrefix + token
		authRedisRepo.EXPECT().SessionExists(mock.Anything, entity.AuthPrefix+dummyToken).Return(false, dbErr)

		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx.Request.Header.Set("Authorization", "Bearer "+dummyToken)

		mw.AuthMiddleware()(ctx)

		assert.True(t, ctx.IsAborted())
	})

	t.Run("failed_session_not_found_in_redis", func(t *testing.T) {
		mw, jwtToken, authRedisRepo := setupJwtMiddlewareTest(t)

		jwtToken.EXPECT().Validate(dummyToken).Return(expectedAuth, nil)
		authRedisRepo.EXPECT().SessionExists(mock.Anything, entity.AuthPrefix+dummyToken).Return(false, nil)

		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx.Request.Header.Set("Authorization", "Bearer "+dummyToken)

		mw.AuthMiddleware()(ctx)

		assert.True(t, ctx.IsAborted())
	})

	t.Run("success_valid_token_and_active_session", func(t *testing.T) {
		mw, jwtToken, authRedisRepo := setupJwtMiddlewareTest(t)

		jwtToken.EXPECT().Validate(dummyToken).Return(expectedAuth, nil)
		authRedisRepo.EXPECT().SessionExists(mock.Anything, entity.AuthPrefix+dummyToken).Return(true, nil)

		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx.Request.Header.Set("Authorization", "Bearer "+dummyToken)

		mw.AuthMiddleware()(ctx)

		assert.False(t, ctx.IsAborted())

		// Verify context values were set properly
		authVal, exists := ctx.Get("auth")
		assert.True(t, exists)
		assert.Equal(t, expectedAuth, authVal)

		tokenVal, exists := ctx.Get("token")
		assert.True(t, exists)
		assert.Equal(t, dummyToken, tokenVal)
	})
}

func TestGetAuthUser(t *testing.T) {
	t.Run("returns_nil_when_auth_key_does_not_exist", func(t *testing.T) {
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)

		user := middleware.GetAuthUser(ctx)
		assert.Nil(t, user)
	})

	t.Run("returns_nil_when_auth_key_is_wrong_type", func(t *testing.T) {
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Set("auth", "invalid-type-string")

		user := middleware.GetAuthUser(ctx)
		assert.Nil(t, user)
	})

	t.Run("returns_auth_struct_when_valid", func(t *testing.T) {
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)

		expectedAuth := &jwt.Auth{ID: 42}
		ctx.Set("auth", expectedAuth)

		user := middleware.GetAuthUser(ctx)
		assert.NotNil(t, user)
		assert.Equal(t, uint(42), user.ID)
	})
}
