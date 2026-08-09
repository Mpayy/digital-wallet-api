package main

import (
	"github.com/Mpayy/digital-wallet-api/internal/config"
)

type ApplicationApi struct {
	App    *config.AppInfra
	Router *Router
}

func NewApplicationApi(app *config.AppInfra, router *Router) *ApplicationApi {
	return &ApplicationApi{
		App:    app,
		Router: router,
	}
}
