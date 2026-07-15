package config

import (
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

type App struct {
	App    *gin.Engine
	Log    *logrus.Logger
	Config *viper.Viper
	DB     *gorm.DB
	Redis  *redis.Client
}

func NewApp(app *gin.Engine, logrus *logrus.Logger, viper *viper.Viper, gorm *gorm.DB, redis *redis.Client) *App {
	return &App{
		App:    app,
		Log:    logrus,
		Config: viper,
		DB:     gorm,
		Redis:  redis,
	}
}
