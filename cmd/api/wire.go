//go:build wireinject
// +build wireinject

package main

import (
	authhandler "github.com/Mpayy/digital-wallet-api/internal/auth/handler"
	jwtmiddleware "github.com/Mpayy/digital-wallet-api/internal/auth/middleware"
	authrepo "github.com/Mpayy/digital-wallet-api/internal/auth/repository"
	authusecase "github.com/Mpayy/digital-wallet-api/internal/auth/usecase"
	"github.com/Mpayy/digital-wallet-api/internal/config"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/jwt"
	loggermiddleware "github.com/Mpayy/digital-wallet-api/internal/pkg/middleware"
	walletrepo "github.com/Mpayy/digital-wallet-api/internal/wallet/repository"
	walletusecase "github.com/Mpayy/digital-wallet-api/internal/wallet/usecase"
	"github.com/google/wire"
)

var authSet = wire.NewSet(
	authrepo.NewAuthRepository,
	authusecase.NewAuthUsecase,
	authhandler.NewAuthHandler,
)

var walletSet = wire.NewSet(
	walletrepo.NewWalletRepository,
	walletusecase.NewWalletUsecase,
)

var transactionSet = wire.NewSet(
	walletrepo.NewTransactionRepository,
)

var idempotencySet = wire.NewSet(
	walletrepo.NewIdempotencyRepository,
	walletusecase.NewIdempotencyService,
)

var middlewareSet = wire.NewSet(
	jwtmiddleware.NewJwtMiddleware,
	loggermiddleware.LoggerMiddleware,
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
		middlewareSet,
		pkgSet,
		NewRouter,
		NewApplication,
	)
	return nil
}
