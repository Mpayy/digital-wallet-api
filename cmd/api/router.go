package main

import (
	authHandler "github.com/Mpayy/digital-wallet-api/internal/auth/handler"
	jwtMiddleware "github.com/Mpayy/digital-wallet-api/internal/auth/middleware"
	loggerMiddleware "github.com/Mpayy/digital-wallet-api/internal/pkg/middleware"
	walletHandler "github.com/Mpayy/digital-wallet-api/internal/wallet/handler"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type Router struct {
	App           *gin.Engine
	Log           *logrus.Logger
	AuthHandler   authHandler.AuthHandler
	WalletHandler walletHandler.WalletHandler
	TransactionHandler walletHandler.TransactionHandler
	JwtMiddleware *jwtMiddleware.JwtMiddleware
}

func NewRouter(app *gin.Engine, log *logrus.Logger, authHandler authHandler.AuthHandler, walletHandler walletHandler.WalletHandler, transactionHandler walletHandler.TransactionHandler, jwtMiddleware *jwtMiddleware.JwtMiddleware) *Router {
	return &Router{
		App:           app,
		Log:           log,
		AuthHandler:   authHandler,
		WalletHandler: walletHandler,
		TransactionHandler: transactionHandler,
		JwtMiddleware: jwtMiddleware,
	}
}

func (r *Router) Setup() {
	v1 := r.App.Group("/api/v1")
	v1.Use(loggerMiddleware.LoggerMiddleware(r.Log))
	{
		auth := v1.Group("/auth")
		auth.POST("/register", r.AuthHandler.Register)
		auth.POST("/login", r.AuthHandler.Login)
		auth.POST("/logout", r.JwtMiddleware.AuthMiddleware(), r.AuthHandler.Logout)

		wallets := v1.Group("/wallets", r.JwtMiddleware.AuthMiddleware())
		wallets.GET("/me", r.WalletHandler.GetMyWallet)
		wallets.POST("/top-up", r.WalletHandler.TopUp)
		wallets.POST("/transfer", r.WalletHandler.Transfer)

		transactions := v1.Group("/transactions", r.JwtMiddleware.AuthMiddleware())
        transactions.GET("", r.TransactionHandler.ListTransactions)
        transactions.GET("/:id", r.TransactionHandler.GetTransactionDetail)
	}
}
