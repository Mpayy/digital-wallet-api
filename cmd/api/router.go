package main

import (
	authhandler "github.com/Mpayy/digital-wallet-api/internal/auth/handler"
	jwtMiddleware "github.com/Mpayy/digital-wallet-api/internal/auth/middleware"
	loggerMiddleware "github.com/Mpayy/digital-wallet-api/internal/pkg/middleware"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type Router struct {
	App           *gin.Engine
	Log           *logrus.Logger
	AuthHandler   authhandler.AuthHandler
	JwtMiddleware *jwtMiddleware.JwtMiddleware
}

func NewRouter(app *gin.Engine, log *logrus.Logger, authHandler authhandler.AuthHandler, jwtMiddleware *jwtMiddleware.JwtMiddleware) *Router {
	return &Router{
		App:           app,
		Log:           log,
		AuthHandler:   authHandler,
		JwtMiddleware: jwtMiddleware,
	}
}

func (r *Router) Setup() {
	r.App.Use(loggerMiddleware.LoggerMiddleware(r.Log))

	public := r.App.Group("/api/v1")
	public.POST("/register", r.AuthHandler.Register)
	public.POST("/login", r.AuthHandler.Login)

	private := r.App.Group("/api/v1")
	private.Use(r.JwtMiddleware.AuthMiddleware())
	private.POST("/logout", r.AuthHandler.Logout)
}
