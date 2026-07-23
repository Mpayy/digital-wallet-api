package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Mpayy/digital-wallet-api/internal/auth/dto"
	"github.com/Mpayy/digital-wallet-api/internal/auth/entity"
	authEntity "github.com/Mpayy/digital-wallet-api/internal/auth/entity"
	authRepo "github.com/Mpayy/digital-wallet-api/internal/auth/repository"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/jwt"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/usecase"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

type AuthUsecase interface {
	Register(ctx context.Context, request dto.RegisterRequest) (*dto.RegisterResponse, error)
	Login(ctx context.Context, request dto.LoginRequest) (*dto.LoginResponse, error)
	Logout(ctx context.Context, token string) error
	GetUserByID(ctx context.Context, id uint) (*dto.UserInfo, error)
}

type authUsecaseImpl struct {
	AuthRepo      authRepo.AuthRepository
	AuthRedisRepo authRepo.AuthRedisRepository
	WalletUsecase usecase.WalletUsecase
	JwtToken      jwt.JwtToken
	Log           *logrus.Logger
}

func NewAuthUsecase(authRepo authRepo.AuthRepository, authRedisRepo authRepo.AuthRedisRepository, walletUsecase usecase.WalletUsecase, jwtToken jwt.JwtToken, log *logrus.Logger) AuthUsecase {
	return &authUsecaseImpl{AuthRepo: authRepo, AuthRedisRepo: authRedisRepo, WalletUsecase: walletUsecase, JwtToken: jwtToken, Log: log}
}

func (u *authUsecaseImpl) Register(ctx context.Context, request dto.RegisterRequest) (*dto.RegisterResponse, error) {
	logger := u.Log.WithFields(logrus.Fields{"email": request.Email})
	logger.Debug("Attempting to register user")

	hashPassword, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w: %w", apperror.ErrInternalServer, err)
	}

	user := &authEntity.User{
		Name:     request.Name,
		Email:    request.Email,
		Password: string(hashPassword),
	}

	err = u.AuthRepo.Create(ctx, user)
	if err != nil {
		if errors.Is(err, apperror.ErrDuplicatedKey) {
			return nil, apperror.ErrDuplicatedEmail
		}
		return nil, fmt.Errorf("create user: %w", err)
	}

	_, err = u.WalletUsecase.CreateWallet(ctx, user.ID)
	if err != nil {
		logger.WithError(err).Error("failed to provision wallet during registration — will self-heal on first wallet access")
	}

	logger.Info("User registered successfully")
	return &dto.RegisterResponse{
		ID:    user.ID,
		Name:  user.Name,
		Email: user.Email,
	}, nil
}

func (u *authUsecaseImpl) Login(ctx context.Context, request dto.LoginRequest) (*dto.LoginResponse, error) {
	logger := u.Log.WithField("email", request.Email)
	logger.Debug("Attempting to login user")

	user, err := u.AuthRepo.FindByEmail(ctx, request.Email)
	if err != nil {
		if errors.Is(err, apperror.ErrRecordNotFound) {
			return nil, apperror.ErrInvalidCredentials
		}
		return nil, fmt.Errorf("find user by email %s: %w", request.Email, err)
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(request.Password))
	if err != nil {
		return nil, apperror.ErrInvalidCredentials
	}

	auth := &jwt.Auth{
		ID: user.ID,
	}

	token, err := u.JwtToken.Create(auth)
	if err != nil {
		return nil, err
	}

	authData, err := json.Marshal(auth)
	if err != nil {
		return nil, fmt.Errorf("marshal auth data for email %s: %w", request.Email, err)
	}

	if err := u.AuthRedisRepo.SaveSession(ctx, entity.AuthPrefix+token, authData, jwt.TokenDuration); err != nil {
		return nil, fmt.Errorf("save session: %w", err)
	}

	logger.Info("User logged in successfully")
	return &dto.LoginResponse{
		Token: token,
	}, nil
}

func (u *authUsecaseImpl) Logout(ctx context.Context, token string) error {
	logger := u.Log.WithField("token", token)
	logger.Debug("Attempting to logout user")

	if err := u.AuthRedisRepo.DeleteSession(ctx, entity.AuthPrefix+token); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	logger.Info("User logged out successfully")
	return nil
}

func (u *authUsecaseImpl) GetUserByID(ctx context.Context, id uint) (*dto.UserInfo, error) {
	logger := u.Log.WithField("user_id", id)
	logger.Debug("Attempting to get user by ID")

	user, err := u.AuthRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, apperror.ErrRecordNotFound) {
			return nil, apperror.ErrUserNotFound
		}
		return nil, fmt.Errorf("find user by id %d: %w", id, err)
	}

	logger.Info("User found successfully")
	return &dto.UserInfo{
		Name: user.Name,
	}, nil
}
