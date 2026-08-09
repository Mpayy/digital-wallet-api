//go:build integration

package integration_test

import (
	"context"
	"errors"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Mpayy/digital-wallet-api/internal/payment/gateway"
	paymentRepo "github.com/Mpayy/digital-wallet-api/internal/payment/repository"
	paymentUC "github.com/Mpayy/digital-wallet-api/internal/payment/usecase"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/queue"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/entity"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/repository"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/usecase"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/sirupsen/logrus"
)

func setupIntegrationDB(t *testing.T) *gorm.DB {
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		dsn = "host=127.0.0.1 user=postgres password=postgres dbname=digital_wallet_test port=5433 sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	sqlDB.SetConnMaxIdleTime(1 * time.Minute)

	t.Cleanup(func() {
		err := db.Exec(`
			TRUNCATE TABLE 
				transactions, 
				transfers, 
				wallets, 
				idempotency_keys, 
				payment_transactions 
			RESTART IDENTITY CASCADE;
		`).Error

		require.NoError(t, err)
	})

	return db
}

func setupRabbitMQ(t *testing.T) (*amqp.Connection, string) {
	url := os.Getenv("TEST_QUEUE_URL")
	if url == "" {
		url = "amqp://guest:guest@127.0.0.1:5673/"
	}
	var conn *amqp.Connection
	var err error
	for i := 0; i < 10; i++ {
		conn, err = amqp.Dial(url)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	require.NoError(t, err, "failed to connect to rabbitmq")

	queueName := "test-queue-" + uuid.NewString()

	ch, err := conn.Channel()
	require.NoError(t, err)
	err = queue.SetupTopology(ch, []queue.QueueConfig{{Name: queueName, DeliveryLimit: 5}})
	require.NoError(t, err)
	_ = ch.Close()

	t.Cleanup(func() {
		cleanupCh, err := conn.Channel()
		if err == nil {
			_, _ = cleanupCh.QueueDelete(queueName, false, false, false)
			_, _ = cleanupCh.QueueDelete(queueName+".dlq", false, false, false)
			_ = cleanupCh.ExchangeDelete(queueName+".dlx", false, false)
			_ = cleanupCh.Close()
		}
		_ = conn.Close()
	})

	return conn, queueName
}

func seedWallet(t *testing.T, db *gorm.DB, userID uint, balance int64) *entity.Wallet {
	w := &entity.Wallet{UserID: userID, Balance: balance}
	require.NoError(t, db.Create(w).Error)
	return w
}

func setupWalletUsecase(t *testing.T, db *gorm.DB) usecase.WalletUsecase {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	walletRepo := repository.NewWalletRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)
	idemRepo := repository.NewIdempotencyRepository(db)
	idemService := usecase.NewIdempotencyService(logger, idemRepo)

	return usecase.NewWalletUsecase(walletRepo, transactionRepo, idemService, logger)
}

func setupTransferUsecase(t *testing.T, db *gorm.DB) usecase.TransferUsecase {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	walletRepo := repository.NewWalletRepository(db)
	transferRepo := repository.NewTransferRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)
	idemRepo := repository.NewIdempotencyRepository(db)
	idemService := usecase.NewIdempotencyService(logger, idemRepo)

	return usecase.NewTransferUsecase(transferRepo, walletRepo, idemService, transactionRepo, logger)
}

type stubPaymentCollector struct {
	mu        sync.Mutex
	callCount int
}

func (s *stubPaymentCollector) CreateCharge(ctx context.Context, req gateway.ChargeRequest) (*gateway.ChargeResult, error) {
	s.mu.Lock()
	s.callCount++
	s.mu.Unlock()
	return &gateway.ChargeResult{ProviderRefID: req.OrderID, RedirectURL: "https://stub.test/pay/" + req.OrderID}, nil
}

func (s *stubPaymentCollector) VerifyWebhookSignature(payload []byte) error {
	return nil
}

func (s *stubPaymentCollector) ParseWebhookPayload(payload []byte) (*gateway.WebhookEvent, error) {
	return nil, errors.New("not used in this test")
}

func setupPaymentUsecase(t *testing.T, db *gorm.DB, stubGW *stubPaymentCollector, conn *amqp.Connection) paymentUC.PaymentUsecase {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	idemRepo := repository.NewIdempotencyRepository(db)
	idemService := usecase.NewIdempotencyService(logger, idemRepo)

	wRepo := repository.NewWalletRepository(db)
	txRepo := repository.NewTransactionRepository(db)
	walletUC := usecase.NewWalletUsecase(wRepo, txRepo, idemService, logger)

	pRepo := paymentRepo.NewPaymentRepository(db)
	ch, err := conn.Channel()
	require.NoError(t, err)
	pub := queue.NewPublisher(ch, logger)
	t.Cleanup(func() { _ = ch.Close() })

	return paymentUC.NewPaymentUsecase(pRepo, stubGW, walletUC, idemService, logger, pub)
}

func setupPublish(t *testing.T, conn *amqp.Connection) queue.Publisher {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	ch, err := conn.Channel()
	require.NoError(t, err)
	t.Cleanup(func() { _ = ch.Close() })
	return queue.NewPublisher(ch, logger)
}

func setupConsumer(t *testing.T, conn *amqp.Connection) queue.Consumer {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	logger.SetLevel(logrus.DebugLevel)
	ch, err := conn.Channel()
	require.NoError(t, err)
	t.Cleanup(func() { _ = ch.Close() })
	return queue.NewConsumer(ch, logger)
}
