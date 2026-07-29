package main

import (
	_ "github.com/Mpayy/digital-wallet-api/docs"
	authHandler "github.com/Mpayy/digital-wallet-api/internal/auth/handler"
	jwtMiddleware "github.com/Mpayy/digital-wallet-api/internal/auth/middleware"
	paymentHandler "github.com/Mpayy/digital-wallet-api/internal/payment/handler"
	loggerMiddleware "github.com/Mpayy/digital-wallet-api/internal/pkg/middleware"
	walletHandler "github.com/Mpayy/digital-wallet-api/internal/wallet/handler"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

type Router struct {
	App                *gin.Engine
	Log                *logrus.Logger
	AuthHandler        authHandler.AuthHandler
	WalletHandler      walletHandler.WalletHandler
	TransactionHandler walletHandler.TransactionHandler
	JwtMiddleware      *jwtMiddleware.JwtMiddleware
	PaymentHandler     paymentHandler.PaymentHandler
	WebhookHandler     paymentHandler.WebhookHandler
}

func NewRouter(
	app *gin.Engine,
	log *logrus.Logger,
	authHandler authHandler.AuthHandler,
	walletHandler walletHandler.WalletHandler,
	transactionHandler walletHandler.TransactionHandler,
	jwtMiddleware *jwtMiddleware.JwtMiddleware,
	paymentHandler paymentHandler.PaymentHandler,
	webhookHandler paymentHandler.WebhookHandler,
) *Router {
	return &Router{
		App:                app,
		Log:                log,
		AuthHandler:        authHandler,
		WalletHandler:      walletHandler,
		TransactionHandler: transactionHandler,
		JwtMiddleware:      jwtMiddleware,
		PaymentHandler:     paymentHandler,
		WebhookHandler:     webhookHandler,
	}
}

func (r *Router) Setup() {
	r.App.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	v1 := r.App.Group("/api/v1")
	v1.Use(loggerMiddleware.LoggerMiddleware(r.Log))
	v1.POST("/webhooks/midtrans", r.WebhookHandler.MidtransNotification)
	{
		auth := v1.Group("/auth")
		auth.POST("/register", r.AuthHandler.Register)
		auth.POST("/login", r.AuthHandler.Login)
		auth.POST("/logout", r.JwtMiddleware.AuthMiddleware(), r.AuthHandler.Logout)

		wallets := v1.Group("/wallets", r.JwtMiddleware.AuthMiddleware())
		wallets.GET("/me", r.WalletHandler.GetMyWallet)
		wallets.POST("/topup", r.WalletHandler.TopUp)
		wallets.POST("/transfer", r.WalletHandler.Transfer)
		wallets.POST("/topup/checkout", r.PaymentHandler.CreateTopUpCheckout)

		transactions := v1.Group("/transactions", r.JwtMiddleware.AuthMiddleware())
		transactions.GET("", r.TransactionHandler.ListTransactions)
		transactions.GET("/:id", r.TransactionHandler.GetTransactionDetail)
	}
}
