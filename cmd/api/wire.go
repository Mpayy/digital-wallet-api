//go:build wireinject
// +build wireinject

package main

import (
	"github.com/Mpayy/digital-wallet-api/internal/config"
	"github.com/google/wire"
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

func InitializeAPI() *Application {
	wire.Build(
		infraSet,
		NewRouter,
		NewApplication,
	)
	return nil
}
