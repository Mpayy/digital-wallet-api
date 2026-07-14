package main

import "github.com/gin-gonic/gin"

type Router struct {
	App *gin.Engine
}

func NewRouter(app *gin.Engine) *Router {
	return &Router{
		App: app,
	}
}

func (r *Router) Setup() {
	v1 := r.App.Group("/api/v1")
	{
		auth := v1.Group("/auth")
		auth.POST("/register")
		auth.POST("/login")
		auth.POST("/logout")

		wallets := v1.Group("/wallets")
		wallets.GET("/me")
		wallets.POST("/topup")
		wallets.POST("/transfer")

		transactions := v1.Group("/transactions")
		transactions.GET("")
		transactions.GET("/:id")
	}
}
