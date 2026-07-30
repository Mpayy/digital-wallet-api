//go:build wireinject
// +build wireinject

package main

import (
	authHandler "github.com/Mpayy/digital-wallet-api/internal/auth/handler"
	jwtMiddleware "github.com/Mpayy/digital-wallet-api/internal/auth/middleware"
	authRepo "github.com/Mpayy/digital-wallet-api/internal/auth/repository"
	authUsecase "github.com/Mpayy/digital-wallet-api/internal/auth/usecase"
	"github.com/Mpayy/digital-wallet-api/internal/config"
	"github.com/Mpayy/digital-wallet-api/internal/payment/gateway"
	paymentHandler "github.com/Mpayy/digital-wallet-api/internal/payment/handler"
	paymentRepo "github.com/Mpayy/digital-wallet-api/internal/payment/repository"
	paymentUsecase "github.com/Mpayy/digital-wallet-api/internal/payment/usecase"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/jwt"
	loggerMiddleware "github.com/Mpayy/digital-wallet-api/internal/pkg/middleware"
	walletHandler "github.com/Mpayy/digital-wallet-api/internal/wallet/handler"
	walletRepo "github.com/Mpayy/digital-wallet-api/internal/wallet/repository"
	walletUsecase "github.com/Mpayy/digital-wallet-api/internal/wallet/usecase"
	"github.com/google/wire"
)

var authSet = wire.NewSet(
	authRepo.NewAuthRepository,
	authRepo.NewAuthRedisRepository,
	authUsecase.NewAuthUsecase,
	authHandler.NewAuthHandler,
)

var walletSet = wire.NewSet(
	walletRepo.NewWalletRepository,
	walletUsecase.NewWalletUsecase,
	walletHandler.NewWalletHandler,
	wire.Bind(new(paymentUsecase.WalletTopUpper), new(walletUsecase.WalletUsecase)),
	wire.Bind(new(authUsecase.WalletProvisioner), new(walletUsecase.WalletUsecase)),
)

var transactionSet = wire.NewSet(
	walletRepo.NewTransactionRepository,
	walletUsecase.NewTransactionUsecase,
	walletHandler.NewTransactionHandler,
)

var idempotencySet = wire.NewSet(
	walletRepo.NewIdempotencyRepository,
	walletUsecase.NewIdempotencyService,
	wire.Bind(new(paymentUsecase.IdempotencyClaimer), new(walletUsecase.IdempotencyService)),
)

var transferSet = wire.NewSet(
	walletRepo.NewTransferRepository,
	walletUsecase.NewTransferUsecase,
)

var paymentSet = wire.NewSet(
	paymentRepo.NewPaymentRepository,
	paymentUsecase.NewPaymentUsecase,
	paymentHandler.NewPaymentHandler,
	paymentHandler.NewWebhookHandler,
	gateway.NewMidtransGateway,
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
		paymentSet,
		middlewareSet,
		pkgSet,
		NewRouter,
		NewApplication,
	)
	return nil
}
