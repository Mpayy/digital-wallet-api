package config

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func NewLogrus(config *viper.Viper) *logrus.Logger {
	log := logrus.New()

	log.SetFormatter(&logrus.JSONFormatter{})

	log.SetLevel(logrus.InfoLevel)
	levelStr := config.GetString("LOG_LEVEL")
	if levelStr != "" {
		level, err := logrus.ParseLevel(levelStr)
		if err == nil {
			log.SetLevel(level)
		}
	}
	
	return log
}