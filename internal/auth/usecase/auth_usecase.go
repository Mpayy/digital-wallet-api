package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Mpayy/digital-wallet-api/internal/auth/dto"
	authEntity "github.com/Mpayy/digital-wallet-api/internal/auth/entity"
	authRepo "github.com/Mpayy/digital-wallet-api/internal/auth/repository"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/jwt"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/usecase"
	"github.com/redis/go-redis/v9"
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
	WalletUsecase usecase.WalletUsecase
	RedisCli      *redis.Client
	JwtToken      jwt.JwtToken
	Log           *logrus.Logger
}

func NewAuthUsecase(authRepo authRepo.AuthRepository, walletUsecase usecase.WalletUsecase, redisCli *redis.Client, jwtToken jwt.JwtToken, log *logrus.Logger) AuthUsecase {
	return &authUsecaseImpl{AuthRepo: authRepo, WalletUsecase: walletUsecase, RedisCli: redisCli, JwtToken: jwtToken, Log: log}
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
		return nil, fmt.Errorf("create user: %w: %w", apperror.ErrInternalServer, err)
	}

	_, err = u.WalletUsecase.CreateWallet(ctx, user.ID)
	if err != nil {
		return nil, err
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

	err = u.RedisCli.Set(ctx, token, authData, jwt.TokenDuration).Err()
	if err != nil {
		return nil, fmt.Errorf("set auth data in redis for email %s: %w", request.Email, err)
	}

	logger.Info("User logged in successfully")
	return &dto.LoginResponse{
		Token: token,
	}, nil
}

func (u *authUsecaseImpl) Logout(ctx context.Context, token string) error {
	logger := u.Log.WithField("token", token)
	logger.Debug("Attempting to logout user")

	err := u.RedisCli.Del(ctx, token).Err()
	if err != nil {
		return fmt.Errorf("delete auth data from redis for token %s: %w", token, err)
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
