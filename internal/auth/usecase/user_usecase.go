package usecase

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Mpayy/digital-wallet-api/internal/auth/dto"
	"github.com/Mpayy/digital-wallet-api/internal/auth/entity"
	"github.com/Mpayy/digital-wallet-api/internal/auth/repository"
	"github.com/Mpayy/digital-wallet-api/internal/config"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/jwt"
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
	userRepo repository.UserRepository
	redisCli *redis.Client
	jwtToken jwt.JwtToken
	log      *logrus.Logger
}

func NewUserUsecase(userRepo repository.UserRepository, redisCli *redis.Client, jwtToken jwt.JwtToken) AuthUsecase {
	return &authUsecaseImpl{userRepo: userRepo, redisCli: redisCli, jwtToken: jwtToken}
}

func (u *authUsecaseImpl) Register(ctx context.Context, request dto.RegisterRequest) (*dto.RegisterResponse, error) {
	u.log.WithField("email", request.Email).Debug("Attempting to register user")

	hashPassword, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
	if err != nil {
		u.log.WithFields(logrus.Fields{"email": request.Email, "error": err}).Error("Failed to hash password")
		return nil, apperror.ErrInternalServer
	}

	newUser := &entity.User{
		Name:     request.Name,
		Email:    request.Email,
		Password: string(hashPassword),
	}

	err = u.userRepo.Create(ctx, newUser)
	if err != nil {
		if errors.Is(err, apperror.ErrDuplicatedKey) {
			u.log.WithFields(logrus.Fields{"email": request.Email, "error": err}).Warn("Failed to create user")
			return nil, apperror.ErrDuplicatedEmail
		}
		u.log.WithFields(logrus.Fields{"email": request.Email, "error": err}).Error("Failed to create user")
		return nil, apperror.ErrInternalServer
	}

	u.log.WithFields(logrus.Fields{"user_id": newUser.ID, "email": newUser.Email}).Info("User registered successfully")
	return &dto.RegisterResponse{
		ID:    newUser.ID,
		Name:  newUser.Name,
		Email: newUser.Email,
	}, nil
}

func (u *authUsecaseImpl) Login(ctx context.Context, request dto.LoginRequest) (*dto.LoginResponse, error) {
	u.log.WithField("email", request.Email).Debug("Attempting to login user")

	user, err := u.userRepo.FindByEmail(ctx, request.Email)
	if err != nil {
		if errors.Is(err, apperror.ErrRecordNotFound) {
			u.log.WithFields(logrus.Fields{"email": request.Email, "error": err}).Warn("Failed to find user")
			return nil, apperror.ErrInvalidCredentials
		}
		u.log.WithFields(logrus.Fields{"email": request.Email, "error": err}).Error("Failed to find user")
		return nil, apperror.ErrInternalServer
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(request.Password))
	if err != nil {
		u.log.WithFields(logrus.Fields{"email": request.Email, "error": err}).Warn("Failed to compare password")
		return nil, apperror.ErrInvalidCredentials
	}

	auth := &jwt.Auth{
		ID: user.ID,
	}

	token, err := u.jwtToken.Create(auth)
	if err != nil {
		u.log.WithFields(logrus.Fields{"email": request.Email, "error": err}).Error("Failed to create token")
		return nil, apperror.ErrInternalServer
	}

	authData, err := json.Marshal(auth)
	if err != nil {
		u.log.WithFields(logrus.Fields{"email": request.Email, "error": err}).Error("Failed to marshal auth data")
		return nil, apperror.ErrInternalServer
	}

	err = u.redisCli.Set(ctx, config.AuthPrefix+token, authData, jwt.TokenDuration).Err()
	if err != nil {
		u.log.WithFields(logrus.Fields{"email": request.Email, "error": err}).Error("Failed to set auth data in redis")
		return nil, apperror.ErrInternalServer
	}

	u.log.WithFields(logrus.Fields{"user_id": user.ID, "email": user.Email}).Info("User logged in successfully")
	return &dto.LoginResponse{
		Token: token,
	}, nil
}

func (u *authUsecaseImpl) Logout(ctx context.Context, token string) error {
	u.log.WithField("token", token).Debug("Attempting to logout user")

	err := u.redisCli.Del(ctx, config.AuthPrefix+token).Err()
	if err != nil {
		u.log.WithFields(logrus.Fields{"token": token, "error": err}).Error("Failed to delete auth data from redis")
		return apperror.ErrInternalServer
	}

	u.log.WithFields(logrus.Fields{"token": token}).Info("User logged out successfully")
	return nil
}

func (u *authUsecaseImpl) GetUserByID(ctx context.Context, id uint) (*dto.UserInfo, error) {
	u.log.WithField("user_id", id).Debug("Attempting to get user by ID")

	user, err := u.userRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, apperror.ErrRecordNotFound) {
			u.log.WithFields(logrus.Fields{"user_id": id, "error": err}).Warn("Failed to find user")
			return nil, apperror.ErrUserNotFound
		}
		u.log.WithFields(logrus.Fields{"user_id": id, "error": err}).Error("Failed to find user")
		return nil, apperror.ErrInternalServer
	}

	u.log.WithFields(logrus.Fields{"user_id": user.ID, "email": user.Email}).Info("User found successfully")
	return &dto.UserInfo{
		Name: user.Name,
	}, nil
}
