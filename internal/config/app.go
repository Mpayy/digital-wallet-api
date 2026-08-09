package config

import (
	"github.com/gin-gonic/gin"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

type AppInfra struct {
	App      *gin.Engine
	Log      *logrus.Logger
	Config   *viper.Viper
	DB       *gorm.DB
	Redis    *redis.Client
	RabbitMQ *amqp.Channel
}

func NewApp(app *gin.Engine, logrus *logrus.Logger, viper *viper.Viper, gorm *gorm.DB, redis *redis.Client, rabbitMQ *amqp.Channel) *AppInfra {
	return &AppInfra{
		App:      app,
		Log:      logrus,
		Config:   viper,
		DB:       gorm,
		Redis:    redis,
		RabbitMQ: rabbitMQ,
	}
}

type WorkerInfra struct {
	RabbitMQ *amqp.Channel
	Log      *logrus.Logger
	Config   *viper.Viper
	DB       *gorm.DB
}

func NewWorker(rabbitmq *amqp.Channel, logrus *logrus.Logger, viper *viper.Viper, gorm *gorm.DB) *WorkerInfra {
	return &WorkerInfra{
		RabbitMQ: rabbitmq,
		Log:      logrus,
		Config:   viper,
		DB:       gorm,
	}
}
