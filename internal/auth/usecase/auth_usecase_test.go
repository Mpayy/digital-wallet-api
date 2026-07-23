package usecase_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/Mpayy/digital-wallet-api/internal/auth/dto"
	"github.com/Mpayy/digital-wallet-api/internal/auth/entity"
	"github.com/Mpayy/digital-wallet-api/internal/auth/mocks"
	"github.com/Mpayy/digital-wallet-api/internal/auth/usecase"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/jwt"
	jwtMocks "github.com/Mpayy/digital-wallet-api/internal/pkg/mocks"
	walletEntity "github.com/Mpayy/digital-wallet-api/internal/wallet/entity"
	walletMocks "github.com/Mpayy/digital-wallet-api/internal/wallet/mocks"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func newTestLoggerAuth() *logrus.Logger {
	log := logrus.New()
	log.SetOutput(io.Discard)
	return log
}

func setupAuthUsecase(t *testing.T) (usecase.AuthUsecase, *mocks.MockAuthRepository, *mocks.MockAuthRedisRepository, *walletMocks.MockWalletUsecase, *jwtMocks.MockJwtToken) {
	authRepo := mocks.NewMockAuthRepository(t)
	authRedisRepo := mocks.NewMockAuthRedisRepository(t)
	walletUsecase := walletMocks.NewMockWalletUsecase(t)
	jwtToken := jwtMocks.NewMockJwtToken(t)
	log := newTestLoggerAuth()

	uc := usecase.NewAuthUsecase(authRepo, authRedisRepo, walletUsecase, jwtToken, log)
	t.Cleanup(func() {
		authRepo.AssertExpectations(t)
		authRedisRepo.AssertExpectations(t)
		walletUsecase.AssertExpectations(t)
		jwtToken.AssertExpectations(t)
	})

	return uc, authRepo, authRedisRepo, walletUsecase, jwtToken
}

func TestAuthUsecase_Register(t *testing.T) {
	ctx := context.Background()
	dbErr := errors.New("unexpected error")
	req := dto.RegisterRequest{
		Name:     "Rifai",
		Email:    "rifai@example.com",
		Password: "password123",
	}

	t.Run("failed_duplicate_email", func(t *testing.T) {
		uc, authRepo, _, _, _ := setupAuthUsecase(t)

		authRepo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(u *entity.User) bool {
			return u.Email == req.Email && u.Password != req.Password // password harus udah di-hash
		})).Return(apperror.ErrDuplicatedKey)

		result, err := uc.Register(ctx, req)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, apperror.ErrDuplicatedEmail)
	})

	t.Run("failed_create_user_unexpected_error", func(t *testing.T) {
		uc, authRepo, _, _, _ := setupAuthUsecase(t)

		authRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(dbErr)

		result, err := uc.Register(ctx, req)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("success_register_with_wallet_provisioned", func(t *testing.T) {
		uc, authRepo, _, walletUsecase, _ := setupAuthUsecase(t)

		authRepo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(u *entity.User) bool {
			u.ID = 1 // simulasikan GORM ngisi ID setelah insert
			return u.Email == req.Email
		})).Return(nil)

		walletUsecase.EXPECT().CreateWallet(mock.Anything, uint(1)).Return(&walletEntity.Wallet{ID: 1, UserID: 1}, nil)

		result, err := uc.Register(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, req.Name, result.Name)
		assert.Equal(t, req.Email, result.Email)
	})

	t.Run("success_register_even_if_wallet_provisioning_fails", func(t *testing.T) {
		uc, authRepo, _, walletUsecase, _ := setupAuthUsecase(t)

		authRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)
		walletUsecase.EXPECT().CreateWallet(mock.Anything, mock.Anything).Return(nil, dbErr)

		result, err := uc.Register(ctx, req)
		assert.NoError(t, err) // <- ini yang membuktikan best-effort: Register tetap sukses
		assert.NotNil(t, result)
	})
}

func TestAuthUsecase_Login(t *testing.T) {
	ctx := context.Background()
	dbErr := errors.New("unexpected error")

	plainPassword := "correct-password"
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
	require.NoError(t, err)

	req := dto.LoginRequest{Email: "rifai@example.com", Password: plainPassword}
	storedUser := &entity.User{ID: 1, Name: "Rifai", Email: req.Email, Password: string(hashedPassword)}

	t.Run("failed_user_not_found", func(t *testing.T) {
		uc, authRepo, _, _, _ := setupAuthUsecase(t)

		authRepo.EXPECT().FindByEmail(mock.Anything, req.Email).Return(nil, apperror.ErrRecordNotFound)

		result, err := uc.Login(ctx, req)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, apperror.ErrInvalidCredentials)
	})

	t.Run("failed_find_by_email_unexpected_error", func(t *testing.T) {
		uc, authRepo, _, _, _ := setupAuthUsecase(t)

		authRepo.EXPECT().FindByEmail(mock.Anything, req.Email).Return(nil, dbErr)

		result, err := uc.Login(ctx, req)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("failed_invalid_password", func(t *testing.T) {
		uc, authRepo, _, _, _ := setupAuthUsecase(t)

		authRepo.EXPECT().FindByEmail(mock.Anything, req.Email).Return(storedUser, nil)

		wrongReq := dto.LoginRequest{Email: req.Email, Password: "wrong-password"}
		result, err := uc.Login(ctx, wrongReq)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, apperror.ErrInvalidCredentials)
	})

	t.Run("failed_create_token_error", func(t *testing.T) {
		uc, authRepo, _, _, jwtToken := setupAuthUsecase(t)

		authRepo.EXPECT().FindByEmail(mock.Anything, req.Email).Return(storedUser, nil)
		jwtToken.EXPECT().Create(mock.Anything).Return("", dbErr)

		result, err := uc.Login(ctx, req)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("failed_save_session_error", func(t *testing.T) {
		uc, authRepo, authRedisRepo, _, jwtToken := setupAuthUsecase(t)

		authRepo.EXPECT().FindByEmail(mock.Anything, req.Email).Return(storedUser, nil)
		jwtToken.EXPECT().Create(mock.Anything).Return("token-abc", nil)
		authRedisRepo.EXPECT().SaveSession(mock.Anything, entity.AuthPrefix+"token-abc", mock.Anything, jwt.TokenDuration).Return(dbErr)

		result, err := uc.Login(ctx, req)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("success_login", func(t *testing.T) {
		uc, authRepo, authRedisRepo, _, jwtToken := setupAuthUsecase(t)

		authRepo.EXPECT().FindByEmail(mock.Anything, req.Email).Return(storedUser, nil)
		jwtToken.EXPECT().Create(mock.MatchedBy(func(a *jwt.Auth) bool {
			return a.ID == storedUser.ID
		})).Return("token-abc", nil)
		authRedisRepo.EXPECT().SaveSession(mock.Anything, entity.AuthPrefix+"token-abc", mock.Anything, jwt.TokenDuration).Return(nil)

		result, err := uc.Login(ctx, req)
		assert.NoError(t, err)
		assert.Equal(t, "token-abc", result.Token)
	})
}

func TestAuthUsecase_Logout(t *testing.T) {
	ctx := context.Background()
	dbErr := errors.New("unexpected error")
	token := "token-abc"

	t.Run("failed_delete_session_error", func(t *testing.T) {
		uc, _, authRedisRepo, _, _ := setupAuthUsecase(t)

		authRedisRepo.EXPECT().DeleteSession(mock.Anything, entity.AuthPrefix+token).Return(dbErr)

		err := uc.Logout(ctx, token)
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("success_logout", func(t *testing.T) {
		uc, _, authRedisRepo, _, _ := setupAuthUsecase(t)

		authRedisRepo.EXPECT().DeleteSession(mock.Anything, entity.AuthPrefix+token).Return(nil)

		err := uc.Logout(ctx, token)
		assert.NoError(t, err)
	})
}

func TestAuthUsecase_GetUserByID(t *testing.T) {
	ctx := context.Background()
	dbErr := errors.New("unexpected error")
	userID := uint(1)

	t.Run("failed_user_not_found", func(t *testing.T) {
		uc, authRepo, _, _, _ := setupAuthUsecase(t)

		authRepo.EXPECT().FindByID(mock.Anything, userID).Return(nil, apperror.ErrRecordNotFound)

		result, err := uc.GetUserByID(ctx, userID)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, apperror.ErrUserNotFound)
	})

	t.Run("failed_find_by_id_unexpected_error", func(t *testing.T) {
		uc, authRepo, _, _, _ := setupAuthUsecase(t)

		authRepo.EXPECT().FindByID(mock.Anything, userID).Return(nil, dbErr)

		result, err := uc.GetUserByID(ctx, userID)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("success_get_user", func(t *testing.T) {
		uc, authRepo, _, _, _ := setupAuthUsecase(t)

		authRepo.EXPECT().FindByID(mock.Anything, userID).Return(&entity.User{ID: userID, Name: "Rifai"}, nil)

		result, err := uc.GetUserByID(ctx, userID)
		assert.NoError(t, err)
		assert.Equal(t, "Rifai", result.Name)
	})
}
