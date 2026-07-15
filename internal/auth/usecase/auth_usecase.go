package usecase

import (
	"context"
	"encoding/json"
	"errors"

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
	u.Log.WithField("email", request.Email).Debug("Attempting to register user")

	hashPassword, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
	if err != nil {
		u.Log.WithFields(logrus.Fields{"email": request.Email, "error": err}).Error("Failed to hash password")
		return nil, apperror.ErrInternalServer
	}

	user := &authEntity.User{
		Name:     request.Name,
		Email:    request.Email,
		Password: string(hashPassword),
	}

	err = u.AuthRepo.Create(ctx, user)
	if err != nil {
		if errors.Is(err, apperror.ErrDuplicatedKey) {
			u.Log.WithFields(logrus.Fields{"email": request.Email, "error": err}).Warn("Failed to create user")
			return nil, apperror.ErrDuplicatedEmail
		}
		u.Log.WithFields(logrus.Fields{"email": request.Email, "error": err}).Error("Failed to create user")
		return nil, apperror.ErrInternalServer
	}

	wallet, err := u.WalletUsecase.CreateWallet(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	u.Log.WithFields(logrus.Fields{"user_id": user.ID, "email": user.Email, "wallet_id": wallet.ID}).Info("User registered successfully")
	return &dto.RegisterResponse{
		ID:    user.ID,
		Name:  user.Name,
		Email: user.Email,
	}, nil
}

func (u *authUsecaseImpl) Login(ctx context.Context, request dto.LoginRequest) (*dto.LoginResponse, error) {
	u.Log.WithField("email", request.Email).Debug("Attempting to login user")

	user, err := u.AuthRepo.FindByEmail(ctx, request.Email)
	if err != nil {
		if errors.Is(err, apperror.ErrRecordNotFound) {
			u.Log.WithFields(logrus.Fields{"email": request.Email, "error": err}).Warn("Failed to find user")
			return nil, apperror.ErrInvalidCredentials
		}
		u.Log.WithFields(logrus.Fields{"email": request.Email, "error": err}).Error("Failed to find user")
		return nil, apperror.ErrInternalServer
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(request.Password))
	if err != nil {
		u.Log.WithFields(logrus.Fields{"email": request.Email, "error": err}).Warn("Failed to compare password")
		return nil, apperror.ErrInvalidCredentials
	}

	auth := &jwt.Auth{
		ID: user.ID,
	}

	token, err := u.JwtToken.Create(auth)
	if err != nil {
		u.Log.WithFields(logrus.Fields{"email": request.Email, "error": err}).Error("Failed to create token")
		return nil, apperror.ErrInternalServer
	}

	authData, err := json.Marshal(auth)
	if err != nil {
		u.Log.WithFields(logrus.Fields{"email": request.Email, "error": err}).Error("Failed to marshal auth data")
		return nil, apperror.ErrInternalServer
	}

	err = u.RedisCli.Set(ctx, token, authData, jwt.TokenDuration).Err()
	if err != nil {
		u.Log.WithFields(logrus.Fields{"email": request.Email, "error": err}).Error("Failed to set auth data in redis")
		return nil, apperror.ErrInternalServer
	}

	u.Log.WithFields(logrus.Fields{"user_id": user.ID, "email": user.Email}).Info("User logged in successfully")
	return &dto.LoginResponse{
		Token: token,
	}, nil
}

func (u *authUsecaseImpl) Logout(ctx context.Context, token string) error {
	u.Log.WithField("token", token).Debug("Attempting to logout user")

	err := u.RedisCli.Del(ctx, token).Err()
	if err != nil {
		u.Log.WithFields(logrus.Fields{"token": token, "error": err}).Error("Failed to delete auth data from redis")
		return apperror.ErrInternalServer
	}

	u.Log.WithFields(logrus.Fields{"token": token}).Info("User logged out successfully")
	return nil
}

func (u *authUsecaseImpl) GetUserByID(ctx context.Context, id uint) (*dto.UserInfo, error) {
	u.Log.WithField("user_id", id).Debug("Attempting to get user by ID")

	user, err := u.AuthRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, apperror.ErrRecordNotFound) {
			u.Log.WithFields(logrus.Fields{"user_id": id, "error": err}).Warn("Failed to find user")
			return nil, apperror.ErrUserNotFound
		}
		u.Log.WithFields(logrus.Fields{"user_id": id, "error": err}).Error("Failed to find user")
		return nil, apperror.ErrInternalServer
	}

	u.Log.WithFields(logrus.Fields{"user_id": user.ID, "email": user.Email}).Info("User found successfully")
	return &dto.UserInfo{
		Name: user.Name,
	}, nil
}
