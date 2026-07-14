package main

import (
	"github.com/Mpayy/digital-wallet-api/internal/config"
)

type Application struct {
	App    *config.App
	Router *Router
}

func NewApplication(app *config.App, router *Router) *Application {
	return &Application{
		App:    app,
		Router: router,
	}
}
