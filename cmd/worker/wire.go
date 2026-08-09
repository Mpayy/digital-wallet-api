//go:build wireinject
// +build wireinject

package main

import (
	"github.com/google/wire"

	"github.com/Mpayy/digital-wallet-api/internal/config"
	"github.com/Mpayy/digital-wallet-api/internal/payment/gateway"
	paymentRepo "github.com/Mpayy/digital-wallet-api/internal/payment/repository"
	paymentUsecase "github.com/Mpayy/digital-wallet-api/internal/payment/usecase"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/queue"
	walletRepo "github.com/Mpayy/digital-wallet-api/internal/wallet/repository"
	walletUsecase "github.com/Mpayy/digital-wallet-api/internal/wallet/usecase"
)

var infraSet = wire.NewSet(
	config.NewViper,
	config.NewLogrus,
	config.NewGorm,
	config.NewRabbitMQ,
	config.NewWorker,
)

var walletSet = wire.NewSet(
	walletRepo.NewWalletRepository,
	walletRepo.NewTransactionRepository,
	walletRepo.NewIdempotencyRepository,
	walletUsecase.NewIdempotencyService,
	walletUsecase.NewWalletUsecase,
	wire.Bind(new(paymentUsecase.WalletTopUpper), new(walletUsecase.WalletUsecase)),
	wire.Bind(new(paymentUsecase.WalletWithdrawer), new(walletUsecase.WalletUsecase)),
	wire.Bind(new(paymentUsecase.IdempotencyClaimer), new(walletUsecase.IdempotencyService)),
)

var paymentSet = wire.NewSet(
	paymentRepo.NewPaymentRepository,
	gateway.NewMidtransGateway,
	gateway.NewXenditGateway,
	paymentUsecase.NewPaymentUsecase,
	paymentUsecase.NewWithdrawalUsecase,
)

var pkgSet = wire.NewSet(
	queue.NewPublisher,
	queue.NewConsumer,
)

func InitializeWorker() (*ApplicationWorker, func(), error) {
	wire.Build(
		infraSet,
		walletSet,
		paymentSet,
		pkgSet,
		NewWorker,
		NewApplicationWorker,
	)
	return nil, nil, nil
}
