package config

import (
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func NewGin(config *viper.Viper) *gin.Engine {
	if config.GetString("APP_ENV") == "production" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	engine := gin.New()
	engine.Use(gin.Recovery())

	return engine
}
