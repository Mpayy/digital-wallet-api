//go:build wireinject
// +build wireinject

package main

import (
	authHandler "github.com/Mpayy/digital-wallet-api/internal/auth/handler"
	jwtMiddleware "github.com/Mpayy/digital-wallet-api/internal/auth/middleware"
	authRepo "github.com/Mpayy/digital-wallet-api/internal/auth/repository"
	authUsecase "github.com/Mpayy/digital-wallet-api/internal/auth/usecase"
	"github.com/Mpayy/digital-wallet-api/internal/config"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/jwt"
	loggerMiddleware "github.com/Mpayy/digital-wallet-api/internal/pkg/middleware"
	walletRepo "github.com/Mpayy/digital-wallet-api/internal/wallet/repository"
	walletUsecase "github.com/Mpayy/digital-wallet-api/internal/wallet/usecase"
	walletHandler "github.com/Mpayy/digital-wallet-api/internal/wallet/handler"
	"github.com/google/wire"
)

var authSet = wire.NewSet(
	authRepo.NewAuthRepository,
	authUsecase.NewAuthUsecase,
	authHandler.NewAuthHandler,
)

var walletSet = wire.NewSet(
	walletRepo.NewWalletRepository,
	walletUsecase.NewWalletUsecase,
	walletHandler.NewWalletHandler,
)

var transactionSet = wire.NewSet(
	walletRepo.NewTransactionRepository,
	walletUsecase.NewTransactionUsecase,
	walletHandler.NewTransactionHandler,
)

var idempotencySet = wire.NewSet(
	walletRepo.NewIdempotencyRepository,
	walletUsecase.NewIdempotencyService,
)

var transferSet = wire.NewSet(
	walletRepo.NewTransferRepository,
	walletUsecase.NewTransferUsecase,
)

var middlewareSet = wire.NewSet(
	jwtMiddleware.NewJwtMiddleware,
	loggerMiddleware.LoggerMiddleware,
)

var infraSet = wire.NewSet(
	config.NewViper,
	config.NewValidator,
	config.NewRedisClient,
	config.NewLogrus,
	config.NewGorm,
	config.NewGin,
	config.NewApp,
)

var pkgSet = wire.NewSet(
	jwt.NewJwtToken,
)

func InitializeAPI() *Application {
	wire.Build(
		infraSet,
		authSet,
		walletSet,
		transactionSet,
		idempotencySet,
		transferSet,
		middlewareSet,
		pkgSet,
		NewRouter,
		NewApplication,
	)
	return nil
}
